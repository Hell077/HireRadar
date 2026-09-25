package engine

import (
	"strings"
	"time"

	jobdomain "github.com/Hell077/HireRadar/apps/backend/internal/job/domain"
	profiledomain "github.com/Hell077/HireRadar/apps/backend/internal/profile/domain"
)

type Candidate struct {
	Profile     profiledomain.Profile
	Skills      []profiledomain.Skill
	Positions   []string
	Preferences profiledomain.Preferences
	Weights     *Weights
}

type Weights struct {
	Skills, Position, Seniority, Location, Salary int
}

type Component struct {
	Code   string `json:"code"`
	Score  int    `json:"score"`
	Weight int    `json:"weight"`
}

type Result struct {
	JobID      string      `json:"job_id"`
	Score      int         `json:"score"`
	Eligible   bool        `json:"eligible"`
	Components []Component `json:"components"`
	Exclusions []string    `json:"exclusions"`
}

var defaultWeights = Weights{Skills: 40, Position: 25, Seniority: 15, Location: 10, Salary: 10}

func DefaultWeights() Weights { return defaultWeights }

// Evaluate applies hard eligibility rules before computing a weighted score.
// Missing evidence receives a neutral score; it never becomes a hard rejection.
func Evaluate(candidate Candidate, job jobdomain.Job, now time.Time) Result {
	result := Result{JobID: job.ID, Eligible: true, Components: []Component{}, Exclusions: []string{}}
	exclude := func(reason string) {
		result.Eligible = false
		result.Exclusions = append(result.Exclusions, reason)
	}
	if job.Status != jobdomain.Active {
		exclude("job_not_active")
	}
	if job.Eligibility == jobdomain.NotEligible && candidate.Profile.Country == "KZ" {
		exclude("country_not_eligible")
	}
	if candidate.Profile.Country != "" && len(job.Countries) > 0 && !contains(job.Countries, candidate.Profile.Country) {
		exclude("country_mismatch")
	}
	if intersects(job.Countries, candidate.Preferences.ExcludedCountries) {
		exclude("excluded_country")
	}
	if len(candidate.Preferences.RemotePolicies) > 0 && !contains(candidate.Preferences.RemotePolicies, string(job.RemotePolicy)) {
		exclude("remote_policy_mismatch")
	}
	if len(candidate.Preferences.EmploymentTypes) > 0 && !intersectsNormalized(job.EmploymentTypes, candidate.Preferences.EmploymentTypes) {
		exclude("employment_type_mismatch")
	}
	if minimum := candidate.Preferences.MinimumSalary; minimum != nil && job.Salary != nil && job.Salary.Currency == minimum.Currency && job.Salary.Period == "year" && job.Salary.Maximum < float64(minimum.Amount) {
		exclude("salary_below_minimum")
	}
	if candidate.Preferences.MaximumJobAgeDays > 0 {
		date := job.FirstSeenAt
		if job.PublishedAt != nil {
			date = *job.PublishedAt
		}
		if !date.IsZero() && now.Sub(date) > time.Duration(candidate.Preferences.MaximumJobAgeDays)*24*time.Hour {
			exclude("job_too_old")
		}
	}
	if !result.Eligible {
		return result
	}

	weights := defaultWeights
	if candidate.Weights != nil && candidate.Weights.Valid() {
		weights = *candidate.Weights
	}
	result.Components = []Component{
		{Code: "skills", Score: skillScore(candidate.Skills, job.Skills), Weight: weights.Skills},
		{Code: "position", Score: positionScore(candidate.Positions, job.Title), Weight: weights.Position},
		{Code: "seniority", Score: seniorityScore(candidate.Profile.Seniority, string(job.Seniority)), Weight: weights.Seniority},
		{Code: "location", Score: locationScore(candidate.Profile.Country, job), Weight: weights.Location},
		{Code: "salary", Score: salaryScore(candidate.Preferences.MinimumSalary, job.Salary), Weight: weights.Salary},
	}
	total := 0
	for _, component := range result.Components {
		total += component.Score * component.Weight
	}
	result.Score = (total + 50) / 100
	if result.Score < candidate.Preferences.MinimumMatchScore {
		result.Eligible = false
		result.Exclusions = append(result.Exclusions, "below_minimum_match_score")
	}
	return result
}

func (w Weights) Valid() bool {
	return w.Skills >= 0 && w.Position >= 0 && w.Seniority >= 0 && w.Location >= 0 && w.Salary >= 0 &&
		w.Skills+w.Position+w.Seniority+w.Location+w.Salary == 100
}

func skillScore(skills []profiledomain.Skill, wanted []jobdomain.JobSkill) int {
	if len(skills) == 0 || len(wanted) == 0 {
		return 50
	}
	known := make(map[string]bool, len(skills))
	for _, skill := range skills {
		known[normalize(skill.Name)] = true
	}
	matched := 0
	for _, skill := range wanted {
		if known[normalize(skill.Name)] {
			matched++
		}
	}
	return matched * 100 / len(wanted)
}

func positionScore(positions []string, title string) int {
	if len(positions) == 0 {
		return 50
	}
	titleWords := words(title)
	best := 0
	for _, position := range positions {
		positionWords := words(position)
		if len(positionWords) == 0 {
			continue
		}
		matched := 0
		for word := range positionWords {
			if titleWords[word] {
				matched++
			}
		}
		score := matched * 100 / len(positionWords)
		if score > best {
			best = score
		}
	}
	return best
}

func seniorityScore(candidate, job string) int {
	if candidate == "" || job == "unknown" || job == "" {
		return 50
	}
	if candidate == "middle" {
		candidate = "mid"
	}
	if job == "middle" {
		job = "mid"
	}
	if candidate == job {
		return 100
	}
	return 0
}

func locationScore(country string, job jobdomain.Job) int {
	if job.Eligibility == jobdomain.Eligible && (country == "" || contains(job.Countries, country)) {
		return 100
	}
	if job.Eligibility == jobdomain.EligibilityUnknown {
		return 40
	}
	return 60
}

func salaryScore(minimum *profiledomain.Money, salary *jobdomain.SalaryRange) int {
	if minimum == nil || salary == nil || salary.Period != "year" || salary.Currency != minimum.Currency {
		return 50
	}
	if salary.Maximum < float64(minimum.Amount) {
		return 0
	}
	if salary.Minimum >= float64(minimum.Amount) {
		return 100
	}
	return 70
}

func words(value string) map[string]bool {
	result := map[string]bool{}
	for _, word := range strings.FieldsFunc(strings.ToLower(value), func(r rune) bool { return r < 'a' || r > 'z' }) {
		if len(word) > 1 {
			result[word] = true
		}
	}
	return result
}

func normalize(value string) string { return strings.ToLower(strings.Join(strings.Fields(value), " ")) }

func normalizeEmployment(value string) string {
	return strings.Join(strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	}), "_")
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if strings.EqualFold(value, wanted) {
			return true
		}
	}
	return false
}

func intersects(left, right []string) bool {
	for _, value := range left {
		if contains(right, value) {
			return true
		}
	}
	return false
}

func intersectsNormalized(left, right []string) bool {
	for _, value := range left {
		for _, wanted := range right {
			if normalizeEmployment(value) == normalizeEmployment(wanted) {
				return true
			}
		}
	}
	return false
}
