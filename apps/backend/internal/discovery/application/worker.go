package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/discovery/domain"
	sourcedomain "github.com/Hell077/HireRadar/apps/backend/internal/source/domain"
)

type Store interface {
	ClaimDueSources(context.Context, int) ([]domain.DiscoverySource, error)
	StartRun(context.Context, string) (string, error)
	SaveTargets(context.Context, string, []domain.DiscoveredTarget) (domain.RunStats, error)
	FinishRun(context.Context, domain.DiscoverySource, string, domain.RepositorySnapshot, domain.RunStats, error) error
	ClaimCandidates(context.Context, int) ([]domain.Candidate, error)
	CandidateUnavailable(context.Context, string, error) error
	CandidateReview(context.Context, string, domain.DetectedProvider, string) error
	CandidateInvalid(context.Context, string, domain.DetectedProvider, error) error
	CreateOrLinkSource(context.Context, string) (string, bool, error)
	ScheduleRun(context.Context, string) error
	Overview(context.Context) (domain.Overview, error)
}

type SnapshotFetcher interface {
	Snapshot(context.Context, domain.DiscoverySource) (domain.RepositorySnapshot, error)
}

type CandidateResolver interface {
	Resolve(context.Context, domain.Candidate) (domain.Resolution, error)
}

type ATSFetcher interface {
	Fetch(context.Context, sourcedomain.Source) (sourcedomain.FetchResult, error)
}

type Worker struct {
	store       Store
	snapshots   SnapshotFetcher
	resolver    CandidateResolver
	ats         ATSFetcher
	parallelism int
	reportError func(error)
}

func NewWorker(store Store, snapshots SnapshotFetcher, resolver CandidateResolver, fetcher ATSFetcher, parallelism int) *Worker {
	if parallelism < 1 {
		parallelism = 1
	}
	if parallelism > 16 {
		parallelism = 16
	}
	return &Worker{store: store, snapshots: snapshots, resolver: resolver, ats: fetcher, parallelism: parallelism}
}

func (w *Worker) SetErrorReporter(report func(error)) { w.reportError = report }

func (w *Worker) Run(ctx context.Context) error {
	slog.Info("discovery worker started", "parallelism", w.parallelism, "schedule", "daily")
	for ctx.Err() == nil {
		worked := false
		sources, err := w.store.ClaimDueSources(ctx, 3)
		if err != nil && ctx.Err() == nil {
			w.report(err)
		} else if len(sources) > 0 {
			worked = true
			for _, source := range sources {
				if err := w.syncSource(ctx, source); err != nil && ctx.Err() == nil {
					slog.Error("discovery source sync failed", "source_id", source.ID, "error", err)
					w.report(err)
				}
			}
		}
		candidates, err := w.store.ClaimCandidates(ctx, w.parallelism)
		if err != nil && ctx.Err() == nil {
			w.report(err)
		} else if len(candidates) > 0 {
			worked = true
			w.resolveCandidates(ctx, candidates)
		}
		if !worked {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(3 * time.Second):
			}
		}
	}
	return nil
}

func (w *Worker) syncSource(ctx context.Context, source domain.DiscoverySource) error {
	runID, err := w.store.StartRun(ctx, source.ID)
	if err != nil {
		return err
	}
	snapshot, fetchErr := w.snapshots.Snapshot(ctx, source)
	stats := domain.RunStats{}
	if fetchErr == nil && !snapshot.NotChanged {
		stats, fetchErr = w.store.SaveTargets(ctx, source.ID, snapshot.Targets)
		if fetchErr != nil {
			stats = domain.RunStats{}
		}
	}
	if err := w.store.FinishRun(ctx, source, runID, snapshot, stats, fetchErr); err != nil {
		return errors.Join(fetchErr, err)
	}
	if fetchErr != nil {
		return fetchErr
	}
	slog.Info("discovery sync complete", "source_id", source.ID, "not_modified", snapshot.NotChanged, "targets", stats.TargetsSeen, "candidates_created", stats.CandidatesCreated, "candidates_reused", stats.CandidatesReused, "provenance_added", stats.ProvenanceAdded)
	w.report(nil)
	return nil
}

