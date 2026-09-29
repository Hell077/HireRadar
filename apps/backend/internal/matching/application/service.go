package application

import (
	"context"
	"fmt"
	"time"

	jobdomain "github.com/Hell077/HireRadar/apps/backend/internal/job/domain"
	"github.com/Hell077/HireRadar/apps/backend/internal/matching/engine"
	user "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
)

type Repository interface {
	LoadCandidate(context.Context, user.UserID) (engine.Candidate, error)
	LoadCandidates(context.Context, []user.UserID) (map[user.UserID]engine.Candidate, error)
	CandidateJobPage(context.Context, user.UserID, engine.Candidate, string, int) (JobPage, error)
	CandidateIDsForJob(context.Context, jobdomain.Job, string, int) ([]user.UserID, string, error)
	Job(context.Context, string) (jobdomain.Job, error)
	RemoveJobMatches(context.Context, string) error
	SaveMatchBatch(context.Context, user.UserID, []engine.Result) error
	CompleteProfileRefresh(context.Context, user.UserID, int64) error
	SaveJobMatch(context.Context, user.UserID, engine.Result) error
	ListMatches(context.Context, user.UserID, int) ([]engine.Result, error)
	ListMatchPage(context.Context, user.UserID, string, int) (MatchPage, error)
}

type MatchPage struct {
	Matches    []engine.Result
	NextCursor string
}

type JobPage struct {
	Jobs       []jobdomain.Job
	NextCursor string
}

type Service struct {
	repository Repository
	now        func() time.Time
}

func NewService(repository Repository, now func() time.Time) *Service {
	return &Service{repository: repository, now: now}
}

func (s *Service) Refresh(ctx context.Context, userID user.UserID) ([]engine.Result, error) {
	return s.refresh(ctx, userID, true)
}

// RefreshProfile performs a restart-safe chunked profile rematch without
// retaining all evaluated jobs in memory.
func (s *Service) RefreshProfile(ctx context.Context, userID user.UserID) error {
	_, err := s.refresh(ctx, userID, false)
	return err
}

func (s *Service) refresh(ctx context.Context, userID user.UserID, collect bool) ([]engine.Result, error) {
	candidate, err := s.repository.LoadCandidate(ctx, userID)
	if err != nil {
		return nil, err
	}
	results := []engine.Result{}
	cursor := ""
	for {
		page, err := s.repository.CandidateJobPage(ctx, userID, candidate, cursor, 500)
		if err != nil {
			return nil, err
		}
		batch := make([]engine.Result, 0, len(page.Jobs))
		for _, job := range page.Jobs {
			result := engine.Evaluate(candidate, job, s.now())
			batch = append(batch, result)
			if collect && result.Eligible {
				results = append(results, result)
			}
		}
		if len(batch) > 0 {
			if err := s.repository.SaveMatchBatch(ctx, userID, batch); err != nil {
				return nil, err
			}
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	if err := s.repository.CompleteProfileRefresh(ctx, userID, candidate.MatchVersion); err != nil {
		return nil, err
	}
	return results, nil
}

func (s *Service) List(ctx context.Context, userID user.UserID, limit int) ([]engine.Result, error) {
	return s.repository.ListMatches(ctx, userID, limit)
}

func (s *Service) ListPage(ctx context.Context, userID user.UserID, cursor string, limit int) (MatchPage, error) {
	return s.repository.ListMatchPage(ctx, userID, cursor, limit)
}

func (s *Service) RefreshJob(ctx context.Context, jobID string) error {
	job, err := s.repository.Job(ctx, jobID)
	if err != nil {
		return err
	}
	if job.Status != jobdomain.Active {
		return s.repository.RemoveJobMatches(ctx, jobID)
	}
	now := s.now()
	cursor := ""
	for {
		users, nextCursor, err := s.repository.CandidateIDsForJob(ctx, job, cursor, 250)
		if err != nil {
			return err
		}
		candidates, err := s.repository.LoadCandidates(ctx, users)
		if err != nil {
			return err
		}
		for _, userID := range users {
			candidate, ok := candidates[userID]
			if !ok {
				return fmt.Errorf("matching candidate %q missing from bulk load", userID)
			}
			result := engine.Evaluate(candidate, job, now)
			if err := s.repository.SaveJobMatch(ctx, userID, result); err != nil {
				return err
			}
		}
		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}
	return nil
}
