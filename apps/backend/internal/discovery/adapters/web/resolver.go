package web

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/discovery/domain"
	"golang.org/x/net/html"
	"golang.org/x/net/idna"
)

const maxPageBytes = 4 << 20

var embeddedURL = regexp.MustCompile(`(?i)(?:https?:)?//[^\s"'<>]+`)

type Resolver struct {
	client   *http.Client
	resolver dnsResolver
}

type dnsResolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

type link struct{ label, href string }

func NewResolver() *Resolver {
	r := &Resolver{resolver: net.DefaultResolver}
	transport := &http.Transport{Proxy: nil, DialContext: r.dialPublic}
	r.client = &http.Client{Transport: transport, Timeout: 12 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("careers page exceeded five redirects")
		}
		return r.validateURL(req.Context(), req.URL)
	}}
	return r
}

func (r *Resolver) Resolve(ctx context.Context, candidate domain.Candidate) (domain.Resolution, error) {
	start := candidate.CareersURL
	if start == "" {
		start = candidate.WebsiteURL
	}
	if start == "" {
		return domain.Resolution{Reason: "candidate has no public website or careers URL"}, nil
	}
	queue := []string{start}
	seen := map[string]bool{}
	var lastReason string
	for len(queue) > 0 && len(seen) < 8 {
		current := queue[0]
		queue = queue[1:]
		resolvedURL, err := url.Parse(current)
		if err != nil {
			lastReason = "invalid careers URL"
			continue
		}
		if !resolvedURL.IsAbs() {
			lastReason = "careers URL must be absolute"
			continue
		}
		key := resolvedURL.String()
		if seen[key] {
			continue
		}
		seen[key] = true
		body, finalURL, err := r.fetch(ctx, resolvedURL)
		if err != nil {
			return domain.Resolution{}, fmt.Errorf("fetch public careers page: %w", err)
		}
		provider := Detect(finalURL, body)
		if provider != nil {
			return domain.Resolution{Provider: provider}, nil
		}
		links := extractLinks(body, finalURL)
		if len(seen) == 1 {
			for _, candidateLink := range careerLinks(links, candidate.OfficialDomain) {
				if len(queue)+len(seen) >= 8 {
					break
				}
				queue = append(queue, candidateLink.href)
			}
		}
		lastReason = "no supported ATS link found in company careers page"
	}
	return domain.Resolution{Reason: lastReason}, nil
}

