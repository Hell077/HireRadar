package normalization

import (
	"testing"

	jobdomain "github.com/Hell077/HireRadar/apps/backend/internal/job/domain"
	sourcedomain "github.com/Hell077/HireRadar/apps/backend/internal/source/domain"
)

func TestClassifyPositionNormalizesFamilySpecialityAndSeniority(t *testing.T) {
	got := ClassifyPosition("Senior Golang Backend Engineer")
	if got.Family != "software_engineering" || got.Speciality != "backend" || got.Seniority != jobdomain.Senior || got.Confidence < 0.8 {
		t.Fatalf("classification=%+v", got)
	}
	for title, family := range map[string]string{
		"Data Scientist":                "data",
		"Site Reliability Engineer":     "devops",
		"Application Security Engineer": "security",
		"Product Manager":               "product",
		"UX Designer":                   "design",
		"Account Executive":             "sales",
		"Recruiter":                     "hr",
	} {
		if got := ClassifyPosition(title); got.Family != family {
			t.Errorf("%q family=%q, want %q", title, got.Family, family)
		}
	}
	unknown := ClassifyPosition("Specialist")
	if unknown.Family != "unknown" || unknown.Confidence != 0.25 {
		t.Fatalf("ambiguous title should remain unknown: %+v", unknown)
	}
}

func TestNormalizeStoresClassificationConfidence(t *testing.T) {
	job := Normalize(sourcedomain.Source{Priority: 10}, sourcedomain.ExternalJob{Title: "Senior Backend Engineer", Location: "Worldwide remote", Description: "Salary: USD 120k - USD 160k per year"})
	if job.Family != "software_engineering" || job.Speciality != "backend" || job.FamilyConfidence <= 0 || job.SeniorityConfidence <= 0 || job.LocationConfidence <= 0 || job.SalaryConfidence <= 0 {
		t.Fatalf("normalized classification=%+v", job)
	}
}
