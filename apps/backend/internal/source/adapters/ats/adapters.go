package ats

import (
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

type Registry struct{ client *http.Client }

func NewRegistry() *Registry { return &Registry{client: &http.Client{Timeout: 30 * time.Second}} }

func (r *Registry) Fetch(ctx context.Context, source domain.Source) (domain.FetchResult, error) {
	switch source.Type {
	case domain.Greenhouse:
		return r.fetchGreenhouse(ctx, source)
	case domain.Lever:
		return r.fetchLever(ctx, source)
	case domain.Ashby:
		return r.fetchAshby(ctx, source)
	default:
		return domain.FetchResult{}, fmt.Errorf("unsupported source type %q", source.Type)
	}
}

type config struct {
	Board string `json:"board"`
	Limit int    `json:"page_size"`
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

type greenhouseResponse struct {
	Jobs []struct {
		ID       int64  `json:"id"`
		Title    string `json:"title"`
		Content  string `json:"content"`
		URL      string `json:"absolute_url"`
		Updated  string `json:"updated_at"`
		Location struct {
			Name string `json:"name"`
		} `json:"location"`
	} `json:"jobs"`
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
	var response greenhouseResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return domain.FetchResult{}, fmt.Errorf("decode Greenhouse response: %w", err)
	}
	result := domain.FetchResult{Jobs: make([]domain.ExternalJob, 0, len(response.Jobs))}
	for _, item := range response.Jobs {
		published := parseTime(item.Updated)
		result.Jobs = append(result.Jobs, domain.ExternalJob{ExternalID: strconv.FormatInt(item.ID, 10), CompanyName: source.CompanyName, Title: item.Title, Description: item.Content, Location: item.Location.Name, ApplyURL: item.URL, PublishedAt: published, Raw: rawFor(body, item.ID)})
	}
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
		if err := json.Unmarshal(body, &rawItems); err != nil {
			return domain.FetchResult{}, fmt.Errorf("decode Lever response: %w", err)
		}
		items := make([]leverItem, 0, len(rawItems))
		for _, raw := range rawItems {
			var item leverItem
			if err := json.Unmarshal(raw, &item); err != nil {
				return domain.FetchResult{}, fmt.Errorf("decode Lever posting: %w", err)
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
		Jobs []json.RawMessage `json:"jobs"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return domain.FetchResult{}, fmt.Errorf("decode Ashby response: %w", err)
	}
	result := domain.FetchResult{Jobs: make([]domain.ExternalJob, 0, len(response.Jobs))}
	for _, raw := range response.Jobs {
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
			return domain.FetchResult{}, err
		}
		result.Jobs = append(result.Jobs, domain.ExternalJob{ExternalID: item.ID, CompanyName: source.CompanyName, Title: item.Title, Description: item.Description, Location: item.Location, EmploymentType: item.EmploymentType, ApplyURL: item.URL, PublishedAt: parseTime(item.Published), Raw: raw})
	}
	return result, nil
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
func rawFor(body []byte, id int64) json.RawMessage {
	var doc map[string]json.RawMessage
	if json.Unmarshal(body, &doc) != nil {
		return json.RawMessage(body)
	}
	var jobs []json.RawMessage
	_ = json.Unmarshal(doc["jobs"], &jobs)
	for _, job := range jobs {
		var item struct {
			ID int64 `json:"id"`
		}
		if json.Unmarshal(job, &item) == nil && item.ID == id {
			return job
		}
	}
	return json.RawMessage(body)
}
