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
		ID: "job-1", Title: "Senior Backend Engineer", Seniority: jobdomain.Senior,
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

func TestEvaluateAppliesHardFiltersBeforeScoring(t *testing.T) {
	now := time.Now().UTC()
	job := jobdomain.Job{ID: "job-2", Status: jobdomain.Active, Eligibility: jobdomain.NotEligible, RemotePolicy: jobdomain.Onsite, Countries: []string{"US"}, EmploymentTypes: []string{"contract"}, FirstSeenAt: now.Add(-90 * 24 * time.Hour)}
	candidate := Candidate{Profile: profiledomain.Profile{Country: "KZ"}, Preferences: profiledomain.Preferences{RemotePolicies: []string{"remote"}, EmploymentTypes: []string{"full_time"}, ExcludedCountries: []string{"US"}, MaximumJobAgeDays: 30}}
	got := Evaluate(candidate, job, now)
	if got.Eligible || len(got.Exclusions) < 4 || len(got.Components) != 0 {
		t.Fatalf("hard filters were not applied before scoring: %+v", got)
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
