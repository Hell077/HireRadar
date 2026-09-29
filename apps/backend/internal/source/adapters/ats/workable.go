package ats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/Hell077/HireRadar/apps/backend/internal/source/domain"
)

// Workable exposes a public JSON feed for published careers-page listings.
// Workable's private SPI endpoints require employer credentials and are not used.
func (r *Registry) fetchWorkable(ctx context.Context, source domain.Source) (domain.FetchResult, error) {
	cfg, err := readConfig(source)
	if err != nil {
		return domain.FetchResult{}, err
	}
	body, err := r.get(ctx, "https://www.workable.com/api/accounts/"+url.PathEscape(cfg.Board)+"?details=true")
	if err != nil {
		return domain.FetchResult{}, err
	}
	var response struct {
		Jobs []json.RawMessage `json:"jobs"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return domain.FetchResult{}, fmt.Errorf("decode Workable response: %w", err)
	}
	if response.Jobs == nil {
		return domain.FetchResult{}, errors.New("decode Workable response: jobs array is missing")
	}
	result := domain.FetchResult{Jobs: make([]domain.ExternalJob, 0, len(response.Jobs)), Mode: domain.SyncSnapshot}
	for _, raw := range response.Jobs {
		var item struct {
			Title         string `json:"title"`
			Shortcode     string `json:"shortcode"`
			URL           string `json:"url"`
			ApplyURL      string `json:"application_url"`
			Description   string `json:"description"`
			Employment    string `json:"employment_type"`
			Published     string `json:"published_on"`
			Country       string `json:"country"`
			City          string `json:"city"`
			State         string `json:"state"`
			Telecommuting bool   `json:"telecommuting"`
			WorkplaceType string `json:"workplace_type"`
			Salary        *struct {
				Minimum  float64 `json:"salary_from"`
				Maximum  float64 `json:"salary_to"`
				Currency string  `json:"salary_currency"`
			} `json:"salary"`
			Locations []struct {
				CountryCode string `json:"countryCode"`
			} `json:"locations"`
		}
		if err := json.Unmarshal(raw, &item); err != nil {
			return domain.FetchResult{}, fmt.Errorf("decode Workable job: %w", err)
		}
		applyURL := strings.TrimSpace(item.ApplyURL)
		if applyURL == "" {
			applyURL = strings.TrimSpace(item.URL)
		}
		if item.Shortcode == "" || strings.TrimSpace(item.Title) == "" || applyURL == "" {
			return domain.FetchResult{}, errors.New("decode Workable job: shortcode, title, or application URL is missing")
		}
		location := strings.Join(nonEmpty(item.City, item.State, item.Country), ", ")
		remote := item.Telecommuting || strings.EqualFold(item.WorkplaceType, "remote")
		if remote {
			location = "Remote — " + location
		}
		countries := make([]string, 0, len(item.Locations)+1)
		for _, entry := range item.Locations {
			if code := strings.ToUpper(strings.TrimSpace(entry.CountryCode)); code != "" {
				countries = append(countries, code)
			}
		}
		if len(countries) == 0 && len(item.Country) == 2 {
			countries = append(countries, strings.ToUpper(item.Country))
		}
		var salary *domain.ExternalSalary
		if item.Salary != nil {
			salary = &domain.ExternalSalary{Minimum: item.Salary.Minimum, Maximum: item.Salary.Maximum, Currency: item.Salary.Currency}
		}
		result.Jobs = append(result.Jobs, domain.ExternalJob{
			ExternalID: item.Shortcode, CompanyName: source.CompanyName, Title: item.Title,
			Description: item.Description, Location: location, EmploymentType: item.Employment,
			Remote: remote, Countries: countries, Salary: salary, ApplyURL: applyURL,
			PublishedAt: parseFlexibleTime(item.Published), Raw: raw,
		})
	}
	return result, nil
}
