package domain

import (
	"encoding/json"
	"errors"
	"time"
)

var ErrInvalidSource = errors.New("invalid source")

type Type string

const (
	Greenhouse Type = "greenhouse"
	Lever      Type = "lever"
	Ashby      Type = "ashby"
)

type Source struct {
	ID                 string          `json:"id"`
	Name               string          `json:"name"`
	Type               Type            `json:"type"`
	CompanyName        string          `json:"company_name"`
	Enabled            bool            `json:"enabled"`
	Priority           int             `json:"priority"`
	SyncIntervalSecond int             `json:"sync_interval_seconds"`
	Config             json.RawMessage `json:"-"`
	Cursor             json.RawMessage `json:"-"`
	LastSyncAt         *time.Time      `json:"last_sync_at"`
}

type ExternalJob struct {
	ExternalID     string          `json:"external_id"`
	CompanyName    string          `json:"company_name"`
	Title          string          `json:"title"`
	Description    string          `json:"description"`
	Location       string          `json:"location"`
	EmploymentType string          `json:"employment_type,omitempty"`
	ApplyURL       string          `json:"apply_url"`
	PublishedAt    *time.Time      `json:"published_at,omitempty"`
	Raw            json.RawMessage `json:"raw"`
}

type FetchResult struct {
	Jobs       []ExternalJob
	NextCursor json.RawMessage
}
