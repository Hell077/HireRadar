package domain

import (
	"errors"
	"testing"
	"time"
)

func TestApplicationTransitions(t *testing.T) {
	now := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
	app, err := NewApplication("app-1", "user-1", "job-1", " Greenhouse ", now)
	if err != nil {
		t.Fatal(err)
	}
	if app.Provider != "greenhouse" || app.Status != StatusRequested {
		t.Fatalf("unexpected initial application: %+v", app)
	}
	for i, status := range []Status{StatusPreparing, StatusReady, StatusSubmitting, StatusSubmitted} {
		if err := app.Transition(status, now.Add(time.Duration(i+1)*time.Minute)); err != nil {
			t.Fatalf("transition to %q: %v", status, err)
		}
	}
	if app.SubmittedAt == nil || !app.SubmittedAt.Equal(now.Add(4*time.Minute)) {
		t.Fatalf("submitted timestamp not set: %v", app.SubmittedAt)
	}
	if err := app.Transition(StatusSubmitted, now.Add(5*time.Minute)); err != nil {
		t.Fatalf("idempotent transition failed: %v", err)
	}
	if !app.UpdatedAt.Equal(now.Add(4 * time.Minute)) {
		t.Fatal("repeating the same transition changed updated_at")
	}
	if err := app.Transition(StatusPreparing, now.Add(6*time.Minute)); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected terminal transition rejection, got %v", err)
	}
}

func TestNeedsInputCanResumePreparation(t *testing.T) {
	now := time.Now().UTC()
	app, _ := NewApplication("app", "user", "job", "greenhouse", now)
	for _, status := range []Status{StatusPreparing, StatusNeedsInput, StatusPreparing, StatusReady} {
		if err := app.Transition(status, now); err != nil {
			t.Fatalf("transition to %q: %v", status, err)
		}
	}
}

func TestNewApplicationRequiresIdentifiers(t *testing.T) {
	if _, err := NewApplication("", "user", "job", "greenhouse", time.Now()); !errors.Is(err, ErrInvalidApplication) {
		t.Fatalf("expected invalid application, got %v", err)
	}
}
