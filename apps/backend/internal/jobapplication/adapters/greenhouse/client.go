package greenhouse

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	jobdomain "github.com/Hell077/HireRadar/apps/backend/internal/job/domain"
	"github.com/Hell077/HireRadar/apps/backend/internal/jobapplication/application"
	"github.com/Hell077/HireRadar/apps/backend/internal/jobapplication/domain"
)

const maxResponseBytes = 4 << 20
const maxResumeBytes = 10 << 20

type Credentials interface{ APIKey(board string) string }

type CredentialMap map[string]string

func (c CredentialMap) APIKey(board string) string { return strings.TrimSpace(c[board]) }

type ResumeAttachment = application.ResumeAttachment

type ResumeReader interface {
	ReadResume(context.Context, string, string) (ResumeAttachment, error)
}

type Provider struct {
	client      *http.Client
	baseURL     string
	credentials Credentials
	resumes     ResumeReader
}

func New(credentials Credentials, resumes ResumeReader) *Provider {
	return NewWithHTTPClient(credentials, resumes, &http.Client{Timeout: 30 * time.Second}, "https://boards-api.greenhouse.io")
}

func NewWithHTTPClient(credentials Credentials, resumes ResumeReader, client *http.Client, baseURL string) *Provider {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	baseURL = strings.TrimRight(baseURL, "/")
	base, _ := url.Parse(baseURL)
	clientCopy := *client
	clientCopy.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if request.URL.Host != base.Host || request.URL.Scheme != base.Scheme || len(via) > 3 {
			return errors.New("Greenhouse redirect rejected")
		}
		return nil
	}
	return &Provider{client: &clientCopy, baseURL: baseURL, credentials: credentials, resumes: resumes}
}

func (p *Provider) Name() domain.Provider { return "greenhouse" }

func (p *Provider) Capabilities() application.ProviderCapabilities {
	return application.ProviderCapabilities{Discovery: true, FetchJobs: true, AutoApply: p.credentials != nil && p.resumes != nil}
}

func (p *Provider) Supports(job jobdomain.Job) bool {
	board, _, err := parseApplyURL(job.ApplyURL)
	return err == nil && p.credentials != nil && p.resumes != nil && p.credentials.APIKey(board) != ""
}

func (p *Provider) Prepare(ctx context.Context, job jobdomain.Job, profile domain.ApplicationProfile) (domain.PreparedForm, error) {
	board, jobID, err := parseApplyURL(job.ApplyURL)
	if err != nil {
		return domain.PreparedForm{}, &domain.ProviderError{Category: domain.ErrorUnsupported, Code: "invalid_greenhouse_job_url"}
	}
	endpoint := p.baseURL + "/v1/boards/" + url.PathEscape(board) + "/jobs/" + jobID + "?questions=true"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return domain.PreparedForm{}, &domain.ProviderError{Category: domain.ErrorTemporary, Code: "request_failed"}
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "HireRadar/1.0")
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
	var posting greenhouseJob
	if err := json.Unmarshal(body, &posting); err != nil {
		return domain.PreparedForm{}, &domain.ProviderError{Category: domain.ErrorProvider, Code: "invalid_job_form"}
	}
	form := domain.PreparedForm{Fields: []domain.Field{}, Questions: []domain.Question{}, Metadata: map[string]string{"board": board, "job_id": jobID}}
	if profile.ResumeID != "" {
		form.Metadata["resume_id"] = profile.ResumeID
	}
	for _, question := range append(append(posting.Questions, posting.LocationQuestions...), posting.ComplianceQuestions...) {
		appendQuestion(&form, question, profile)
	}
	for _, item := range posting.DataCompliance {
		if item.RequiresConsent || item.RequiresProcessingConsent || item.RequiresRetentionConsent || item.RequiresDemographicConsent {
			appendQuestion(&form, greenhouseQuestion{Required: true, Label: "Greenhouse data processing consent", Fields: []greenhouseField{{Name: "data_compliance[" + consentField(item) + "]", Type: "input_checkbox"}}}, profile)
		}
	}
	if posting.DemographicQuestions != nil {
		for _, question := range posting.DemographicQuestions.Questions {
			if question.Required {
				fields := []greenhouseField{{Name: "demographic_answers[" + strconv.Itoa(question.ID) + "]", Type: question.Type}}
				appendQuestion(&form, greenhouseQuestion{Required: true, Label: question.Label, Fields: fields}, profile)
			}
		}
	}
	return form, nil
}

