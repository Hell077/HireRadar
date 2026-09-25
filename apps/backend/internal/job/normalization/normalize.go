package normalization

import (
	"html"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	jobdomain "github.com/Hell077/HireRadar/apps/backend/internal/job/domain"
	sourcedomain "github.com/Hell077/HireRadar/apps/backend/internal/source/domain"
)

var (
	tags        = regexp.MustCompile(`<[^>]*>`)
	spaces      = regexp.MustCompile(`\s+`)
	nonWord     = regexp.MustCompile(`[^a-z0-9]+`)
	salaryLabel = regexp.MustCompile(`(?i)(salary|compensation|pay range|base pay)`)
	salaryRange = regexp.MustCompile(`(?i)(USD|EUR|GBP|CAD|AUD|NZD|SGD|INR|KZT|[$€£])?\s*([0-9][0-9,]*(?:\.[0-9]+)?\s*[kK]?)\s*(?:-|–|—|to)\s*(?:USD|EUR|GBP|CAD|AUD|NZD|SGD|INR|KZT|[$€£])?\s*([0-9][0-9,]*(?:\.[0-9]+)?\s*[kK]?)\s*(USD|EUR|GBP|CAD|AUD|NZD|SGD|INR|KZT)?`)
)

func CompanyKey(name string) string {
	value := strings.ToLower(strings.TrimSpace(name))
	value = nonWord.ReplaceAllString(value, " ")
	words := strings.Fields(value)
	for len(words) > 0 {
		switch words[len(words)-1] {
		case "inc", "incorporated", "llc", "ltd", "limited", "corp", "corporation", "company", "co":
			words = words[:len(words)-1]
		default:
			goto done
		}
	}
done:
	return strings.Join(words, " ")
}

func TitleKey(title string) string {
	return strings.TrimSpace(nonWord.ReplaceAllString(strings.ToLower(title), " "))
}

func CleanDescription(value string) string {
	value = tags.ReplaceAllString(value, " ")
	value = html.UnescapeString(value)
	return strings.Join(strings.Fields(spaces.ReplaceAllString(value, " ")), " ")
}

func Normalize(source sourcedomain.Source, external sourcedomain.ExternalJob) jobdomain.Job {
	policy, eligibility, countries := ClassifyLocation(external.Location)
	types := []string{}
	if value := strings.TrimSpace(external.EmploymentType); value != "" {
		types = append(types, value)
	}
	description := CleanDescription(external.Description)
	return jobdomain.Job{Company: strings.TrimSpace(external.CompanyName), Title: strings.TrimSpace(external.Title), NormalizedTitle: TitleKey(external.Title), Seniority: ClassifySeniority(external.Title), Description: description, Salary: ExtractSalary(external.Description), EmploymentTypes: types, RemotePolicy: policy, Location: strings.TrimSpace(external.Location), Countries: countries, Eligibility: eligibility, ApplyURL: strings.TrimSpace(external.ApplyURL), PublishedAt: external.PublishedAt, SourcePriority: source.Priority, Status: jobdomain.Active}
}

func ExtractSalary(description string) *jobdomain.SalaryRange {
	clean := CleanDescription(description)
	label := salaryLabel.FindStringIndex(clean)
	if label == nil {
		return nil
	}
	end := label[1] + 180
	if end > len(clean) {
		end = len(clean)
	}
	window := clean[label[1]:end]
	match := salaryRange.FindStringSubmatch(window)
	if match == nil {
		return nil
	}
	minimum, ok := parseSalaryAmount(match[2])
	if !ok {
		return nil
	}
	maximum, ok := parseSalaryAmount(match[3])
	if !ok || minimum <= 0 || maximum < minimum || maximum > 100_000_000 {
		return nil
	}
	currency := strings.ToUpper(match[4])
	if currency == "" {
		currency = strings.ToUpper(match[1])
	}
	switch currency {
	case "$":
		currency = "USD"
	case "€":
		currency = "EUR"
	case "£":
		currency = "GBP"
	}
	if currency == "" {
		return nil
	}
	period := "unspecified"
	periodText := strings.ToLower(window)
	switch {
	case containsAny(periodText, "annually", "annual", "per year", "yearly", "/year", " a year"):
		period = "year"
	case containsAny(periodText, "per month", "monthly", "/month"):
		period = "month"
	case containsAny(periodText, "per week", "weekly", "/week"):
		period = "week"
	case containsAny(periodText, "per hour", "hourly", "/hour"):
		period = "hour"
	}
	return &jobdomain.SalaryRange{Minimum: minimum, Maximum: maximum, Currency: currency, Period: period}
}

func parseSalaryAmount(value string) (float64, bool) {
	value = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), ",", ""))
	multiplier := 1.0
	if strings.HasSuffix(value, "k") {
		multiplier = 1000
		value = strings.TrimSpace(strings.TrimSuffix(value, "k"))
	}
	amount, err := strconv.ParseFloat(value, 64)
	return amount * multiplier, err == nil
}

type SkillTerm struct{ ID, Name, Normalized string }

