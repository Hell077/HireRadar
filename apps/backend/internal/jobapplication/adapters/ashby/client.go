package ashby

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	jobdomain "github.com/Hell077/HireRadar/apps/backend/internal/job/domain"
	"github.com/Hell077/HireRadar/apps/backend/internal/jobapplication/application"
	"github.com/Hell077/HireRadar/apps/backend/internal/jobapplication/domain"
	"github.com/google/uuid"
)

const maxResponseBytes = 4 << 20
const maxResumeBytes = 10 << 20

type Credentials interface{ APIKey(org string) string }
type CredentialMap map[string]string

func (c CredentialMap) APIKey(org string) string { return strings.TrimSpace(c[org]) }

type ResumeReader interface {
	ReadResume(context.Context, string, string) (application.ResumeAttachment, error)
}

type Provider struct {
	client      *http.Client
	baseURL     string
	credentials Credentials
	resumes     ResumeReader
}

func New(credentials Credentials, resumes ResumeReader) *Provider {
	return NewWithHTTPClient(credentials, resumes, &http.Client{Timeout: 30 * time.Second}, "https://api.ashbyhq.com")
}

func NewWithHTTPClient(credentials Credentials, resumes ResumeReader, client *http.Client, baseURL string) *Provider {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	baseURL = strings.TrimRight(baseURL, "/")
	clientCopy := *client
	clientCopy.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) == 0 || request.URL.Host != via[0].URL.Host || request.URL.Scheme != via[0].URL.Scheme || len(via) > 3 {
			return errors.New("Ashby redirect rejected")
		}
		return nil
	}
	return &Provider{client: &clientCopy, baseURL: baseURL, credentials: credentials, resumes: resumes}
}

func (p *Provider) Name() domain.Provider { return "ashby" }

func (p *Provider) Capabilities() application.ProviderCapabilities {
	return application.ProviderCapabilities{Discovery: true, FetchJobs: true, AutoApply: p.credentials != nil && p.resumes != nil}
}

func (p *Provider) Supports(job jobdomain.Job) bool {
	org, _, err := parseApplyURL(job.ApplyURL)
	return err == nil && p.credentials != nil && p.resumes != nil && p.credentials.APIKey(org) != ""
}

