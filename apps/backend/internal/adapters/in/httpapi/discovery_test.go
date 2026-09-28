package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Hell077/HireRadar/apps/backend/internal/application/health"
	"github.com/Hell077/HireRadar/apps/backend/internal/discovery/domain"
)

type fakeDiscoveryOperations struct {
	queued string
}

func (f *fakeDiscoveryOperations) Overview(context.Context) (domain.Overview, error) {
	return domain.Overview{Sources: []domain.DiscoverySourceStatus{{ID: "remoteintech", Name: "Remote In Tech"}}, CandidateStates: []domain.Count{{Key: "discovered", Count: 3}}}, nil
}
func (f *fakeDiscoveryOperations) ScheduleRun(_ context.Context, id string) error {
	f.queued = id
	return nil
}

func TestAdminDiscoveryStatusAndForceRunRequireOperatorToken(t *testing.T) {
	operations := &fakeDiscoveryOperations{}
	app := New(health.NewService(), AuthServices{Discovery: operations, OperatorAPIToken: "test-operator-token"})
	for _, path := range []string{"/api/v1/admin/discovery", "/api/v1/admin/discovery/remoteintech/run"} {
		method := http.MethodGet
		if path != "/api/v1/admin/discovery" {
			method = http.MethodPost
		}
		response, err := app.Test(httptest.NewRequest(method, path, nil))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s without operator token returned %d", path, response.StatusCode)
		}
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/discovery", nil)
	request.Header.Set("X-Operator-Token", "test-operator-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("overview status=%d", response.StatusCode)
	}
	request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/discovery/remoteintech/run", nil)
	request.Header.Set("X-Operator-Token", "test-operator-token")
	response, err = app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK || operations.queued != "remoteintech" {
		t.Fatalf("force run status=%d source=%q", response.StatusCode, operations.queued)
	}
}
