package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/application/health"
	"github.com/Hell077/HireRadar/apps/backend/internal/notification/application"
	"github.com/Hell077/HireRadar/apps/backend/internal/notification/domain"
	user "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
)

type fakeTelegramService struct {
	started, callbacks             int
	feedbackAction, feedbackReason string
}

func (f *fakeTelegramService) Connection(context.Context, user.UserID) (application.Account, error) {
	return application.Account{}, nil
}
func (f *fakeTelegramService) CreateLink(context.Context, user.UserID) (application.Link, error) {
	return application.Link{URL: "https://t.me/example_bot?start=secret", ExpiresAt: time.Now().Add(time.Minute)}, nil
}
func (f *fakeTelegramService) Disconnect(context.Context, user.UserID) error { return nil }
func (f *fakeTelegramService) Preferences(context.Context, user.UserID) (domain.NotificationPreferences, error) {
	return domain.DefaultPreferences(), nil
}
func (f *fakeTelegramService) SavePreferences(context.Context, user.UserID, domain.NotificationPreferences) error {
	return nil
}
func (f *fakeTelegramService) SavedJobs(context.Context, user.UserID, int) ([]application.SavedJob, error) {
	return nil, nil
}
func (f *fakeTelegramService) RemoveSavedJob(context.Context, user.UserID, string) error { return nil }
func (f *fakeTelegramService) ApplyUserFeedback(_ context.Context, _ user.UserID, _ string, action, reason string) error {
	f.feedbackAction, f.feedbackReason = action, reason
	return nil
}
func (*fakeTelegramService) OpenNotification(context.Context, string) (string, error) {
	return "https://jobs.example/apply", nil
}
func (f *fakeTelegramService) HandleStart(context.Context, int64, int64, string, string) error {
	f.started++
	return nil
}
func (f *fakeTelegramService) HandleCallback(context.Context, int64, string, string) error {
	f.callbacks++
	return nil
}

func TestTelegramLinkRequiresAuthentication(t *testing.T) {
	app := New(health.NewService(), AuthServices{Telegram: &fakeTelegramService{}, Verifier: fakeVerifier{}})
	response, err := app.Test(httptest.NewRequest(http.MethodPost, "/api/v1/telegram/link", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.StatusCode)
	}
}

func TestSavedJobsRequireAuthentication(t *testing.T) {
	app := New(health.NewService(), AuthServices{Telegram: &fakeTelegramService{}, Verifier: fakeVerifier{}})
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/saved-jobs", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.StatusCode)
	}
}

func TestJobFeedbackRequiresAuthentication(t *testing.T) {
	app := New(health.NewService(), AuthServices{Telegram: &fakeTelegramService{}, Verifier: fakeVerifier{}})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/b2d45992-9a64-42f3-92a8-b511562184d2/feedback", strings.NewReader(`{"action":"applied"}`))
	request.Header.Set("Content-Type", "application/json")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.StatusCode)
	}
}

func TestJobFeedbackAcceptsControlledReason(t *testing.T) {
	service := &fakeTelegramService{}
	app := New(health.NewService(), AuthServices{Telegram: service, Verifier: fakeVerifier{}})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/b2d45992-9a64-42f3-92a8-b511562184d2/feedback", strings.NewReader(`{"action":"hide","reason":"wrong_stack"}`))
	request.Header.Set("Authorization", "Bearer valid")
	request.Header.Set("Content-Type", "application/json")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || service.feedbackAction != "hide" || service.feedbackReason != "wrong_stack" {
		t.Fatalf("status=%d action=%q reason=%q", response.StatusCode, service.feedbackAction, service.feedbackReason)
	}
}

func TestNotificationOpenRedirectsToStoredApplicationURL(t *testing.T) {
	service := &fakeTelegramService{}
	app := New(health.NewService(), AuthServices{Telegram: service})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/open/b2d45992-9a64-42f3-92a8-b511562184d2", nil)
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusFound || response.Header.Get("Location") != "https://jobs.example/apply" {
		t.Fatalf("status=%d location=%q", response.StatusCode, response.Header.Get("Location"))
	}
}
