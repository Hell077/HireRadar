package processing

import (
	"context"
	"testing"

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
