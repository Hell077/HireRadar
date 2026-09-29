package telegramwebhook

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type testHandler struct {
	messages, callbacks int
	text                string
}

func (h *testHandler) HandleMessage(_ context.Context, userID, chatID int64, chatType, _, text string) error {
	if userID != chatID || chatType != "private" {
		return nil
	}
	h.messages++
	h.text = text
	return nil
}
func (h *testHandler) HandleCallback(context.Context, int64, string, string) error {
	h.callbacks++
	return nil
}

type testPinger struct{ err error }

func (p testPinger) Ping(context.Context) error { return p.err }

func TestWebhookValidatesSecretAndDispatchesUpdates(t *testing.T) {
	service := &testHandler{}
	server := httptest.NewServer(New("secret", service, testPinger{}))
	defer server.Close()
	post := func(body, secret string) *http.Response {
		req, _ := http.NewRequest(http.MethodPost, server.URL+"/telegram/webhook", strings.NewReader(body))
		req.Header.Set("X-Telegram-Bot-Api-Secret-Token", secret)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	bad := post(`{"message":{"text":"/help"}}`, "wrong")
	bad.Body.Close()
	if bad.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong secret status=%d", bad.StatusCode)
	}
	message := post(`{"message":{"text":"/help","from":{"id":10},"chat":{"id":10,"type":"private"}}}`, "secret")
	message.Body.Close()
	if message.StatusCode != http.StatusOK || service.messages != 1 || service.text != "/help" {
		t.Fatalf("message status=%d handler=%+v", message.StatusCode, service)
	}
	callback := post(`{"callback_query":{"id":"q1","data":"job:save:abc","from":{"id":10}}}`, "secret")
	callback.Body.Close()
	if callback.StatusCode != http.StatusOK || service.callbacks != 1 {
		t.Fatalf("callback status=%d callbacks=%d", callback.StatusCode, service.callbacks)
	}
}

func TestWebhookLimitsBodyAndReadiness(t *testing.T) {
	server := httptest.NewServer(New("secret", &testHandler{}, testPinger{}))
	defer server.Close()
	request, _ := http.NewRequest(http.MethodPost, server.URL+"/telegram/webhook", strings.NewReader(strings.Repeat("x", 65*1024)))
	request.Header.Set("X-Telegram-Bot-Api-Secret-Token", "secret")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("oversized update status=%d", response.StatusCode)
	}
	ready, err := http.Get(server.URL + "/ready")
	if err != nil {
		t.Fatal(err)
	}
	ready.Body.Close()
	if ready.StatusCode != http.StatusOK {
		t.Fatalf("ready status=%d", ready.StatusCode)
	}
	server.Close()
	broken := httptest.NewServer(New("secret", &testHandler{}, testPinger{err: context.DeadlineExceeded}))
	defer broken.Close()
	ready, err = http.Get(broken.URL + "/ready")
	if err != nil {
		t.Fatal(err)
	}
	ready.Body.Close()
	if ready.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("unready status=%d", ready.StatusCode)
	}
}
