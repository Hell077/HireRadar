package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Hell077/HireRadar/apps/backend/internal/application/health"
	"github.com/Hell077/HireRadar/apps/backend/internal/auth/application"
	"github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
)

type failingPinger struct{}

func (failingPinger) Ping(context.Context) error { return errors.New("down") }

type fakeRegistrar struct{ err error }

func (f fakeRegistrar) Register(context.Context, string, string) (domain.UserID, error) {
	return "test-user-id", f.err
}

func TestHealthEndpoint(t *testing.T) {
	app := New(health.NewService())

	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != "ok" {
		t.Fatalf("body status = %q, want %q", body.Status, "ok")
	}
}

func TestReadinessEndpointReportsDependencyFailure(t *testing.T) {
	checker := health.NewService(health.Dependency{Name: "postgres", Pinger: failingPinger{}})
	app := New(checker)
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/ready", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusServiceUnavailable)
	}
}

func TestRegisterEndpoint(t *testing.T) {
	request := func(appRegistrar ...Registrar) *http.Response {
		t.Helper()
		app := New(health.NewService(), appRegistrar...)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"person@example.com","password":"a sufficiently long password"}`))
		req.Header.Set("Content-Type", "application/json")
		response, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		return response
	}

	success := request(fakeRegistrar{})
	defer success.Body.Close()
	if success.StatusCode != http.StatusCreated {
		t.Fatalf("register status = %d, want 201", success.StatusCode)
	}
	var body struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(success.Body).Decode(&body); err != nil || body.UserID != "test-user-id" {
		t.Fatalf("register body = %+v, error = %v", body, err)
	}

	conflict := request(fakeRegistrar{err: application.ErrEmailTaken})
	defer conflict.Body.Close()
	if conflict.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate status = %d, want 409", conflict.StatusCode)
	}

	unavailable := request()
	defer unavailable.Body.Close()
	if unavailable.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("unavailable status = %d, want 503", unavailable.StatusCode)
	}
}
