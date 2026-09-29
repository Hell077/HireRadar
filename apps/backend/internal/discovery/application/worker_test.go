package application

import (
	"context"
	"errors"
	"testing"

	"github.com/Hell077/HireRadar/apps/backend/internal/discovery/domain"
	sourcedomain "github.com/Hell077/HireRadar/apps/backend/internal/source/domain"
)

type candidateStore struct {
	state, provider, sourceID string
	created                   bool
	invalid                   bool
	createCalls               int
}

func (s *candidateStore) ClaimDueSources(context.Context, int) ([]domain.DiscoverySource, error) {
	return nil, nil
}
func (s *candidateStore) StartRun(context.Context, string) (string, error) { return "run", nil }
func (s *candidateStore) SaveTargets(context.Context, string, []domain.DiscoveredTarget) (domain.RunStats, error) {
	return domain.RunStats{}, nil
}
func (s *candidateStore) FinishRun(context.Context, domain.DiscoverySource, string, domain.RepositorySnapshot, domain.RunStats, error) error {
	return nil
}
func (s *candidateStore) ClaimCandidates(context.Context, int) ([]domain.Candidate, error) {
	return nil, nil
}
func (s *candidateStore) CandidateUnavailable(context.Context, string, error) error { return nil }
func (s *candidateStore) CandidateReview(_ context.Context, _ string, provider domain.DetectedProvider, _ string) error {
	s.state, s.provider = "review", provider.Type
	return nil
}
func (s *candidateStore) CandidateInvalid(_ context.Context, _ string, provider domain.DetectedProvider, _ error) error {
	s.invalid, s.provider = true, provider.Type
	return nil
}
func (s *candidateStore) CreateOrLinkSource(context.Context, string) (string, bool, error) {
	s.createCalls++
	s.state = "source_created"
	return s.sourceID, s.created, nil
}
func (s *candidateStore) ScheduleRun(context.Context, string) error { return nil }
func (s *candidateStore) Overview(context.Context) (domain.Overview, error) {
	return domain.Overview{}, nil
}

type fixedResolver struct {
	result domain.Resolution
	err    error
}

func (f fixedResolver) Resolve(context.Context, domain.Candidate) (domain.Resolution, error) {
	return f.result, f.err
}

type fixedATS struct {
	result sourcedomain.FetchResult
	err    error
	calls  int
}

type fixedIssueBoardVerifier struct {
	result domain.GitHubRepositoryVerification
	err    error
}

func (f fixedIssueBoardVerifier) Snapshot(context.Context, domain.DiscoverySource) (domain.RepositorySnapshot, error) {
	return domain.RepositorySnapshot{}, nil
}
func (f fixedIssueBoardVerifier) VerifyJobRepository(context.Context, string, string) (domain.GitHubRepositoryVerification, error) {
	return f.result, f.err
}

func (f *fixedATS) Fetch(context.Context, sourcedomain.Source) (sourcedomain.FetchResult, error) {
	f.calls++
	return f.result, f.err
}

func TestVerifiedEmptyBoardCanBeRegisteredThroughExistingConnector(t *testing.T) {
	store := &candidateStore{sourceID: "discovered-greenhouse-example-deadbeef", created: true}
	ats := &fixedATS{result: sourcedomain.FetchResult{Jobs: []sourcedomain.ExternalJob{}}}
	worker := NewWorker(store, nil, nil, ats, 1)
	candidate := domain.Candidate{ID: "candidate", Kind: domain.CompanyTarget, Name: "Example", WebsiteURL: "https://example.org", CareersURL: "https://example.org/careers"}
	provider := domain.DetectedProvider{Type: "greenhouse", Key: "example", URL: "https://boards.greenhouse.io/example", Confidence: 0.99}
	worker.resolver = fixedResolver{result: domain.Resolution{Provider: &provider}}
	if err := worker.resolveCandidate(context.Background(), candidate); err != nil {
		t.Fatal(err)
	}
	if ats.calls != 1 || store.createCalls != 1 || store.state != "source_created" {
		t.Fatalf("verification calls=%d source registrations=%d state=%s", ats.calls, store.createCalls, store.state)
	}
}

