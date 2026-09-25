package normalization

import (
	"html"
	"regexp"
	"strings"

	jobdomain "github.com/Hell077/HireRadar/apps/backend/internal/job/domain"
	sourcedomain "github.com/Hell077/HireRadar/apps/backend/internal/source/domain"
)

var (
	tags    = regexp.MustCompile(`<[^>]*>`)
	spaces  = regexp.MustCompile(`\s+`)
	nonWord = regexp.MustCompile(`[^a-z0-9]+`)
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
	return jobdomain.Job{Company: strings.TrimSpace(external.CompanyName), Title: strings.TrimSpace(external.Title), NormalizedTitle: TitleKey(external.Title), Description: CleanDescription(external.Description), EmploymentTypes: types, RemotePolicy: policy, Location: strings.TrimSpace(external.Location), Countries: countries, Eligibility: eligibility, ApplyURL: strings.TrimSpace(external.ApplyURL), PublishedAt: external.PublishedAt, SourcePriority: source.Priority, Status: jobdomain.Active}
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