func ExtractSkills(text string, vocabulary []SkillTerm) []jobdomain.JobSkill {
	result := []jobdomain.JobSkill{}
	seen := map[string]bool{}
	for _, term := range vocabulary {
		if term.ID == "" || term.Name == "" || term.Normalized == "" || seen[term.ID] {
			continue
		}
		if containsTerm(text, term.Name) || containsTerm(text, term.Normalized) {
			result = append(result, jobdomain.JobSkill{ID: term.ID, Name: term.Name, Required: false, Confidence: 0.78})
			seen[term.ID] = true
		}
	}
	return result
}

func ClassifySeniority(title string) jobdomain.Seniority {
	value := strings.ToLower(title)
	switch {
	case containsTerm(value, "intern"), containsTerm(value, "internship"):
		return jobdomain.Intern
	case containsTerm(value, "junior"), containsTerm(value, "jr"):
		return jobdomain.Junior
	case containsTerm(value, "principal"):
		return jobdomain.Principal
	case containsTerm(value, "staff"):
		return jobdomain.Staff
	case containsTerm(value, "director"):
		return jobdomain.Director
	case containsTerm(value, "vp"), containsTerm(value, "vice president"), containsTerm(value, "chief"):
		return jobdomain.Executive
	case containsTerm(value, "manager"):
		return jobdomain.Manager
	case containsTerm(value, "lead"):
		return jobdomain.Lead
	case containsTerm(value, "senior"), containsTerm(value, "sr"):
		return jobdomain.Senior
	case containsTerm(value, "mid-level"), containsTerm(value, "mid level"), containsTerm(value, "middle"):
		return jobdomain.Mid
	default:
		return jobdomain.SeniorityUnknown
	}
}

func containsTerm(text, term string) bool {
	textRunes := []rune(strings.ToLower(text))
	needle := []rune(strings.ToLower(strings.TrimSpace(term)))
	if len(needle) == 0 {
		return false
	}
	for start := 0; start+len(needle) <= len(textRunes); start++ {
		match := true
		for index, r := range needle {
			if textRunes[start+index] != r {
				match = false
				break
			}
		}
		if !match {
			continue
		}
		before := start == 0 || !unicode.IsLetter(textRunes[start-1]) && !unicode.IsDigit(textRunes[start-1])
		end := start + len(needle)
		after := end == len(textRunes) || !unicode.IsLetter(textRunes[end]) && !unicode.IsDigit(textRunes[end])
		if before && after {
			return true
		}
	}
	return false
}

func ClassifyLocation(raw string) (jobdomain.RemotePolicy, jobdomain.Eligibility, []string) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" || value == "unknown" {
		return jobdomain.RemoteUnknown, jobdomain.EligibilityUnknown, []string{}
	}
	kz := containsAny(value, "kazakhstan", "kazakh", "(kz)", "kz only", "kz-based")
	world := containsAny(value, "worldwide", "global", "anywhere", "work from anywhere", "fully distributed")
	remote := strings.Contains(value, "remote")
	countries := explicitCountries(value)
	policy := jobdomain.Onsite
	switch {
	case world:
		policy = jobdomain.RemoteWorldwide
	case strings.Contains(value, "hybrid"):
		policy = jobdomain.Hybrid
	case strings.Contains(value, "emea") || strings.Contains(value, "latam") || strings.Contains(value, "europe"):
		policy = jobdomain.RemoteRegion
	case remote:
		policy = jobdomain.Remote
	}
	if containsAny(value, "except kazakhstan", "excluding kazakhstan", "excluding kz", "not in kazakhstan", "not eligible in kazakhstan") {
		return policy, jobdomain.NotEligible, countries
	}
	if kz {
		return policy, jobdomain.Eligible, []string{"KZ"}
	}
	if world {
		return policy, jobdomain.Eligible, countries
	}
	if containsAny(value, "us only", "usa only", "united states only", "canada only", "eu only", "europe only", "uk only", "india only", "must be located in the us", "authorized to work in the united states", "us-based only") {
		return policy, jobdomain.NotEligible, countries
	}
	if containsAny(value, "new york", "california", "united states", "u.s.", "usa", "canada", "united kingdom", "london", "germany", "france", "india") {
		return policy, jobdomain.NotEligible, countries
	}
	// A bare remote label and broad regions such as EMEA do not establish
	// whether a candidate in Kazakhstan can legally work from that location.
	return policy, jobdomain.EligibilityUnknown, []string{}
}

func explicitCountries(value string) []string {
	codes := []string{}
	add := func(needle, code string) {
		if strings.Contains(value, needle) {
			for _, old := range codes {
				if old == code {
					return
				}
			}
			codes = append(codes, code)
		}
	}
	add("kazakhstan", "KZ")
	add("kazakh", "KZ")
	add("(kz)", "KZ")
	add("united states", "US")
	add("u.s.", "US")
	add("usa", "US")
	add("us only", "US")
	add("new york", "US")
	add("california", "US")
	add("canada", "CA")
	add("united kingdom", "GB")
	add("london", "GB")
	add("india", "IN")
	add("germany", "DE")
	add("france", "FR")
	return codes
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
