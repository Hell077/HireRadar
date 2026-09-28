package domain

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

var nonWord = regexp.MustCompile(`[^a-z0-9]+`)

func NameKey(name string) string {
	parts := strings.Fields(nonWord.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), " "))
	for len(parts) > 0 {
		switch parts[len(parts)-1] {
		case "inc", "incorporated", "llc", "ltd", "limited", "corp", "corporation", "company", "co":
			parts = parts[:len(parts)-1]
		default:
			return strings.Join(parts, " ")
		}
	}
	return strings.Join(parts, " ")
}

type TargetKind string

const (
	CompanyTarget           TargetKind = "company"
	JobBoardTarget          TargetKind = "job_board"
	FreelancePlatformTarget TargetKind = "freelance_platform"
	GitHubJobsTarget        TargetKind = "github_jobs"
)

func (k TargetKind) String() string { return string(k) }

type DiscoverySource struct {
	ID, Name, Owner, Repo, Parser string
	ETag, LastCommitSHA           string
	IntervalSeconds               int
}

type DiscoveredTarget struct {
	Kind            TargetKind      `json:"kind"`
	ExternalKey     string          `json:"external_key"`
	Name            string          `json:"name"`
	WebsiteURL      string          `json:"website_url,omitempty"`
	CareersURL      string          `json:"careers_url,omitempty"`
	OfficialDomain  string          `json:"official_domain,omitempty"`
	Regions         []string        `json:"regions,omitempty"`
	Technologies    []string        `json:"technologies,omitempty"`
	DiscoveryFields json.RawMessage `json:"discovery_fields,omitempty"`
}

type Candidate struct {
	ID                 string          `json:"id"`
	Kind               TargetKind      `json:"kind"`
	Name               string          `json:"name"`
	NormalizedName     string          `json:"normalized_name"`
	OfficialDomain     string          `json:"official_domain,omitempty"`
	WebsiteURL         string          `json:"website_url,omitempty"`
	CareersURL         string          `json:"careers_url,omitempty"`
	Regions            []string        `json:"regions"`
	Technologies       []string        `json:"technologies"`
	DiscoveryMetadata  json.RawMessage `json:"discovery_metadata"`
	Status             string          `json:"status"`
	DetectedProvider   string          `json:"detected_provider,omitempty"`
	ProviderKey        string          `json:"provider_key,omitempty"`
	ProviderURL        string          `json:"provider_url,omitempty"`
	ProviderConfidence float64         `json:"provider_confidence,omitempty"`
	ProviderEvidence   json.RawMessage `json:"provider_evidence"`
	SourceID           string          `json:"source_id,omitempty"`
	LastCheckedAt      *time.Time      `json:"last_checked_at,omitempty"`
	NextCheckAt        time.Time       `json:"next_check_at"`
	LastError          string          `json:"last_error,omitempty"`
}

type DetectedProvider struct {
	Type       string   `json:"type"`
	Key        string   `json:"key"`
	URL        string   `json:"url"`
	Confidence float64  `json:"confidence"`
	Evidence   []string `json:"evidence"`
}

type Resolution struct {
	Provider *DetectedProvider
	Reason   string
}

type RepositorySnapshot struct {
	ETag       string
	CommitSHA  string
	NotChanged bool
	Targets    []DiscoveredTarget
}

type GitHubRepositoryVerification struct {
	Verified bool
	Reason   string
	Evidence []string
}

type RunStats struct {
	TargetsSeen       int `json:"targets_seen"`
	CandidatesCreated int `json:"candidates_created"`
	CandidatesReused  int `json:"candidates_reused"`
	ProvenanceAdded   int `json:"provenance_added"`
}

type DiscoverySourceStatus struct {
	ID                string     `json:"id"`
	Name              string     `json:"name"`
	RepoOwner         string     `json:"repo_owner"`
	RepoName          string     `json:"repo_name"`
	Status            string     `json:"last_status,omitempty"`
	LastError         string     `json:"last_error,omitempty"`
	LastRunAt         *time.Time `json:"last_run_at,omitempty"`
	TargetsSeen       int        `json:"targets_seen"`
	CandidatesCreated int        `json:"candidates_created"`
	CandidatesReused  int        `json:"candidates_reused"`
	ProvenanceAdded   int        `json:"provenance_added"`
	NextRunAt         time.Time  `json:"next_run_at"`
}

type Count struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

type Overview struct {
	Sources         []DiscoverySourceStatus `json:"sources"`
	CandidateStates []Count                 `json:"candidate_states"`
	Providers       []Count                 `json:"providers"`
}
