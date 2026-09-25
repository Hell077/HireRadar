package domain

import "testing"

func TestPreferencesNormalize(t *testing.T) {
	p, err := (Preferences{RemotePolicies: []string{"remote"}, EmploymentTypes: []string{"full_time"}, ExcludedCountries: []string{"kz"}, MinimumSalary: &Money{Amount: 10, Currency: "usd"}, MinimumMatchScore: 60, MaximumJobAgeDays: 30}).Normalize()
	if err != nil || p.ExcludedCountries[0] != "KZ" || p.MinimumSalary.Currency != "USD" {
		t.Fatalf("preferences = %+v, %v", p, err)
	}
	if _, err := (Preferences{RemotePolicies: []string{"anywhere"}, MaximumJobAgeDays: 30}).Normalize(); err == nil {
		t.Fatal("unknown remote policy accepted")
	}
	if _, err := (Preferences{AllowedRegions: []string{""}, MaximumJobAgeDays: 30}).Normalize(); err == nil {
		t.Fatal("empty region accepted")
	}
}

func TestSourcePreferencesNormalize(t *testing.T) {
	values, err := NormalizeSourcePreferences([]SourcePreference{{SourceID: " Greenhouse ", Enabled: true}})
	if err != nil || values[0].SourceID != "greenhouse" {
		t.Fatalf("source preferences = %+v, %v", values, err)
	}
	if _, err := NormalizeSourcePreferences([]SourcePreference{{SourceID: ""}}); err == nil {
		t.Fatal("empty source accepted")
	}
}
