package httpapi

import (
	"context"
	"crypto/subtle"
	"errors"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/notification/application"
	"github.com/Hell077/HireRadar/apps/backend/internal/notification/domain"
	user "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/danielgtaylor/huma/v2"
)

type TelegramService interface {
	Connection(context.Context, user.UserID) (application.Account, error)
	CreateLink(context.Context, user.UserID) (application.Link, error)
	Disconnect(context.Context, user.UserID) error
	Preferences(context.Context, user.UserID) (domain.NotificationPreferences, error)
	SavePreferences(context.Context, user.UserID, domain.NotificationPreferences) error
	HandleStart(context.Context, int64, int64, string, string) error
}

type telegramGetInput struct {
	Authorization string `header:"Authorization" required:"false"`
}
type telegramLinkInput struct {
	Authorization string `header:"Authorization" required:"false"`
}
type telegramDeleteInput struct {
	Authorization string `header:"Authorization" required:"false"`
}
type telegramPreferencesInput struct {
	Authorization string `header:"Authorization" required:"false"`
	Body          domain.NotificationPreferences
}
type telegramOutput struct {
	Body struct {
		Connected   bool       `json:"connected"`
		Username    *string    `json:"username,omitempty"`
		ConnectedAt *time.Time `json:"connected_at,omitempty"`
	}
}
type telegramLinkOutput struct {
	Body struct {
		URL       string    `json:"url"`
		ExpiresAt time.Time `json:"expires_at"`
	}
}
type telegramPreferencesOutput struct {
	Body domain.NotificationPreferences
}
type telegramOKOutput struct {
	Body struct {
		Status string `json:"status"`
	}
}

type telegramWebhookInput struct {
	Secret string `header:"X-Telegram-Bot-Api-Secret-Token" required:"true"`
	Body   struct {
		Message *struct {
			Text string `json:"text"`
			From struct {
				ID       int64  `json:"id"`
				Username string `json:"username"`
			} `json:"from"`
			Chat struct {
				ID int64 `json:"id"`
			} `json:"chat"`
		} `json:"message,omitempty"`
	}
}

func registerTelegram(api huma.API, service TelegramService, verifier AccessVerifier, webhookSecret string) {
	huma.Register(api, huma.Operation{OperationID: "telegram-status", Method: "GET", Path: "/api/v1/telegram", Summary: "Get Telegram connection status"}, func(ctx context.Context, input *telegramGetInput) (*telegramOutput, error) {
		if service == nil {
			return nil, huma.Error503ServiceUnavailable("Telegram unavailable")
		}
		id, err := profileUser(input.Authorization, verifier)
		if err != nil {
			return nil, err
		}
		account, err := service.Connection(ctx, id)
		if err != nil {
			return nil, huma.Error500InternalServerError("Telegram status lookup failed")
		}
		out := &telegramOutput{}
		out.Body.Connected = account.ID != ""
		out.Body.Username, out.Body.ConnectedAt = account.Username, optionalTime(account.ConnectedAt, out.Body.Connected)
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "telegram-link", Method: "POST", Path: "/api/v1/telegram/link", Summary: "Create a one-time Telegram account link"}, func(ctx context.Context, input *telegramLinkInput) (*telegramLinkOutput, error) {
		if service == nil {
			return nil, huma.Error503ServiceUnavailable("Telegram unavailable")
		}
		id, err := profileUser(input.Authorization, verifier)
		if err != nil {
			return nil, err
		}
		link, err := service.CreateLink(ctx, id)
		if err != nil {
			return nil, huma.Error503ServiceUnavailable("Telegram linking unavailable")
		}
		out := &telegramLinkOutput{}
		out.Body.URL, out.Body.ExpiresAt = link.URL, link.ExpiresAt
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "telegram-disconnect", Method: "DELETE", Path: "/api/v1/telegram", Summary: "Disconnect Telegram"}, func(ctx context.Context, input *telegramDeleteInput) (*telegramOKOutput, error) {
		if service == nil {
			return nil, huma.Error503ServiceUnavailable("Telegram unavailable")
		}
		id, err := profileUser(input.Authorization, verifier)
		if err != nil {
			return nil, err
		}
		if err := service.Disconnect(ctx, id); err != nil {
			return nil, huma.Error500InternalServerError("Telegram disconnect failed")
		}
		out := &telegramOKOutput{}
		out.Body.Status = "disconnected"
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "telegram-preferences-get", Method: "GET", Path: "/api/v1/telegram/preferences", Summary: "Get notification preferences"}, func(ctx context.Context, input *telegramGetInput) (*telegramPreferencesOutput, error) {
		if service == nil {
			return nil, huma.Error503ServiceUnavailable("Telegram unavailable")
		}
		id, err := profileUser(input.Authorization, verifier)
		if err != nil {
			return nil, err
		}
		preferences, err := service.Preferences(ctx, id)
		if err != nil {
			return nil, huma.Error500InternalServerError("notification preferences lookup failed")
		}
		return &telegramPreferencesOutput{Body: preferences}, nil
	})
	huma.Register(api, huma.Operation{OperationID: "telegram-preferences-put", Method: "PUT", Path: "/api/v1/telegram/preferences", Summary: "Update notification preferences"}, func(ctx context.Context, input *telegramPreferencesInput) (*telegramPreferencesOutput, error) {
		if service == nil {
			return nil, huma.Error503ServiceUnavailable("Telegram unavailable")
		}
		id, err := profileUser(input.Authorization, verifier)
		if err != nil {
			return nil, err
		}
		if err := service.SavePreferences(ctx, id, input.Body); err != nil {
			if errors.Is(err, domain.ErrInvalidPreferences) {
				return nil, huma.Error400BadRequest("invalid notification preferences")
			}
			return nil, huma.Error500InternalServerError("notification preferences update failed")
		}
		return &telegramPreferencesOutput{Body: input.Body}, nil
	})
	huma.Register(api, huma.Operation{OperationID: "telegram-webhook", Method: "POST", Path: "/webhooks/telegram", Summary: "Receive Telegram bot updates"}, func(ctx context.Context, input *telegramWebhookInput) (*telegramOKOutput, error) {
		if service == nil || webhookSecret == "" {
			return nil, huma.Error503ServiceUnavailable("Telegram webhook unavailable")
		}
		if len(input.Secret) != len(webhookSecret) || subtle.ConstantTimeCompare([]byte(input.Secret), []byte(webhookSecret)) != 1 {
			return nil, huma.Error401Unauthorized("invalid webhook secret")
		}
		if input.Body.Message != nil {
			message := input.Body.Message
			if len(message.Text) > 7 && message.Text[:7] == "/start " {
				if err := service.HandleStart(ctx, message.From.ID, message.Chat.ID, message.From.Username, message.Text[7:]); err != nil {
					if errors.Is(err, application.ErrLinkExpired) || errors.Is(err, application.ErrTelegramInUse) {
						return nil, huma.Error400BadRequest("Telegram link is invalid, expired, or already connected")
					}
					return nil, huma.Error500InternalServerError("Telegram link could not be confirmed")
				}
			}
		}
		out := &telegramOKOutput{}
		out.Body.Status = "ok"
		return out, nil
	})
}

func optionalTime(value time.Time, enabled bool) *time.Time {
	if !enabled {
		return nil
	}
	return &value
}
