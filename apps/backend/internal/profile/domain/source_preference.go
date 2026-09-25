package domain

import (
	"errors"
	"strings"
)

var ErrInvalidSourcePreferences = errors.New("invalid source preferences")

type SourcePreference struct {
	SourceID string `json:"source_id"`
	Enabled  bool   `json:"enabled"`
}

func NormalizeSourcePreferences(values []SourcePreference) ([]SourcePreference, error) {
	if len(values) > 100 {
		return nil, ErrInvalidSourcePreferences
	}
	result := make([]SourcePreference, len(values))
	seen := map[string]bool{}
	for i, value := range values {
		value.SourceID = strings.ToLower(strings.TrimSpace(value.SourceID))
		if !validSlug(value.SourceID) || seen[value.SourceID] {
			return nil, ErrInvalidSourcePreferences
		}
		seen[value.SourceID] = true
		result[i] = value
	}
	return result, nil
}
