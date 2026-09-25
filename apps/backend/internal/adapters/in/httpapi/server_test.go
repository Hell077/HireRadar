package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/application/health"
	"github.com/Hell077/HireRadar/apps/backend/internal/auth/application"
	"github.com/Hell077/HireRadar/apps/backend/internal/matching/engine"
	sourcedomain "github.com/Hell077/HireRadar/apps/backend/internal/source/domain"
	userdomain "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
)

type failingPinger struct{}

func (failingPinger) Ping(context.Context) error { return errors.New("down") }

type fakeRegistrar struct{ err error }

func (f fakeRegistrar) Register(context.Context, string, string) (userdomain.UserID, error) {
	return "test-user-id", f.err
}

type fakeSourceCatalog struct {
	sources []sourcedomain.Source
	err     error
}

func (f fakeSourceCatalog) ListEnabled(context.Context) ([]sourcedomain.Source, error) {
	return f.sources, f.err
}

func TestPublicSourceCatalog(t *testing.T) {
	app := New(health.NewService(), AuthServices{SourceCatalog: fakeSourceCatalog{sources: []sourcedomain.Source{{ID: "acme", Name: "Acme", Type: sourcedomain.Greenhouse, CompanyName: "Acme", Enabled: true, LastSyncStatus: "succeeded", LastFetchedJobs: 12}}}})
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/sources", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}
	var body struct {
		Sources []struct {
			ID      string `json:"id"`
			Type    string `json:"type"`
			Status  string `json:"last_sync_status"`
			Fetched int    `json:"last_fetched_jobs"`
		} `json:"sources"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Sources) != 1 || body.Sources[0].ID != "acme" || body.Sources[0].Type != "greenhouse" || body.Sources[0].Status != "succeeded" || body.Sources[0].Fetched != 12 {
		t.Fatalf("unexpected source response: %+v", body)
	}
}

type fakeMatchService struct{ refreshed int }

func (f *fakeMatchService) Refresh(context.Context, userdomain.UserID) ([]engine.Result, error) {
	f.refreshed++
	return []engine.Result{{JobID: "job-id", Score: 88, Eligible: true}}, nil
}
func (f *fakeMatchService) List(_ context.Context, _ userdomain.UserID, _ int) ([]engine.Result, error) {
	return []engine.Result{{JobID: "job-id", Score: 88, Eligible: true}}, nil
}

func TestMatchEndpointsRequireAuthentication(t *testing.T) {
	service := &fakeMatchService{}
	app := New(health.NewService(), AuthServices{Matches: service, Verifier: fakeVerifier{}})
	for _, test := range []struct{ method, path string }{{http.MethodGet, "/api/v1/matches"}, {http.MethodPost, "/api/v1/matches/refresh"}} {
		req := httptest.NewRequest(test.method, test.path, nil)
		response, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s %s without token returned %d", test.method, test.path, response.StatusCode)
		}
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/matches/refresh", nil)
	request.Header.Set("Authorization", "Bearer valid")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || service.refreshed != 1 {
		t.Fatalf("refresh status=%d calls=%d", response.StatusCode, service.refreshed)
	}
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

func TestMetricsExposeRequestCounters(t *testing.T) {
	requestMetrics.Lock()
	requestMetrics.values = make(map[string]requestMetric)
	requestMetrics.Unlock()
	app := New(health.NewService())
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	response, err = app.Test(httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), `hireradar_http_requests_total{method="GET",status="200"} 1`) {
		t.Fatalf("status=%d metrics=%s", response.StatusCode, body)
	}
}

func TestAdminSourceOperationsRequireOperatorToken(t *testing.T) {
	app := New(health.NewService(), AuthServices{OperatorAPIToken: "01234567890123456789012345678901"})
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/admin/sources", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", response.StatusCode)
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
