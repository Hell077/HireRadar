package domain

import (
	"errors"
	"strings"

	user "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
)

var ErrInvalidProfile = errors.New("invalid candidate profile")

type Money struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

type Profile struct {
	UserID          user.UserID `json:"-"`
	FirstName       string      `json:"first_name"`
	LastName        string      `json:"last_name"`
	Country         string      `json:"country"`
	City            string      `json:"city"`
	Timezone        string      `json:"timezone"`
	ExperienceYears int         `json:"experience_years"`
	Seniority       string      `json:"seniority"`
	DesiredSalary   *Money      `json:"desired_salary"`
}

var seniorities = map[string]bool{
	"": true, "intern": true, "junior": true, "middle": true, "senior": true,
	"staff": true, "principal": true, "lead": true,
}

func (p Profile) Normalize() (Profile, error) {
	p.FirstName = strings.TrimSpace(p.FirstName)
	p.LastName = strings.TrimSpace(p.LastName)
	p.Country = strings.ToUpper(strings.TrimSpace(p.Country))
	p.City = strings.TrimSpace(p.City)
	p.Timezone = strings.TrimSpace(p.Timezone)
	p.Seniority = strings.ToLower(strings.TrimSpace(p.Seniority))
	if p.UserID == "" || len(p.FirstName) > 100 || len(p.LastName) > 100 || len(p.City) > 120 ||
		p.ExperienceYears < 0 || p.ExperienceYears > 70 || !seniorities[p.Seniority] {
		return Profile{}, ErrInvalidProfile
	}
	if p.Country != "" && (len(p.Country) != 2 || p.Country[0] < 'A' || p.Country[0] > 'Z' || p.Country[1] < 'A' || p.Country[1] > 'Z') {
		return Profile{}, ErrInvalidProfile
	}
	if p.DesiredSalary != nil {
		p.DesiredSalary.Currency = strings.ToUpper(strings.TrimSpace(p.DesiredSalary.Currency))
		if p.DesiredSalary.Amount < 0 || len(p.DesiredSalary.Currency) != 3 {
			return Profile{}, ErrInvalidProfile
		}
		for _, c := range p.DesiredSalary.Currency {
			if c < 'A' || c > 'Z' {
				return Profile{}, ErrInvalidProfile
			}
		}
	}
	if len(p.Timezone) > 100 {
		return Profile{}, ErrInvalidProfile
	}
	return p, nil
}
