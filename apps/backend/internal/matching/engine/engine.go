package engine

import (
	"strings"
	"time"

	jobdomain "github.com/Hell077/HireRadar/apps/backend/internal/job/domain"
	"github.com/Hell077/HireRadar/apps/backend/internal/job/normalization"
	profiledomain "github.com/Hell077/HireRadar/apps/backend/internal/profile/domain"
)

type Candidate struct {
	MatchVersion int64
	Profile      profiledomain.Profile
	Skills       []profiledomain.Skill
	Positions    []string
	Languages    []string
	Preferences  profiledomain.Preferences
	Weights      *Weights
}

type Weights struct {
	Skills, Position, Seniority, Location, Salary int
}

type Component struct {
	Code             string   `json:"code"`
	Score            int      `json:"score"`
	Weight           int      `json:"weight"`
	Confidence       int      `json:"confidence"`
	Matched          []string `json:"matched,omitempty"`
	MissingRequired  []string `json:"missing_required,omitempty"`
	MissingPreferred []string `json:"missing_preferred,omitempty"`
	Unknown          []string `json:"unknown,omitempty"`
}

type Result struct {
	JobID            string      `json:"job_id"`
	CandidateVersion int64       `json:"-"`
	JobVersion       int64       `json:"-"`
	Score            int         `json:"score"`
	Confidence       int         `json:"confidence"`
	Eligible         bool        `json:"eligible"`
	Components       []Component `json:"components"`
	Exclusions       []string    `json:"exclusions"`
}

var defaultWeights = Weights{Skills: 40, Position: 25, Seniority: 15, Location: 10, Salary: 10}

func DefaultWeights() Weights { return defaultWeights }

