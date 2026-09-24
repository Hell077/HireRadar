package domain

import (
	"errors"
	"strings"
)

var ErrInvalidSkills = errors.New("invalid candidate skills")

type Skill struct {
	Name  string   `json:"name"`
	Years *float64 `json:"years,omitempty"`
	Level string   `json:"level,omitempty"`
}

func NormalizeSkills(skills []Skill) ([]Skill, error) {
	if len(skills) > 100 {
		return nil, ErrInvalidSkills
	}
	result := make([]Skill, len(skills))
	seen := map[string]bool{}
	validLevels := map[string]bool{"": true, "beginner": true, "intermediate": true, "advanced": true, "expert": true}
	for i, skill := range skills {
		skill.Name = strings.Join(strings.Fields(skill.Name), " ")
		skill.Level = strings.ToLower(strings.TrimSpace(skill.Level))
		key := strings.ToLower(skill.Name)
		if key == "" || len(skill.Name) > 100 || seen[key] || !validLevels[skill.Level] ||
			(skill.Years != nil && (*skill.Years < 0 || *skill.Years > 70)) {
			return nil, ErrInvalidSkills
		}
		seen[key] = true
		result[i] = skill
	}
	return result, nil
}
