package domain

import (
	"errors"
	"strings"
)

var ErrInvalidPositions = errors.New("invalid desired positions")

func NormalizePositions(titles []string) ([]string, error) {
	if len(titles) > 20 {
		return nil, ErrInvalidPositions
	}
	result := make([]string, len(titles))
	seen := map[string]bool{}
	for i, title := range titles {
		title = strings.Join(strings.Fields(title), " ")
		key := strings.ToLower(title)
		if title == "" || len(title) > 120 || seen[key] {
			return nil, ErrInvalidPositions
		}
		seen[key] = true
		result[i] = title
	}
	return result, nil
}
