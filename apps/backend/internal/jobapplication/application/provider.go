package application

import (
	"context"

	jobdomain "github.com/Hell077/HireRadar/apps/backend/internal/job/domain"
	"github.com/Hell077/HireRadar/apps/backend/internal/jobapplication/domain"
)

type ResumeAttachment struct {
	Filename    string
	ContentType string
	Content     []byte
}

type ProviderCapabilities struct {
	Discovery bool
	FetchJobs bool
	AutoApply bool
}

// Provider owns provider-specific form preparation and submission. Implementations
// must not turn unknown required answers into invented values.
type Provider interface {
	Name() domain.Provider
	Capabilities() ProviderCapabilities
	Supports(jobdomain.Job) bool
	Prepare(context.Context, jobdomain.Job, domain.ApplicationProfile) (domain.PreparedForm, error)
	Submit(context.Context, domain.Application, domain.PreparedForm) (domain.SubmissionResult, error)
}

type Registry interface {
	Resolve(jobdomain.Job) (Provider, bool)
}

type ProviderRegistry struct{ providers []Provider }

func NewProviderRegistry(providers ...Provider) *ProviderRegistry {
	return &ProviderRegistry{providers: append([]Provider(nil), providers...)}
}

func (r *ProviderRegistry) Resolve(job jobdomain.Job) (Provider, bool) {
	for _, provider := range r.providers {
		if provider != nil && provider.Capabilities().AutoApply && provider.Supports(job) {
			return provider, true
		}
	}
	return nil, false
}
