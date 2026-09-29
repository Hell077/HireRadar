package lever

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	jobdomain "github.com/Hell077/HireRadar/apps/backend/internal/job/domain"
	"github.com/Hell077/HireRadar/apps/backend/internal/jobapplication/domain"
)

type resumeReaderFunc func(context.Context, string, string) (ResumeAttachment, error)

func (f resumeReaderFunc) ReadResume(ctx context.Context, userID, resumeID string) (ResumeAttachment, error) {
	return f(ctx, userID, resumeID)
}

func TestPrepareUsesPublicPostingAndOnlyExplicitStandardFields(t *testing.T) {
	postingID := "39446629-e634-42b4-af62-39332fec714e"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v0/postings/acme/"+postingID {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"`+postingID+`","text":"Senior Engineer","applyUrl":"https://jobs.lever.co/acme/`+postingID+`/apply"}`)
	}))
	defer server.Close()
	provider := NewWithHTTPClient(CredentialMap{"acme": "authorized-key"}, resumeReaderFunc(func(context.Context, string, string) (ResumeAttachment, error) {
		return ResumeAttachment{}, nil
	}), server.Client(), server.URL)
	job := jobdomain.Job{ApplyURL: "https://jobs.lever.co/acme/" + postingID + "/apply"}
	if !provider.Supports(job) {
		t.Fatal("provider should support an authorized Lever site")
	}
	form, err := provider.Prepare(context.Background(), job, domain.ApplicationProfile{FirstName: "Ada", ResumeID: "resume-id"})
	if err != nil {
		t.Fatal(err)
	}
	if len(form.Fields) != 5 || len(form.Questions) != 1 || form.Questions[0].ExternalKey != "email" || form.Metadata["site"] != "acme" {
		t.Fatalf("prepared form=%+v", form)
	}
}

func TestSubmitUsesAuthorizedKeyAndUploadsSelectedResume(t *testing.T) {
	postingID := "39446629-e634-42b4-af62-39332fec714e"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v0/postings/acme/"+postingID || r.URL.Query().Get("key") != "authorized-key" {
			t.Errorf("unexpected Lever request: %s %s", r.Method, r.URL.Redacted())
		}
		if err := r.ParseMultipartForm(maxResumeBytes); err != nil {
			t.Error(err)
			return
		}
		if r.FormValue("name") != "Ada Lovelace" || r.FormValue("email") != "ada@example.test" || r.FormValue("urls[LinkedIn]") != "https://linkedin.example/ada" {
			t.Errorf("standard candidate fields missing: %#v", r.MultipartForm.Value)
		}
		file, _, err := r.FormFile("resume")
		if err != nil {
			t.Error(err)
		} else {
			defer file.Close()
			data, _ := io.ReadAll(file)
			if string(data) != "application resume" {
				t.Errorf("resume upload=%q", data)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"ok":true,"applicationId":"lever-application-1"}`)
	}))
	defer server.Close()
	provider := NewWithHTTPClient(CredentialMap{"acme": "authorized-key"}, resumeReaderFunc(func(_ context.Context, userID, resumeID string) (ResumeAttachment, error) {
		if userID != "user-id" || resumeID != "resume-id" {
			t.Errorf("resume owner/id = %q/%q", userID, resumeID)
		}
		return ResumeAttachment{Filename: "resume.pdf", ContentType: "application/pdf", Content: []byte("application resume")}, nil
	}), server.Client(), server.URL)
	form := domain.PreparedForm{
		Fields: []domain.Field{
			{Key: "name", Required: true, Value: "Ada Lovelace"},
			{Key: "email", Required: true, Value: "ada@example.test"},
			{Key: "urls", Value: map[string]any{"LinkedIn": "https://linkedin.example/ada"}},
			{Key: "resume", Required: true, Value: "resume-id"},
		},
		Metadata: map[string]string{"site": "acme", "posting_id": postingID, "api_host": "api.lever.co", "resume_id": "resume-id"},
	}
	result, err := provider.Submit(context.Background(), domain.Application{UserID: "user-id"}, form)
	if err != nil || result.Status != domain.StatusSubmitted || result.ExternalApplicationID != "lever-application-1" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestLeverSubmissionRateLimitIsRetryableAndValidationRequiresManualReview(t *testing.T) {
	limited := classifyHTTP(http.StatusTooManyRequests, true)
	if limited.Category != domain.ErrorRateLimited {
		t.Fatalf("429 category=%q", limited.Category)
	}
	validation := classifyHTTP(http.StatusBadRequest, true)
	if validation.Category != domain.ErrorManualRequired || strings.Contains(validation.Error(), "provider") == false {
		t.Fatalf("custom form validation must stop for manual review: %+v", validation)
	}
}

func TestSubmitDoesNotExposeCredentialInErrors(t *testing.T) {
	postingID := "39446629-e634-42b4-af62-39332fec714e"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"server response with secret"}`, http.StatusInternalServerError)
	}))
	defer server.Close()
	provider := NewWithHTTPClient(CredentialMap{"acme": "secret-key"}, resumeReaderFunc(func(context.Context, string, string) (ResumeAttachment, error) {
		return ResumeAttachment{Filename: "resume.pdf", ContentType: "application/pdf", Content: []byte("resume")}, nil
	}), server.Client(), server.URL)
	form := domain.PreparedForm{Fields: []domain.Field{{Key: "name", Required: true, Value: "Ada"}, {Key: "email", Required: true, Value: "ada@example.test"}}, Metadata: map[string]string{"site": "acme", "posting_id": postingID, "api_host": "api.lever.co", "resume_id": "resume-id"}}
	_, err := provider.Submit(context.Background(), domain.Application{UserID: "user-id"}, form)
	if err == nil || strings.Contains(err.Error(), "secret-key") || strings.Contains(err.Error(), "server response") {
		t.Fatalf("unsafe Lever error: %v", err)
	}
}
