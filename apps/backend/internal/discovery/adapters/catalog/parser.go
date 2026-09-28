package catalog

import (
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/Hell077/HireRadar/apps/backend/internal/discovery/domain"
	yaml "go.yaml.in/yaml/v3"
	nethtml "golang.org/x/net/html"
)

var (
	markdownLink = regexp.MustCompile(`\[([^\]]+)\]\((https?://[^)\s]+)\)`)
	htmlTag      = regexp.MustCompile(`<[^>]+>`)
	urlInText    = regexp.MustCompile(`https?://[^\s<>)"']+`)
)

func Parse(parser string, files map[string][]byte) ([]domain.DiscoveredTarget, error) {
	switch parser {
	case "remoteintech":
		return parseRemoteInTech(files)
	case "established_remote":
		return parseMarkdownCompanyTable(files, false)
	case "global_hiring":
		return parseMarkdownCompanyTable(files, true)
	case "european_remote", "remote_by_default":
		return parseRegionalCompanyTable(files, parser)
	case "awesome_remote_job":
		return parseAwesomeRemoteJob(files)
	case "remote_freelancer":
		return parseRemoteFreelancer(files)
	case "remote_developer_directory":
		return parseRemoteDeveloperDirectory(files)
	case "github_issue_boards":
		return parseGitHubIssueBoards(files)
	default:
		return nil, fmt.Errorf("unknown discovery parser %q", parser)
	}
}

func parseRegionalCompanyTable(files map[string][]byte, parser string) ([]domain.DiscoveredTarget, error) {
	body, ok := files["README.md"]
	if !ok {
		return nil, fmt.Errorf("README.md is missing")
	}
	targets := make([]domain.DiscoveredTarget, 0)
	for _, line := range strings.Split(string(body), "\n") {
		if !strings.Contains(line, "|") || strings.Contains(line, "---") {
			continue
		}
		cells := splitRow(line)
		if len(cells) < 4 || !strings.Contains(cells[0], "|") && strings.Contains(strings.ToLower(cells[0]), "company") {
			continue
		}
		name := cleanCell(cells[0])
		name = strings.TrimSpace(regexp.MustCompile(`\s+[\x{1F1E6}-\x{1F1FF}]{1,2}$`).ReplaceAllString(name, ""))
		profile := ""
		if len(cells) > 5 {
			_, profile = firstLink(cells[5])
		}
		if name == "" || profile == "" {
			continue
		}
		tech := splitTechnologyList(stripTechnologyMarkers(cells[2]))
		metadata, _ := json.Marshal(map[string]any{"industry": cleanCell(cells[1]), "profile_url": profile, "salary_transparency": cleanCell(cells[3]), "global_salary": cleanCell(cells[4])})
		targets = append(targets, domain.DiscoveredTarget{Kind: domain.CompanyTarget, ExternalKey: path.Base(strings.TrimSuffix(profile, "/")), Name: name, Technologies: tech, DiscoveryFields: metadata})
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("%s company table contained no profile rows", parser)
	}
	return deduplicateTargets(targets), nil
}

func parseAwesomeRemoteJob(files map[string][]byte) ([]domain.DiscoveredTarget, error) {
	body, ok := files["README.md"]
	if !ok {
		return nil, fmt.Errorf("README.md is missing")
	}
	section := ""
	targets := make([]domain.DiscoveredTarget, 0)
	for _, line := range strings.Split(string(body), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			section = strings.ToLower(strings.TrimSpace(strings.TrimLeft(trimmed, "#")))
			continue
		}
		kind := domain.TargetKind("")
		switch {
		case containsAny(section, "job board", "job site", "aggregator", "job platform"):
			kind = domain.JobBoardTarget
		case containsAny(section, "freelance", "contract"):
			kind = domain.FreelancePlatformTarget
		case containsAny(section, "remote compan", "companies"):
			kind = domain.CompanyTarget
		}
		if kind == "" {
			continue
		}
		for _, match := range markdownLink.FindAllStringSubmatch(trimmed, -1) {
			name, href := strings.TrimSpace(html.UnescapeString(match[1])), strings.TrimSpace(match[2])
			if name == "" || !validHTTPURL(href) {
				continue
			}
			fields, _ := json.Marshal(map[string]string{"catalog_section": section})
			targets = append(targets, domain.DiscoveredTarget{Kind: kind, ExternalKey: firstNonEmpty(domainOf(href), domain.NameKey(name)), Name: name, WebsiteURL: href, OfficialDomain: domainOf(href), DiscoveryFields: fields})
		}
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("awesome-remote-job README had no supported sections")
	}
	return deduplicateTargets(targets), nil
}

