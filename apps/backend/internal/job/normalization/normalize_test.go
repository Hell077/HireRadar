package normalization

import (
	"testing"

	jobdomain "github.com/Hell077/HireRadar/apps/backend/internal/job/domain"
)

func TestClassifyLocationKazakhstanEligibility(t *testing.T) {
	tests := []struct {
		name   string
		want   jobdomain.Eligibility
		policy jobdomain.RemotePolicy
	}{
		{"explicit Kazakhstan remote", jobdomain.Eligible, jobdomain.Remote},
		{"global remote", jobdomain.Eligible, jobdomain.RemoteWorldwide},
		{"US-only remote", jobdomain.NotEligible, jobdomain.Remote},
		{"remote without allowed countries", jobdomain.EligibilityUnknown, jobdomain.Remote},
		{"EMEA remote", jobdomain.EligibilityUnknown, jobdomain.RemoteRegion},
		{"missing location", jobdomain.EligibilityUnknown, jobdomain.RemoteUnknown},
	}
	values := []string{"Remote — Kazakhstan", "Worldwide remote", "Remote, US only", "Remote", "EMEA remote", ""}
	for i, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policy, eligibility, _ := ClassifyLocation(values[i])
			if eligibility != test.want || policy != test.policy {
				t.Fatalf("got %s / %s, want %s / %s", policy, eligibility, test.policy, test.want)
			}
		})
	}
}

func TestClassifyLocationRetainsExplicitCountryCodes(t *testing.T) {
	_, eligibility, countries := ClassifyLocation("Remote — US only")
	if eligibility != jobdomain.NotEligible || len(countries) != 1 || countries[0] != "US" {
		t.Fatalf("US location yielded %s and %v", eligibility, countries)
	}
}

func TestNormalizationPreservesDisplayDataAndCanonicalizesKeys(t *testing.T) {
	if got := CompanyKey("Acme, Inc."); got != "acme" {
		t.Fatalf("company key=%q", got)
	}
	if got := TitleKey("Sr. Go/Backend Engineer!"); got != "sr go backend engineer" {
		t.Fatalf("title key=%q", got)
	}
	if got := CleanDescription("<p>Build&nbsp;tools &amp; APIs</p>"); got != "Build tools & APIs" {
		t.Fatalf("description=%q", got)
	}
}
