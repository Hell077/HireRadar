package application

import (
	"context"

	"github.com/Hell077/HireRadar/apps/backend/internal/profile/domain"
	user "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
)

type Store interface {
	Get(context.Context, user.UserID) (domain.Profile, error)
	Save(context.Context, domain.Profile) error
	GetSkills(context.Context, user.UserID) ([]domain.Skill, error)
	SaveSkills(context.Context, user.UserID, []domain.Skill) error
	GetPositions(context.Context, user.UserID) ([]string, error)
	SavePositions(context.Context, user.UserID, []string) error
	GetPreferences(context.Context, user.UserID) (domain.Preferences, error)
	SavePreferences(context.Context, user.UserID, domain.Preferences) error
	GetSourcePreferences(context.Context, user.UserID) ([]domain.SourcePreference, error)
	SaveSourcePreferences(context.Context, user.UserID, []domain.SourcePreference) error
}

type Service struct{ store Store }

func NewService(store Store) *Service { return &Service{store: store} }

func (s *Service) Get(ctx context.Context, userID user.UserID) (domain.Profile, error) {
	return s.store.Get(ctx, userID)
}

func (s *Service) Save(ctx context.Context, profile domain.Profile) error {
	normalized, err := profile.Normalize()
	if err != nil {
		return err
	}
	return s.store.Save(ctx, normalized)
}

func (s *Service) GetSkills(ctx context.Context, userID user.UserID) ([]domain.Skill, error) {
	return s.store.GetSkills(ctx, userID)
}

func (s *Service) SaveSkills(ctx context.Context, userID user.UserID, skills []domain.Skill) error {
	normalized, err := domain.NormalizeSkills(skills)
	if err != nil {
		return err
	}
	return s.store.SaveSkills(ctx, userID, normalized)
}

func (s *Service) GetPositions(ctx context.Context, userID user.UserID) ([]string, error) {
	return s.store.GetPositions(ctx, userID)
}

func (s *Service) SavePositions(ctx context.Context, userID user.UserID, titles []string) error {
	normalized, err := domain.NormalizePositions(titles)
	if err != nil {
		return err
	}
	return s.store.SavePositions(ctx, userID, normalized)
}

func (s *Service) GetPreferences(ctx context.Context, userID user.UserID) (domain.Preferences, error) {
	return s.store.GetPreferences(ctx, userID)
}

func (s *Service) SavePreferences(ctx context.Context, userID user.UserID, preferences domain.Preferences) error {
	normalized, err := preferences.Normalize()
	if err != nil {
		return err
	}
	return s.store.SavePreferences(ctx, userID, normalized)
}

func (s *Service) GetSourcePreferences(ctx context.Context, userID user.UserID) ([]domain.SourcePreference, error) {
	return s.store.GetSourcePreferences(ctx, userID)
}

func (s *Service) SaveSourcePreferences(ctx context.Context, userID user.UserID, preferences []domain.SourcePreference) error {
	normalized, err := domain.NormalizeSourcePreferences(preferences)
	if err != nil {
		return err
	}
	return s.store.SaveSourcePreferences(ctx, userID, normalized)
}
