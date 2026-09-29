package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Hell077/HireRadar/apps/backend/internal/application/health"
	"github.com/Hell077/HireRadar/apps/backend/internal/jobapplication/domain"
)

type fakeApplicationProfileService struct {
	saved domain.ApplicationProfile
	err   error
}

func (f *fakeApplicationProfileService) Get(_ context.Context, userID string) (domain.ApplicationProfile, error) {
	return domain.ApplicationProfile{UserID: userID, Email: "candidate@example.test"}, nil
}
func (f *fakeApplicationProfileService) Save(_ context.Context, profile domain.ApplicationProfile) error {
	f.saved = profile
	return f.err
}

func TestApplicationProfileIsAuthenticatedAndScopedToTokenOwner(t *testing.T) {
	service := &fakeApplicationProfileService{}
	app := New(health.NewService(), AuthServices{ApplicationProfile: service, Verifier: fakeVerifier{}})
	for _, authorization := range []string{"", "Bearer invalid"} {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/application-profile", nil)
		request.Header.Set("Authorization", authorization)
		response, err := app.Test(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusUnauthorized {
			t.Fatalf("unauthorized request returned %d", response.StatusCode)
		}
	}
	request := httptest.NewRequest(http.MethodPut, "/api/v1/application-profile", strings.NewReader(`{"first_name":"Ada","last_name":"Lovelace","email":"ada@example.test","phone":"","country":"GB","city":"London","address":"10 Example Street","linkedin_url":"","github_url":"","website_url":"","work_authorization":[],"custom_answers":{"question_1":"user supplied"}}`))
	request.Header.Set("Authorization", "Bearer valid")
	request.Header.Set("Content-Type", "application/json")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK || service.saved.UserID != "owner-id" || service.saved.FirstName != "Ada" {
		t.Fatalf("status=%d saved owner=%q profile=%+v", response.StatusCode, service.saved.UserID, service.saved)
	}
}

func TestApplicationProfileRejectsInvalidInput(t *testing.T) {
	service := &fakeApplicationProfileService{err: domain.ErrInvalidApplicationProfile}
	app := New(health.NewService(), AuthServices{ApplicationProfile: service, Verifier: fakeVerifier{}})
	request := httptest.NewRequest(http.MethodPut, "/api/v1/application-profile", strings.NewReader(`{"first_name":"Ada","last_name":"","email":"invalid","phone":"","country":"","city":"","address":"","linkedin_url":"","github_url":"","website_url":"","work_authorization":[],"custom_answers":{}}`))
	request.Header.Set("Authorization", "Bearer valid")
	request.Header.Set("Content-Type", "application/json")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid profile status=%d", response.StatusCode)
	}
}
