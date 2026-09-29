package greenhouse

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	jobdomain "github.com/Hell077/HireRadar/apps/backend/internal/job/domain"
	"github.com/Hell077/HireRadar/apps/backend/internal/jobapplication/domain"
)

type resumeReaderFunc func(context.Context, string, string) (ResumeAttachment, error)

func (f resumeReaderFunc) ReadResume(ctx context.Context, userID, resumeID string) (ResumeAttachment, error) {
	return f(ctx, userID, resumeID)
}

func TestSupportsOnlyGreenhouseJobsWithAuthorizedCredential(t *testing.T) {
	provider := New(CredentialMap{"acme": "employer-key"}, resumeReaderFunc(func(context.Context, string, string) (ResumeAttachment, error) {
		return ResumeAttachment{}, nil
	}))
	if !provider.Supports(jobdomain.Job{ApplyURL: "https://boards.greenhouse.io/acme/jobs/123"}) {
		t.Fatal("configured Greenhouse job was not supported")
	}
	for _, applyURL := range []string{
		"https://boards.greenhouse.io.evil.example/acme/jobs/123",
		"http://boards.greenhouse.io/acme/jobs/123",
		"https://jobs.example/acme/jobs/123",
		"https://boards.greenhouse.io/acme/jobs/not-a-number",
	} {
		if provider.Supports(jobdomain.Job{ApplyURL: applyURL}) {
			t.Fatalf("unsafe or invalid apply URL supported: %s", applyURL)
		}
	}
	if New(nil, nil).Capabilities().AutoApply {
		t.Fatal("provider without credentials and resume support advertises auto-apply")
	}
}

