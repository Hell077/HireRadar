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
