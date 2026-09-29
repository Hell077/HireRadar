package ats

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/source/domain"
)

const maxResponseBytes = 32 << 20

type Fetcher interface {
	Fetch(context.Context, domain.Source) (domain.FetchResult, error)
}

type Registry struct {
	client      *http.Client
	githubToken string
}

func NewRegistry() *Registry { return NewRegistryWithGitHubToken("") }

func NewRegistryWithGitHubToken(token string) *Registry {
	return &Registry{client: &http.Client{Timeout: 30 * time.Second}, githubToken: strings.TrimSpace(token)}
}

func (r *Registry) Fetch(ctx context.Context, source domain.Source) (domain.FetchResult, error) {
	switch source.Type {
	case domain.Greenhouse:
		return r.fetchGreenhouse(ctx, source)
	case domain.Lever:
		return r.fetchLever(ctx, source)
	case domain.Ashby:
		return r.fetchAshby(ctx, source)
	case domain.GitHub:
		return r.fetchGitHub(ctx, source)
	case domain.RemoteOK:
		return r.fetchRemoteOK(ctx, source)
	case domain.Jobicy:
		return r.fetchJobicy(ctx, source)
	case domain.WeWorkRemotely:
		return r.fetchWeWorkRemotely(ctx, source)
	default:
		return domain.FetchResult{}, fmt.Errorf("unsupported source type %q", source.Type)
	}
}

type config struct {
	Board string `json:"board"`
	Limit int    `json:"page_size"`
}

type githubConfig struct {
	Owner  string   `json:"owner"`
	Repo   string   `json:"repo"`
	Labels []string `json:"labels"`
	Limit  int      `json:"page_size"`
}

func readConfig(source domain.Source) (config, error) {
	var value config
	if err := json.Unmarshal(source.Config, &value); err != nil {
		return value, fmt.Errorf("decode source config: %w", err)
	}
	value.Board = strings.TrimSpace(value.Board)
	if value.Board == "" || strings.Contains(value.Board, "/") || strings.Contains(value.Board, "..") {
		return value, domain.ErrInvalidSource
	}
	if value.Limit < 1 || value.Limit > 100 {
		value.Limit = 100
	}
	return value, nil
}

func (r *Registry) get(ctx context.Context, endpoint string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "HireRadar/1.0 (+https://github.com/Hell077/HireRadar)")
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("ATS returned HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxResponseBytes {
		return nil, errors.New("ATS response exceeds 32 MiB")
	}
	return data, nil
}

