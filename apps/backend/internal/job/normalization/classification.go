package normalization

import (
	"strings"

	jobdomain "github.com/Hell077/HireRadar/apps/backend/internal/job/domain"
)

type PositionClassification struct {
	Family     string
	Speciality string
	Seniority  jobdomain.Seniority
	Confidence float64
}

func ClassifyPosition(title string) PositionClassification {
	value := strings.ToLower(strings.TrimSpace(title))
	result := PositionClassification{Family: "unknown", Seniority: ClassifySeniority(title), Confidence: 0.25}
	classify := func(family, speciality string, confidence float64) PositionClassification {
		return PositionClassification{Family: family, Speciality: speciality, Seniority: result.Seniority, Confidence: confidence}
	}
	switch {
	case containsAny(value, "grc", "governance risk compliance", "data annotation", "annotator", "barista", "bartender", "waiter", "waitress", "chef", "line cook", "housekeeper", "cashier", "retail associate", "flight attendant", "delivery driver", "truck driver", "warehouse associate", "production team member") || strings.Contains(value, "咖啡师") || strings.Contains(value, "バリスタ") || strings.Contains(value, "바리스타"):
		return classify("other", "other", 0.85)
	case containsAny(value, "security", "infosec", "information security", "appsec", "penetration test", "pentest"):
		return classify("security", "security", 0.9)
	case containsAny(value, "devops", "site reliability", "sre", "platform engineer", "infrastructure", "cloud engineer"):
		return classify("devops", "platform", 0.9)
	case containsAny(value, "qa", "quality assurance", "test automation", "automation engineer", "test engineer"):
		return classify("qa", "quality", 0.85)
	case containsAny(value, "data scientist", "data analyst", "analytics engineer", "business intelligence", "machine learning", "data engineer"):
		return classify("data", "data", 0.9)
	case containsAny(value, "backend", "back end", "frontend", "front end", "full stack", "fullstack", "software engineer", "software developer", "programmer", "developer", "engineer"):
		speciality := "general"
		switch {
		case containsAny(value, "backend", "back end"):
			speciality = "backend"
		case containsAny(value, "frontend", "front end"):
			speciality = "frontend"
		case containsAny(value, "full stack", "fullstack"):
			speciality = "fullstack"
		case containsAny(value, "mobile", "ios", "android"):
			speciality = "mobile"
		}
		return classify("software_engineering", speciality, 0.85)
	case containsAny(value, "product manager", "product owner", "product operations"):
		return classify("product", "product", 0.85)
	case containsAny(value, "designer", "design", "ux", "ui ", "user experience", "user interface"):
		return classify("design", "design", 0.85)
	case containsAny(value, "sales", "account executive", "business development", "sales development", "account manager", "partnership", "ecosystem"):
		return classify("sales", "sales", 0.85)
	case containsAny(value, "marketing", "growth marketing", "social media", "paid media", "paid social", "communications", "public relations", "lifecycle"):
		return classify("marketing", "marketing", 0.85)
	case containsAny(value, "customer support", "customer success", "support specialist", "technical support", "customer engineer"):
		return classify("support", "customer_support", 0.85)
	case containsAny(value, "recruiter", "talent acquisition", "human resources", "people operations", "hr "):
		return classify("hr", "people", 0.85)
	case containsAny(value, "accountant", "accounting", "finance", "controller", "financial analyst", "mortgage", "loan officer", "underwriter", "finops"):
		return classify("finance", "finance", 0.85)
	case containsAny(value, "operations", "supply chain", "logistics"):
		return classify("operations", "operations", 0.8)
	case containsAny(value, "manager", "director", "vice president", " vp ", "chief", "head of"):
		return classify("management", "management", 0.65)
	default:
		return result
	}
}
