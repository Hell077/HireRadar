package processing

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/Hell077/HireRadar/apps/backend/internal/resume/domain"
	pdf "github.com/giraffesyo/pdf"
)

const (
	maxPages         = 100
	maxExtractedText = 1 << 20
)

var ErrNoText = errors.New("PDF contains no extractable text; scanned PDFs are not supported yet")

type SkillTerm struct {
	Name       string
	Normalized string
}

func ExtractText(ctx context.Context, data []byte) (string, error) {
	if len(data) < 5 || int64(len(data)) > domain.MaxFileSize || !bytes.HasPrefix(data, []byte("%PDF-")) {
		return "", errors.New("invalid PDF object")
	}
	reader := bytes.NewReader(data)
	var text strings.Builder
	pages := 0
	_, err := pdf.ExtractPages(ctx, reader, int64(len(data)), pdf.Options{
		Strict: true,
		Limits: pdf.Limits{MaxStreamBytes: 20 << 20, MaxOperatorsPerPage: 250_000, MaxGlyphsPerPage: 100_000, MaxFormDepth: 12, MaxImagesPerPage: 64, MaxImageBytesPerPage: 1 << 20, MaxImagePixels: 1_000_000},
	}, func(page pdf.Page) error {
		pages++
		if pages > maxPages {
			return errors.New("PDF exceeds page limit")
		}
		pageText := strings.TrimSpace(page.Text())
		if pageText == "" {
			return nil
		}
		if text.Len()+len(pageText)+1 > maxExtractedText {
			return errors.New("PDF extracted text exceeds limit")
		}
		text.WriteString(pageText)
		text.WriteByte('\n')
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("extract PDF text: %w", err)
	}
	normalized := normalizeText(text.String())
	if normalized == "" {
		return "", ErrNoText
	}
	return normalized, nil
}

func Analyze(resumeID domain.ID, text string, vocabulary []SkillTerm) domain.ParsedResume {
	lines := strings.Split(text, "\n")
	result := domain.ParsedResume{ResumeID: resumeID, Text: text, Skills: []domain.DetectedSkill{}, Positions: []domain.DetectedPosition{}}
	seenSkills := map[string]bool{}
	for _, term := range vocabulary {
		canonical := strings.ToLower(term.Name)
		if term.Name == "" || term.Normalized == "" || seenSkills[canonical] {
			continue
		}
		if containsTerm(text, term.Name) || containsTerm(text, term.Normalized) {
			result.Skills = append(result.Skills, domain.DetectedSkill{Name: term.Name, Confidence: 0.92})
			seenSkills[canonical] = true
		}
	}
	slices.SortFunc(result.Skills, func(a, b domain.DetectedSkill) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	seenPositions := map[string]bool{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) < 4 || len(line) > 120 {
			continue
		}
		lower := strings.ToLower(line)
		if strings.Contains(lower, "@") || strings.Contains(lower, "http") {
			continue
		}
		if hasPositionSignal(lower) {
			key := strings.ToLower(line)
			if !seenPositions[key] {
				result.Positions = append(result.Positions, domain.DetectedPosition{Title: line, Confidence: 0.62})
				seenPositions[key] = true
			}
		}
	}
	if years := yearsExperience.FindStringSubmatch(text); len(years) == 2 {
		var n int
		_, _ = fmt.Sscanf(years[1], "%d", &n)
		if n <= 70 {
			result.TotalExperienceMonths = n * 12
		}
	}
	return result
}

var yearsExperience = regexp.MustCompile(`(?i)(\d{1,2})\+?\s+years?\s+(?:of\s+)?experience`)

func containsTerm(text, term string) bool {
	term = strings.ToLower(strings.TrimSpace(term))
	if term == "" {
		return false
	}
	needle := []rune(term)
	textRunes := []rune(strings.ToLower(text))
	for start := 0; start+len(needle) <= len(textRunes); start++ {
		if !slices.Equal(textRunes[start:start+len(needle)], needle) {
			continue
		}
		beforeOK := start == 0 || !unicode.IsLetter(textRunes[start-1]) && !unicode.IsDigit(textRunes[start-1])
		end := start + len(needle)
		afterOK := end == len(textRunes) || !unicode.IsLetter(textRunes[end]) && !unicode.IsDigit(textRunes[end])
		if beforeOK && afterOK {
			return true
		}
	}
	return false
}

func hasPositionSignal(line string) bool {
	for _, term := range []string{"engineer", "developer", "architect", "manager", "analyst", "designer", "scientist", "recruiter", "consultant", "директор", "разработчик", "инженер", "аналитик", "менеджер"} {
		if containsTerm(line, term) {
			return true
		}
	}
	return false
}

func normalizeText(text string) string {
	text = strings.ToValidUTF8(text, "")
	lines := strings.Split(text, "\n")
	clean := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.Map(func(r rune) rune {
			if unicode.IsControl(r) && r != '\t' {
				return -1
			}
			return r
		}, line)
		line = strings.Join(strings.Fields(line), " ")
		if line != "" {
			clean = append(clean, line)
		}
	}
	return strings.Join(clean, "\n")
}
