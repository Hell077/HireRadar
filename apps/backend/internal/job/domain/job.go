package domain

import (
	"errors"
	"time"
)

var ErrInvalidCursor = errors.New("invalid job cursor")
var ErrInvalidQuery = errors.New("invalid job query")

type RemotePolicy string

const (
	RemoteWorldwide RemotePolicy = "worldwide"
	Remote          RemotePolicy = "remote"
	RemoteRegion    RemotePolicy = "remote_region"
	RemoteCountry   RemotePolicy = "remote_country"
	Hybrid          RemotePolicy = "hybrid"
	Onsite          RemotePolicy = "onsite"
	RemoteUnknown   RemotePolicy = "unknown"
)

type Eligibility string

const (
	Eligible           Eligibility = "eligible"
	NotEligible        Eligibility = "not_eligible"
	EligibilityUnknown Eligibility = "unknown"
)

type Status string

const (
	Active  Status = "active"
	Closed  Status = "closed"
	Expired Status = "expired"
	Unknown Status = "unknown"
)

type Seniority string

const (
	Intern           Seniority = "intern"
	Junior           Seniority = "junior"
	Mid              Seniority = "mid"
	Senior           Seniority = "senior"
	Staff            Seniority = "staff"
	Principal        Seniority = "principal"
	Lead             Seniority = "lead"
	Manager          Seniority = "manager"
	Director         Seniority = "director"
	Executive        Seniority = "executive"
	SeniorityUnknown Seniority = "unknown"
)

type JobSkill struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Required     bool     `json:"required"`
	Confidence   float64  `json:"confidence"`
	MinimumYears *float64 `json:"minimum_years,omitempty"`
	MinimumLevel string   `json:"minimum_level,omitempty"`
}

type SalaryRange struct {
	Minimum  float64 `json:"minimum"`
	Maximum  float64 `json:"maximum"`
	Currency string  `json:"currency"`
	Period   string  `json:"period"`
}

type Company struct {
	ID             string       `json:"id"`
	Name           string       `json:"name"`
	NormalizedName string       `json:"normalized_name"`
	WebsiteURL     string       `json:"website_url,omitempty"`
	CareersURL     string       `json:"careers_url,omitempty"`
	RemotePolicy   RemotePolicy `json:"remote_policy"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
}

type Job struct {
	ID                  string       `json:"id"`
	MatchVersion        int64        `json:"-"`
	Family              string       `json:"job_family"`
	Speciality          string       `json:"job_speciality"`
	FamilyConfidence    float64      `json:"job_family_confidence"`
	SeniorityConfidence float64      `json:"seniority_confidence"`
	LocationConfidence  float64      `json:"location_confidence"`
	SalaryConfidence    float64      `json:"salary_confidence"`
	CompanyID           string       `json:"company_id"`
	Company             string       `json:"company"`
	Title               string       `json:"title"`
	NormalizedTitle     string       `json:"normalized_title"`
	Seniority           Seniority    `json:"seniority"`
	Description         string       `json:"description"`
	Skills              []JobSkill   `json:"skills"`
	Salary              *SalaryRange `json:"salary,omitempty"`
	EmploymentTypes     []string     `json:"employment_types"`
	RemotePolicy        RemotePolicy `json:"remote_policy"`
	Location            string       `json:"location"`
	Countries           []string     `json:"countries"`
	Eligibility         Eligibility  `json:"eligibility"`
	ApplyURL            string       `json:"apply_url"`
	PublishedAt         *time.Time   `json:"published_at,omitempty"`
	FirstSeenAt         time.Time    `json:"first_seen_at"`
	LastSeenAt          time.Time    `json:"last_seen_at"`
	Status              Status       `json:"status"`
	SourcePriority      int          `json:"source_priority"`
	CreatedAt           time.Time    `json:"created_at"`
	UpdatedAt           time.Time    `json:"updated_at"`
}

type SourceReference struct {
	JobID        string    `json:"job_id"`
	SourceID     string    `json:"source_id"`
	ExternalID   string    `json:"external_id"`
	URL          string    `json:"url"`
	FirstSeenAt  time.Time `json:"first_seen_at"`
	LastSeenAt   time.Time `json:"last_seen_at"`
	MissingCount int       `json:"missing_count"`
}
