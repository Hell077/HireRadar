package application

import (
	"context"

	"github.com/Hell077/HireRadar/apps/backend/internal/profile/domain"
	user "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
)

type Store interface {
	Get(context.Context, user.UserID) (domain.Profile, error)
	Save(context.Context, domain.Profile) error
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
