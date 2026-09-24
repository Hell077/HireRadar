package httpapi

import (
	"context"
	"errors"

	"github.com/Hell077/HireRadar/apps/backend/internal/auth/application"
	"github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/danielgtaylor/huma/v2"
)

type Registrar interface {
	Register(context.Context, string, string) (domain.UserID, error)
}

type unavailableRegistrar struct{}

func (unavailableRegistrar) Register(context.Context, string, string) (domain.UserID, error) {
	return "", errAuthUnavailable
}

var errAuthUnavailable = errors.New("authentication storage unavailable")

type registerInput struct {
	Body struct {
		Email    string `json:"email" required:"true" maxLength:"320"`
		Password string `json:"password" required:"true" minLength:"12" maxLength:"256"`
	}
}

type registerOutput struct {
	Body struct {
		UserID string `json:"user_id"`
	}
}

func registerAuth(api huma.API, registrar Registrar) {
	huma.Register(api, huma.Operation{
		OperationID:   "auth-register",
		Method:        "POST",
		Path:          "/api/v1/auth/register",
		Summary:       "Register a user",
		DefaultStatus: 201,
	}, func(ctx context.Context, input *registerInput) (*registerOutput, error) {
		userID, err := registrar.Register(ctx, input.Body.Email, input.Body.Password)
		if err != nil {
			switch {
			case errors.Is(err, errAuthUnavailable):
				return nil, huma.Error503ServiceUnavailable("authentication unavailable")
			case errors.Is(err, application.ErrEmailTaken):
				return nil, huma.Error409Conflict("email already registered")
			case errors.Is(err, application.ErrInvalidPassword), errors.Is(err, domain.ErrInvalidEmail):
				return nil, huma.Error400BadRequest("invalid registration details")
			default:
				return nil, huma.Error500InternalServerError("registration failed")
			}
		}
		output := &registerOutput{}
		output.Body.UserID = string(userID)
		return output, nil
	})
}
