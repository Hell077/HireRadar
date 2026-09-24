package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Hell077/HireRadar/apps/backend/internal/application/health"
)

type failingPinger struct{}

func (failingPinger) Ping(context.Context) error { return errors.New("down") }

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
