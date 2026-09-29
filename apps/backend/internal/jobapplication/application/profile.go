package application

import (
	"context"

	"github.com/Hell077/HireRadar/apps/backend/internal/jobapplication/domain"
)

type ProfileStore interface {
	GetProfile(context.Context, string) (domain.ApplicationProfile, error)
	SaveProfile(context.Context, domain.ApplicationProfile) error
}

type ProfileService struct{ store ProfileStore }

func NewProfileService(store ProfileStore) *ProfileService { return &ProfileService{store: store} }

func (s *ProfileService) Get(ctx context.Context, userID string) (domain.ApplicationProfile, error) {
	return s.store.GetProfile(ctx, userID)
}

func (s *ProfileService) Save(ctx context.Context, profile domain.ApplicationProfile) error {
	normalized, err := profile.Normalize()
	if err != nil {
		return err
	}
	return s.store.SaveProfile(ctx, normalized)
}
