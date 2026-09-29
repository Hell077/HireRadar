package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/notification/application"
)

func TestSendMessageUsesTelegramInlineButtons(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bottoken/sendMessage" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var body struct {
			ChatID      int64 `json:"chat_id"`
			ReplyMarkup struct {
				Keyboard [][]Button `json:"inline_keyboard"`
			} `json:"reply_markup"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.ChatID != 123 || len(body.ReplyMarkup.Keyboard) != 1 || body.ReplyMarkup.Keyboard[0][0].CallbackData != "job:save:job-id" {
			t.Errorf("request body = %+v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	defer server.Close()
	client := NewClientWithBaseURL("token", server.URL, server.Client())
	if err := client.SendMessage(context.Background(), 123, "match", [][]Button{{{Text: "Save", CallbackData: "job:save:job-id"}}}); err != nil {
		t.Fatal(err)
	}
}

func TestTelegramClientReturnsProviderFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"ok":false,"description":"bad request"}`))
	}))
	defer server.Close()
	client := NewClientWithBaseURL("token", server.URL, server.Client())
	if err := client.AnswerCallback(context.Background(), "query", "Saved"); err == nil {
		t.Fatal("provider failure was accepted")
	}
}

func TestSetWebhookUsesSecretTokenAndAllowedUpdates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			URL           string   `json:"url"`
			Secret        string   `json:"secret_token"`
			AllowedUpdate []string `json:"allowed_updates"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.URL != "https://example.test/webhooks/telegram" || body.Secret != "secret" || len(body.AllowedUpdate) != 2 {
			t.Errorf("webhook request = %+v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	client := NewClientWithBaseURL("token", server.URL, server.Client())
	if err := client.SetWebhook(context.Background(), "https://example.test/webhooks/telegram", "secret"); err != nil {
		t.Fatal(err)
	}
}

func TestRateLimitErrorPreservesRetryAfterWithoutToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/botsensitive-test-token/sendMessage" {
			t.Errorf("wrong Telegram endpoint path: %q", r.URL.Path)
		}
		w.Header().Set("Retry-After", "7")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"ok":false,"description":"Too Many Requests","parameters":{"retry_after":5}}`))
	}))
	defer server.Close()
	client := NewClientWithBaseURL("sensitive-test-token", server.URL, server.Client())
	err := client.SendMessage(context.Background(), 1, "hi", nil)
	var apiErr *application.TelegramAPIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusTooManyRequests || apiErr.RetryAfter != 5*time.Second || strings.Contains(err.Error(), "sensitive-test-token") {
		t.Fatalf("rate limit error = %+v", err)
	}
}

func TestBlockedTelegramAccountErrorIsTerminal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"ok":false,"description":"bot was blocked by the user"}`))
	}))
	defer server.Close()
	client := NewClientWithBaseURL("token", server.URL, server.Client())
	err := client.SendMessage(context.Background(), 1, "hi", nil)
	var apiErr *application.TelegramAPIError
	if !errors.As(err, &apiErr) || !apiErr.Blocked {
		t.Fatalf("blocked error = %+v", err)
	}
}
