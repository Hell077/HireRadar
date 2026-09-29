package ats

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/source/domain"
)

// These public feeds are rolling windows, not complete snapshots. Their jobs
// must never be closed just because an older listing fell outside the window.
func (r *Registry) fetchRemoteOK(ctx context.Context, source domain.Source) (domain.FetchResult, error) {
	body, err := r.get(ctx, "https://remoteok.com/api")
	if err != nil {
		return domain.FetchResult{}, err
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(body, &rows); err != nil {
		return domain.FetchResult{}, fmt.Errorf("decode RemoteOK response: %w", err)
	}
	result := domain.FetchResult{Jobs: []domain.ExternalJob{}}
	for _, raw := range rows {
		var item struct {
			ID          json.RawMessage `json:"id"`
			Position    string          `json:"position"`
			Company     string          `json:"company"`
			URL         string          `json:"url"`
			ApplyURL    string          `json:"apply_url"`
			Description string          `json:"description"`
			Location    string          `json:"location"`
			Date        string          `json:"date"`
		}
		if err := json.Unmarshal(raw, &item); err != nil {
			return domain.FetchResult{}, fmt.Errorf("decode RemoteOK job: %w", err)
		}
		if strings.TrimSpace(item.Position) == "" { // The first array entry is feed metadata.
			continue
		}
		id := strings.Trim(strings.TrimSpace(string(item.ID)), `"`)
		applyURL := strings.TrimSpace(item.URL)
		if applyURL == "" {
			applyURL = strings.TrimSpace(item.ApplyURL)
		}
		if id == "" || applyURL == "" || strings.TrimSpace(item.Company) == "" {
			return domain.FetchResult{}, errors.New("decode RemoteOK job: id, company, or url is missing")
		}
		result.Jobs = append(result.Jobs, domain.ExternalJob{ExternalID: id, CompanyName: item.Company, Title: item.Position, Description: item.Description, Location: item.Location, ApplyURL: applyURL, PublishedAt: parseFlexibleTime(item.Date), Raw: raw})
	}
	return result, nil
}

func (r *Registry) fetchJobicy(ctx context.Context, source domain.Source) (domain.FetchResult, error) {
	body, err := r.get(ctx, "https://jobicy.com/api/v2/remote-jobs?count=200")
	if err != nil {
		return domain.FetchResult{}, err
	}
	var response struct {
		Jobs []json.RawMessage `json:"jobs"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return domain.FetchResult{}, fmt.Errorf("decode Jobicy response: %w", err)
	}
	if response.Jobs == nil {
		return domain.FetchResult{}, errors.New("decode Jobicy response: jobs array is missing")
	}
	result := domain.FetchResult{Jobs: make([]domain.ExternalJob, 0, len(response.Jobs))}
	for _, raw := range response.Jobs {
		var item struct {
			ID             json.RawMessage `json:"id"`
			Title          string          `json:"jobTitle"`
			Company        string          `json:"companyName"`
			URL            string          `json:"url"`
			Description    string          `json:"jobDescription"`
			Location       string          `json:"jobGeo"`
			EmploymentType json.RawMessage `json:"jobType"`
			Published      string          `json:"pubDate"`
		}
		if err := json.Unmarshal(raw, &item); err != nil {
			return domain.FetchResult{}, fmt.Errorf("decode Jobicy job: %w", err)
		}
		id := strings.Trim(strings.TrimSpace(string(item.ID)), `"`)
		if id == "" || strings.TrimSpace(item.Title) == "" || strings.TrimSpace(item.Company) == "" || strings.TrimSpace(item.URL) == "" {
			return domain.FetchResult{}, errors.New("decode Jobicy job: id, title, company, or url is missing")
		}
		var employment string
		if err := json.Unmarshal(item.EmploymentType, &employment); err != nil {
			var values []string
			if arrayErr := json.Unmarshal(item.EmploymentType, &values); arrayErr == nil {
				employment = strings.Join(values, ", ")
			}
		}
		result.Jobs = append(result.Jobs, domain.ExternalJob{ExternalID: id, CompanyName: item.Company, Title: item.Title, Description: item.Description, Location: item.Location, EmploymentType: employment, ApplyURL: item.URL, PublishedAt: parseFlexibleTime(item.Published), Raw: raw})
	}
	return result, nil
}

type remoteRSS struct {
	Items []struct {
		Title       string `xml:"title"`
		Link        string `xml:"link"`
		GUID        string `xml:"guid"`
		Description string `xml:"description"`
		Region      string `xml:"region"`
		Country     string `xml:"country"`
		Category    string `xml:"category"`
		JobType     string `xml:"type"`
		PubDate     string `xml:"pubDate"`
	} `xml:"channel>item"`
}

func (r *Registry) fetchWeWorkRemotely(ctx context.Context, source domain.Source) (domain.FetchResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://weworkremotely.com/remote-jobs.rss", nil)
	if err != nil {
		return domain.FetchResult{}, err
	}
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml")
	req.Header.Set("User-Agent", "HireRadar/1.0 (+https://github.com/Hell077/HireRadar)")
	resp, err := r.client.Do(req)
	if err != nil {
		return domain.FetchResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return domain.FetchResult{}, fmt.Errorf("We Work Remotely returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return domain.FetchResult{}, err
	}
	if len(body) > maxResponseBytes {
		return domain.FetchResult{}, errors.New("We Work Remotely response exceeds 32 MiB")
	}
	var feed remoteRSS
	if err := xml.Unmarshal(body, &feed); err != nil {
		return domain.FetchResult{}, fmt.Errorf("decode We Work Remotely feed: %w", err)
	}
	if feed.Items == nil {
		return domain.FetchResult{}, errors.New("decode We Work Remotely feed: channel items are missing")
	}
	result := domain.FetchResult{Jobs: make([]domain.ExternalJob, 0, len(feed.Items))}
	for _, item := range feed.Items {
		id := strings.TrimSpace(item.GUID)
		if id == "" {
			id = strings.TrimSpace(item.Link)
		}
		company, title := splitRSSJobTitle(item.Title)
		if id == "" || title == "" || company == "" || strings.TrimSpace(item.Link) == "" {
			return domain.FetchResult{}, errors.New("decode We Work Remotely job: guid, company/title, or link is missing")
		}
		location := strings.TrimSpace(strings.Join(nonEmpty(item.Region, item.Country), ", "))
		result.Jobs = append(result.Jobs, domain.ExternalJob{ExternalID: id, CompanyName: company, Title: title, Description: item.Description, Location: location, EmploymentType: item.JobType, ApplyURL: item.Link, PublishedAt: parseFlexibleTime(item.PubDate), Raw: json.RawMessage(mustJSON(item))})
	}
	return result, nil
}

func splitRSSJobTitle(value string) (company, title string) {
	parts := strings.SplitN(strings.TrimSpace(value), ":", 2)
	if len(parts) != 2 {
		return "", strings.TrimSpace(value)
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
}

func parseFlexibleTime(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, time.RFC1123Z, time.RFC1123, "2006-01-02"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			parsed = parsed.UTC()
			return &parsed
		}
	}
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds > 0 {
		parsed := time.Unix(seconds, 0).UTC()
		return &parsed
	}
	return nil
}

func nonEmpty(values ...string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func mustJSON(value any) []byte {
	data, _ := json.Marshal(value)
	return data
}
