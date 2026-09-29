package domain

import (
	"errors"
	"time"
)

var ErrAnalysisNotReady = errors.New("resume analysis is not ready")
var ErrSuggestionReviewed = errors.New("resume suggestion was already reviewed")

type DetectedSkill struct {
	Name                      string     `json:"name"`
	Confidence                float64    `json:"confidence"`
	EstimatedExperienceMonths int        `json:"estimated_experience_months,omitempty"`
	FirstUsed                 *time.Time `json:"first_used,omitempty"`
	LastUsed                  *time.Time `json:"last_used,omitempty"`
	Current                   bool       `json:"current,omitempty"`
}

type DetectedPosition struct {
	Title      string  `json:"title"`
	Confidence float64 `json:"confidence"`
}

type ParsedResume struct {
	ResumeID              ID                 `json:"resume_id"`
	Text                  string             `json:"text,omitempty"`
	Skills                []DetectedSkill    `json:"skills"`
	Positions             []DetectedPosition `json:"positions"`
	Experiences           []Experience       `json:"experiences"`
	Languages             []string           `json:"languages"`
	TotalExperienceMonths int                `json:"total_experience_months"`
}

type Experience struct {
	Company    string          `json:"company,omitempty"`
	Title      string          `json:"title"`
	StartDate  *time.Time      `json:"start_date,omitempty"`
	EndDate    *time.Time      `json:"end_date,omitempty"`
	Current    bool            `json:"current"`
	Skills     []DetectedSkill `json:"skills"`
	Confidence float64         `json:"confidence"`
}

type Suggestion struct {
	ID         string  `json:"id"`
	ResumeID   ID      `json:"resume_id"`
	Kind       string  `json:"kind"`
	Value      string  `json:"value"`
	Confidence float64 `json:"confidence"`
	Status     string  `json:"status"`
}