func (r *Resolver) fetch(ctx context.Context, target *url.URL) ([]byte, *url.URL, error) {
	if err := r.validateURL(ctx, target); err != nil {
		return nil, nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, nil, err
	}
	request.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	request.Header.Set("User-Agent", "HireRadar-SourceDiscovery/1.0 (+https://github.com/Hell077/HireRadar)")
	response, err := r.client.Do(request)
	if err != nil {
		return nil, nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 400 {
		return nil, nil, fmt.Errorf("careers page returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxPageBytes+1))
	if err != nil {
		return nil, nil, err
	}
	if len(body) > maxPageBytes {
		return nil, nil, fmt.Errorf("careers page exceeded %d bytes", maxPageBytes)
	}
	finalURL := *response.Request.URL
	return body, &finalURL, nil
}

func (r *Resolver) validateURL(ctx context.Context, target *url.URL) error {
	if target == nil || (target.Scheme != "https" && target.Scheme != "http") || target.User != nil {
		return errors.New("careers URL must use HTTP or HTTPS without credentials")
	}
	host := strings.TrimSuffix(strings.ToLower(target.Hostname()), ".")
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") || strings.HasSuffix(host, ".test") || strings.HasSuffix(host, ".invalid") || host == "host.docker.internal" {
		return fmt.Errorf("careers URL host %q is not public", host)
	}
	port := target.Port()
	if port != "" && !((target.Scheme == "https" && port == "443") || (target.Scheme == "http" && port == "80")) {
		return errors.New("careers URL uses a nonstandard port")
	}
	addresses, err := r.lookup(ctx, host)
	if err != nil {
		return fmt.Errorf("resolve careers host: %w", err)
	}
	if len(addresses) == 0 {
		return errors.New("careers host has no addresses")
	}
	for _, address := range addresses {
		if !publicIP(address.IP) {
			return fmt.Errorf("careers host %q resolves to a non-public address", host)
		}
	}
	return nil
}

func (r *Resolver) lookup(ctx context.Context, host string) ([]net.IPAddr, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IPAddr{{IP: ip}}, nil
	}
	ascii, err := idna.Lookup.ToASCII(host)
	if err != nil {
		return nil, err
	}
	return r.resolver.LookupIPAddr(ctx, ascii)
}

func (r *Resolver) dialPublic(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	addresses, err := r.lookup(ctx, host)
	if err != nil {
		return nil, err
	}
	for _, address := range addresses {
		if !publicIP(address.IP) {
			return nil, fmt.Errorf("host %q resolves to a non-public address", host)
		}
	}
	dialer := &net.Dialer{Timeout: 8 * time.Second, KeepAlive: 30 * time.Second}
	var lastErr error
	for _, address := range addresses {
		connection, err := dialer.DialContext(ctx, network, net.JoinHostPort(address.IP.String(), port))
		if err == nil {
			return connection, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("host has no public address")
	}
	return nil, lastErr
}

func publicIP(ip net.IP) bool {
	if ip == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		reserved := []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4"}
		for _, cidr := range reserved {
			_, network, _ := net.ParseCIDR(cidr)
			if network.Contains(ipv4) {
				return false
			}
		}
		return true
	}
	reserved := []string{"2001:db8::/32", "2001::/23", "2002::/16"}
	for _, cidr := range reserved {
		_, network, _ := net.ParseCIDR(cidr)
		if network.Contains(ip) {
			return false
		}
	}
	return true
}

func extractLinks(body []byte, page *url.URL) []link {
	if page == nil {
		return nil
	}
	root, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil
	}
	base := page
	var links []link
	var visit func(*html.Node)
	visit = func(node *html.Node) {
		if len(links) >= 500 {
			return
		}
		if node.Type == html.ElementNode {
			label := ""
			if node.Data == "a" || node.Data == "iframe" || node.Data == "script" || node.Data == "form" {
				label = strings.TrimSpace(nodeText(node))
			}
			for _, attribute := range node.Attr {
				if attribute.Key == "href" || attribute.Key == "src" || attribute.Key == "action" || strings.HasPrefix(attribute.Key, "data-") {
					value := strings.TrimSpace(attribute.Val)
					if node.Data == "base" && attribute.Key == "href" {
						if parsed, err := page.Parse(value); err == nil && parsed.IsAbs() {
							base = parsed
						}
					}
					if resolved, err := base.Parse(value); err == nil && resolved.IsAbs() && (resolved.Scheme == "http" || resolved.Scheme == "https") {
						links = append(links, link{label: label, href: resolved.String()})
						if len(links) >= 500 {
							return
						}
					}
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(root)
	for _, match := range embeddedURL.FindAll(body, 200) {
		value := strings.TrimRight(string(match), ".,;:!?) ]}'\"")
		if strings.HasPrefix(value, "//") {
			value = page.Scheme + ":" + value
		}
		if parsed, err := url.Parse(value); err == nil && parsed.IsAbs() {
			links = append(links, link{href: parsed.String()})
		}
	}
	return links
}

func nodeText(node *html.Node) string {
	if node.Type == html.TextNode {
		return node.Data
	}
	var parts []string
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		parts = append(parts, nodeText(child))
	}
	return strings.Join(parts, " ")
}

func careerLinks(links []link, companyDomain string) []link {
	seen := map[string]bool{}
	result := make([]link, 0)
	for _, item := range links {
		parsed, err := url.Parse(item.href)
		if err != nil || parsed.Hostname() == "" {
			continue
		}
		host := strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
		if companyDomain != "" && host != companyDomain && !strings.HasSuffix(host, "."+companyDomain) {
			// ATS-hosted links are useful at this stage; unrelated outbound links are not.
			if DetectURL(parsed) == nil {
				continue
			}
		}
		signal := strings.ToLower(item.label + " " + parsed.Path)
		if DetectURL(parsed) == nil && !strings.Contains(signal, "career") && !strings.Contains(signal, "job") && !strings.Contains(signal, "opening") && !strings.Contains(signal, "position") && !strings.Contains(signal, "vacanc") && !strings.Contains(signal, "join us") && !strings.Contains(signal, "work with us") {
			continue
		}
		if !seen[item.href] {
			seen[item.href] = true
			result = append(result, item)
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		iCareer := strings.Contains(strings.ToLower(result[i].label), "career")
		jCareer := strings.Contains(strings.ToLower(result[j].label), "career")
		return iCareer && !jCareer
	})
	return result
}

func Detect(page *url.URL, body []byte) *domain.DetectedProvider {
	if page == nil {
		return nil
	}
	candidates := make([]string, 0, 202)
	if page != nil {
		candidates = append(candidates, page.String())
	}
	for _, item := range extractLinks(body, page) {
		candidates = append(candidates, item.href)
	}
	for _, candidate := range candidates {
		parsed, err := url.Parse(candidate)
		if err != nil || parsed.Hostname() == "" {
			continue
		}
		if detected := DetectURL(parsed); detected != nil {
			return detected
		}
	}
	return nil
}

func DetectURL(parsed *url.URL) *domain.DetectedProvider {
	if parsed == nil {
		return nil
	}
	host := strings.ToLower(strings.TrimPrefix(parsed.Hostname(), "www."))
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	key := ""
	provider := ""
	switch {
	case host == "boards.greenhouse.io" || host == "job-boards.greenhouse.io":
		provider = "greenhouse"
		if len(parts) == 1 && parts[0] != "embed" || len(parts) == 2 && parts[0] != "embed" && parts[1] == "jobs" {
			key = parts[0]
		} else if len(parts) >= 2 && parts[0] == "embed" {
			key = parsed.Query().Get("for")
		}
	case host == "greenhouse.io" && strings.Contains(parsed.Path, "/embed/job_board"):
		provider, key = "greenhouse", parsed.Query().Get("for")
	case host == "api.greenhouse.io" && len(parts) >= 3 && parts[0] == "v1" && parts[1] == "boards":
		provider, key = "greenhouse", parts[2]
	case host == "jobs.lever.co" || host == "jobs.eu.lever.co":
		provider = "lever"
		if len(parts) == 1 {
			key = parts[0]
		}
	case host == "api.lever.co" && len(parts) >= 3 && parts[0] == "v0" && parts[1] == "postings":
		provider, key = "lever", parts[2]
	case host == "jobs.ashbyhq.com":
		provider = "ashby"
		if len(parts) == 1 {
			key = parts[0]
		}
	case host == "api.ashbyhq.com" && strings.Contains(parsed.Path, "/posting-api/job-board/"):
		provider = "ashby"
		index := strings.Index(parsed.Path, "/posting-api/job-board/") + len("/posting-api/job-board/")
		if index <= len(parsed.Path) {
			key = strings.Split(strings.Trim(parsed.Path[index:], "/"), "/")[0]
		}
	case host == "jobs.smartrecruiters.com" || host == "careers.smartrecruiters.com":
		provider = "smartrecruiters"
		if len(parts) > 0 {
			key = parts[0]
		}
	case host == "apply.workable.com" || strings.HasSuffix(host, ".workable.com"):
		provider = "workable"
		if host == "apply.workable.com" && (len(parts) == 0 || parts[0] == "j") {
			provider = ""
		} else if len(parts) > 0 {
			key = parts[0]
		}
	case strings.HasSuffix(host, ".teamtailor.com") || strings.HasSuffix(host, ".teamtailor.site"):
		provider, key = "teamtailor", strings.Split(host, ".")[0]
	case strings.HasSuffix(host, ".recruitee.com"):
		provider, key = "recruitee", strings.Split(host, ".")[0]
	case strings.HasSuffix(host, ".jobs.personio.com"):
		provider, key = "personio", strings.TrimSuffix(host, ".jobs.personio.com")
	case host == "myworkdayjobs.com" || strings.HasSuffix(host, ".myworkdayjobs.com"):
		provider, key = "workday", host
		if len(parts) > 0 {
			key += "/" + parts[0]
		}
	case host == "jobs.jobvite.com":
		provider = "jobvite"
		if len(parts) > 0 {
			key = parts[0]
		}
	case strings.Contains(host, ".icims.com") || strings.HasSuffix(host, ".icims.com"):
		provider, key = "icims", host
	}
	key = strings.TrimSpace(key)
	if provider == "" || key == "" || !validProviderKey(key) {
		return nil
	}
	u := *parsed
	return &domain.DetectedProvider{Type: provider, Key: key, URL: u.String(), Confidence: 0.99, Evidence: []string{"ATS host " + host}}
}

func validProviderKey(key string) bool {
	if len(key) > 200 || strings.Contains(key, "..") || strings.ContainsAny(key, "\\\"'<> ") {
		return false
	}
	for _, r := range key {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("-_.:/", r)) {
			return false
		}
	}
	return true
}
