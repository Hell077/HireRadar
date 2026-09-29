package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/application/health"
	"github.com/Hell077/HireRadar/apps/backend/internal/jobapplication/domain"
	userdomain "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
)

type fakeApplications struct {
	requestedBy  userdomain.UserID
	requestedJob string
}

func (f *fakeApplications) Request(_ context.Context, userID userdomain.UserID, jobID string) (domain.Application, error) {
	f.requestedBy, f.requestedJob = userID, jobID
	return domain.Application{ID: "application-id", UserID: string(userID), JobID: jobID, Status: domain.StatusRequested, RequestedAt: time.Now().UTC()}, nil
}
func (*fakeApplications) Get(_ context.Context, userID userdomain.UserID, id domain.ID) (domain.Application, error) {
	return domain.Application{ID: id, UserID: string(userID), JobID: "job-id", Status: domain.StatusSubmitted}, nil
}
func (*fakeApplications) List(_ context.Context, userID userdomain.UserID, _ int) ([]domain.Application, error) {
	return []domain.Application{{ID: "application-id", UserID: string(userID), JobID: "job-id", Status: domain.StatusRequested}}, nil
}
func (*fakeApplications) Questions(context.Context, userdomain.UserID, domain.ID) ([]domain.ApplicationQuestion, error) {
	return []domain.ApplicationQuestion{}, nil
}
func (*fakeApplications) AnswerQuestion(context.Context, userdomain.UserID, string, string) error {
	return nil
}
func (*fakeApplications) Cancel(context.Context, userdomain.UserID, domain.ID) error { return nil }
func (*fakeApplications) Retry(context.Context, userdomain.UserID, domain.ID) error  { return nil }

func TestApplicationEndpointsRequireAuthenticationAndScopeRequests(t *testing.T) {
	service := &fakeApplications{}
	app := New(health.NewService(), AuthServices{Applications: service, Verifier: fakeVerifier{}})
	unauthorized, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/applications", nil))
	if err != nil {
		t.Fatal(err)
	}
	unauthorized.Body.Close()
	if unauthorized.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthorized list returned %d", unauthorized.StatusCode)
	}
	jobID := "b2d45992-9a64-42f3-92a8-b511562184d2"
	request := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/"+jobID+"/apply", strings.NewReader(`{}`))
	request.Header.Set("Authorization", "Bearer valid")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var output struct {
		Application domain.Application `json:"application"`
	}
	if err := json.NewDecoder(response.Body).Decode(&output); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || service.requestedBy != "owner-id" || service.requestedJob != jobID || output.Application.UserID != "" {
		t.Fatalf("status=%d owner=%q job=%q application=%+v", response.StatusCode, service.requestedBy, service.requestedJob, output.Application)
	}
	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/applications", nil)
	listRequest.Header.Set("Authorization", "Bearer valid")
	listResponse, err := app.Test(listRequest)
	if err != nil {
		t.Fatal(err)
	}
	listResponse.Body.Close()
	if listResponse.StatusCode != http.StatusOK {
		t.Fatalf("authenticated application list returned %d", listResponse.StatusCode)
	}
}
