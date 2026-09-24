package httpapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Hell077/HireRadar/apps/backend/internal/application/health"
	profile "github.com/Hell077/HireRadar/apps/backend/internal/profile/domain"
	user "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
)

type fakeProfileService struct{ saved profile.Profile }

func (f *fakeProfileService) Get(_ context.Context, id user.UserID) (profile.Profile, error) {
	return profile.Profile{UserID: id}, nil
}
func (f *fakeProfileService) Save(_ context.Context, p profile.Profile) error {
	f.saved = p
	return nil
}

type fakeVerifier struct{}

func (fakeVerifier) Verify(raw string) (user.UserID, error) {
	if raw != "valid" {
		return "", errors.New("invalid")
	}
	return "owner-id", nil
}

func TestProfileUsesAccessTokenOwner(t *testing.T) {
	service := &fakeProfileService{}
	app := New(health.NewService(), AuthServices{Profile: service, Verifier: fakeVerifier{}})
	request := func(auth string) (int, string) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPut, "/api/v1/profile", strings.NewReader(`{"first_name":"Alex","last_name":"","country":"","city":"","timezone":"","experience_years":0,"seniority":""}`))
		req.Header.Set("Content-Type", "application/json")
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		res, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(body)
	}
	if got, body := request(""); got != http.StatusUnauthorized {
		t.Fatalf("missing token = %d: %s", got, body)
	}
	if got, _ := request("Bearer invalid"); got != http.StatusUnauthorized {
		t.Fatalf("invalid token = %d", got)
	}
	if got, _ := request("Bearer valid"); got != http.StatusOK {
		t.Fatalf("valid token = %d", got)
	}
	if service.saved.UserID != "owner-id" {
		t.Fatalf("saved owner = %q", service.saved.UserID)
	}
}
