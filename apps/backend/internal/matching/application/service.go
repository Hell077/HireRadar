package application

import (
	"context"
	"time"

	jobdomain "github.com/Hell077/HireRadar/apps/backend/internal/job/domain"
	"github.com/Hell077/HireRadar/apps/backend/internal/matching/engine"
	user "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
)

type Repository interface {
	LoadCandidate(context.Context, user.UserID) (engine.Candidate, error)
	CandidateJobs(context.Context, user.UserID, engine.Candidate) ([]jobdomain.Job, error)
	SaveMatches(context.Context, user.UserID, []engine.Result) error
	ListMatches(context.Context, user.UserID, int) ([]engine.Result, error)
}

type Service struct {
	repository Repository
	now        func() time.Time
}

func NewService(repository Repository, now func() time.Time) *Service {
	return &Service{repository: repository, now: now}
}

func (s *Service) Refresh(ctx context.Context, userID user.UserID) ([]engine.Result, error) {
	candidate, err := s.repository.LoadCandidate(ctx, userID)
	if err != nil {
		return nil, err
	}
	jobs, err := s.repository.CandidateJobs(ctx, userID, candidate)
	if err != nil {
		return nil, err
	}
	results := make([]engine.Result, 0, len(jobs))
	now := s.now()
	for _, job := range jobs {
		result := engine.Evaluate(candidate, job, now)
		if result.Eligible {
			results = append(results, result)
		}
	}
	if err := s.repository.SaveMatches(ctx, userID, results); err != nil {
		return nil, err
	}
	return results, nil
}

func (s *Service) List(ctx context.Context, userID user.UserID, limit int) ([]engine.Result, error) {
	return s.repository.ListMatches(ctx, userID, limit)
}
