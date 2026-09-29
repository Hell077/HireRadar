package engine

import (
	"testing"
	"time"

	jobdomain "github.com/Hell077/HireRadar/apps/backend/internal/job/domain"
	profiledomain "github.com/Hell077/HireRadar/apps/backend/internal/profile/domain"
)

func TestEvaluateProducesWeightedExplanation(t *testing.T) {
	now := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	salary := &jobdomain.SalaryRange{Minimum: 120000, Maximum: 160000, Currency: "USD", Period: "year"}
	job := jobdomain.Job{
		ID: "job-1", Title: "Senior Go Backend Engineer", Description: "Technologies: Go, PostgreSQL", Seniority: jobdomain.Senior,
		Status: jobdomain.Active, Eligibility: jobdomain.Eligible, RemotePolicy: jobdomain.Remote,
		Countries: []string{"KZ"}, EmploymentTypes: []string{"full_time"}, FirstSeenAt: now.Add(-time.Hour), Salary: salary,
		Skills: []jobdomain.JobSkill{{Name: "Go"}, {Name: "PostgreSQL"}},
	}
	candidate := Candidate{
		Profile: profiledomain.Profile{Country: "KZ", Seniority: "senior"},
		Skills:  []profiledomain.Skill{{Name: "Go"}, {Name: "PostgreSQL"}}, Positions: []string{"Backend Engineer"},
		Preferences: profiledomain.Preferences{RemotePolicies: []string{"remote"}, EmploymentTypes: []string{"full_time"}, MinimumSalary: &profiledomain.Money{Amount: 100000, Currency: "USD"}, MinimumMatchScore: 70, MaximumJobAgeDays: 30},
	}
	got := Evaluate(candidate, job, now)
	if !got.Eligible || got.Score != 100 || len(got.Components) != 5 || got.Components[0].Code != "skills" || got.Components[0].Score != 100 {
		t.Fatalf("unexpected match result: %+v", got)
	}
}

func TestEmploymentTypeAliasesReachScorer(t *testing.T) {
	now := time.Now().UTC()
	candidate := Candidate{Profile: profiledomain.Profile{Country: "KZ"}, Preferences: profiledomain.Preferences{
		EmploymentTypes: []string{"full_time"}, MinimumMatchScore: 0, MaximumJobAgeDays: 30,
	}}
	for _, test := range []struct{ jobType, preference string }{
		{"Full-Time", "full_time"}, {"full_time", "full_time"},
		{"Contract", "contract"}, {"Part-Time", "part_time"}, {"Freelance", "freelance"},
	} {
		candidate.Preferences.EmploymentTypes = []string{test.preference}
		got := Evaluate(candidate, jobdomain.Job{ID: "j", Title: "Backend Engineer", Status: jobdomain.Active, Eligibility: jobdomain.Eligible, RemotePolicy: jobdomain.RemoteWorldwide, EmploymentTypes: []string{test.jobType}, FirstSeenAt: now}, now)
		if !got.Eligible {
			t.Errorf("employment %q did not match canonical preference %q (%+v)", test.jobType, test.preference, got)
		}
	}
}

func TestSkillScoreDoesNotTrustGoTagWithoutProgrammingContext(t *testing.T) {
	profileSkills := []profiledomain.Skill{{Name: "Go"}}
	jobSkills := []jobdomain.JobSkill{{Name: "Go"}}
	if got := skillScore(profileSkills, jobSkills, "Customer Success\nPlease go above and beyond for our customers."); got != 50 {
		t.Fatalf("ordinary use of go received skill credit: %d", got)
	}
	if got := skillScore(profileSkills, jobSkills, "Senior Go Engineer\nBuild APIs in Go."); got != 100 {
		t.Fatalf("technical Go mention did not receive skill credit: %d", got)
	}
}

func TestEvaluateAppliesHardFiltersBeforeScoring(t *testing.T) {
	now := time.Now().UTC()
	job := jobdomain.Job{ID: "job-2", Status: jobdomain.Active, Eligibility: jobdomain.NotEligible, RemotePolicy: jobdomain.Onsite, Countries: []string{"US"}, EmploymentTypes: []string{"contract"}, FirstSeenAt: now.Add(-90 * 24 * time.Hour)}
	candidate := Candidate{Profile: profiledomain.Profile{Country: "KZ"}, Preferences: profiledomain.Preferences{RemotePolicies: []string{"remote"}, EmploymentTypes: []string{"full_time"}, ExcludedCountries: []string{"US"}, MaximumJobAgeDays: 30}}
	got := Evaluate(candidate, job, now)
	if got.Eligible || len(got.Exclusions) < 4 || len(got.Components) != 0 {
		t.Fatalf("hard filters were not applied before scoring: %+v", got)
	}
}

