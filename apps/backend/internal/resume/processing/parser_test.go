package processing

import (
	"context"
	"testing"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/resume/domain"
	"github.com/giraffesyo/pdf/pdftest"
)

func TestExtractAndAnalyzePDF(t *testing.T) {
	data := pdftest.Build(1, pdftest.Catalog(2), pdftest.Pages(3), pdftest.Page(2, 4, `<< /Font << /F1 5 0 R >> >>`), pdftest.Stream("", `BT /F1 12 Tf 72 720 Td (Senior Go developer with 5 years experience) Tj ET`), pdftest.Helvetica())
	text, err := ExtractText(context.Background(), data)
	if err != nil {
		t.Fatal(err)
	}
	if text == "" {
		t.Fatal("extracted text is empty")
	}
	parsed := Analyze(domain.ID("resume-id"), text, []SkillTerm{{Name: "Go", Normalized: "go"}, {Name: "Golang", Normalized: "golang"}, {Name: "PostgreSQL", Normalized: "postgresql"}})
	if len(parsed.Skills) != 1 || parsed.Skills[0].Name != "Go" {
		t.Fatalf("skills = %+v", parsed.Skills)
	}
	if len(parsed.Positions) != 1 || parsed.Positions[0].Title != "Senior Go developer with 5 years experience" {
		t.Fatalf("positions = %+v", parsed.Positions)
	}
	if parsed.TotalExperienceMonths != 60 {
		t.Fatalf("experience months = %d", parsed.TotalExperienceMonths)
	}
}

func TestExtractRejectsScannedPDFWithoutText(t *testing.T) {
	data := pdftest.Build(1, pdftest.Catalog(2), pdftest.Pages(3), pdftest.Page(2, 4, `<< >>`), pdftest.Stream("", ""))
	if _, err := ExtractText(context.Background(), data); err == nil {
		t.Fatal("expected textless PDF to fail")
	}
}

func TestAnalyzeMatchesWholeSkillTerms(t *testing.T) {
	parsed := Analyze("id", "golang developer", []SkillTerm{{Name: "Go", Normalized: "go"}})
	if len(parsed.Skills) != 0 {
		t.Fatalf("partial skill match: %+v", parsed.Skills)
	}
}

func TestAnalyzeExtractsLanguagesFromResumeSection(t *testing.T) {
	parsed := Analyze("id", "Senior backend developer\nLanguages: Russian (native), English — fluent\nExperience: Worked with Spanish customers", nil)
	if len(parsed.Languages) != 2 || parsed.Languages[0] != "en" || parsed.Languages[1] != "ru" {
		t.Fatalf("languages = %v", parsed.Languages)
	}
}

func TestAnalyzeBuildsExperienceAndEstimatesSkillUse(t *testing.T) {
	text := "Senior Go Engineer at Acme — Jan 2020 - Present\nBuilt backend APIs with Go and PostgreSQL.\nSoftware Developer | Beta Labs | Feb 2018 – Dec 2019\nMaintained Python services."
	parsed := Analyze("id", text, []SkillTerm{{Name: "Go", Normalized: "go"}, {Name: "PostgreSQL", Normalized: "postgresql"}, {Name: "Python", Normalized: "python"}})
	if len(parsed.Experiences) != 2 || parsed.Experiences[0].Title != "Senior Go Engineer" || parsed.Experiences[0].Company != "Acme" || !parsed.Experiences[0].Current {
		t.Fatalf("experiences=%+v", parsed.Experiences)
	}
	var goSkill, pythonSkill *domain.DetectedSkill
	for index := range parsed.Skills {
		switch parsed.Skills[index].Name {
		case "Go":
			goSkill = &parsed.Skills[index]
		case "Python":
			pythonSkill = &parsed.Skills[index]
		}
	}
	if goSkill == nil || goSkill.EstimatedExperienceMonths < 70 || !goSkill.Current || goSkill.FirstUsed == nil || goSkill.FirstUsed.Year() != 2020 || goSkill.LastUsed != nil {
		t.Fatalf("Go experience estimate=%+v", goSkill)
	}
	if pythonSkill == nil || pythonSkill.EstimatedExperienceMonths != 22 || pythonSkill.LastUsed == nil || pythonSkill.LastUsed.Year() != 2019 {
		t.Fatalf("Python experience estimate=%+v", pythonSkill)
	}
	if goSkill.Confidence >= .92 || parsed.TotalExperienceMonths != 0 {
		t.Fatalf("estimated skill confidence or unsupported total experience was overstated: %+v", goSkill)
	}
	if now := time.Now().UTC(); goSkill.FirstUsed.After(now) {
		t.Fatalf("skill first-use date is in the future: %s", goSkill.FirstUsed)
	}
}