func parseRemoteFreelancer(files map[string][]byte) ([]domain.DiscoveredTarget, error) {
	body, ok := files["README.md"]
	if !ok {
		return nil, fmt.Errorf("README.md is missing")
	}
	section := ""
	targets := make([]domain.DiscoveredTarget, 0)
	for _, line := range strings.Split(string(body), "\n") {
		if !strings.Contains(line, "|") || strings.Contains(line, "---") {
			continue
		}
		cells := splitRow(line)
		if len(cells) == 0 {
			continue
		}
		label := strings.ToLower(cleanCell(cells[0]))
		if len(cells) > 1 && strings.TrimSpace(cells[1]) == "" && strings.HasPrefix(label, "**") {
			section = strings.Trim(label, "* ")
			continue
		}
		name, href := firstLink(cells[0])
		if name == "" || !validHTTPURL(href) {
			continue
		}
		kind := domain.FreelancePlatformTarget
		if section == "jobs" {
			kind = domain.JobBoardTarget
		}
		if section != "clients" && section != "tutoring" && section != "jobs" {
			continue
		}
		fields, _ := json.Marshal(map[string]string{"catalog_section": section, "details": strings.Join(cells[1:], " ")})
		targets = append(targets, domain.DiscoveredTarget{Kind: kind, ExternalKey: domainOf(href), Name: name, WebsiteURL: href, OfficialDomain: domainOf(href), DiscoveryFields: fields})
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("TheRemoteFreelancer README had no supported catalog rows")
	}
	return deduplicateTargets(targets), nil
}