func (r *Registry) fetchGreenhouse(ctx context.Context, source domain.Source) (domain.FetchResult, error) {
	cfg, err := readConfig(source)
	if err != nil {
		return domain.FetchResult{}, err
	}
	u := "https://boards-api.greenhouse.io/v1/boards/" + url.PathEscape(cfg.Board) + "/jobs?content=true"
	body, err := r.get(ctx, u)
	if err != nil {
		return domain.FetchResult{}, err
	}
	var response struct {
		Jobs *[]json.RawMessage `json:"jobs"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return domain.FetchResult{}, fmt.Errorf("decode Greenhouse response: %w", err)
	}
	if response.Jobs == nil {
		return domain.FetchResult{}, errors.New("decode Greenhouse response: jobs array is missing")
	}
	result := domain.FetchResult{Jobs: make([]domain.ExternalJob, 0, len(*response.Jobs))}
	for _, raw := range *response.Jobs {
		var item struct {
			ID       int64  `json:"id"`
			Title    string `json:"title"`
			Content  string `json:"content"`
			URL      string `json:"absolute_url"`
			Updated  string `json:"updated_at"`
			Location struct {
				Name string `json:"name"`
			} `json:"location"`
		}
		if err := json.Unmarshal(raw, &item); err != nil {
			return domain.FetchResult{}, fmt.Errorf("decode Greenhouse job: %w", err)
		}
		if item.ID < 1 || strings.TrimSpace(item.Title) == "" || strings.TrimSpace(item.URL) == "" {
			return domain.FetchResult{}, errors.New("decode Greenhouse job: id, title, or absolute_url is missing")
		}
		published := parseTime(item.Updated)
		result.Jobs = append(result.Jobs, domain.ExternalJob{ExternalID: strconv.FormatInt(item.ID, 10), CompanyName: source.CompanyName, Title: item.Title, Description: item.Content, Location: item.Location.Name, ApplyURL: item.URL, PublishedAt: published, Raw: raw})
	}
	result.AuthoritativeSnapshot = true
	return result, nil
}

type leverItem struct {
	ID          string `json:"id"`
	Text        string `json:"text"`
	Description string `json:"descriptionPlain"`
	URL         string `json:"hostedUrl"`
	Created     int64  `json:"createdAt"`
	Categories  struct {
		Location   string `json:"location"`
		Commitment string `json:"commitment"`
	} `json:"categories"`
	Raw json.RawMessage `json:"-"`
}

func (r *Registry) fetchLever(ctx context.Context, source domain.Source) (domain.FetchResult, error) {
	cfg, err := readConfig(source)
	if err != nil {
		return domain.FetchResult{}, err
	}
	const maxPages = 1000
	result := domain.FetchResult{Jobs: []domain.ExternalJob{}}
	for offset := 0; offset < maxPages*cfg.Limit; offset += cfg.Limit {
		endpoint := fmt.Sprintf("https://api.lever.co/v0/postings/%s?mode=json&skip=%d&limit=%d", url.PathEscape(cfg.Board), offset, cfg.Limit)
		body, err := r.get(ctx, endpoint)
		if err != nil {
			return domain.FetchResult{}, err
		}
		var rawItems []json.RawMessage
		if len(bytes.TrimSpace(body)) == 0 || bytes.TrimSpace(body)[0] != '[' {
			return domain.FetchResult{}, errors.New("decode Lever response: expected postings array")
		}
		if err := json.Unmarshal(body, &rawItems); err != nil {
			return domain.FetchResult{}, fmt.Errorf("decode Lever response: %w", err)
		}
		items := make([]leverItem, 0, len(rawItems))
		for _, raw := range rawItems {
			var item leverItem
			if err := json.Unmarshal(raw, &item); err != nil {
				return domain.FetchResult{}, fmt.Errorf("decode Lever posting: %w", err)
			}
			if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.Text) == "" || strings.TrimSpace(item.URL) == "" {
				return domain.FetchResult{}, errors.New("decode Lever posting: id, text, or hostedUrl is missing")
			}
			item.Raw = raw
			items = append(items, item)
		}
		for _, item := range items {
			var published *time.Time
			if item.Created > 0 {
				t := time.UnixMilli(item.Created).UTC()
				published = &t
			}
			result.Jobs = append(result.Jobs, domain.ExternalJob{ExternalID: item.ID, CompanyName: source.CompanyName, Title: item.Text, Description: item.Description, Location: item.Categories.Location, EmploymentType: item.Categories.Commitment, ApplyURL: item.URL, PublishedAt: published, Raw: item.Raw})
		}
		if len(items) < cfg.Limit {
			result.AuthoritativeSnapshot = true
			return result, nil
		}
	}
	return domain.FetchResult{}, errors.New("Lever pagination exceeded 1000 pages")
}

type ashbyResponse struct {
	Jobs []struct {
		ID             string `json:"id"`
		Title          string `json:"title"`
		Description    string `json:"descriptionPlain"`
		URL            string `json:"jobUrl"`
		Location       string `json:"location"`
		EmploymentType string `json:"employmentType"`
		Published      string `json:"publishedAt"`
	} `json:"jobs"`
}

func (r *Registry) fetchAshby(ctx context.Context, source domain.Source) (domain.FetchResult, error) {
	cfg, err := readConfig(source)
	if err != nil {
		return domain.FetchResult{}, err
	}
	endpoint := "https://api.ashbyhq.com/posting-api/job-board/" + url.PathEscape(cfg.Board) + "?includeCompensation=true"
	body, err := r.get(ctx, endpoint)
	if err != nil {
		return domain.FetchResult{}, err
	}
	var response struct {
		Jobs *[]json.RawMessage `json:"jobs"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return domain.FetchResult{}, fmt.Errorf("decode Ashby response: %w", err)
	}
	if response.Jobs == nil {
		return domain.FetchResult{}, errors.New("decode Ashby response: jobs array is missing")
	}
	result := domain.FetchResult{Jobs: make([]domain.ExternalJob, 0, len(*response.Jobs))}
	for _, raw := range *response.Jobs {
		var item struct {
			ID             string `json:"id"`
			Title          string `json:"title"`
			Description    string `json:"descriptionPlain"`
			URL            string `json:"jobUrl"`
			Location       string `json:"location"`
			EmploymentType string `json:"employmentType"`
			Published      string `json:"publishedAt"`
		}
		if err := json.Unmarshal(raw, &item); err != nil {
			return domain.FetchResult{}, fmt.Errorf("decode Ashby job: %w", err)
		}
		if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.Title) == "" || strings.TrimSpace(item.URL) == "" {
			return domain.FetchResult{}, errors.New("decode Ashby job: id, title, or jobUrl is missing")
		}
		result.Jobs = append(result.Jobs, domain.ExternalJob{ExternalID: item.ID, CompanyName: source.CompanyName, Title: item.Title, Description: item.Description, Location: item.Location, EmploymentType: item.EmploymentType, ApplyURL: item.URL, PublishedAt: parseTime(item.Published), Raw: raw})
	}
	result.AuthoritativeSnapshot = true
	return result, nil
}

type githubIssue struct {
	Number      int64           `json:"number"`
	Title       string          `json:"title"`
	Body        string          `json:"body"`
	URL         string          `json:"html_url"`
	Created     string          `json:"created_at"`
	PullRequest json.RawMessage `json:"pull_request"`
}

