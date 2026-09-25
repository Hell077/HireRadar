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
	ID              string       `json:"id"`
	CompanyID       string       `json:"company_id"`
	Company         string       `json:"company"`
	Title           string       `json:"title"`
	NormalizedTitle string       `json:"normalized_title"`
	Description     string       `json:"description"`
	EmploymentTypes []string     `json:"employment_types"`
	RemotePolicy    RemotePolicy `json:"remote_policy"`
	Location        string       `json:"location"`
	Countries       []string     `json:"countries"`
	Eligibility     Eligibility  `json:"eligibility"`
	ApplyURL        string       `json:"apply_url"`
	PublishedAt     *time.Time   `json:"published_at,omitempty"`
	FirstSeenAt     time.Time    `json:"first_seen_at"`
	LastSeenAt      time.Time    `json:"last_seen_at"`
	Status          Status       `json:"status"`
	SourcePriority  int          `json:"source_priority"`
	CreatedAt       time.Time    `json:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at"`
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
