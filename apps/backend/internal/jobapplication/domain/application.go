package domain

import (
	"errors"
	"strings"
	"time"
)

var ErrInvalidApplication = errors.New("invalid job application")
var ErrInvalidTransition = errors.New("invalid job application status transition")

type ID string
type Provider string
type Status string

const (
	StatusRequested      Status = "requested"
	StatusPreparing      Status = "preparing"
	StatusNeedsInput     Status = "needs_input"
	StatusReady          Status = "ready"
	StatusSubmitting     Status = "submitting"
	StatusSubmitted      Status = "submitted"
	StatusManualRequired Status = "manual_required"
	StatusFailed         Status = "failed"
	StatusCancelled      Status = "cancelled"
)

type Application struct {
	ID          ID         `json:"id"`
	UserID      string     `json:"-"`
	JobID       string     `json:"job_id"`
	Provider    Provider   `json:"provider"`
	Status      Status     `json:"status"`
	Attempts    int        `json:"attempts"`
	RequestedAt time.Time  `json:"requested_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	SubmittedAt *time.Time `json:"submitted_at,omitempty"`
}

type FailedApplication struct {
	ID        ID        `json:"id"`
	JobID     string    `json:"job_id"`
	Title     string    `json:"title"`
	Company   string    `json:"company"`
	Provider  Provider  `json:"provider"`
	Status    Status    `json:"status"`
	Attempts  int       `json:"attempts"`
	ErrorCode string    `json:"error_code,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

func NewApplication(id ID, userID, jobID string, provider Provider, now time.Time) (Application, error) {
	if strings.TrimSpace(string(id)) == "" || strings.TrimSpace(userID) == "" ||
		strings.TrimSpace(jobID) == "" || strings.TrimSpace(string(provider)) == "" || now.IsZero() {
		return Application{}, ErrInvalidApplication
	}
	return Application{
		ID: id, UserID: userID, JobID: jobID, Provider: Provider(strings.ToLower(strings.TrimSpace(string(provider)))),
		Status: StatusRequested, RequestedAt: now, UpdatedAt: now,
	}, nil
}

// Transition applies a state change. Repeating the current state is safe for retries.
func (a *Application) Transition(next Status, now time.Time) error {
	if a == nil || now.IsZero() || !validStatus(next) {
		return ErrInvalidTransition
	}
	if a.Status == next {
		return nil
	}
	if !canTransition(a.Status, next) {
		return ErrInvalidTransition
	}
	a.Status = next
	a.UpdatedAt = now
	if next == StatusSubmitted {
		submittedAt := now
		a.SubmittedAt = &submittedAt
	}
	return nil
}

func validStatus(status Status) bool {
	switch status {
	case StatusRequested, StatusPreparing, StatusNeedsInput, StatusReady, StatusSubmitting,
		StatusSubmitted, StatusManualRequired, StatusFailed, StatusCancelled:
		return true
	default:
		return false
	}
}

func canTransition(current, next Status) bool {
	switch current {
	case StatusRequested:
		return next == StatusPreparing || next == StatusManualRequired || next == StatusFailed || next == StatusCancelled
	case StatusPreparing:
		return next == StatusNeedsInput || next == StatusReady || next == StatusManualRequired || next == StatusFailed || next == StatusCancelled
	case StatusNeedsInput:
		return next == StatusPreparing || next == StatusCancelled
	case StatusReady:
		return next == StatusSubmitting || next == StatusManualRequired || next == StatusFailed || next == StatusCancelled
	case StatusSubmitting:
		return next == StatusSubmitted || next == StatusNeedsInput || next == StatusManualRequired || next == StatusFailed
	case StatusFailed:
		return next == StatusRequested || next == StatusCancelled
	case StatusManualRequired:
		return next == StatusCancelled
	default:
		return false
	}
}
