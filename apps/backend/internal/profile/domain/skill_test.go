package domain

import (
	"errors"
	"testing"
)

func TestNormalizeSkills(t *testing.T) {
	skills, err := NormalizeSkills([]Skill{{Name: " Go  lang ", Level: "Advanced"}})
	if err != nil || skills[0].Name != "Go lang" || skills[0].Level != "advanced" {
		t.Fatalf("skills = %+v, %v", skills, err)
	}
	for _, bad := range [][]Skill{
		{{Name: ""}},
		{{Name: "Go"}, {Name: "go"}},
		{{Name: "Go", Level: "wizard"}},
	} {
		if _, err := NormalizeSkills(bad); !errors.Is(err, ErrInvalidSkills) {
			t.Fatalf("accepted %+v", bad)
		}
	}
}
