package lever

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
	"strings"
	"time"

	jobdomain "github.com/Hell077/HireRadar/apps/backend/internal/job/domain"
	"github.com/Hell077/HireRadar/apps/backend/internal/jobapplication/application"
	"github.com/Hell077/HireRadar/apps/backend/internal/jobapplication/domain"
	"github.com/google/uuid"
)

const maxResponseBytes = 4 << 20
const maxResumeBytes = 10 << 20

type Credentials interface{ APIKey(site string) string }

type CredentialMap map[string]string

func (c CredentialMap) APIKey(site string) string { return strings.TrimSpace(c[site]) }

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
	return NewWithHTTPClient(credentials, resumes, &http.Client{Timeout: 30 * time.Second}, "https://api.lever.co")
}

func NewWithHTTPClient(credentials Credentials, resumes ResumeReader, client *http.Client, baseURL string) *Provider {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	baseURL = strings.TrimRight(baseURL, "/")
	clientCopy := *client
	clientCopy.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) == 0 || request.URL.Host != via[0].URL.Host || request.URL.Scheme != via[0].URL.Scheme || len(via) > 3 {
			return errors.New("Lever redirect rejected")
		}
		return nil
	}
	return &Provider{client: &clientCopy, baseURL: baseURL, credentials: credentials, resumes: resumes}
}

func (p *Provider) Name() domain.Provider { return "lever" }

func (p *Provider) Capabilities() application.ProviderCapabilities {
	return application.ProviderCapabilities{Discovery: true, FetchJobs: true, AutoApply: p.credentials != nil && p.resumes != nil}
}

func (p *Provider) Supports(job jobdomain.Job) bool {
	site, _, _, err := parseApplyURL(job.ApplyURL)
	return err == nil && p.credentials != nil && p.resumes != nil && p.credentials.APIKey(site) != ""
}

func (p *Provider) Prepare(ctx context.Context, job jobdomain.Job, profile domain.ApplicationProfile) (domain.PreparedForm, error) {
	site, postingID, apiHost, err := parseApplyURL(job.ApplyURL)
	if err != nil {
		return domain.PreparedForm{}, &domain.ProviderError{Category: domain.ErrorUnsupported, Code: "invalid_lever_job_url"}
	}
	endpoint := p.endpoint(apiHost, "/v0/postings/"+url.PathEscape(site)+"/"+url.PathEscape(postingID))
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
		return domain.PreparedForm{}, classifyHTTP(response.StatusCode, false)
	}
	var posting posting
	if err := json.Unmarshal(body, &posting); err != nil {
		return domain.PreparedForm{}, &domain.ProviderError{Category: domain.ErrorProvider, Code: "invalid_job_form"}
	}
	if posting.ID != "" && posting.ID != postingID {
		return domain.PreparedForm{}, &domain.ProviderError{Category: domain.ErrorProvider, Code: "posting_identity_mismatch"}
	}
	name := strings.TrimSpace(profile.FirstName + " " + profile.LastName)
	urls := map[string]string{}
	if profile.LinkedInURL != "" {
		urls["LinkedIn"] = profile.LinkedInURL
	}
	if profile.GitHubURL != "" {
		urls["GitHub"] = profile.GitHubURL
	}
	if profile.WebsiteURL != "" {
		urls["Website"] = profile.WebsiteURL
	}
	form := domain.PreparedForm{
		Fields: []domain.Field{
			{Key: "name", Type: "text", Required: true, Value: optional(name)},
			{Key: "email", Type: "email", Required: true, Value: optional(profile.Email)},
			{Key: "phone", Type: "text", Value: optional(profile.Phone)},
			{Key: "urls", Type: "object", Value: urls},
			{Key: "resume", Type: "file", Required: true, Value: optional(profile.ResumeID)},
		},
		Questions: []domain.Question{},
		Metadata:  map[string]string{"site": site, "posting_id": postingID, "api_host": apiHost},
	}
	if profile.ResumeID != "" {
		form.Metadata["resume_id"] = profile.ResumeID
	}
	if name == "" {
		form.Questions = append(form.Questions, domain.Question{ExternalKey: "name", Text: "Full name required by the Lever application", Type: "text", Required: true})
	}
	if profile.Email == "" {
		form.Questions = append(form.Questions, domain.Question{ExternalKey: "email", Text: "Email address required by the Lever application", Type: "email", Required: true})
	}
	if profile.ResumeID == "" {
		form.Questions = append(form.Questions, domain.Question{ExternalKey: "resume", Text: "Select a resume for the Lever application", Type: "file", Required: true})
	}
	return form, nil
}