// Evaluate applies hard eligibility rules before computing a weighted score.
// Missing evidence receives a neutral score; it never becomes a hard rejection.
func Evaluate(candidate Candidate, job jobdomain.Job, now time.Time) Result {
	result := Result{JobID: job.ID, CandidateVersion: candidate.MatchVersion, JobVersion: job.MatchVersion, Eligible: true, Components: []Component{}, Exclusions: []string{}}
	exclude := func(reason string) {
		result.Eligible = false
		result.Exclusions = append(result.Exclusions, reason)
	}
	if job.Status != jobdomain.Active {
		exclude("job_not_active")
	}
	family, familyConfidence := job.Family, job.FamilyConfidence
	if family == "" || family == "unknown" || familyConfidence < 0.75 {
		classification := normalization.ClassifyPosition(job.Title)
		family, familyConfidence = classification.Family, classification.Confidence
	}
	if isSoftwareCandidate(candidate) && familyConfidence >= 0.75 && unrelatedToSoftware(family) {
		exclude("role_mismatch")
	}
	if len(candidate.Languages) > 0 && hasLanguageMismatch(candidate.Languages, job.Title+"\n"+job.Description) {
		exclude("language_mismatch")
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
	remotePolicyMatches := contains(candidate.Preferences.RemotePolicies, string(job.RemotePolicy))
	// Generic remote listings without an explicit country restriction fit a
	// worldwide preference; explicit country limits are checked above.
	if contains(candidate.Preferences.RemotePolicies, "worldwide") && job.RemotePolicy == jobdomain.Remote && len(job.Countries) == 0 {
		remotePolicyMatches = true
	}
	if len(candidate.Preferences.RemotePolicies) > 0 && !remotePolicyMatches {
		exclude("remote_policy_mismatch")
	}
	if len(candidate.Preferences.AllowedRegions) > 0 {
		regions := jobRegions(job)
		if len(regions) > 0 && !intersects(regions, candidate.Preferences.AllowedRegions) {
			exclude("region_mismatch")
		}
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
		skillComponent(candidate.Skills, job.Skills, job.Title+"\n"+job.Description, weights.Skills),
		{Code: "position", Score: positionScore(candidate.Positions, job.Title), Weight: weights.Position, Confidence: evidenceConfidence(len(candidate.Positions) > 0, job.FamilyConfidence)},
		{Code: "seniority", Score: seniorityScore(candidate.Profile.Seniority, string(job.Seniority)), Weight: weights.Seniority, Confidence: evidenceConfidence(candidate.Profile.Seniority != "" && job.Seniority != jobdomain.SeniorityUnknown, job.SeniorityConfidence)},
		{Code: "location", Score: locationScore(candidate.Profile.Country, job), Weight: weights.Location, Confidence: evidenceConfidence(candidate.Profile.Country != "" && job.LocationConfidence > 0, job.LocationConfidence)},
		{Code: "salary", Score: salaryScore(candidate.Preferences.MinimumSalary, job.Salary), Weight: weights.Salary, Confidence: salaryEvidenceConfidence(candidate, job)},
	}
	total := 0
	confidenceTotal := 0
	for _, component := range result.Components {
		total += component.Score * component.Weight
		confidenceTotal += component.Confidence * component.Weight
	}
	result.Score = (total + 50) / 100
	result.Confidence = (confidenceTotal + 50) / 100
	if result.Score < candidate.Preferences.MinimumMatchScore {
		result.Eligible = false
		result.Exclusions = append(result.Exclusions, "below_minimum_match_score")
	}
	return result
}

func unrelatedToSoftware(family string) bool {
	switch family {
	case "product", "design", "sales", "marketing", "support", "hr", "finance", "operations", "other":
		return true
	default:
		return false
	}
}

func jobRegions(job jobdomain.Job) []string {
	regions := []string{}
	add := func(value string) {
		if value != "" && !contains(regions, value) {
			regions = append(regions, value)
		}
	}
	location := strings.ToLower(job.Location)
	for _, region := range []string{"emea", "latam", "apac", "mena", "americas", "europe", "africa", "asia_pacific", "latin_america"} {
		needle := strings.ReplaceAll(region, "_", " ")
		if strings.Contains(location, needle) {
			add(region)
			if region == "asia_pacific" {
				add("apac")
			}
			if region == "latin_america" {
				add("latam")
			}
		}
	}
	for _, country := range job.Countries {
		switch country {
		case "US", "CA":
			add("north_america")
			add("americas")
		case "MX", "BR", "AR", "CL", "CO", "PE", "CR", "UY", "DO":
			add("latin_america")
			add("latam")
			add("americas")
		case "GB", "IE", "DE", "FR", "NL", "ES", "PT", "IT", "CH", "AT", "BE", "SE", "NO", "DK", "FI", "PL", "CZ", "RO", "GR", "UA":
			add("europe")
			add("emea")
		case "AE", "SA", "IL", "TR", "EG":
			add("middle_east")
			add("mena")
			add("emea")
		case "ZA", "NG", "KE":
			add("africa")
			add("emea")
		case "IN", "PK", "BD":
			add("south_asia")
			add("apac")
		case "SG", "MY", "ID", "PH", "TH", "VN":
			add("southeast_asia")
			add("apac")
		case "JP", "KR", "CN", "TW":
			add("east_asia")
			add("apac")
		case "AU", "NZ":
			add("oceania")
			add("apac")
		case "KZ", "KG", "UZ", "TJ", "TM":
			add("central_asia")
		}
	}
	return regions
}

func (w Weights) Valid() bool {
	return w.Skills >= 0 && w.Position >= 0 && w.Seniority >= 0 && w.Location >= 0 && w.Salary >= 0 &&
		w.Skills+w.Position+w.Seniority+w.Location+w.Salary == 100
}

func skillScore(skills []profiledomain.Skill, wanted []jobdomain.JobSkill, jobText string) int {
	return skillComponent(skills, wanted, jobText, 0).Score
}

func skillComponent(skills []profiledomain.Skill, wanted []jobdomain.JobSkill, jobText string, weight int) Component {
	component := Component{Code: "skills", Score: 50, Weight: weight, Confidence: 20, Matched: []string{}, MissingRequired: []string{}, MissingPreferred: []string{}, Unknown: []string{}}
	verified := make([]jobdomain.JobSkill, 0, len(wanted))
	for _, skill := range wanted {
		if normalization.ContainsSkill(jobText, skill.Name) {
			verified = append(verified, skill)
		}
	}
	if len(verified) == 0 {
		return component
	}
	known := make(map[string]profiledomain.Skill, len(skills))
	for _, skill := range skills {
		known[canonicalSkill(skill.Name)] = skill
	}
	if len(known) == 0 {
		for _, skill := range verified {
			component.Unknown = append(component.Unknown, skill.Name)
		}
		return component
	}

	total, totalWeight, confidenceTotal := 0.0, 0.0, 0.0
	for _, required := range verified {
		itemWeight := 1.0
		if required.Required {
			itemWeight = 2
		}
		jobConfidence := effectiveConfidence(required.Confidence)
		candidate, ok := known[canonicalSkill(required.Name)]
		candidateConfidence := 0.45
		itemScore := 0.0
		if ok {
			component.Matched = append(component.Matched, required.Name)
			candidateConfidence = effectiveConfidence(candidate.Confidence)
			itemScore = skillEvidenceScore(candidate, required)
		} else if required.Required {
			component.MissingRequired = append(component.MissingRequired, required.Name)
		} else {
			component.MissingPreferred = append(component.MissingPreferred, required.Name)
		}
		evidence := jobConfidence * candidateConfidence
		if evidence < 0.5 {
			component.Unknown = append(component.Unknown, required.Name)
		}
		adjusted := 50 + (itemScore-50)*evidence
		total += adjusted * itemWeight
		totalWeight += itemWeight
		confidenceTotal += evidence * itemWeight
	}
	if totalWeight > 0 {
		component.Score = int(total/totalWeight + 0.5)
		component.Confidence = int(confidenceTotal/totalWeight*100 + 0.5)
	}
	return component
}

func skillEvidenceScore(candidate profiledomain.Skill, requirement jobdomain.JobSkill) float64 {
	if requirement.MinimumYears != nil {
		if candidate.Years == nil || *requirement.MinimumYears == 0 {
			return 60
		}
		if *candidate.Years >= *requirement.MinimumYears {
			return 100
		}
		return 50 + 50*(*candidate.Years / *requirement.MinimumYears)
	}
	if requirement.MinimumLevel != "" {
		candidateRank, candidateOK := skillLevelRank(candidate.Level)
		requiredRank, requiredOK := skillLevelRank(requirement.MinimumLevel)
		if !candidateOK || !requiredOK {
			return 60
		}
		if candidateRank >= requiredRank {
			return 100
		}
		return 50 + 50*float64(candidateRank)/float64(requiredRank)
	}
	level, levelOK := skillLevelRank(candidate.Level)
	strength := 100.0
	if levelOK {
		strength = 65 + float64(level-1)*11.7
	}
	if candidate.Years != nil {
		yearsScore := 60 + min(*candidate.Years, 5)*8
		strength = (strength + yearsScore) / 2
	}
	return min(strength, 100)
}

func skillLevelRank(level string) (int, bool) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "beginner":
		return 1, true
	case "intermediate":
		return 2, true
	case "advanced":
		return 3, true
	case "expert":
		return 4, true
	default:
		return 0, false
	}
}

