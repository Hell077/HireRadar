package domain

import (
	"errors"
	"strings"
)

var ErrInvalidPreferences = errors.New("invalid job preferences")

type Preferences struct {
	RemotePolicies       []string `json:"remote_policies"`
	EmploymentTypes      []string `json:"employment_types"`
	AllowedRegions       []string `json:"allowed_regions"`
	ExcludedCountries    []string `json:"excluded_countries"`
	MinimumSalary        *Money   `json:"minimum_salary,omitempty"`
	MinimumMatchScore    int      `json:"minimum_match_score"`
	MaximumJobAgeDays    int      `json:"maximum_job_age_days"`
	NotificationsEnabled bool     `json:"notifications_enabled"`
}

func (p Preferences) Normalize() (Preferences, error) {
	if p.MinimumMatchScore < 0 || p.MinimumMatchScore > 100 {
		return Preferences{}, ErrInvalidPreferences
	}
	if p.MaximumJobAgeDays < 1 || p.MaximumJobAgeDays > 365 {
		return Preferences{}, ErrInvalidPreferences
	}
	var err error
	if p.RemotePolicies, err = normalizeSet(p.RemotePolicies, map[string]bool{"worldwide": true, "remote": true, "remote_region": true, "remote_country": true, "hybrid": true, "onsite": true, "unknown": true}, false); err != nil {
		return Preferences{}, ErrInvalidPreferences
	}
	if p.EmploymentTypes, err = normalizeSet(p.EmploymentTypes, map[string]bool{"full_time": true, "part_time": true, "contract": true, "freelance": true, "b2b": true, "internship": true}, false); err != nil {
		return Preferences{}, ErrInvalidPreferences
	}
	if p.AllowedRegions, err = normalizeSet(p.AllowedRegions, nil, false); err != nil {
		return Preferences{}, ErrInvalidPreferences
	}
	if p.ExcludedCountries, err = normalizeSet(p.ExcludedCountries, nil, true); err != nil {
		return Preferences{}, ErrInvalidPreferences
	}
	if p.MinimumSalary != nil {
		p.MinimumSalary.Currency = strings.ToUpper(strings.TrimSpace(p.MinimumSalary.Currency))
		if p.MinimumSalary.Amount < 0 || len(p.MinimumSalary.Currency) != 3 {
			return Preferences{}, ErrInvalidPreferences
		}
		for _, c := range p.MinimumSalary.Currency {
			if c < 'A' || c > 'Z' {
				return Preferences{}, ErrInvalidPreferences
			}
		}
	}
	return p, nil
}

func normalizeSet(values []string, allowed map[string]bool, country bool) ([]string, error) {
	if len(values) > 100 {
		return nil, ErrInvalidPreferences
	}
	result := make([]string, len(values))
	seen := map[string]bool{}
	for i, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if country {
			value = strings.ToUpper(value)
		}
		if value == "" || seen[value] {
			return nil, ErrInvalidPreferences
		}
		if allowed != nil && !allowed[value] {
			return nil, ErrInvalidPreferences
		}
		if country && (len(value) != 2 || value[0] < 'A' || value[0] > 'Z' || value[1] < 'A' || value[1] > 'Z') {
			return nil, ErrInvalidPreferences
		}
		if allowed == nil && !country && !validSlug(value) {
			return nil, ErrInvalidPreferences
		}
		seen[value] = true
		result[i] = value
	}
	return result, nil
}

func validSlug(value string) bool {
	if len(value) == 0 || len(value) > 100 || value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for _, c := range value {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}