func (p *Provider) Submit(ctx context.Context, app domain.Application, form domain.PreparedForm) (domain.SubmissionResult, error) {
	site, postingID, apiHost := form.Metadata["site"], form.Metadata["posting_id"], form.Metadata["api_host"]
	if !validSite(site) || !validPostingID(postingID) || (apiHost != "api.lever.co" && apiHost != "api.eu.lever.co") {
		return domain.SubmissionResult{}, &domain.ProviderError{Category: domain.ErrorUnsupported, Code: "invalid_lever_posting"}
	}
	if p.credentials == nil || p.resumes == nil || p.credentials.APIKey(site) == "" {
		return domain.SubmissionResult{}, &domain.ProviderError{Category: domain.ErrorManualRequired, Code: "lever_credentials_unavailable"}
	}
	fields := make(map[string]any, len(form.Fields))
	for _, field := range form.Fields {
		if field.Key == "resume" {
			continue
		}
		if missing(field.Value) {
			if field.Required {
				return domain.SubmissionResult{}, &domain.ProviderError{Category: domain.ErrorMissingInput, Code: "required_application_field_missing"}
			}
			continue
		}
		fields[field.Key] = field.Value
	}
	name, _ := fields["name"].(string)
	email, _ := fields["email"].(string)
	if name == "" || email == "" {
		return domain.SubmissionResult{}, &domain.ProviderError{Category: domain.ErrorMissingInput, Code: "required_candidate_field_missing"}
	}
	resumeID := form.Metadata["resume_id"]
	if resumeID == "" {
		return domain.SubmissionResult{}, &domain.ProviderError{Category: domain.ErrorMissingInput, Code: "selected_resume_required"}
	}
	attachment, err := p.resumes.ReadResume(ctx, app.UserID, resumeID)
	if err != nil {
		return domain.SubmissionResult{}, &domain.ProviderError{Category: domain.ErrorMissingInput, Code: "selected_resume_unavailable"}
	}
	if len(attachment.Content) == 0 || len(attachment.Content) > maxResumeBytes || attachment.ContentType != "application/pdf" {
		return domain.SubmissionResult{}, &domain.ProviderError{Category: domain.ErrorValidation, Code: "invalid_resume_file"}
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		if key == "urls" {
			urls := make(map[string]string)
			switch raw := value.(type) {
			case map[string]string:
				urls = raw
			case map[string]any:
				for label, rawLink := range raw {
					if link, ok := rawLink.(string); ok {
						urls[label] = link
					}
				}
			default:
				return domain.SubmissionResult{}, &domain.ProviderError{Category: domain.ErrorValidation, Code: "invalid_profile_urls"}
			}
			for label, link := range urls {
				if link != "" {
					if err := writer.WriteField("urls["+label+"]", link); err != nil {
						return domain.SubmissionResult{}, &domain.ProviderError{Category: domain.ErrorValidation, Code: "invalid_application_form"}
					}
				}
			}
			continue
		}
		if err := writer.WriteField(key, fmt.Sprint(value)); err != nil {
			return domain.SubmissionResult{}, &domain.ProviderError{Category: domain.ErrorValidation, Code: "invalid_application_form"}
		}
	}
	file, err := writer.CreateFormFile("resume", attachment.Filename)
	if err != nil {
		return domain.SubmissionResult{}, &domain.ProviderError{Category: domain.ErrorValidation, Code: "invalid_resume_file"}
	}
	if _, err := file.Write(attachment.Content); err != nil {
		return domain.SubmissionResult{}, &domain.ProviderError{Category: domain.ErrorValidation, Code: "invalid_resume_file"}
	}
	if err := writer.Close(); err != nil {
		return domain.SubmissionResult{}, &domain.ProviderError{Category: domain.ErrorValidation, Code: "invalid_application_form"}
	}
	endpoint := p.endpoint(apiHost, "/v0/postings/"+url.PathEscape(site)+"/"+url.PathEscape(postingID))
	parsed, _ := url.Parse(endpoint)
	query := parsed.Query()
	query.Set("key", p.credentials.APIKey(site))
	parsed.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, parsed.String(), &body)
	if err != nil {
		return domain.SubmissionResult{}, &domain.ProviderError{Category: domain.ErrorValidation, Code: "invalid_application_request"}
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "HireRadar/1.0")
	response, err := p.client.Do(request)
	if err != nil {
		return domain.SubmissionResult{}, errors.New("Lever submission outcome is uncertain")
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil || len(responseBody) > maxResponseBytes {
		return domain.SubmissionResult{}, errors.New("Lever submission outcome is uncertain")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if response.StatusCode >= 500 {
			return domain.SubmissionResult{}, errors.New("Lever submission outcome is uncertain")
		}
		return domain.SubmissionResult{}, classifyHTTP(response.StatusCode, true)
	}
	var result struct {
		OK            bool   `json:"ok"`
		ApplicationID string `json:"applicationId"`
	}
	if err := json.Unmarshal(responseBody, &result); err != nil || !result.OK || result.ApplicationID == "" {
		return domain.SubmissionResult{}, errors.New("Lever submission result could not be confirmed")
	}
	return domain.SubmissionResult{Status: domain.StatusSubmitted, ExternalApplicationID: result.ApplicationID}, nil
}

