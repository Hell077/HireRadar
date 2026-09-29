package domain

import (
	"errors"
	"net/url"
	"strings"
	"unicode"

	userdomain "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
)

var ErrInvalidApplicationProfile = errors.New("invalid application profile")

type ApplicationMoney struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

type WorkAuthorization struct {
	Country string `json:"country"`
	Status  string `json:"status"`
}

type ApplicationProfile struct {
	UserID            string              `json:"-"`
	FirstName         string              `json:"first_name"`
	LastName          string              `json:"last_name"`
	Email             string              `json:"email"`
	Phone             string              `json:"phone"`
	Country           string              `json:"country"`
	City              string              `json:"city"`
	Address           string              `json:"address"`
	Latitude          *float64            `json:"latitude,omitempty"`
	Longitude         *float64            `json:"longitude,omitempty"`
	LinkedInURL       string              `json:"linkedin_url"`
	GitHubURL         string              `json:"github_url"`
	WebsiteURL        string              `json:"website_url"`
	ResumeID          string              `json:"resume_id,omitempty"`
	ExpectedSalary    *ApplicationMoney   `json:"expected_salary,omitempty"`
	NoticePeriodDays  *int                `json:"notice_period_days,omitempty"`
	WorkAuthorization []WorkAuthorization `json:"work_authorization"`
	CustomAnswers     map[string]string   `json:"custom_answers"`
}

func (p ApplicationProfile) Normalize() (ApplicationProfile, error) {
	p.FirstName = strings.TrimSpace(p.FirstName)
	p.LastName = strings.TrimSpace(p.LastName)
	p.Email = strings.ToLower(strings.TrimSpace(p.Email))
	p.Phone = strings.TrimSpace(p.Phone)
	p.Country = strings.ToUpper(strings.TrimSpace(p.Country))
	p.City = strings.TrimSpace(p.City)
	p.Address = strings.TrimSpace(p.Address)
	p.LinkedInURL = strings.TrimSpace(p.LinkedInURL)
	p.GitHubURL = strings.TrimSpace(p.GitHubURL)
	p.WebsiteURL = strings.TrimSpace(p.WebsiteURL)
	p.ResumeID = strings.TrimSpace(p.ResumeID)
	if strings.TrimSpace(p.UserID) == "" || len(p.FirstName) > 100 || len(p.LastName) > 100 ||
		len(p.Email) > 320 || len(p.Phone) > 40 || len(p.City) > 120 || len(p.Address) > 255 ||
		(p.Country != "" && (len(p.Country) != 2 || !asciiLetters(p.Country))) ||
		!validOptionalEmail(p.Email) || !validOptionalURL(p.LinkedInURL) ||
		!validOptionalURL(p.GitHubURL) || !validOptionalURL(p.WebsiteURL) {
		return ApplicationProfile{}, ErrInvalidApplicationProfile
	}
	if (p.Latitude == nil) != (p.Longitude == nil) ||
		(p.Latitude != nil && (*p.Latitude < -90 || *p.Latitude > 90 || *p.Longitude < -180 || *p.Longitude > 180)) {
		return ApplicationProfile{}, ErrInvalidApplicationProfile
	}
	if p.ExpectedSalary != nil {
		p.ExpectedSalary.Currency = strings.ToUpper(strings.TrimSpace(p.ExpectedSalary.Currency))
		if p.ExpectedSalary.Amount < 0 || len(p.ExpectedSalary.Currency) != 3 || !asciiLetters(p.ExpectedSalary.Currency) {
			return ApplicationProfile{}, ErrInvalidApplicationProfile
		}
	}
	if p.NoticePeriodDays != nil && (*p.NoticePeriodDays < 0 || *p.NoticePeriodDays > 365) {
		return ApplicationProfile{}, ErrInvalidApplicationProfile
	}
	if len(p.WorkAuthorization) > 100 || len(p.CustomAnswers) > 100 {
		return ApplicationProfile{}, ErrInvalidApplicationProfile
	}
	for i := range p.WorkAuthorization {
		item := &p.WorkAuthorization[i]
		item.Country = strings.ToUpper(strings.TrimSpace(item.Country))
		item.Status = strings.ToLower(strings.TrimSpace(item.Status))
		if len(item.Country) != 2 || !asciiLetters(item.Country) || !validWorkAuthorization(item.Status) {
			return ApplicationProfile{}, ErrInvalidApplicationProfile
		}
	}
	for key, answer := range p.CustomAnswers {
		if strings.TrimSpace(key) == "" || len(key) > 200 || len(answer) > 4000 {
			return ApplicationProfile{}, ErrInvalidApplicationProfile
		}
	}
	if p.WorkAuthorization == nil {
		p.WorkAuthorization = []WorkAuthorization{}
	}
	if p.CustomAnswers == nil {
		p.CustomAnswers = map[string]string{}
	}
	return p, nil
}

func validOptionalEmail(value string) bool {
	if value == "" {
		return true
	}
	_, err := userdomain.NewEmail(value)
	return err == nil
}

func validOptionalURL(value string) bool {
	if value == "" {
		return true
	}
	parsed, err := url.ParseRequestURI(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Hostname() != "" && parsed.User == nil
}

func asciiLetters(value string) bool {
	for _, r := range value {
		if !unicode.IsUpper(r) || r > unicode.MaxASCII {
			return false
		}
	}
	return true
}

func validWorkAuthorization(status string) bool {
	switch status {
	case "authorized", "requires_sponsorship", "not_authorized", "unknown":
		return true
	default:
		return false
	}
}
