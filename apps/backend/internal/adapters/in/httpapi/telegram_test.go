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

type fakeTelegramService struct{ started int }

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
func (f *fakeTelegramService) HandleStart(context.Context, int64, int64, string, string) error {
	f.started++
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

func TestTelegramWebhookRequiresSecretAndConsumesStart(t *testing.T) {
	service := &fakeTelegramService{}
	app := New(health.NewService(), AuthServices{Telegram: service, TelegramWebhookSecret: "webhook-secret"})
	request := func(secret string) *http.Response {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/webhooks/telegram", strings.NewReader(`{"message":{"text":"/start single-use","from":{"id":10,"username":"alex"},"chat":{"id":10}}}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Telegram-Bot-Api-Secret-Token", secret)
		response, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		return response
	}
	bad := request("wrong")
	bad.Body.Close()
	if bad.StatusCode != http.StatusUnauthorized || service.started != 0 {
		t.Fatalf("invalid secret status=%d starts=%d", bad.StatusCode, service.started)
	}
	ok := request("webhook-secret")
	defer ok.Body.Close()
	if ok.StatusCode != http.StatusOK || service.started != 1 {
		t.Fatalf("valid secret status=%d starts=%d", ok.StatusCode, service.started)
	}
}
