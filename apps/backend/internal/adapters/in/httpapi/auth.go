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

type Sessions interface {
	Login(context.Context, string, string, string, string) (application.Tokens, error)
	Refresh(context.Context, string) (application.Tokens, error)
	Logout(context.Context, string) error
}

type unavailableSessions struct{}

func (unavailableSessions) Login(context.Context, string, string, string, string) (application.Tokens, error) {
	return application.Tokens{}, errAuthUnavailable
}
func (unavailableSessions) Refresh(context.Context, string) (application.Tokens, error) {
	return application.Tokens{}, errAuthUnavailable
}
func (unavailableSessions) Logout(context.Context, string) error { return errAuthUnavailable }

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

type loginInput struct {
	UserAgent string `header:"User-Agent"`
	Body      struct {
		Email    string `json:"email" required:"true" maxLength:"320"`
		Password string `json:"password" required:"true"`
	}
}

type refreshInput struct {
	Body struct {
		RefreshToken string `json:"refresh_token" required:"true"`
	}
}

type tokenOutput struct {
	Body struct {
		AccessToken      string `json:"access_token"`
		AccessExpiresAt  string `json:"access_expires_at"`
		RefreshToken     string `json:"refresh_token"`
		RefreshExpiresAt string `json:"refresh_expires_at"`
	}
}

func newTokenOutput(tokens application.Tokens) *tokenOutput {
	output := &tokenOutput{}
	output.Body.AccessToken = tokens.AccessToken
	output.Body.AccessExpiresAt = tokens.AccessExpiresAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	output.Body.RefreshToken = tokens.RefreshToken
	output.Body.RefreshExpiresAt = tokens.RefreshExpiresAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	return output
}

func sessionError(err error) error {
	switch {
	case errors.Is(err, errAuthUnavailable):
		return huma.Error503ServiceUnavailable("authentication unavailable")
	case errors.Is(err, application.ErrInvalidCredentials), errors.Is(err, application.ErrInvalidRefreshToken):
		return huma.Error401Unauthorized("invalid credentials")
	default:
		return huma.Error500InternalServerError("authentication failed")
	}
}

func registerSessions(api huma.API, sessions Sessions) {
	huma.Register(api, huma.Operation{
		OperationID: "auth-login", Method: "POST", Path: "/api/v1/auth/login", Summary: "Log in",
	}, func(ctx context.Context, input *loginInput) (*tokenOutput, error) {
		tokens, err := sessions.Login(ctx, input.Body.Email, input.Body.Password, input.UserAgent, "")
		if err != nil {
			return nil, sessionError(err)
		}
		return newTokenOutput(tokens), nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "auth-refresh", Method: "POST", Path: "/api/v1/auth/refresh", Summary: "Rotate a refresh token",
	}, func(ctx context.Context, input *refreshInput) (*tokenOutput, error) {
		tokens, err := sessions.Refresh(ctx, input.Body.RefreshToken)
		if err != nil {
			return nil, sessionError(err)
		}
		return newTokenOutput(tokens), nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "auth-logout", Method: "POST", Path: "/api/v1/auth/logout", Summary: "Revoke a refresh token", DefaultStatus: 204,
	}, func(ctx context.Context, input *refreshInput) (*struct{}, error) {
		if err := sessions.Logout(ctx, input.Body.RefreshToken); err != nil {
			return nil, sessionError(err)
		}
		return &struct{}{}, nil
	})
}
