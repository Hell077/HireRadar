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
	jobapplication "github.com/Hell077/HireRadar/apps/backend/internal/jobapplication/domain"
	"github.com/Hell077/HireRadar/apps/backend/internal/lifecycle"
	matchapp "github.com/Hell077/HireRadar/apps/backend/internal/matching/application"
	"github.com/Hell077/HireRadar/apps/backend/internal/matching/engine"
	"github.com/Hell077/HireRadar/apps/backend/internal/observability"
	"github.com/Hell077/HireRadar/apps/backend/internal/outbox"
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

type fakeAdminOutbox struct{ retried string }

func (f *fakeAdminOutbox) ListFailed(context.Context, int) ([]outbox.FailedEvent, error) {
	return []outbox.FailedEvent{{ID: "5db228eb-0fa0-4e31-a9a8-64e488a9846f", EventType: "fixture"}}, nil
}
func (f *fakeAdminOutbox) RetryFailed(_ context.Context, id string) error { f.retried = id; return nil }

type fakeAdminApplications struct{ retried jobapplication.ID }

func (f *fakeAdminApplications) ListFailed(context.Context, int) ([]jobapplication.FailedApplication, error) {
	return []jobapplication.FailedApplication{{ID: "5db228eb-0fa0-4e31-a9a8-64e488a9846f", Status: jobapplication.StatusFailed}}, nil
}
func (f *fakeAdminApplications) RetryFailed(_ context.Context, id jobapplication.ID) error {
	f.retried = id
	return nil
}

func TestAdminRecoveryOperationsRequireOperatorToken(t *testing.T) {
	secret := "01234567890123456789012345678901"
	outboxOps := &fakeAdminOutbox{}
	applicationOps := &fakeAdminApplications{}
	app := New(health.NewService(), AuthServices{OperatorAPIToken: secret, AdminOutbox: outboxOps, AdminApplications: applicationOps})
	for _, test := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/admin/outbox/failed"},
		{http.MethodPost, "/api/v1/admin/outbox/5db228eb-0fa0-4e31-a9a8-64e488a9846f/retry"},
		{http.MethodGet, "/api/v1/admin/applications/failed"},
		{http.MethodPost, "/api/v1/admin/applications/5db228eb-0fa0-4e31-a9a8-64e488a9846f/retry"},
	} {
		response, err := app.Test(httptest.NewRequest(test.method, test.path, nil))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s %s status=%d want 401", test.method, test.path, response.StatusCode)
		}
	}
	for _, test := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/admin/outbox/failed"},
		{http.MethodPost, "/api/v1/admin/outbox/5db228eb-0fa0-4e31-a9a8-64e488a9846f/retry"},
		{http.MethodGet, "/api/v1/admin/applications/failed"},
		{http.MethodPost, "/api/v1/admin/applications/5db228eb-0fa0-4e31-a9a8-64e488a9846f/retry"},
	} {
		request := httptest.NewRequest(test.method, test.path, nil)
		request.Header.Set("X-Operator-Token", secret)
		response, err := app.Test(request)
		if err != nil {
			t.Fatal(err)
		}
		var body struct {
			ID string `json:"id"`
		}
		if test.method == http.MethodPost {
			_ = json.NewDecoder(response.Body).Decode(&body)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("%s %s status=%d want 200", test.method, test.path, response.StatusCode)
		}
		if test.method == http.MethodPost && body.ID != "5db228eb-0fa0-4e31-a9a8-64e488a9846f" {
			t.Fatalf("%s %s returned id %q", test.method, test.path, body.ID)
		}
	}
	const id = "5db228eb-0fa0-4e31-a9a8-64e488a9846f"
	_ = id
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
func (f *fakeMatchService) ListPage(_ context.Context, _ userdomain.UserID, _ string, _ int) (matchapp.MatchPage, error) {
	return matchapp.MatchPage{Matches: []engine.Result{{JobID: "job-id", Score: 88, Eligible: true}}}, nil
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
	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/matches?limit=10", nil)
	listRequest.Header.Set("Authorization", "Bearer valid")
	listResponse, err := app.Test(listRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer listResponse.Body.Close()
	var listBody struct {
		Matches    []engine.Result `json:"matches"`
		NextCursor string          `json:"next_cursor"`
	}
	if err := json.NewDecoder(listResponse.Body).Decode(&listBody); err != nil {
		t.Fatal(err)
	}
	if listResponse.StatusCode != http.StatusOK || len(listBody.Matches) != 1 {
		t.Fatalf("match page status=%d response=%+v", listResponse.StatusCode, listBody)
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
	observability.DefaultMetrics.Add("hireradar_jobs_fetched_total", nil, 3)
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
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), `hireradar_http_requests_total{method="GET",status="200"} 1`) || !strings.Contains(string(body), "hireradar_jobs_fetched_total 3") {
		t.Fatalf("status=%d metrics=%s", response.StatusCode, body)
	}
}

func TestHTTPTraceContextIsAcceptedAndReturned(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "")
	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	shutdown, err := observability.InitTracing(context.Background(), "httpapi-test")
	if err != nil {
		t.Fatal(err)
	}
	defer shutdown(context.Background())
	app := New(health.NewService())
	request := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	request.Header.Set("traceparent", "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if got := response.Header.Get("X-Trace-ID"); got != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("X-Trace-ID=%q", got)
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

func TestAdminServiceControlsRequireOperatorTokenAndExposeStatus(t *testing.T) {
	manager := lifecycle.New()
	if err := manager.Register("fixture", false, "disabled for test", nil); err != nil {
		t.Fatal(err)
	}
	secret := "01234567890123456789012345678901"
	app := New(health.NewService(), AuthServices{OperatorAPIToken: secret, Services: manager})
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/admin/services", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status=%d want 401", response.StatusCode)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/services", nil)
	request.Header.Set("X-Operator-Token", secret)
	response, err = app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("authenticated status=%d want 200", response.StatusCode)
	}
	var body struct {
		Services []struct {
			Name  string `json:"name"`
			State string `json:"state"`
		} `json:"services"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Services) != 1 || body.Services[0].Name != "fixture" || body.Services[0].State != "disabled" {
		t.Fatalf("unexpected services: %+v", body.Services)
	}
	action := httptest.NewRequest(http.MethodPost, "/api/v1/admin/services/fixture", strings.NewReader(`{"action":"start"}`))
	action.Header.Set("Content-Type", "application/json")
	action.Header.Set("X-Operator-Token", secret)
	response, err = app.Test(action)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("disabled service action status=%d want 400", response.StatusCode)
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
