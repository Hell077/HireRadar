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

func TestClassifySeniorityAndExtractSkills(t *testing.T) {
	for title, want := range map[string]jobdomain.Seniority{"Senior Go Engineer": jobdomain.Senior, "Jr Backend Developer": jobdomain.Junior, "Principal Engineer": jobdomain.Principal, "Engineering Manager": jobdomain.Manager, "Product Designer": jobdomain.SeniorityUnknown} {
		if got := ClassifySeniority(title); got != want {
			t.Errorf("ClassifySeniority(%q)=%s want %s", title, got, want)
		}
	}
	vocabulary := []SkillTerm{{ID: "go", Name: "Go", Normalized: "go"}, {ID: "golang", Name: "Golang", Normalized: "golang"}, {ID: "postgres", Name: "PostgreSQL", Normalized: "postgresql"}, {ID: "docker", Name: "Docker", Normalized: "docker"}}
	got := ExtractSkills("Senior Engineer with Go, PostgreSQL and Docker experience; Golang is also mentioned.", vocabulary)
	if len(got) != 4 {
		t.Fatalf("skills=%+v", got)
	}
	if got := ExtractSkills("We are a Google team", vocabulary); len(got) != 0 {
		t.Fatalf("matched skills in unrelated word: %+v", got)
	}
	aliases := []SkillTerm{{ID: "go-id", Name: "Go", Normalized: "go"}, {ID: "go-id", Name: "Go", Normalized: "golang"}}
	if got := ExtractSkills("Go and Golang", aliases); len(got) != 1 || got[0].ID != "go-id" {
		t.Fatalf("skill alias produced duplicate job skills: %+v", got)
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

func TestExtractSalaryRequiresLabeledCurrencyRange(t *testing.T) {
	tests := []struct {
		text     string
		min, max float64
		currency string
		period   string
	}{
		{"Salary: $120k - $160k annually", 120000, 160000, "USD", "year"},
		{"Compensation range: EUR 4,000 to 5,500 per month", 4000, 5500, "EUR", "month"},
		{"Base pay £40,000-£50,000 yearly", 40000, 50000, "GBP", "year"},
	}
	for _, test := range tests {
		got := ExtractSalary(test.text)
		if got == nil || got.Minimum != test.min || got.Maximum != test.max || got.Currency != test.currency || got.Period != test.period {
			t.Errorf("ExtractSalary(%q)=%+v", test.text, got)
		}
	}
	for _, text := range []string{"Experience: 5-7 years. Salary depends on experience.", "Salary competitive", "Compensation $500-$400 per year", "Pay range 100000-150000"} {
		if got := ExtractSalary(text); got != nil {
			t.Errorf("ExtractSalary(%q)=%+v; want nil", text, got)
		}
	}
}
