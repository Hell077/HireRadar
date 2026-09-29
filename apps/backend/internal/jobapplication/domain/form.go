package domain

import "time"

type Field struct {
	Key      string `json:"key"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
	Value    any    `json:"value,omitempty"`
}

type Question struct {
	ExternalKey string   `json:"external_key,omitempty"`
	Text        string   `json:"text"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Options     []Option `json:"options,omitempty"`
}

type Option struct {
	Value any    `json:"value"`
	Label string `json:"label"`
}

type ApplicationQuestion struct {
	ID          string   `json:"id"`
	Application ID       `json:"application_id"`
	ExternalKey string   `json:"external_key,omitempty"`
	Text        string   `json:"question"`
	Type        string   `json:"question_type"`
	Required    bool     `json:"required"`
	Options     []Option `json:"options,omitempty"`
	Answer      any      `json:"answer,omitempty"`
	Status      string   `json:"status"`
}

type PreparedForm struct {
	Fields    []Field           `json:"fields"`
	Questions []Question        `json:"questions"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type SubmissionResult struct {
	Status                Status    `json:"status"`
	ExternalApplicationID string    `json:"external_application_id,omitempty"`
	ConfirmationURL       string    `json:"confirmation_url,omitempty"`
	SubmittedAt           time.Time `json:"submitted_at,omitempty"`
}

type ErrorCategory string

const (
	ErrorTemporary      ErrorCategory = "temporary"
	ErrorRateLimited    ErrorCategory = "rate_limited"
	ErrorValidation     ErrorCategory = "validation"
	ErrorMissingInput   ErrorCategory = "missing_input"
	ErrorManualRequired ErrorCategory = "manual_required"
	ErrorUnsupported    ErrorCategory = "unsupported"
	ErrorJobClosed      ErrorCategory = "job_closed"
	ErrorAuthentication ErrorCategory = "authentication"
	ErrorProvider       ErrorCategory = "provider_error"
	ErrorUnknown        ErrorCategory = "unknown"
)

type ProviderError struct {
	Category ErrorCategory
	Code     string
}

func (e *ProviderError) Error() string {
	if e == nil || e.Code == "" {
		return "application provider error"
	}
	return "application provider error: " + e.Code
}