func canonicalSkill(value string) string {
	value = normalize(value)
	switch value {
	case "golang", "go lang":
		return "go"
	case "postgres", "postgre sql":
		return "postgresql"
	case "reactjs", "react js", "react.js":
		return "react"
	case "nextjs", "next js", "next":
		return "next.js"
	case "js":
		return "javascript"
	case "ts":
		return "typescript"
	case "k8s":
		return "kubernetes"
	default:
		return value
	}
}

func effectiveConfidence(value float64) float64 {
	if value <= 0 {
		return 1
	}
	if value > 1 {
		return 1
	}
	return value
}

func evidenceConfidence(available bool, confidence float64) int {
	if !available {
		return 20
	}
	if confidence <= 0 {
		return 60
	}
	return int(confidence*100 + 0.5)
}

func salaryEvidenceConfidence(candidate Candidate, job jobdomain.Job) int {
	if candidate.Preferences.MinimumSalary == nil {
		return 50
	}
	if job.Salary == nil || job.SalaryConfidence <= 0 {
		return 0
	}
	return int(job.SalaryConfidence*100 + 0.5)
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
	candidateRank, candidateOK := seniorityRank(candidate)
	jobRank, jobOK := seniorityRank(job)
	if !candidateOK || !jobOK {
		return 50
	}
	if candidateRank == jobRank {
		return 100
	}
	distance := candidateRank - jobRank
	if distance < 0 {
		distance = -distance
	}
	switch distance {
	case 1:
		if candidateRank > jobRank {
			return 80
		}
		return 60
	case 2:
		return 25
	default:
		return 0
	}
}

func seniorityRank(value string) (int, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "intern", "internship":
		return 0, true
	case "junior", "entry", "entry level":
		return 1, true
	case "mid", "middle", "mid level", "mid-level":
		return 2, true
	case "senior", "sr":
		return 3, true
	case "staff":
		return 4, true
	case "lead", "manager":
		return 5, true
	case "principal":
		return 6, true
	case "director":
		return 7, true
	case "executive", "vp", "chief":
		return 8, true
	default:
		return 0, false
	}
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