type posting struct {
	ID         string `json:"id"`
	Text       string `json:"text"`
	ApplyURL   string `json:"applyUrl"`
	HostedURL  string `json:"hostedUrl"`
	Categories struct {
		Location string `json:"location"`
	} `json:"categories"`
}

func (p *Provider) endpoint(host, path string) string {
	if strings.HasPrefix(p.baseURL, "http://") || strings.HasPrefix(p.baseURL, "https://") {
		base, _ := url.Parse(p.baseURL)
		if base.Host != "api.lever.co" && base.Host != "api.eu.lever.co" {
			return strings.TrimRight(p.baseURL, "/") + path
		}
	}
	return "https://" + host + path
}

func parseApplyURL(raw string) (site, postingID, apiHost string, err error) {
	u, parseErr := url.Parse(raw)
	if parseErr != nil || u.Scheme != "https" || (u.Hostname() != "jobs.lever.co" && u.Hostname() != "jobs.eu.lever.co") || u.User != nil {
		return "", "", "", errors.New("not a Lever hosted job URL")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 || len(parts) > 3 || !validSite(parts[0]) || !validPostingID(parts[1]) || (len(parts) == 3 && parts[2] != "apply") {
		return "", "", "", errors.New("invalid Lever job URL")
	}
	host := "api.lever.co"
	if u.Hostname() == "jobs.eu.lever.co" {
		host = "api.eu.lever.co"
	}
	return parts[0], parts[1], host, nil
}

func validSite(value string) bool {
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

func validPostingID(value string) bool {
	return uuid.Validate(value) == nil
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

func classifyHTTP(status int, submitting bool) *domain.ProviderError {
	switch {
	case status == http.StatusTooManyRequests:
		return &domain.ProviderError{Category: domain.ErrorRateLimited, Code: "rate_limited"}
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return &domain.ProviderError{Category: domain.ErrorAuthentication, Code: "lever_authentication_required"}
	case status == http.StatusNotFound || status == http.StatusGone:
		return &domain.ProviderError{Category: domain.ErrorJobClosed, Code: "lever_job_unavailable"}
	case status >= 500:
		if submitting {
			return &domain.ProviderError{Category: domain.ErrorUnknown, Code: "lever_submission_uncertain"}
		}
		return &domain.ProviderError{Category: domain.ErrorTemporary, Code: "lever_provider_error"}
	case status == http.StatusBadRequest || status == http.StatusUnprocessableEntity:
		return &domain.ProviderError{Category: domain.ErrorManualRequired, Code: "lever_form_requires_review"}
	default:
		return &domain.ProviderError{Category: domain.ErrorProvider, Code: "lever_provider_rejected_request"}
	}
}

var _ application.Provider = (*Provider)(nil)
