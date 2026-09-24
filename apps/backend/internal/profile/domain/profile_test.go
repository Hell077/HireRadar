package domain

import (
	"errors"
	"testing"

	user "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
)

func TestProfileNormalize(t *testing.T) {
	p, err := (Profile{UserID: user.UserID("id"), Country: " kz ", Seniority: "Senior", DesiredSalary: &Money{Amount: 100, Currency: "usd"}}).Normalize()
	if err != nil || p.Country != "KZ" || p.Seniority != "senior" || p.DesiredSalary.Currency != "USD" {
		t.Fatalf("normalized profile = %+v, %v", p, err)
	}
	for _, bad := range []Profile{
		{UserID: "id", Country: "Kazakhstan"},
		{UserID: "id", ExperienceYears: -1},
		{UserID: "id", Seniority: "guru"},
		{UserID: "id", DesiredSalary: &Money{Amount: -1, Currency: "USD"}},
	} {
		if _, err := bad.Normalize(); !errors.Is(err, ErrInvalidProfile) {
			t.Fatalf("accepted %+v", bad)
		}
	}
}
