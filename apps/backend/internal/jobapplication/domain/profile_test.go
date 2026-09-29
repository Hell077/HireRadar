package domain

import (
	"errors"
	"testing"
)

func TestApplicationProfileNormalize(t *testing.T) {
	profile := ApplicationProfile{
		UserID: "user", FirstName: " Ada ", Email: "ADA@example.com", Country: "kz",
		LinkedInURL: "https://www.linkedin.com/in/ada", ExpectedSalary: &ApplicationMoney{Amount: 120000, Currency: "usd"},
		WorkAuthorization: []WorkAuthorization{{Country: "kz", Status: "authorized"}},
	}
	got, err := profile.Normalize()
	if err != nil {
		t.Fatal(err)
	}
	if got.FirstName != "Ada" || got.Email != "ada@example.com" || got.Country != "KZ" || got.ExpectedSalary.Currency != "USD" {
		t.Fatalf("profile was not normalized: %+v", got)
	}
}

func TestApplicationProfileRejectsInvalidAndUnsafeValues(t *testing.T) {
	for name, profile := range map[string]ApplicationProfile{
		"email":          {UserID: "user", Email: "not-an-email"},
		"url":            {UserID: "user", WebsiteURL: "javascript:alert(1)"},
		"authorization":  {UserID: "user", WorkAuthorization: []WorkAuthorization{{Country: "US", Status: "yes"}}},
		"notice":         {UserID: "user", NoticePeriodDays: intPointer(500)},
		"location-pair":  {UserID: "user", Latitude: floatPointer(51.5)},
		"location-range": {UserID: "user", Latitude: floatPointer(91), Longitude: floatPointer(0)},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := profile.Normalize(); !errors.Is(err, ErrInvalidApplicationProfile) {
				t.Fatalf("expected invalid profile, got %v", err)
			}
		})
	}
}

func intPointer(value int) *int           { return &value }
func floatPointer(value float64) *float64 { return &value }
