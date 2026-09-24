package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

type fakeSessions struct{ err error }

func (f fakeSessions) Login(context.Context, string, string, string, string) (application.Tokens, error) {
	return testTokens(), f.err
}
func (f fakeSessions) Refresh(context.Context, string) (application.Tokens, error) {
	return testTokens(), f.err
}
func (f fakeSessions) Logout(context.Context, string) error { return f.err }

func testTokens() application.Tokens {
	return application.Tokens{AccessToken: "access", AccessExpiresAt: time.Unix(1000, 0), RefreshToken: "refresh", RefreshExpiresAt: time.Unix(2000, 0)}
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
		services := AuthServices{}
		if len(appRegistrar) > 0 {
			services.Registrar = appRegistrar[0]
		}
		app := New(health.NewService(), services)
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

func TestSessionEndpoints(t *testing.T) {
	for _, test := range []struct {
		name, path, body string
		service          Sessions
		want             int
	}{
		{"login", "/api/v1/auth/login", `{"email":"person@example.com","password":"password"}`, fakeSessions{}, http.StatusOK},
		{"bad login", "/api/v1/auth/login", `{"email":"person@example.com","password":"password"}`, fakeSessions{application.ErrInvalidCredentials}, http.StatusUnauthorized},
		{"refresh", "/api/v1/auth/refresh", `{"refresh_token":"opaque"}`, fakeSessions{}, http.StatusOK},
		{"replay", "/api/v1/auth/refresh", `{"refresh_token":"opaque"}`, fakeSessions{application.ErrInvalidRefreshToken}, http.StatusUnauthorized},
		{"logout", "/api/v1/auth/logout", `{"refresh_token":"opaque"}`, fakeSessions{}, http.StatusNoContent},
		{"unavailable", "/api/v1/auth/login", `{"email":"person@example.com","password":"password"}`, nil, http.StatusServiceUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			app := New(health.NewService(), AuthServices{Sessions: test.service})
			req := httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(test.body))
			req.Header.Set("Content-Type", "application/json")
			response, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != test.want {
				t.Fatalf("status = %d, want %d", response.StatusCode, test.want)
			}
		})
	}
}