// IsJobPosting rejects ordinary repository discussion and software bugs from
// generic GitHub Issues feeds while accepting common hiring/listing formats.
func IsJobPosting(title, body string) bool {
	titleLower := strings.ToLower(strings.TrimSpace(title))
	text := strings.ToLower(title + "\n" + body)
	for _, phrase := range []string{"hiring:", "we're hiring", "we are hiring", "job opening", "job posting", "job opportunity", "open position", "vacancy", "apply now", "looking for a", "seeking a", "contract opportunity", "internship opportunity"} {
		if strings.Contains(titleLower, phrase) {
			return true
		}
	}
	hasRole := false
	for _, role := range []string{"engineer", "developer", "designer", "analyst", "product manager", "technical writer", "devops", "intern", "recruiter", "researcher", "architect"} {
		if strings.Contains(titleLower, role) {
			hasRole = true
			break
		}
	}
	if !hasRole {
		return false
	}
	for _, context := range []string{"apply", "application", "requirements", "responsibilities", "compensation", "salary", "location", "remote", "experience", "qualifications", "resume", "cv"} {
		if strings.Contains(text, context) {
			return true
		}
	}
	return false
}

func (r *Registry) fetchGitHub(ctx context.Context, source domain.Source) (domain.FetchResult, error) {
	var cfg githubConfig
	if err := json.Unmarshal(source.Config, &cfg); err != nil {
		return domain.FetchResult{}, fmt.Errorf("decode GitHub source config: %w", err)
	}
	cfg.Owner = strings.TrimSpace(cfg.Owner)
	cfg.Repo = strings.TrimSpace(cfg.Repo)
	if cfg.Owner == "" || cfg.Repo == "" || strings.ContainsAny(cfg.Owner, "/\\") || strings.ContainsAny(cfg.Repo, "/\\") || cfg.Owner == "." || cfg.Owner == ".." || cfg.Repo == "." || cfg.Repo == ".." {
		return domain.FetchResult{}, domain.ErrInvalidSource
	}
	if cfg.Limit < 1 || cfg.Limit > 100 {
		cfg.Limit = 100
	}
	const maxPages = 100
	result := domain.FetchResult{Jobs: []domain.ExternalJob{}}
	for page := 1; page <= maxPages; page++ {
		query := url.Values{"state": {"open"}, "per_page": {strconv.Itoa(cfg.Limit)}, "page": {strconv.Itoa(page)}}
		if len(cfg.Labels) > 0 {
			labels := make([]string, 0, len(cfg.Labels))
			for _, label := range cfg.Labels {
				if value := strings.TrimSpace(label); value != "" {
					labels = append(labels, value)
				}
			}
			if len(labels) > 0 {
				query.Set("labels", strings.Join(labels, ","))
			}
		}
		endpoint := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues?%s", url.PathEscape(cfg.Owner), url.PathEscape(cfg.Repo), query.Encode())
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return domain.FetchResult{}, err
		}
		request.Header.Set("Accept", "application/vnd.github+json")
		request.Header.Set("X-GitHub-Api-Version", "2026-03-10")
		request.Header.Set("User-Agent", "HireRadar-SourceWorker")
		if r.githubToken != "" {
			request.Header.Set("Authorization", "Bearer "+r.githubToken)
		}
		resp, err := r.client.Do(request)
		if err != nil {
			return domain.FetchResult{}, err
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
		_ = resp.Body.Close()
		if readErr != nil {
			return domain.FetchResult{}, readErr
		}
		if len(body) > maxResponseBytes {
			return domain.FetchResult{}, errors.New("GitHub response exceeds 32 MiB")
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return domain.FetchResult{}, fmt.Errorf("GitHub returned HTTP %d", resp.StatusCode)
		}
		var rows []json.RawMessage
		if len(bytes.TrimSpace(body)) == 0 || bytes.TrimSpace(body)[0] != '[' {
			return domain.FetchResult{}, errors.New("decode GitHub issues: expected issues array")
		}
		if err := json.Unmarshal(body, &rows); err != nil {
			return domain.FetchResult{}, fmt.Errorf("decode GitHub issues: %w", err)
		}
		for _, raw := range rows {
			var issue githubIssue
			if err := json.Unmarshal(raw, &issue); err != nil {
				return domain.FetchResult{}, err
			}
			if len(issue.PullRequest) > 0 || issue.Number < 1 {
				continue
			}
			if strings.TrimSpace(issue.Title) == "" || strings.TrimSpace(issue.URL) == "" {
				return domain.FetchResult{}, errors.New("decode GitHub issue: title or html_url is missing")
			}
			if !IsJobPosting(issue.Title, issue.Body) {
				continue
			}
			result.Jobs = append(result.Jobs, domain.ExternalJob{ExternalID: strconv.FormatInt(issue.Number, 10), CompanyName: source.CompanyName, Title: issue.Title, Description: issue.Body, ApplyURL: issue.URL, PublishedAt: parseTime(issue.Created), Raw: raw})
		}
		if len(rows) < cfg.Limit {
			result.NextCursor, _ = json.Marshal(struct {
				LastPage int `json:"last_page"`
			}{page})
			result.AuthoritativeSnapshot = true
			return result, nil
		}
	}
	return domain.FetchResult{}, errors.New("GitHub issue pagination exceeded 100 pages")
}

func parseTime(value string) *time.Time {
	if value == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil
	}
	t = t.UTC()
	return &t
}