func (p *Provider) Submit(ctx context.Context, app domain.Application, form domain.PreparedForm) (domain.SubmissionResult, error) {
	board, ok := form.Metadata["board"]
	if !ok || !validToken(board) {
		return domain.SubmissionResult{}, &domain.ProviderError{Category: domain.ErrorUnsupported, Code: "invalid_greenhouse_board"}
	}
	jobID := form.Metadata["job_id"]
	if !decimalID(jobID) {
		return domain.SubmissionResult{}, &domain.ProviderError{Category: domain.ErrorUnsupported, Code: "invalid_greenhouse_job_id"}
	}
	if p.credentials == nil || p.resumes == nil || p.credentials.APIKey(board) == "" {
		return domain.SubmissionResult{}, &domain.ProviderError{Category: domain.ErrorManualRequired, Code: "greenhouse_credentials_unavailable"}
	}
	payload := map[string]any{"id": jobID}
	for _, field := range form.Fields {
		if missing(field.Value) {
			if field.Required {
				return domain.SubmissionResult{}, &domain.ProviderError{Category: domain.ErrorMissingInput, Code: "required_application_field_missing"}
			}
			continue
		}
		if field.Key == "resume" || field.Key == "resume_text" {
			continue
		}
		payload[field.Key] = field.Value
	}
	if resumeID := form.Metadata["resume_id"]; resumeID != "" {
		attachment, err := p.resumes.ReadResume(ctx, app.UserID, resumeID)
		if err != nil {
			return domain.SubmissionResult{}, &domain.ProviderError{Category: domain.ErrorMissingInput, Code: "selected_resume_unavailable"}
		}
		if len(attachment.Content) == 0 || int64(len(attachment.Content)) > maxResumeBytes || attachment.ContentType != "application/pdf" {
			return domain.SubmissionResult{}, &domain.ProviderError{Category: domain.ErrorValidation, Code: "invalid_resume_file"}
		}
		payload["resume_content"] = base64.StdEncoding.EncodeToString(attachment.Content)
		payload["resume_content_filename"] = attachment.Filename
	} else if formHasRequiredResume(form) {
		return domain.SubmissionResult{}, &domain.ProviderError{Category: domain.ErrorMissingInput, Code: "resume_required"}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return domain.SubmissionResult{}, &domain.ProviderError{Category: domain.ErrorValidation, Code: "invalid_application_form"}
	}
	endpoint := p.baseURL + "/v1/boards/" + url.PathEscape(board) + "/jobs/" + jobID
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return domain.SubmissionResult{}, &domain.ProviderError{Category: domain.ErrorValidation, Code: "invalid_application_request"}
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "HireRadar/1.0")
	request.SetBasicAuth(p.credentials.APIKey(board), "")
	response, err := p.client.Do(request)
	if err != nil {
		// The provider may have received the body before the network failed.
		// The worker treats unclassified submit errors as uncertain and will not resubmit.
		return domain.SubmissionResult{}, errors.New("Greenhouse submission outcome is uncertain")
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxResponseBytes))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if response.StatusCode >= 500 {
			return domain.SubmissionResult{}, errors.New("Greenhouse submission outcome is uncertain")
		}
		return domain.SubmissionResult{}, classifyHTTP(response.StatusCode)
	}
	return domain.SubmissionResult{Status: domain.StatusSubmitted, ConfirmationURL: jobApplyURL(board, jobID)}, nil
}

type greenhouseJob struct {
	Questions           []greenhouseQuestion `json:"questions"`
	LocationQuestions   []greenhouseQuestion `json:"location_questions"`
	ComplianceQuestions []greenhouseQuestion `json:"compliance"`
	DataCompliance      []struct {
		RequiresConsent            bool `json:"requires_consent"`
		RequiresProcessingConsent  bool `json:"requires_processing_consent"`
		RequiresRetentionConsent   bool `json:"requires_retention_consent"`
		RequiresDemographicConsent bool `json:"requires_demographic_data_consent"`
	} `json:"data_compliance"`
	DemographicQuestions *struct {
		Questions []struct {
			ID       int    `json:"id"`
			Label    string `json:"label"`
			Required bool   `json:"required"`
			Type     string `json:"type"`
		} `json:"questions"`
	} `json:"demographic_questions"`
}

type greenhouseQuestion struct {
	Required bool              `json:"required"`
	Label    string            `json:"label"`
	Fields   []greenhouseField `json:"fields"`
}

type greenhouseField struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Values []struct {
		Value any    `json:"value"`
		Label string `json:"label"`
	} `json:"values"`
}

func appendQuestion(form *domain.PreparedForm, question greenhouseQuestion, profile domain.ApplicationProfile) {
	if strings.TrimSpace(question.Label) == "" {
		question.Label = "Required application information"
	}
	options := []domain.Option{}
	missingRequired := false
	for _, field := range question.Fields {
		if field.Name == "" {
			continue
		}
		value, hasValue := profileValue(field.Name, question.Label, field.Type, field.Values, profile)
		if field.Name == "resume" && profile.ResumeID != "" {
			value, hasValue = profile.ResumeID, true
		}
		form.Fields = append(form.Fields, domain.Field{Key: field.Name, Type: field.Type, Required: question.Required, Value: value})
		if !hasValue && question.Required {
			missingRequired = true
			for _, option := range field.Values {
				options = append(options, domain.Option{Value: option.Value, Label: option.Label})
			}
		}
	}
	if question.Required && missingRequired && len(question.Fields) > 0 {
		form.Questions = append(form.Questions, domain.Question{
			ExternalKey: question.Fields[0].Name, Text: question.Label,
			Type: question.Fields[0].Type, Required: true, Options: options,
		})
	}
}