func TestWorldwidePreferenceAcceptsUnrestrictedRemoteButNotCountryLimitedListings(t *testing.T) {
	now := time.Now().UTC()
	candidate := Candidate{Profile: profiledomain.Profile{Country: "KZ"}, Preferences: profiledomain.Preferences{RemotePolicies: []string{"worldwide"}, MaximumJobAgeDays: 30}}
	job := jobdomain.Job{ID: "generic-remote", Title: "Go Engineer", Status: jobdomain.Active, RemotePolicy: jobdomain.Remote, Eligibility: jobdomain.EligibilityUnknown, FirstSeenAt: now}
	if got := Evaluate(candidate, job, now); !got.Eligible {
		t.Fatalf("unrestricted generic remote job should fit worldwide preference: %+v", got)
	}
	job.ID = "us-only"
	job.Countries = []string{"US"}
	job.Location = "US Remote"
	job.Eligibility = jobdomain.NotEligible
	if got := Evaluate(candidate, job, now); got.Eligible || !contains(got.Exclusions, "country_not_eligible") {
		t.Fatalf("country-limited US job must not fit Kazakhstan profile: %+v", got)
	}
}

func TestEvaluateKeepsUnknownEvidenceNeutral(t *testing.T) {
	now := time.Now().UTC()
	job := jobdomain.Job{ID: "job-3", Status: jobdomain.Active, Eligibility: jobdomain.EligibilityUnknown, Seniority: jobdomain.SeniorityUnknown, FirstSeenAt: now}
	got := Evaluate(Candidate{Profile: profiledomain.Profile{Country: "KZ"}, Preferences: profiledomain.Preferences{MaximumJobAgeDays: 30}}, job, now)
	if !got.Eligible || got.Score != 49 {
		t.Fatalf("unknown information should remain neutral: %+v", got)
	}
}

func TestSeniorityScoreUsesDistanceAndDirection(t *testing.T) {
	for _, test := range []struct {
		candidate, job string
		want           int
	}{
		{"senior", "senior", 100},
		{"staff", "senior", 80},
		{"junior", "mid", 60},
		{"senior", "junior", 25},
		{"intern", "senior", 0},
		{"unknown", "senior", 50},
	} {
		if got := seniorityScore(test.candidate, test.job); got != test.want {
			t.Errorf("seniorityScore(%q,%q)=%d, want %d", test.candidate, test.job, got, test.want)
		}
	}
}

func TestSkillComponentSupportsAliasesRequirementsAndExplanations(t *testing.T) {
	years := 3.0
	component := skillComponent(
		[]profiledomain.Skill{{Name: "Go Lang", Years: &years, Level: "intermediate", Confidence: 0.9}},
		[]jobdomain.JobSkill{
			{Name: "Golang", Required: true, Confidence: 0.95, MinimumYears: floatPointer(5), MinimumLevel: "advanced"},
			{Name: "AWS", Confidence: 0.8},
		},
		"Skills: Golang and AWS",
		35,
	)
	if component.Code != "skills" || component.Weight != 35 || component.Score >= 100 || component.Confidence == 0 {
		t.Fatalf("unexpected skill component: %+v", component)
	}
	if len(component.Matched) != 1 || component.Matched[0] != "Golang" || len(component.MissingPreferred) != 1 || component.MissingPreferred[0] != "AWS" {
		t.Fatalf("skill explanation missing evidence: %+v", component)
	}
}

func TestSkillComponentTreatsAbsentCandidateProfileAsUnknown(t *testing.T) {
	component := skillComponent(nil, []jobdomain.JobSkill{{Name: "Go", Required: true, Confidence: 0.9}}, "Go Engineer", 35)
	if component.Score != 50 || component.Confidence != 20 || len(component.Unknown) != 1 || len(component.MissingRequired) != 0 {
		t.Fatalf("missing profile skills became a definite mismatch: %+v", component)
	}
}

func TestMatchConfidenceFallsWhenSalaryEvidenceIsUnknown(t *testing.T) {
	now := time.Now().UTC()
	minimum := &profiledomain.Money{Amount: 100000, Currency: "USD"}
	job := jobdomain.Job{ID: "unknown-salary", Title: "Backend Engineer", Status: jobdomain.Active, Eligibility: jobdomain.EligibilityUnknown, Seniority: jobdomain.Senior, FirstSeenAt: now}
	candidate := Candidate{Profile: profiledomain.Profile{Seniority: "senior"}, Preferences: profiledomain.Preferences{MinimumSalary: minimum, MaximumJobAgeDays: 30}}
	result := Evaluate(candidate, job, now)
	if result.Confidence >= 60 {
		t.Fatalf("unknown salary should reduce confidence: %+v", result)
	}
	for _, component := range result.Components {
		if component.Code == "salary" && component.Confidence != 0 {
			t.Fatalf("unknown salary component confidence=%d", component.Confidence)
		}
	}
}

func floatPointer(value float64) *float64 { return &value }

