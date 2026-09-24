package httpapi

import (
	"context"
	"errors"
	"strings"

	profile "github.com/Hell077/HireRadar/apps/backend/internal/profile/domain"
	user "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/danielgtaylor/huma/v2"
)

type ProfileService interface {
	Get(context.Context, user.UserID) (profile.Profile, error)
	Save(context.Context, profile.Profile) error
}

type AccessVerifier interface {
	Verify(string) (user.UserID, error)
}

type profileGetInput struct {
	Authorization string `header:"Authorization" required:"false"`
}
type profilePutInput struct {
	Authorization string `header:"Authorization" required:"false"`
	Body          profile.Profile
}
type profileOutput struct{ Body profile.Profile }

func profileUser(auth string, verifier AccessVerifier) (user.UserID, error) {
	if verifier == nil {
		return "", huma.Error503ServiceUnavailable("profile unavailable")
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", huma.Error401Unauthorized("access token required")
	}
	userID, err := verifier.Verify(parts[1])
	if err != nil {
		return "", huma.Error401Unauthorized("invalid access token")
	}
	return userID, nil
}

func registerProfile(api huma.API, service ProfileService, verifier AccessVerifier) {
	huma.Register(api, huma.Operation{
		OperationID: "profile-get", Method: "GET", Path: "/api/v1/profile", Summary: "Get candidate profile",
	}, func(ctx context.Context, input *profileGetInput) (*profileOutput, error) {
		if service == nil {
			return nil, huma.Error503ServiceUnavailable("profile unavailable")
		}
		userID, err := profileUser(input.Authorization, verifier)
		if err != nil {
			return nil, err
		}
		result, err := service.Get(ctx, userID)
		if err != nil {
			return nil, huma.Error500InternalServerError("profile lookup failed")
		}
		return &profileOutput{Body: result}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "profile-put", Method: "PUT", Path: "/api/v1/profile", Summary: "Update candidate profile",
	}, func(ctx context.Context, input *profilePutInput) (*profileOutput, error) {
		if service == nil {
			return nil, huma.Error503ServiceUnavailable("profile unavailable")
		}
		userID, err := profileUser(input.Authorization, verifier)
		if err != nil {
			return nil, err
		}
		input.Body.UserID = userID
		if err := service.Save(ctx, input.Body); err != nil {
			if errors.Is(err, profile.ErrInvalidProfile) {
				return nil, huma.Error400BadRequest("invalid profile")
			}
			return nil, huma.Error500InternalServerError("profile update failed")
		}
		result, err := service.Get(ctx, userID)
		if err != nil {
			return nil, huma.Error500InternalServerError("profile lookup failed")
		}
		return &profileOutput{Body: result}, nil
	})
}
