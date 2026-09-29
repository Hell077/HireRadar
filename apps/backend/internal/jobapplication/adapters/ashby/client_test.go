package ashby

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	jobdomain "github.com/Hell077/HireRadar/apps/backend/internal/job/domain"
	"github.com/Hell077/HireRadar/apps/backend/internal/jobapplication/application"
	"github.com/Hell077/HireRadar/apps/backend/internal/jobapplication/domain"
)

type resumeReaderFunc func(context.Context, string, string) (application.ResumeAttachment, error)

func (f resumeReaderFunc) ReadResume(ctx context.Context, userID, resumeID string) (application.ResumeAttachment, error) {
	return f(ctx, userID, resumeID)
}

func TestPrepareLoadsAshbyFormAndPersistsUnknownRequiredQuestion(t *testing.T) {
	postingID := "26a4281b-4ab2-4829-b664-7c43d7dbd409"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/jobPosting.info" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		user, password, ok := r.BasicAuth()
		if !ok || user != "authorized-key" || password != "" {
			t.Fatal("Ashby API key was not sent as Basic Auth")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"success":true,"results":{"id":"`+postingID+`","applicationFormDefinition":{"fields":[{"isRequired":true,"field":{"type":"String","path":"_systemfield_name","title":"Name"}},{"isRequired":true,"field":{"type":"Email","path":"_systemfield_email","title":"Email"}},{"isRequired":true,"field":{"type":"File","path":"_systemfield_resume","title":"Resume"}},{"isRequired":true,"field":{"type":"ValueSelect","path":"custom_choice","title":"Sponsorship choice","options":[{"value":1,"label":"Yes"},{"value":0,"label":"No"}]}},{"isRequired":true,"field":{"type":"Boolean","path":"custom_123","humanReadablePath":"Do you need sponsorship?","title":"Sponsorship"}}]}}}`)
	}))
	defer server.Close()
	provider := NewWithHTTPClient(CredentialMap{"acme": "authorized-key"}, resumeReaderFunc(func(context.Context, string, string) (application.ResumeAttachment, error) {
		return application.ResumeAttachment{}, nil
	}), server.Client(), server.URL)
	job := jobdomain.Job{ApplyURL: "https://jobs.ashbyhq.com/acme/" + postingID}
	if !provider.Supports(job) {
		t.Fatal("provider should support the configured Ashby organization")
	}
	form, err := provider.Prepare(context.Background(), job, domain.ApplicationProfile{FirstName: "Ada", LastName: "Lovelace", Email: "ada@example.test", ResumeID: "resume-id", CustomAnswers: map[string]string{"custom_choice": "No"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(form.Fields) != 5 || form.Fields[3].Value != float64(0) || len(form.Questions) != 1 || form.Questions[0].ExternalKey != "custom_123" || form.Metadata["resume_id"] != "resume-id" {
		t.Fatalf("prepared form=%+v", form)
	}
}

func TestSubmitUploadsResumeAndUsesExplicitFieldValues(t *testing.T) {
	postingID := "26a4281b-4ab2-4829-b664-7c43d7dbd409"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/applicationForm.submit" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		user, password, ok := r.BasicAuth()
		if !ok || user != "authorized-key" || password != "" {
			t.Error("Ashby API key was not sent as Basic Auth")
		}
		if err := r.ParseMultipartForm(maxResumeBytes); err != nil {
			t.Error(err)
			return
		}
		if r.FormValue("jobPostingId") != postingID {
			t.Errorf("job posting id=%q", r.FormValue("jobPostingId"))
		}
		var applicationForm struct {
			FieldSubmissions []struct {
				Path  string `json:"path"`
				Value any    `json:"value"`
			} `json:"fieldSubmissions"`
		}
		if err := json.Unmarshal([]byte(r.FormValue("applicationForm")), &applicationForm); err != nil {
			t.Error(err)
		}
		if len(applicationForm.FieldSubmissions) != 4 || applicationForm.FieldSubmissions[0].Value != "Ada Lovelace" || applicationForm.FieldSubmissions[3].Value != "resume_1" {
			t.Errorf("unexpected submitted values: %+v", applicationForm.FieldSubmissions)
		}
		file, _, err := r.FormFile("resume_1")
		if err != nil {
			t.Error(err)
		} else {
			defer file.Close()
			content, _ := io.ReadAll(file)
			if string(content) != "selected PDF" {
				t.Errorf("uploaded content=%q", content)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"success":true,"results":{"submittedFormInstance":{"id":"ashby-submission"}}}`)
	}))
	defer server.Close()
	provider := NewWithHTTPClient(CredentialMap{"acme": "authorized-key"}, resumeReaderFunc(func(_ context.Context, userID, resumeID string) (application.ResumeAttachment, error) {
		if userID != "user-id" || resumeID != "resume-id" {
			t.Errorf("resume owner/id=%q/%q", userID, resumeID)
		}
		return application.ResumeAttachment{Filename: "resume.pdf", ContentType: "application/pdf", Content: []byte("selected PDF")}, nil
	}), server.Client(), server.URL)
	form := domain.PreparedForm{Fields: []domain.Field{
		{Key: "_systemfield_name", Type: "String", Required: true, Value: "Ada Lovelace"},
		{Key: "_systemfield_email", Type: "Email", Required: true, Value: "ada@example.test"},
		{Key: "_systemfield_resume", Type: "File", Required: true, Value: "resume-id"},
		{Key: "custom_123", Type: "Boolean", Required: true, Value: true},
	}, Metadata: map[string]string{"org": "acme", "posting_id": postingID, "resume_id": "resume-id"}}
	result, err := provider.Submit(context.Background(), domain.Application{UserID: "user-id"}, form)
	if err != nil || result.Status != domain.StatusSubmitted || result.ExternalApplicationID != "ashby-submission" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestAshbySubmissionFailuresDoNotLeakSecrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "provider error with private response", http.StatusInternalServerError)
	}))
	defer server.Close()
	provider := NewWithHTTPClient(CredentialMap{"acme": "secret-key"}, resumeReaderFunc(func(context.Context, string, string) (application.ResumeAttachment, error) {
		return application.ResumeAttachment{Filename: "resume.pdf", ContentType: "application/pdf", Content: []byte("resume")}, nil
	}), server.Client(), server.URL)
	form := domain.PreparedForm{Fields: []domain.Field{{Key: "name", Type: "String", Required: true, Value: "Ada"}}, Metadata: map[string]string{"org": "acme", "posting_id": "26a4281b-4ab2-4829-b664-7c43d7dbd409", "resume_id": "resume-id"}}
	_, err := provider.Submit(context.Background(), domain.Application{UserID: "user-id"}, form)
	if err == nil || strings.Contains(err.Error(), "secret-key") || strings.Contains(err.Error(), "private response") {
		t.Fatalf("unsafe provider error: %v", err)
	}
}