func TestEvaluateExcludesUnrelatedOccupationsForSoftwareCandidate(t *testing.T) {
	now := time.Now().UTC()
	candidate := Candidate{
		Skills:      []profiledomain.Skill{{Name: "Go"}, {Name: "JavaScript"}},
		Positions:   []string{"Senior Backend Developer"},
		Preferences: profiledomain.Preferences{MaximumJobAgeDays: 30},
	}
	for _, title := range []string{
		"Barista", "咖啡师", "バリスタ", "Implementation Project Manager - Mortgage",
		"Corporate Communications Manager", "Manager, Tech & Ecosystem Partnerships",
		"Technical Account Manager", "Head of Paid Social, Insurance",
		"3D & LiDAR Data Annotation Analyst", "Senior Visual Designer - Advertising",
	} {
		job := jobdomain.Job{ID: title, Title: title, Status: jobdomain.Active, Eligibility: jobdomain.EligibilityUnknown, FirstSeenAt: now}
		got := Evaluate(candidate, job, now)
		if got.Eligible || !contains(got.Exclusions, "role_mismatch") || len(got.Components) != 0 {
			t.Errorf("unrelated role %q was not rejected before scoring: %+v", title, got)
		}
	}
	job := jobdomain.Job{ID: "tech", Title: "Senior Go Engineer", Status: jobdomain.Active, Eligibility: jobdomain.EligibilityUnknown, FirstSeenAt: now}
	if got := Evaluate(candidate, job, now); !got.Eligible {
		t.Fatalf("technical role should remain eligible: %+v", got)
	}
}

func TestEvaluateFiltersRequiredAndPrimaryJobLanguages(t *testing.T) {
	now := time.Now().UTC()
	candidate := Candidate{Languages: []string{"en", "ru"}, Preferences: profiledomain.Preferences{MaximumJobAgeDays: 30}}
	for _, job := range []jobdomain.Job{
		{ID: "required-spanish", Title: "Backend Engineer", Description: "Fluent Spanish is required for this role.", Status: jobdomain.Active, Eligibility: jobdomain.EligibilityUnknown, FirstSeenAt: now},
		{ID: "chinese-post", Title: "软件工程师", Description: "我们正在招聘软件工程师，负责开发和维护云平台。候选人需要丰富的工程经验，并与团队合作。", Status: jobdomain.Active, Eligibility: jobdomain.EligibilityUnknown, FirstSeenAt: now},
	} {
		got := Evaluate(candidate, job, now)
		if got.Eligible || !contains(got.Exclusions, "language_mismatch") {
			t.Errorf("unsupported job language was not rejected: %+v", got)
		}
	}
	optional := jobdomain.Job{ID: "optional-spanish", Title: "Backend Engineer", Description: "Spanish is a plus. This is an English language posting about software engineering.", Status: jobdomain.Active, Eligibility: jobdomain.EligibilityUnknown, FirstSeenAt: now}
	if got := Evaluate(candidate, optional, now); !got.Eligible {
		t.Fatalf("optional language mention should not reject a job: %+v", got)
	}
}

func TestWeightsAreValidatedAndConfigurable(t *testing.T) {
	custom := Weights{Skills: 70, Position: 10, Seniority: 10, Location: 5, Salary: 5}
	if !custom.Valid() || (Weights{Skills: 99}).Valid() {
		t.Fatal("weight validation failed")
	}
	job := jobdomain.Job{ID: "weighted", Status: jobdomain.Active, Eligibility: jobdomain.Eligible, Seniority: jobdomain.Senior, FirstSeenAt: time.Now()}
	candidate := Candidate{Profile: profiledomain.Profile{Seniority: "senior"}, Weights: &custom, Preferences: profiledomain.Preferences{MaximumJobAgeDays: 30}}
	got := Evaluate(candidate, job, time.Now())
	if got.Components[0].Weight != 70 || got.Score != 58 {
		t.Fatalf("custom weights were not applied: %+v", got)
	}
}

func TestAllowedRegionHardFilterUsesCountryAndRegionAliases(t *testing.T) {
	now := time.Now().UTC()
	job := jobdomain.Job{ID: "region", Title: "Engineer", Status: jobdomain.Active, Countries: []string{"DE"}, Location: "Berlin, Germany", Eligibility: jobdomain.NotEligible, FirstSeenAt: now}
	candidate := Candidate{Profile: profiledomain.Profile{Country: "KZ"}, Preferences: profiledomain.Preferences{AllowedRegions: []string{"apac"}, MaximumJobAgeDays: 30}}
	got := Evaluate(candidate, job, now)
	if got.Eligible || len(got.Exclusions) == 0 || got.Exclusions[0] != "country_not_eligible" {
		t.Fatalf("hard eligibility should run before region scoring: %+v", got)
	}
	job.Eligibility = jobdomain.EligibilityUnknown
	candidate.Profile.Country = ""
	got = Evaluate(candidate, job, now)
	if got.Eligible || !contains(got.Exclusions, "region_mismatch") || len(got.Components) != 0 {
		t.Fatalf("explicitly disallowed region was scored: %+v", got)
	}
	candidate.Preferences.AllowedRegions = []string{"emea"}
	got = Evaluate(candidate, job, now)
	if !got.Eligible {
		t.Fatalf("EMEA alias did not accept a European country: %+v", got)
	}
}