func (p *Provider) Prepare(ctx context.Context, job jobdomain.Job, profile domain.ApplicationProfile) (domain.PreparedForm, error) {
	org, postingID, err := parseApplyURL(job.ApplyURL)
	if err != nil {
		return domain.PreparedForm{}, &domain.ProviderError{Category: domain.ErrorUnsupported, Code: "invalid_ashby_job_url"}
	}
	requestBody, _ := json.Marshal(map[string]string{"jobPostingId": postingID})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint("/jobPosting.info"), bytes.NewReader(requestBody))
	if err != nil {
		return domain.PreparedForm{}, &domain.ProviderError{Category: domain.ErrorTemporary, Code: "request_failed"}
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json; version=1")
	request.Header.Set("User-Agent", "HireRadar/1.0")
	request.SetBasicAuth(p.credentials.APIKey(org), "")
	response, err := p.client.Do(request)
	if err != nil {
		return domain.PreparedForm{}, &domain.ProviderError{Category: domain.ErrorTemporary, Code: "provider_unavailable"}
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil || len(body) > maxResponseBytes {
		return domain.PreparedForm{}, &domain.ProviderError{Category: domain.ErrorTemporary, Code: "invalid_provider_response"}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return domain.PreparedForm{}, classifyHTTP(response.StatusCode)
	}
	var posting postingInfo
	if err := json.Unmarshal(body, &posting); err != nil || !posting.Success {
		return domain.PreparedForm{}, &domain.ProviderError{Category: domain.ErrorProvider, Code: "invalid_job_form"}
	}
	if posting.Results.ID != "" && posting.Results.ID != postingID {
		return domain.PreparedForm{}, &domain.ProviderError{Category: domain.ErrorProvider, Code: "posting_identity_mismatch"}
	}
	form := domain.PreparedForm{Fields: []domain.Field{}, Questions: []domain.Question{}, Metadata: map[string]string{"org": org, "posting_id": postingID}}
	for _, definition := range posting.Results.ApplicationFormDefinition.Fields {
		path := definition.Field.Path
		if path == "" {
			return domain.PreparedForm{}, &domain.ProviderError{Category: domain.ErrorUnsupported, Code: "unsupported_ashby_form_field"}
		}
		value := profileValue(path, definition.Field.Title, definition.Field.Type, definition.Field.Options, profile)
		if definition.IsRequired && definition.Field.Type == "MultiValueSelect" {
			return domain.PreparedForm{}, &domain.ProviderError{Category: domain.ErrorManualRequired, Code: "unsupported_ashby_multi_select"}
		}
		if path == "_systemfield_resume" && profile.ResumeID != "" {
			value = profile.ResumeID
			form.Metadata["resume_id"] = profile.ResumeID
		}
		field := domain.Field{Key: path, Type: definition.Field.Type, Required: definition.IsRequired, Value: value}
		form.Fields = append(form.Fields, field)
		if definition.IsRequired && missing(value) {
			text := definition.Field.HumanReadablePath
			if text == "" {
				text = definition.Field.Title
			}
			if text == "" {
				text = "Required application information"
			}
			form.Questions = append(form.Questions, domain.Question{ExternalKey: path, Text: text, Type: definition.Field.Type, Required: true, Options: definition.Field.Options})
		}
	}
	return form, nil
}

func (p *Provider) Submit(ctx context.Context, app domain.Application, form domain.PreparedForm) (domain.SubmissionResult, error) {
	org, postingID := form.Metadata["org"], form.Metadata["posting_id"]
	if !validOrg(org) || uuid.Validate(postingID) != nil {
		return domain.SubmissionResult{}, &domain.ProviderError{Category: domain.ErrorUnsupported, Code: "invalid_ashby_posting"}
	}
	if p.credentials == nil || p.resumes == nil || p.credentials.APIKey(org) == "" {
		return domain.SubmissionResult{}, &domain.ProviderError{Category: domain.ErrorManualRequired, Code: "ashby_credentials_unavailable"}
	}
	type fieldSubmission struct {
		Path  string `json:"path"`
		Value any    `json:"value"`
	}
	submissions := make([]fieldSubmission, 0, len(form.Fields))
	resumeField := ""
	resumeRequired := false
	var resume application.ResumeAttachment
	for _, field := range form.Fields {
		if field.Type == "File" {
			if resumeField != "" {
				return domain.SubmissionResult{}, &domain.ProviderError{Category: domain.ErrorManualRequired, Code: "multiple_ashby_file_fields"}
			}
			resumeField = field.Key
			resumeRequired = field.Required
			continue
		}
		if missing(field.Value) {
			if field.Required {
				return domain.SubmissionResult{}, &domain.ProviderError{Category: domain.ErrorMissingInput, Code: "required_application_field_missing"}
			}
			continue
		}
		submissions = append(submissions, fieldSubmission{Path: field.Key, Value: field.Value})
	}
	resumeID := form.Metadata["resume_id"]
	if resumeField != "" && resumeID == "" && resumeRequired {
		return domain.SubmissionResult{}, &domain.ProviderError{Category: domain.ErrorMissingInput, Code: "selected_resume_required"}
	}
	if resumeField != "" && resumeID != "" {
		attachment, err := p.resumes.ReadResume(ctx, app.UserID, resumeID)
		if err != nil {
			return domain.SubmissionResult{}, &domain.ProviderError{Category: domain.ErrorMissingInput, Code: "selected_resume_unavailable"}
		}
		if len(attachment.Content) == 0 || len(attachment.Content) > maxResumeBytes || attachment.ContentType != "application/pdf" {
			return domain.SubmissionResult{}, &domain.ProviderError{Category: domain.ErrorValidation, Code: "invalid_resume_file"}
		}
		resume = attachment
		submissions = append(submissions, fieldSubmission{Path: resumeField, Value: "resume_1"})
	}
	applicationForm, err := json.Marshal(map[string]any{"fieldSubmissions": submissions})
	if err != nil {
		return domain.SubmissionResult{}, &domain.ProviderError{Category: domain.ErrorValidation, Code: "invalid_application_form"}
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("jobPostingId", postingID); err != nil {
		return domain.SubmissionResult{}, &domain.ProviderError{Category: domain.ErrorValidation, Code: "invalid_application_form"}
	}
	if err := writer.WriteField("applicationForm", string(applicationForm)); err != nil {
		return domain.SubmissionResult{}, &domain.ProviderError{Category: domain.ErrorValidation, Code: "invalid_application_form"}
	}
	if len(resume.Content) > 0 {
		file, err := writer.CreateFormFile("resume_1", resume.Filename)
		if err != nil {
			return domain.SubmissionResult{}, &domain.ProviderError{Category: domain.ErrorValidation, Code: "invalid_resume_file"}
		}
		if _, err := file.Write(resume.Content); err != nil {
			return domain.SubmissionResult{}, &domain.ProviderError{Category: domain.ErrorValidation, Code: "invalid_resume_file"}
		}
	}
	if err := writer.Close(); err != nil {
		return domain.SubmissionResult{}, &domain.ProviderError{Category: domain.ErrorValidation, Code: "invalid_application_form"}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint("/applicationForm.submit"), &body)
	if err != nil {
		return domain.SubmissionResult{}, &domain.ProviderError{Category: domain.ErrorValidation, Code: "invalid_application_request"}
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Accept", "application/json; version=1")
	request.Header.Set("User-Agent", "HireRadar/1.0")
	request.SetBasicAuth(p.credentials.APIKey(org), "")
	response, err := p.client.Do(request)
	if err != nil {
		return domain.SubmissionResult{}, errors.New("Ashby submission outcome is uncertain")
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil || len(responseBody) > maxResponseBytes {
		return domain.SubmissionResult{}, errors.New("Ashby submission outcome is uncertain")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if response.StatusCode >= 500 {
			return domain.SubmissionResult{}, errors.New("Ashby submission outcome is uncertain")
		}
		return domain.SubmissionResult{}, classifyHTTP(response.StatusCode)
	}
	var result submitResponse
	if err := json.Unmarshal(responseBody, &result); err != nil || !result.Success {
		return domain.SubmissionResult{}, errors.New("Ashby submission outcome could not be confirmed")
	}
	if result.Blocked || result.Results.Blocked {
		return domain.SubmissionResult{}, &domain.ProviderError{Category: domain.ErrorValidation, Code: "ashby_application_blocked"}
	}
	externalID := result.Results.SubmittedFormInstance.ID
	return domain.SubmissionResult{Status: domain.StatusSubmitted, ExternalApplicationID: externalID, ConfirmationURL: "https://jobs.ashbyhq.com/" + url.PathEscape(org) + "/" + url.PathEscape(postingID)}, nil
}

type postingInfo struct {
	Success bool `json:"success"`
	Results struct {
		ID                        string `json:"id"`
		ApplicationFormDefinition struct {
			Fields []fieldDefinition `json:"fields"`
		} `json:"applicationFormDefinition"`
	} `json:"results"`
}

type fieldDefinition struct {
	IsRequired bool `json:"isRequired"`
	Field      struct {
		Type              string          `json:"type"`
		Path              string          `json:"path"`
		HumanReadablePath string          `json:"humanReadablePath"`
		Title             string          `json:"title"`
		Options           []domain.Option `json:"options"`
	} `json:"field"`
}

type submitResponse struct {
	Success bool `json:"success"`
	Blocked bool `json:"blocked"`
	Results struct {
		Blocked               bool `json:"blocked"`
		SubmittedFormInstance struct {
			ID string `json:"id"`
		} `json:"submittedFormInstance"`
	} `json:"results"`
}

func (p *Provider) endpoint(path string) string {
	if p.baseURL != "https://api.ashbyhq.com" {
		return p.baseURL + path
	}
	return "https://api.ashbyhq.com" + path
}

func parseApplyURL(raw string) (string, string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() != "jobs.ashbyhq.com" || u.User != nil {
		return "", "", errors.New("not an Ashby hosted job URL")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 || len(parts) > 3 || !validOrg(parts[0]) || uuid.Validate(parts[1]) != nil || (len(parts) == 3 && parts[2] != "application") {
		return "", "", errors.New("invalid Ashby job URL")
	}
	return parts[0], parts[1], nil
}

func validOrg(value string) bool {
	if value == "" || len(value) > 120 {
		return false
	}
	for _, r := range value {
		if !(r == '-' || r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

func profileValue(path, title, fieldType string, options []domain.Option, profile domain.ApplicationProfile) any {
	var value any
	switch path {
	case "_systemfield_name":
		value = optional(strings.TrimSpace(profile.FirstName + " " + profile.LastName))
	case "_systemfield_email":
		value = optional(profile.Email)
	case "_systemfield_phone":
		value = optional(profile.Phone)
	case "_systemfield_linkedin":
		value = optional(profile.LinkedInURL)
	case "_systemfield_website":
		value = optional(profile.WebsiteURL)
	default:
		if answer, ok := profile.CustomAnswers[path]; ok {
			value = optional(answer)
		} else if answer, ok := profile.CustomAnswers[title]; ok {
			value = optional(answer)
		}
	}
	if missing(value) {
		return nil
	}
	switch fieldType {
	case "ValueSelect":
		for _, option := range options {
			candidate := fmt.Sprint(value)
			if candidate == fmt.Sprint(option.Value) || strings.EqualFold(candidate, option.Label) {
				return option.Value
			}
		}
		return nil
	case "Boolean":
		if boolean, ok := value.(bool); ok {
			return boolean
		}
		parsed, err := strconv.ParseBool(fmt.Sprint(value))
		if err != nil {
			return nil
		}
		return parsed
	case "Number":
		if _, ok := value.(float64); ok {
			return value
		}
		parsed, err := strconv.ParseFloat(fmt.Sprint(value), 64)
		if err != nil {
			return nil
		}
		return parsed
	}
	return value
}

func optional(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func missing(value any) bool {
	if value == nil {
		return true
	}
	text, ok := value.(string)
	return ok && strings.TrimSpace(text) == ""
}

func classifyHTTP(status int) *domain.ProviderError {
	switch {
	case status == http.StatusTooManyRequests:
		return &domain.ProviderError{Category: domain.ErrorRateLimited, Code: "rate_limited"}
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return &domain.ProviderError{Category: domain.ErrorAuthentication, Code: "ashby_authentication_required"}
	case status == http.StatusNotFound || status == http.StatusGone:
		return &domain.ProviderError{Category: domain.ErrorJobClosed, Code: "ashby_job_unavailable"}
	case status == http.StatusBadRequest || status == http.StatusUnprocessableEntity:
		return &domain.ProviderError{Category: domain.ErrorManualRequired, Code: "ashby_form_requires_review"}
	default:
		return &domain.ProviderError{Category: domain.ErrorProvider, Code: fmt.Sprintf("ashby_http_%d", status)}
	}
}

var _ application.Provider = (*Provider)(nil)
