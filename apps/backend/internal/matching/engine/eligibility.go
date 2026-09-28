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

// isClearlyNonSoftwareRole is intentionally narrow. It excludes obviously
// unrelated occupations from a confirmed software candidate without rejecting
// ambiguous engineering, product, data, or technical operations roles.
func isClearlyNonSoftwareRole(title string) bool {
	value := normalizeRole(title)
	if isSoftwareTitle(title) {
		return false
	}
	for _, term := range []string{
		"barista", "бариста", "バリスタ", "바리스타", "咖啡师", "bartender", "waiter", "waitress", "chef", "line cook", "cook", "baker",
		"housekeeper", "housekeeping", "food service", "restaurant", "hospitality", "cafe", "café",
		"cashier", "retail associate", "store associate", "store manager", "store supervisor", "sales associate", "sales", "account executive", "account manager", "technical account manager",
		"business development", "partner manager", "partnerships", "ecosystem partnerships", "relationship manager", "growth marketing", "marketing", "paid social", "social media", "marketing generalist", "marketing manager", "product manager", "go to market manager", "gtm manager",
		"recruiter", "talent acquisition", "human resources", "hr manager", "people operations",
		"corporate communications", "communications manager", "public relations", "pr manager", "customer experience", "customer engineer",
		"project manager", "program manager", "program management", "mortgage", "loan officer", "underwriter",
		"customer success", "customer support", "customer experience", "product support", "support specialist", "call center", "receptionist", "flight attendant",
		"nurse", "caregiver", "teacher", "delivery driver", "truck driver", "warehouse associate", "production team member", "supply chain", "leader in training", "shift supervisor",
		"designer", "data annotation", "annotator", "growth operations", "lifecycle specialist", "finops", "accounting", "grc manager",
	} {
		if strings.Contains(value, term) {
			// Preserve software team/program management roles when the title says
			// the role is specifically about engineering.
			if (term == "project manager" || term == "program manager" || term == "program management") &&
				(strings.Contains(value, "engineering") || strings.Contains(value, "software")) {
				continue
			}
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