func (w *Worker) resolveCandidates(ctx context.Context, candidates []domain.Candidate) {
	var group sync.WaitGroup
	for _, candidate := range candidates {
		candidate := candidate
		group.Add(1)
		go func() {
			defer group.Done()
			if err := w.resolveCandidate(ctx, candidate); err != nil && ctx.Err() == nil {
				slog.Error("source candidate resolution failed", "candidate_id", candidate.ID, "company", candidate.Name, "error", err)
				w.report(err)
				var saveErr error
				switch {
				case isPermanentResolutionError(err):
					saveErr = w.store.CandidateInvalid(ctx, candidate.ID, domain.DetectedProvider{}, err)
				case requiresManualReview(err):
					saveErr = w.store.CandidateReview(ctx, candidate.ID, domain.DetectedProvider{}, err.Error())
				default:
					saveErr = w.store.CandidateUnavailable(ctx, candidate.ID, err)
				}
				if saveErr != nil {
					w.report(saveErr)
				}
			} else if err == nil {
				w.report(nil)
			}
		}()
	}
	group.Wait()
	// Candidate resolution performs live requests to third-party career sites and
	// ATS boards. Pace batches so a large catalog does not fan out unboundedly.
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
	}
}

func isPermanentResolutionError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "http 404") || strings.Contains(message, "http 410") || strings.Contains(message, "not a safe public") || strings.Contains(message, "ssrf")
}

func requiresManualReview(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "http 403") || strings.Contains(message, "exceeded 4194304 bytes") || strings.Contains(message, "blocked private")
}

func (w *Worker) resolveCandidate(ctx context.Context, candidate domain.Candidate) error {
	if candidate.Kind == domain.GitHubJobsTarget {
		return w.resolveGitHubJobBoard(ctx, candidate)
	}
	if candidate.Kind != domain.CompanyTarget {
		return w.store.CandidateReview(ctx, candidate.ID, domain.DetectedProvider{}, "catalog target retained for a future supported job-board feed")
	}
	if candidate.CareersURL == "" && candidate.WebsiteURL == "" {
		return w.store.CandidateReview(ctx, candidate.ID, domain.DetectedProvider{}, "catalog has no public website or careers URL")
	}
	result, err := w.resolver.Resolve(ctx, candidate)
	if err != nil {
		return err
	}
	if result.Provider == nil {
		return w.store.CandidateReview(ctx, candidate.ID, domain.DetectedProvider{}, result.Reason)
	}
	provider := *result.Provider
	switch provider.Type {
	case "greenhouse", "lever", "ashby":
	default:
		return w.store.CandidateReview(ctx, candidate.ID, provider, "provider detected but connector is not available")
	}
	if provider.Key == "" {
		return w.store.CandidateInvalid(ctx, candidate.ID, provider, errors.New("ATS board key could not be determined"))
	}
	config, _ := json.Marshal(map[string]string{"board": provider.Key})
	verificationSource := sourcedomain.Source{Type: sourcedomain.Type(provider.Type), Name: candidate.Name + " jobs", CompanyName: candidate.Name, Enabled: true, Config: config}
	verified, err := w.ats.Fetch(ctx, verificationSource)
	if err != nil {
		if isPermanentVerificationError(err) {
			return w.store.CandidateInvalid(ctx, candidate.ID, provider, err)
		}
		return err
	}
	provider.Evidence = append(provider.Evidence, fmt.Sprintf("existing %s connector verified public payload (%d postings)", provider.Type, len(verified.Jobs)))
	if err := w.store.CandidateReview(ctx, candidate.ID, provider, ""); err != nil {
		return err
	}
	sourceID, created, err := w.store.CreateOrLinkSource(ctx, candidate.ID)
	if err != nil {
		return err
	}
	slog.Info("verified discovery source registered", "candidate_id", candidate.ID, "source_id", sourceID, "company", candidate.Name, "provider", provider.Type, "provider_key", provider.Key, "created", created, "verified_jobs", len(verified.Jobs))
	return nil
}