func TestPrepareMapsProfileAndLeavesUnknownRequiredQuestionUnanswered(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/boards/acme/jobs/123" || r.URL.Query().Get("questions") != "true" {
			t.Fatalf("unexpected request: %s", r.URL.String())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"questions":[{"required":true,"label":"First Name","fields":[{"name":"first_name","type":"input_text"}]},{"required":true,"label":"Do you require visa sponsorship?","fields":[{"name":"question_987","type":"multi_value_single_select","values":[{"value":1,"label":"Yes"},{"value":0,"label":"No"}]}]}]}`)
	}))
	defer server.Close()
	provider := NewWithHTTPClient(CredentialMap{"acme": "key"}, resumeReaderFunc(func(context.Context, string, string) (ResumeAttachment, error) {
		return ResumeAttachment{}, nil
	}), server.Client(), server.URL)
	form, err := provider.Prepare(context.Background(), jobdomain.Job{ApplyURL: "https://boards.greenhouse.io/acme/jobs/123"}, domain.ApplicationProfile{FirstName: "Ada"})
	if err != nil {
		t.Fatal(err)
	}
	if len(form.Fields) != 2 || form.Fields[0].Value != "Ada" || form.Fields[1].Value != nil || len(form.Questions) != 1 || form.Questions[0].Text != "Do you require visa sponsorship?" {
		t.Fatalf("prepared form=%+v", form)
	}
	if len(form.Questions[0].Options) != 2 || form.Questions[0].Options[0] != (domain.Option{Value: float64(1), Label: "Yes"}) || form.Questions[0].Options[1] != (domain.Option{Value: float64(0), Label: "No"}) {
		t.Fatalf("Greenhouse answer options lost their submitted values: %+v", form.Questions[0].Options)
	}
}

func TestSubmitUsesEmployerCredentialAndUploadsSelectedResume(t *testing.T) {
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok || username != "employer-key" || password != "" {
			t.Error("Greenhouse employer API credential was not sent as Basic Auth")
		}
		if r.URL.Path != "/v1/boards/acme/jobs/123" || r.Method != http.MethodPost {
			t.Errorf("unexpected submission request %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Error(err)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()
	provider := NewWithHTTPClient(CredentialMap{"acme": "employer-key"}, resumeReaderFunc(func(_ context.Context, userID, resumeID string) (ResumeAttachment, error) {
		if userID != "user-1" || resumeID != "resume-1" {
			t.Fatalf("resume ownership lookup user=%q resume=%q", userID, resumeID)
		}
		return ResumeAttachment{Filename: "resume.pdf", ContentType: "application/pdf", Content: []byte("%PDF-1.7 test")}, nil
	}), server.Client(), server.URL)
	form := domain.PreparedForm{
		Metadata: map[string]string{"board": "acme", "job_id": "123", "resume_id": "resume-1"},
		Fields: []domain.Field{
			{Key: "first_name", Value: "Ada", Required: true},
			{Key: "email", Value: "ada@example.test", Required: true},
			{Key: "question_999", Value: "Yes", Required: true},
			{Key: "resume", Value: "resume-1", Required: true},
		},
	}
	result, err := provider.Submit(context.Background(), domain.Application{UserID: "user-1"}, form)
	if err != nil {
		t.Fatal(err)
	}
	resumeBytes, err := base64.StdEncoding.DecodeString(received["resume_content"].(string))
	if err != nil || string(resumeBytes) != "%PDF-1.7 test" || received["resume_content_filename"] != "resume.pdf" ||
		received["first_name"] != "Ada" || received["email"] != "ada@example.test" || received["question_999"] != "Yes" {
		t.Fatalf("submitted form mapping is invalid: %+v", received)
	}
	if result.Status != domain.StatusSubmitted || result.ConfirmationURL != "https://boards.greenhouse.io/acme/jobs/123" {
		t.Fatalf("submission result=%+v", result)
	}
}

func TestSubmitClassifiesAuthAndDoesNotRepeatUnknownNetworkOutcome(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) }))
	provider := NewWithHTTPClient(CredentialMap{"acme": "key"}, resumeReaderFunc(func(context.Context, string, string) (ResumeAttachment, error) {
		return ResumeAttachment{}, nil
	}), server.Client(), server.URL)
	form := domain.PreparedForm{Metadata: map[string]string{"board": "acme", "job_id": "123"}}
	_, err := provider.Submit(context.Background(), domain.Application{}, form)
	var providerErr *domain.ProviderError
	if !errors.As(err, &providerErr) || providerErr.Category != domain.ErrorAuthentication {
		t.Fatalf("auth error=%v", err)
	}
	server.Close()
	provider = NewWithHTTPClient(CredentialMap{"acme": "key"}, resumeReaderFunc(func(context.Context, string, string) (ResumeAttachment, error) {
		return ResumeAttachment{}, nil
	}), server.Client(), server.URL)
	_, err = provider.Submit(context.Background(), domain.Application{}, form)
	if err == nil || strings.Contains(err.Error(), "key") {
		t.Fatalf("expected sanitized uncertain submission error, got %v", err)
	}
}

func TestSubmissionRedirectCannotSendCredentialToAnotherHost(t *testing.T) {
	var redirected bool
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { redirected = true; w.WriteHeader(http.StatusOK) }))
	defer second.Close()
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Redirect(w, &http.Request{}, second.URL, http.StatusTemporaryRedirect)
	}))
	defer first.Close()
	provider := NewWithHTTPClient(CredentialMap{"acme": "key"}, resumeReaderFunc(func(context.Context, string, string) (ResumeAttachment, error) {
		return ResumeAttachment{}, nil
	}), first.Client(), first.URL)
	_, err := provider.Submit(context.Background(), domain.Application{}, domain.PreparedForm{Metadata: map[string]string{"board": "acme", "job_id": "123"}})
	if err == nil || redirected {
		t.Fatalf("redirect followed=%v err=%v", redirected, err)
	}
}

func TestPrepareClassifiesUnavailableProvider(t *testing.T) {
	provider := NewWithHTTPClient(CredentialMap{"acme": "key"}, resumeReaderFunc(func(context.Context, string, string) (ResumeAttachment, error) {
		return ResumeAttachment{}, nil
	}), &http.Client{Timeout: time.Second}, "http://127.0.0.1:1")
	_, err := provider.Prepare(context.Background(), jobdomain.Job{ApplyURL: "https://boards.greenhouse.io/acme/jobs/123"}, domain.ApplicationProfile{})
	var providerErr *domain.ProviderError
	if !errors.As(err, &providerErr) || providerErr.Category != domain.ErrorTemporary || fmt.Sprint(providerErr) == "" {
		t.Fatalf("provider availability error=%v", err)
	}
}
