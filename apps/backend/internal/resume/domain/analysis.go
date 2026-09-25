package domain

import "errors"

var ErrAnalysisNotReady = errors.New("resume analysis is not ready")
var ErrSuggestionReviewed = errors.New("resume suggestion was already reviewed")

type DetectedSkill struct {
	Name       string  `json:"name"`
	Confidence float64 `json:"confidence"`
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
	TotalExperienceMonths int                `json:"total_experience_months"`
}

type Suggestion struct {
	ID         string  `json:"id"`
	ResumeID   ID      `json:"resume_id"`
	Kind       string  `json:"kind"`
	Value      string  `json:"value"`
	Confidence float64 `json:"confidence"`
	Status     string  `json:"status"`
}