type issueBoardVerifier interface {
	VerifyJobRepository(context.Context, string, string) (domain.GitHubRepositoryVerification, error)
}

func (w *Worker) resolveGitHubJobBoard(ctx context.Context, candidate domain.Candidate) error {
	verifier, ok := w.snapshots.(issueBoardVerifier)
	if !ok {
		return w.store.CandidateReview(ctx, candidate.ID, domain.DetectedProvider{}, "GitHub Issues repository verifier is unavailable")
	}
	key := candidate.Name
	if !strings.Contains(key, "/") && candidate.WebsiteURL != "" {
		parsed, err := url.Parse(candidate.WebsiteURL)
		if err == nil {
			key = strings.Trim(parsed.Path, "/")
		}
	}
	parts := strings.Split(key, "/")
	if len(parts) != 2 {
		return w.store.CandidateInvalid(ctx, candidate.ID, domain.DetectedProvider{}, errors.New("GitHub repository must use owner/repository identity"))
	}
	verification, err := verifier.VerifyJobRepository(ctx, parts[0], parts[1])
	if err != nil {
		if isPermanentVerificationError(err) {
			return w.store.CandidateInvalid(ctx, candidate.ID, domain.DetectedProvider{Type: "github", Key: key}, err)
		}
		return err
	}
	provider := domain.DetectedProvider{Type: "github", Key: key, URL: "https://github.com/" + key + "/issues", Confidence: 0.99, Evidence: verification.Evidence}
	if !verification.Verified {
		return w.store.CandidateReview(ctx, candidate.ID, provider, verification.Reason)
	}
	config, _ := json.Marshal(map[string]any{"owner": parts[0], "repo": parts[1], "page_size": 100})
	verificationSource := sourcedomain.Source{Type: sourcedomain.GitHub, Name: key + " jobs", CompanyName: candidate.Name, Enabled: true, Config: config}
	result, err := w.ats.Fetch(ctx, verificationSource)
	if err != nil {
		if isPermanentVerificationError(err) {
			return w.store.CandidateInvalid(ctx, candidate.ID, provider, err)
		}
		return err
	}
	if len(result.Jobs) == 0 {
		return w.store.CandidateReview(ctx, candidate.ID, provider, "existing GitHub Issues connector found no job-like postings")
	}
	provider.Evidence = append(provider.Evidence, fmt.Sprintf("existing GitHub Issues connector verified %d job-like postings", len(result.Jobs)))
	if err := w.store.CandidateReview(ctx, candidate.ID, provider, ""); err != nil {
		return err
	}
	sourceID, created, err := w.store.CreateOrLinkSource(ctx, candidate.ID)
	if err != nil {
		return err
	}
	slog.Info("verified GitHub Issues job source registered", "candidate_id", candidate.ID, "source_id", sourceID, "repository", key, "created", created, "verified_jobs", len(result.Jobs))
	return nil
}

func isPermanentVerificationError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "http 404") || strings.Contains(message, "invalid source") || strings.Contains(message, "decode greenhouse response") || strings.Contains(message, "decode ashby response") || strings.Contains(message, "decode lever response") || strings.Contains(message, "decode lever posting")
}

func (w *Worker) report(err error) {
	if w.reportError != nil {
		w.reportError(err)
	}
}

func (w *Worker) ScheduleRun(ctx context.Context, sourceID string) error {
	return w.store.ScheduleRun(ctx, sourceID)
}

func (w *Worker) Overview(ctx context.Context) (domain.Overview, error) {
	return w.store.Overview(ctx)
}
