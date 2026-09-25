package httpapi

import (
	"context"
	"errors"

	profile "github.com/Hell077/HireRadar/apps/backend/internal/profile/domain"
	user "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/danielgtaylor/huma/v2"
)

type PreferencesService interface {
	GetPreferences(context.Context, user.UserID) (profile.Preferences, error)
	SavePreferences(context.Context, user.UserID, profile.Preferences) error
}

type SourcePreferenceService interface {
	GetSourcePreferences(context.Context, user.UserID) ([]profile.SourcePreference, error)
	SaveSourcePreferences(context.Context, user.UserID, []profile.SourcePreference) error
}

type preferencesGetInput struct {
	Authorization string `header:"Authorization" required:"false"`
}
type preferencesPutInput struct {
	Authorization string `header:"Authorization" required:"false"`
	Body          profile.Preferences
}
type preferencesOutput struct{ Body profile.Preferences }

type sourcePreferencesGetInput struct {
	Authorization string `header:"Authorization" required:"false"`
}
type sourcePreferencesPutInput struct {
	Authorization string `header:"Authorization" required:"false"`
	Body          struct {
		Sources []profile.SourcePreference `json:"sources" required:"true" maxItems:"100"`
	}
}
type sourcePreferencesOutput struct {
	Body struct {
		Sources []profile.SourcePreference `json:"sources"`
	}
}

func registerPreferences(api huma.API, service PreferencesService, verifier AccessVerifier) {
	huma.Register(api, huma.Operation{
		OperationID: "profile-preferences-get", Method: "GET", Path: "/api/v1/profile/preferences", Summary: "Get job preferences",
	}, func(ctx context.Context, input *preferencesGetInput) (*preferencesOutput, error) {
		if service == nil {
			return nil, huma.Error503ServiceUnavailable("profile unavailable")
		}
		id, err := profileUser(input.Authorization, verifier)
		if err != nil {
			return nil, err
		}
		p, err := service.GetPreferences(ctx, id)
		if err != nil {
			return nil, huma.Error500InternalServerError("preference lookup failed")
		}
		return &preferencesOutput{Body: p}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "profile-preferences-put", Method: "PUT", Path: "/api/v1/profile/preferences", Summary: "Update job preferences",
	}, func(ctx context.Context, input *preferencesPutInput) (*preferencesOutput, error) {
		if service == nil {
			return nil, huma.Error503ServiceUnavailable("profile unavailable")
		}
		id, err := profileUser(input.Authorization, verifier)
		if err != nil {
			return nil, err
		}
		if err := service.SavePreferences(ctx, id, input.Body); err != nil {
			if errors.Is(err, profile.ErrInvalidPreferences) {
				return nil, huma.Error400BadRequest("invalid job preferences")
			}
			return nil, huma.Error500InternalServerError("preference update failed")
		}
		p, err := service.GetPreferences(ctx, id)
		if err != nil {
			return nil, huma.Error500InternalServerError("preference lookup failed")
		}
		return &preferencesOutput{Body: p}, nil
	})
}

func registerSourcePreferences(api huma.API, service SourcePreferenceService, verifier AccessVerifier) {
	huma.Register(api, huma.Operation{
		OperationID: "profile-sources-get", Method: "GET", Path: "/api/v1/profile/sources", Summary: "Get source preferences",
	}, func(ctx context.Context, input *sourcePreferencesGetInput) (*sourcePreferencesOutput, error) {
		if service == nil {
			return nil, huma.Error503ServiceUnavailable("profile unavailable")
		}
		id, err := profileUser(input.Authorization, verifier)
		if err != nil {
			return nil, err
		}
		values, err := service.GetSourcePreferences(ctx, id)
		if err != nil {
			return nil, huma.Error500InternalServerError("source preference lookup failed")
		}
		output := &sourcePreferencesOutput{}
		output.Body.Sources = values
		return output, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "profile-sources-put", Method: "PUT", Path: "/api/v1/profile/sources", Summary: "Update source preferences",
	}, func(ctx context.Context, input *sourcePreferencesPutInput) (*sourcePreferencesOutput, error) {
		if service == nil {
			return nil, huma.Error503ServiceUnavailable("profile unavailable")
		}
		id, err := profileUser(input.Authorization, verifier)
		if err != nil {
			return nil, err
		}
		if err := service.SaveSourcePreferences(ctx, id, input.Body.Sources); err != nil {
			if errors.Is(err, profile.ErrInvalidSourcePreferences) {
				return nil, huma.Error400BadRequest("invalid source preferences")
			}
			return nil, huma.Error500InternalServerError("source preference update failed")
		}
		values, err := service.GetSourcePreferences(ctx, id)
		if err != nil {
			return nil, huma.Error500InternalServerError("source preference lookup failed")
		}
		output := &sourcePreferencesOutput{}
		output.Body.Sources = values
		return output, nil
	})
}
