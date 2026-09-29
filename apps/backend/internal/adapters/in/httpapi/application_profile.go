package httpapi

import (
	"context"
	"errors"

	"github.com/Hell077/HireRadar/apps/backend/internal/jobapplication/application"
	"github.com/Hell077/HireRadar/apps/backend/internal/jobapplication/domain"
	"github.com/danielgtaylor/huma/v2"
)

type ApplicationProfileService interface {
	Get(context.Context, string) (domain.ApplicationProfile, error)
	Save(context.Context, domain.ApplicationProfile) error
}

type applicationProfileInput struct {
	Authorization string `header:"Authorization" required:"false"`
	Body          domain.ApplicationProfile
}

type applicationProfileGetInput struct {
	Authorization string `header:"Authorization" required:"false"`
}

type applicationProfileOutput struct{ Body domain.ApplicationProfile }

func registerApplicationProfile(api huma.API, service ApplicationProfileService, verifier AccessVerifier) {
	huma.Register(api, huma.Operation{OperationID: "application-profile-get", Method: "GET", Path: "/api/v1/application-profile", Summary: "Get application profile"}, func(ctx context.Context, input *applicationProfileGetInput) (*applicationProfileOutput, error) {
		if service == nil {
			return nil, huma.Error503ServiceUnavailable("application profile unavailable")
		}
		userID, err := profileUser(input.Authorization, verifier)
		if err != nil {
			return nil, err
		}
		profile, err := service.Get(ctx, string(userID))
		if err != nil {
			return nil, huma.Error500InternalServerError("application profile lookup failed")
		}
		return &applicationProfileOutput{Body: profile}, nil
	})
	huma.Register(api, huma.Operation{OperationID: "application-profile-put", Method: "PUT", Path: "/api/v1/application-profile", Summary: "Update application profile"}, func(ctx context.Context, input *applicationProfileInput) (*applicationProfileOutput, error) {
		if service == nil {
			return nil, huma.Error503ServiceUnavailable("application profile unavailable")
		}
		userID, err := profileUser(input.Authorization, verifier)
		if err != nil {
			return nil, err
		}
		input.Body.UserID = string(userID)
		if err := service.Save(ctx, input.Body); err != nil {
			if errors.Is(err, domain.ErrInvalidApplicationProfile) {
				return nil, huma.Error400BadRequest("invalid application profile")
			}
			if errors.Is(err, application.ErrApplicationUnavailable) {
				return nil, huma.Error400BadRequest("invalid selected resume")
			}
			return nil, huma.Error500InternalServerError("application profile update failed")
		}
		profile, err := service.Get(ctx, string(userID))
		if err != nil {
			return nil, huma.Error500InternalServerError("application profile lookup failed")
		}
		return &applicationProfileOutput{Body: profile}, nil
	})
}
