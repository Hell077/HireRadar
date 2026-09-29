package application

import (
	"context"
	"testing"

	jobdomain "github.com/Hell077/HireRadar/apps/backend/internal/job/domain"
	"github.com/Hell077/HireRadar/apps/backend/internal/jobapplication/domain"
)

type testProvider struct {
	name         domain.Provider
	capabilities ProviderCapabilities
	supports     bool
}

func (p testProvider) Name() domain.Provider              { return p.name }
func (p testProvider) Capabilities() ProviderCapabilities { return p.capabilities }
func (p testProvider) Supports(jobdomain.Job) bool        { return p.supports }
func (testProvider) Prepare(context.Context, jobdomain.Job, domain.ApplicationProfile) (domain.PreparedForm, error) {
	return domain.PreparedForm{}, nil
}
func (testProvider) Submit(context.Context, domain.Application, domain.PreparedForm) (domain.SubmissionResult, error) {
	return domain.SubmissionResult{}, nil
}

func TestRegistryOnlyResolvesAutoApplyProviders(t *testing.T) {
	job := jobdomain.Job{}
	registry := NewProviderRegistry(
		testProvider{name: "feed", capabilities: ProviderCapabilities{FetchJobs: true}, supports: true},
		testProvider{name: "greenhouse", capabilities: ProviderCapabilities{Discovery: true, FetchJobs: true, AutoApply: true}, supports: true},
	)
	provider, ok := registry.Resolve(job)
	if !ok || provider.Name() != "greenhouse" {
		t.Fatalf("resolved provider=%v, ok=%v", provider, ok)
	}
	if _, ok := NewProviderRegistry(testProvider{name: "unsupported", supports: true}).Resolve(job); ok {
		t.Fatal("provider without auto-apply capability was resolved")
	}
}