func parseRemoteDeveloperDirectory(files map[string][]byte) ([]domain.DiscoveredTarget, error) {
	body, ok := files["README.md"]
	if !ok {
		return nil, fmt.Errorf("README.md is missing")
	}
	doc, err := nethtml.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	section := ""
	targets := make([]domain.DiscoveredTarget, 0)
	var walk func(*nethtml.Node)
	walk = func(node *nethtml.Node) {
		if node.Type == nethtml.ElementNode && (node.Data == "h1" || node.Data == "h2" || node.Data == "h3") {
			section = strings.ToLower(strings.TrimSpace(nodeText(node)))
		}
		if node.Type == nethtml.ElementNode && node.Data == "li" && (containsAny(section, "remote jobs director", "freelancing", "placement consultan")) {
			text := strings.TrimSpace(nodeText(node))
			links := urlInText.FindAllString(text, -1)
			if len(links) > 0 {
				href := strings.TrimRight(strings.Trim(links[0], "<>()"), ".,;:!")
				label := strings.TrimSpace(strings.SplitN(text, ":", 2)[0])
				if validHTTPURL(href) && label != "" {
					kind := domain.JobBoardTarget
					if containsAny(section, "freelancing", "placement consultan") {
						kind = domain.FreelancePlatformTarget
					}
					fields, _ := json.Marshal(map[string]string{"catalog_section": section})
					targets = append(targets, domain.DiscoveredTarget{Kind: kind, ExternalKey: domainOf(href), Name: label, WebsiteURL: href, OfficialDomain: domainOf(href), DiscoveryFields: fields})
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	if len(targets) == 0 {
		return nil, fmt.Errorf("remote developer directory README had no job or freelance targets")
	}
	return deduplicateTargets(targets), nil
}

func parseGitHubIssueBoards(files map[string][]byte) ([]domain.DiscoveredTarget, error) {
	body, ok := files["README.md"]
	if !ok {
		return nil, fmt.Errorf("README.md is missing")
	}
	targets := make([]domain.DiscoveredTarget, 0)
	for _, line := range strings.Split(string(body), "\n") {
		for _, match := range markdownLink.FindAllStringSubmatch(line, -1) {
			parsed, err := url.Parse(match[2])
			if err != nil || !strings.EqualFold(parsed.Hostname(), "github.com") {
				continue
			}
			parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
			if len(parts) != 2 {
				continue
			}
			owner, repo := parts[0], strings.TrimSuffix(parts[1], ".git")
			if owner == "" || repo == "" || strings.HasPrefix(owner, "#") {
				continue
			}
			key := owner + "/" + repo
			description := strings.TrimSpace(strings.TrimPrefix(line, "- "))
			fields, _ := json.Marshal(map[string]string{"description": description, "repository": key})
			targets = append(targets, domain.DiscoveredTarget{Kind: domain.GitHubJobsTarget, ExternalKey: key, Name: key, WebsiteURL: "https://github.com/" + key, OfficialDomain: "github.com/" + strings.ToLower(key), DiscoveryFields: fields})
		}
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("awesome-github-issues-job-boards README had no repository links")
	}
	return deduplicateTargets(targets), nil
}

func nodeText(node *nethtml.Node) string {
	var b strings.Builder
	var walk func(*nethtml.Node)
	walk = func(current *nethtml.Node) {
		if current.Type == nethtml.TextNode {
			b.WriteString(current.Data)
			b.WriteByte(' ')
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return b.String()
}

func stripTechnologyMarkers(value string) string {
	for _, marker := range []string{"🖥", "🎨", "☁️", "☁"} {
		value = strings.ReplaceAll(value, marker, ",")
	}
	return value
}

func validHTTPURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Hostname() != ""
}

func parseRemoteInTech(files map[string][]byte) ([]domain.DiscoveredTarget, error) {
	targets := make([]domain.DiscoveredTarget, 0, len(files))
	for file, body := range files {
		if !strings.HasPrefix(file, "src/companies/") || !strings.HasSuffix(file, ".md") || len(body) > 1<<20 {
			continue
		}
		frontmatter, err := yamlFrontmatter(body)
		if err != nil {
			slog.Warn("skip malformed Remote In Tech company profile", "file", file, "error", err)
			continue
		}
		name := stringValue(frontmatter["title"])
		if name == "" {
			name = stringValue(frontmatter["name"])
		}
		if name == "" {
			name = strings.TrimSuffix(path.Base(file), path.Ext(file))
		}
		website := stringValue(frontmatter["website"])
		careers := stringValue(frontmatter["careers_url"])
		if website == "" && careers == "" {
			continue
		}
		fields, _ := json.Marshal(frontmatter)
		regions := stringList(frontmatter["region"])
		technologies := stringList(frontmatter["technologies"])
		officialDomain := domainOf(website)
		if officialDomain == "" && !isKnownATSHost(domainOf(careers)) {
			officialDomain = domainOf(careers)
		}
		targets = append(targets, domain.DiscoveredTarget{
			Kind: domain.CompanyTarget, ExternalKey: strings.TrimSuffix(path.Base(file), path.Ext(file)),
			Name: name, WebsiteURL: website, CareersURL: careers,
			OfficialDomain: officialDomain, Regions: regions, Technologies: technologies, DiscoveryFields: fields,
		})
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].ExternalKey < targets[j].ExternalKey })
	return targets, nil
}

func yamlFrontmatter(body []byte) (map[string]any, error) {
	text := strings.TrimPrefix(strings.TrimSpace(string(body)), "---")
	end := strings.Index(text, "\n---")
	if end < 0 {
		return nil, fmt.Errorf("YAML frontmatter delimiters are missing")
	}
	var fields map[string]any
	if err := yaml.Unmarshal([]byte(text[:end]), &fields); err != nil {
		return nil, err
	}
	return fields, nil
}

func parseMarkdownCompanyTable(files map[string][]byte, globalHiring bool) ([]domain.DiscoveredTarget, error) {
	body, ok := files["README.md"]
	if !ok {
		return nil, fmt.Errorf("README.md is missing")
	}
	targets := make([]domain.DiscoveredTarget, 0)
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "|") || strings.Contains(line, "---") {
			continue
		}
		cells := splitRow(line)
		if len(cells) < 4 || strings.Contains(strings.ToLower(strings.Join(cells, " ")), "company") && strings.Contains(strings.ToLower(cells[0]), "company") {
			continue
		}
		name, website := firstLink(cells[0])
		if name == "" || website == "" {
			continue
		}
		careers := ""
		var technologies []string
		metadata := map[string]any{"business": cleanCell(cells[1])}
		if globalHiring {
			technologies = splitTechnologyList(cells[2])
			if len(cells) > 4 {
				metadata["open_salary"] = cleanCell(cells[3])
				metadata["global_hiring_restrictions"] = cleanCell(cells[4])
			}
		} else {
			technologies = splitTechnologyList(cells[2])
			if len(cells) > 4 {
				metadata["globally_competitive_compensation"] = cleanCell(cells[3])
				metadata["other_links"] = cleanCell(cells[4])
			}
		}
		for _, link := range markdownLink.FindAllStringSubmatch(cells[len(cells)-1], -1) {
			label, href := strings.ToLower(link[1]), link[2]
			if containsAny(label, "job", "career", "work", "opening", "position", "vacanc") || careers == "" && containsAny(href, "/jobs", "/career", "/work-with-us", "/open-position", "/careers") {
				careers = href
				break
			}
		}
		fields, _ := json.Marshal(metadata)
		externalKey := domain.NameKey(name)
		targets = append(targets, domain.DiscoveredTarget{Kind: domain.CompanyTarget, ExternalKey: externalKey, Name: name, WebsiteURL: website, CareersURL: careers, OfficialDomain: domainOf(website), Technologies: technologies, DiscoveryFields: fields})
	}
	return deduplicateTargets(targets), nil
}

func splitRow(line string) []string {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "|") {
		line = strings.TrimPrefix(line, "|")
	}
	if strings.HasSuffix(line, "|") {
		line = strings.TrimSuffix(line, "|")
	}
	parts := strings.Split(line, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func firstLink(value string) (string, string) {
	match := markdownLink.FindStringSubmatch(value)
	if len(match) != 3 {
		return "", ""
	}
	return strings.TrimSpace(html.UnescapeString(match[1])), strings.TrimSpace(match[2])
}

func cleanCell(value string) string {
	value = strings.ReplaceAll(value, "<br>", " ")
	value = strings.ReplaceAll(value, "<br/>", " ")
	value = htmlTag.ReplaceAllString(value, " ")
	value = markdownLink.ReplaceAllString(value, "$1")
	return strings.Join(strings.Fields(html.UnescapeString(value)), " ")
}

func splitTechnologyList(value string) []string {
	value = cleanCell(value)
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		part = strings.TrimSpace(strings.Trim(part, "*`"))
		if part == "" || strings.EqualFold(part, "and more") || strings.EqualFold(part, "n/a") {
			continue
		}
		key := strings.ToLower(part)
		if !seen[key] {
			seen[key] = true
			result = append(result, part)
		}
	}
	return result
}

func domainOf(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	return strings.TrimPrefix(host, "www.")
}

func isKnownATSHost(host string) bool {
	for _, atsHost := range []string{"greenhouse.io", "lever.co", "ashbyhq.com", "myworkdayjobs.com", "smartrecruiters.com", "workable.com", "teamtailor.com", "recruitee.com", "personio.com", "jobvite.com", "icims.com"} {
		if host == atsHost || strings.HasSuffix(host, "."+atsHost) {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func stringValue(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func stringList(value any) []string {
	switch values := value.(type) {
	case string:
		return []string{strings.TrimSpace(values)}
	case []any:
		result := make([]string, 0, len(values))
		for _, item := range values {
			if text := stringValue(item); text != "" {
				result = append(result, text)
			}
		}
		return result
	default:
		return nil
	}
}

func deduplicateTargets(targets []domain.DiscoveredTarget) []domain.DiscoveredTarget {
	seen := map[string]bool{}
	result := make([]domain.DiscoveredTarget, 0, len(targets))
	for _, target := range targets {
		key := target.OfficialDomain
		if key == "" {
			key = domain.NameKey(target.Name)
		}
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, target)
	}
	return result
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