func profileValue(key, label, kind string, options []struct {
	Value any    `json:"value"`
	Label string `json:"label"`
}, profile domain.ApplicationProfile) (any, bool) {
	var candidate any
	switch key {
	case "first_name":
		candidate = profile.FirstName
	case "last_name":
		candidate = profile.LastName
	case "email":
		candidate = profile.Email
	case "phone":
		candidate = profile.Phone
	case "location":
		parts := []string{profile.Address, profile.City, profile.Country}
		candidate = strings.Trim(strings.Join(nonEmpty(parts), ", "), ", ")
	case "latitude":
		if profile.Latitude != nil {
			candidate = strconv.FormatFloat(*profile.Latitude, 'f', -1, 64)
		}
	case "longitude":
		if profile.Longitude != nil {
			candidate = strconv.FormatFloat(*profile.Longitude, 'f', -1, 64)
		}
	case "country_short_name":
		candidate = profile.Country
	case "linkedin_url":
		candidate = profile.LinkedInURL
	case "github_url":
		candidate = profile.GitHubURL
	case "website_url":
		candidate = profile.WebsiteURL
	default:
		if value, ok := profile.CustomAnswers[key]; ok {
			candidate = value
		} else if value, ok := profile.CustomAnswers[strings.TrimSpace(label)]; ok {
			candidate = value
		}
	}
	if missing(candidate) {
		return nil, false
	}
	if kind == "multi_value_single_select" || kind == "multi_value_multi_select" {
		for _, option := range options {
			if strings.EqualFold(fmt.Sprint(candidate), option.Label) || fmt.Sprint(candidate) == fmt.Sprint(option.Value) {
				return option.Value, true
			}
		}
		return nil, false
	}
	return candidate, true
}

func missing(value any) bool {
	if value == nil {
		return true
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text) == ""
	}
	return false
}

func formHasRequiredResume(form domain.PreparedForm) bool {
	for _, field := range form.Fields {
		if field.Key == "resume" && field.Required {
			return true
		}
	}
	return false
}

func parseApplyURL(raw string) (string, string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || (u.Hostname() != "boards.greenhouse.io" && u.Hostname() != "job-boards.greenhouse.io") || u.User != nil {
		return "", "", errors.New("not a Greenhouse hosted job URL")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 3 || parts[len(parts)-2] != "jobs" || !validToken(parts[0]) || !decimalID(parts[len(parts)-1]) {
		return "", "", errors.New("invalid Greenhouse job URL")
	}
	return parts[0], parts[len(parts)-1], nil
}

func validToken(value string) bool {
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

func decimalID(value string) bool {
	if value == "" {
		return false
	}
	_, err := strconv.ParseUint(value, 10, 64)
	return err == nil
}

func classifyHTTP(status int) *domain.ProviderError {
	switch {
	case status == http.StatusTooManyRequests:
		return &domain.ProviderError{Category: domain.ErrorRateLimited, Code: "rate_limited"}
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return &domain.ProviderError{Category: domain.ErrorAuthentication, Code: "greenhouse_authentication_required"}
	case status == http.StatusNotFound || status == http.StatusGone:
		return &domain.ProviderError{Category: domain.ErrorJobClosed, Code: "greenhouse_job_unavailable"}
	case status >= 500:
		return &domain.ProviderError{Category: domain.ErrorUnknown, Code: "greenhouse_submission_uncertain"}
	default:
		return &domain.ProviderError{Category: domain.ErrorValidation, Code: "greenhouse_rejected_application"}
	}
}

func consentField(item struct {
	RequiresConsent            bool `json:"requires_consent"`
	RequiresProcessingConsent  bool `json:"requires_processing_consent"`
	RequiresRetentionConsent   bool `json:"requires_retention_consent"`
	RequiresDemographicConsent bool `json:"requires_demographic_data_consent"`
}) string {
	if item.RequiresProcessingConsent {
		return "gdpr_processing_consent_given"
	}
	if item.RequiresRetentionConsent {
		return "gdpr_retention_consent_given"
	}
	if item.RequiresDemographicConsent {
		return "gdpr_demographic_data_consent_given"
	}
	return "gdpr_consent_given"
}

func jobApplyURL(board, jobID string) string {
	return "https://boards.greenhouse.io/" + url.PathEscape(board) + "/jobs/" + jobID
}

func nonEmpty(parts []string) []string {
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			values = append(values, strings.TrimSpace(part))
		}
	}
	return values
}

var _ application.Provider = (*Provider)(nil)
