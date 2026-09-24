package httpapi

import (
	"context"
	"errors"

	"github.com/Hell077/HireRadar/apps/backend/internal/auth/application"
	"github.com/danielgtaylor/huma/v2"
)

type EmailActions interface {
	Verify(context.Context, string) error
	RequestReset(context.Context, string) error
	ResetPassword(context.Context, string, string) error
}

type unavailableEmail struct{}

func (unavailableEmail) Verify(context.Context, string) error       { return errAuthUnavailable }
func (unavailableEmail) RequestReset(context.Context, string) error { return errAuthUnavailable }
func (unavailableEmail) ResetPassword(context.Context, string, string) error {
	return errAuthUnavailable
}

type verifyEmailInput struct {
	Body struct {
		Token string `json:"token" required:"true"`
	}
}
type requestResetInput struct {
	Body struct {
		Email string `json:"email" required:"true"`
	}
}
type resetPasswordInput struct {
	Body struct {
		Token    string `json:"token" required:"true"`
		Password string `json:"password" required:"true" minLength:"12" maxLength:"256"`
	}
}

func emailActionError(err error) error {
	switch {
	case errors.Is(err, errAuthUnavailable):
		return huma.Error503ServiceUnavailable("authentication unavailable")
	case errors.Is(err, application.ErrInvalidVerificationToken), errors.Is(err, application.ErrInvalidResetToken):
		return huma.Error400BadRequest("invalid or expired token")
	case errors.Is(err, application.ErrInvalidPassword):
		return huma.Error400BadRequest("invalid password")
	default:
		return huma.Error500InternalServerError("authentication failed")
	}
}

func registerEmail(api huma.API, service EmailActions) {
	huma.Register(api, huma.Operation{
		OperationID: "auth-verify-email", Method: "POST", Path: "/api/v1/auth/verify-email",
		Summary: "Confirm an email address", DefaultStatus: 204,
	}, func(ctx context.Context, input *verifyEmailInput) (*struct{}, error) {
		if err := service.Verify(ctx, input.Body.Token); err != nil {
			return nil, emailActionError(err)
		}
		return &struct{}{}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "auth-request-password-reset", Method: "POST", Path: "/api/v1/auth/forgot-password",
		Summary: "Request a password reset", DefaultStatus: 202,
	}, func(ctx context.Context, input *requestResetInput) (*struct{}, error) {
		if err := service.RequestReset(ctx, input.Body.Email); err != nil {
			return nil, emailActionError(err)
		}
		return &struct{}{}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "auth-reset-password", Method: "POST", Path: "/api/v1/auth/reset-password",
		Summary: "Set a new password", DefaultStatus: 204,
	}, func(ctx context.Context, input *resetPasswordInput) (*struct{}, error) {
		if err := service.ResetPassword(ctx, input.Body.Token, input.Body.Password); err != nil {
			return nil, emailActionError(err)
		}
		return &struct{}{}, nil
	})
}