func TestInvalidATSBoardDoesNotCreateSource(t *testing.T) {
	store := &candidateStore{}
	ats := &fixedATS{err: errors.New("ATS returned HTTP 404")}
	provider := domain.DetectedProvider{Type: "lever", Key: "missing-board", URL: "https://jobs.lever.co/missing-board", Confidence: 0.99}
	worker := NewWorker(store, nil, fixedResolver{result: domain.Resolution{Provider: &provider}}, ats, 1)
	if err := worker.resolveCandidate(context.Background(), domain.Candidate{ID: "candidate", Kind: domain.CompanyTarget, Name: "Missing", WebsiteURL: "https://example.org"}); err != nil {
		t.Fatal(err)
	}
	if ats.calls != 1 || !store.invalid || store.createCalls != 0 {
		t.Fatalf("verification calls=%d invalid=%v source registrations=%d", ats.calls, store.invalid, store.createCalls)
	}
}

func TestWorkableATSUsesPublicFeedAndCreatesSource(t *testing.T) {
	store := &candidateStore{sourceID: "workable-example", created: true}
	ats := &fixedATS{result: sourcedomain.FetchResult{Jobs: []sourcedomain.ExternalJob{{ExternalID: "posting"}}}}
	provider := domain.DetectedProvider{Type: "workable", Key: "acme", URL: "https://apply.workable.com/acme", Confidence: 0.99}
	worker := NewWorker(store, nil, fixedResolver{result: domain.Resolution{Provider: &provider}}, ats, 1)
	if err := worker.resolveCandidate(context.Background(), domain.Candidate{ID: "candidate", Kind: domain.CompanyTarget, Name: "Acme", WebsiteURL: "https://example.org"}); err != nil {
		t.Fatal(err)
	}
	if ats.calls != 1 || store.createCalls != 1 || store.provider != "workable" || store.state != "source_created" {
		t.Fatalf("verification calls=%d source registrations=%d detected=%s", ats.calls, store.createCalls, store.provider)
	}
}

func TestUnverifiedGitHubIssuesRepositoryDoesNotCreateSource(t *testing.T) {
	store := &candidateStore{}
	ats := &fixedATS{}
	verifier := fixedIssueBoardVerifier{result: domain.GitHubRepositoryVerification{Reason: "sample Issues are not job postings"}}
	worker := NewWorker(store, verifier, nil, ats, 1)
	err := worker.resolveCandidate(context.Background(), domain.Candidate{ID: "candidate", Kind: domain.GitHubJobsTarget, Name: "owner/repo", WebsiteURL: "https://github.com/owner/repo"})
	if err != nil || store.createCalls != 0 || ats.calls != 0 || store.provider != "github" {
		t.Fatalf("err=%v registered=%d connector calls=%d provider=%q", err, store.createCalls, ats.calls, store.provider)
	}
}

func TestVerifiedGitHubIssuesRepositoryUsesExistingConnector(t *testing.T) {
	store := &candidateStore{sourceID: "discovered-github-owner-repo-deadbeef", created: true}
	jobs := sourcedomain.FetchResult{Jobs: []sourcedomain.ExternalJob{{ExternalID: "1", Title: "Hiring: Engineer", ApplyURL: "https://github.com/owner/repo/issues/1"}}}
	ats := &fixedATS{result: jobs}
	verifier := fixedIssueBoardVerifier{result: domain.GitHubRepositoryVerification{Verified: true, Evidence: []string{"public repository", "2 job-like issues"}}}
	worker := NewWorker(store, verifier, nil, ats, 1)
	err := worker.resolveCandidate(context.Background(), domain.Candidate{ID: "candidate", Kind: domain.GitHubJobsTarget, Name: "owner/repo", WebsiteURL: "https://github.com/owner/repo"})
	if err != nil || store.createCalls != 1 || ats.calls != 1 || store.provider != "github" || store.state != "source_created" {
		t.Fatalf("err=%v registered=%d connector calls=%d provider=%q state=%q", err, store.createCalls, ats.calls, store.provider, store.state)
	}
}
