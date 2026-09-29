package engine

import (
	"strings"
	"unicode"

	"github.com/Hell077/HireRadar/apps/backend/internal/language"
)

func isSoftwareCandidate(candidate Candidate) bool {
	for _, position := range candidate.Positions {
		if isSoftwareTitle(position) {
			return true
		}
	}
	for _, skill := range candidate.Skills {
		if isProgrammingSkill(skill.Name) {
			return true
		}
	}
	return false
}

func isSoftwareTitle(title string) bool {
	value := normalizeRole(title)
	for _, term := range []string{
		"software", "developer", "programmer", "backend", "back end", "frontend", "front end",
		"full stack", "fullstack", "devops", "site reliability", "sre", "platform engineer",
		"data engineer", "qa engineer", "test automation", "automation engineer", "web engineer",
		"mobile engineer", "cloud engineer", "application engineer", "golang", "programming",
	} {
		if strings.Contains(value, term) {
			return true
		}
	}
	return false
}

func isProgrammingSkill(value string) bool {
	value = normalizeRole(value)
	for _, term := range []string{
		"go", "golang", "javascript", "typescript", "python", "java", "rust", "ruby", "php",
		"kotlin", "swift", "c++", "c#", "react", "next js", "angular", "vue", "node js",
		"kubernetes", "docker", "postgresql", "mysql", "terraform", "aws", "azure", "gcp",
	} {
		if value == normalizeRole(term) {
			return true
		}
	}
	return false
}

func hasLanguageMismatch(known []string, jobText string) bool {
	for _, required := range language.RequiredLanguages(jobText) {
		if !hasLanguage(known, required) {
			return true
		}
	}
	primary := language.ContentLanguage(jobText)
	return primary != "" && !hasLanguage(known, primary)
}

func hasLanguage(values []string, code string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), code) {
			return true
		}
	}
	return false
}

func normalizeRole(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.Join(strings.FieldsFunc(value, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsNumber(r) || r == '+' || r == '#')
	}), " ")
}
