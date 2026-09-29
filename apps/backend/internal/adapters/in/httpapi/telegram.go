package httpapi

import (
	"context"
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
	SavedJobs(context.Context, user.UserID, int) ([]application.SavedJob, error)
	RemoveSavedJob(context.Context, user.UserID, string) error
	ApplyUserFeedback(context.Context, user.UserID, string, string, string) error
	OpenNotification(context.Context, string) (string, error)
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
		Enabled     bool       `json:"enabled"`
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
type notificationOpenInput struct {
	ID string `path:"id"`
}
type notificationOpenOutput struct {
	Location string `header:"Location"`
}
type savedJobsInput struct {
	Authorization string `header:"Authorization" required:"false"`
	Limit         int    `query:"limit" default:"50" minimum:"1" maximum:"100"`
}
type savedJobDeleteInput struct {
	Authorization string `header:"Authorization" required:"false"`
	ID            string `path:"id"`
}
type jobFeedbackInput struct {
	Authorization string `header:"Authorization" required:"false"`
	ID            string `path:"id"`
	Body          struct {
		Action string `json:"action" enum:"hide,applied"`
		Reason string `json:"reason,omitempty" enum:"wrong_stack,wrong_role,wrong_seniority,wrong_location,wrong_salary,wrong_company,duplicate,already_seen,not_interested,other"`
	}
}
type savedJobsOutput struct {
	Body struct {
		Items []application.SavedJob `json:"items"`
	}
}

func registerTelegram(api huma.API, service TelegramService, verifier AccessVerifier) {
	huma.Register(api, huma.Operation{OperationID: "notification-open", Method: "GET", Path: "/api/v1/notifications/open/{id}", Summary: "Record a notification open and redirect to the application", DefaultStatus: 302}, func(ctx context.Context, input *notificationOpenInput) (*notificationOpenOutput, error) {
		if service == nil {
			return nil, huma.Error503ServiceUnavailable("notification open tracking unavailable")
		}
		applyURL, err := service.OpenNotification(ctx, input.ID)
		if errors.Is(err, application.ErrNotificationNotFound) {
			return nil, huma.Error404NotFound("notification not found")
		}
		if err != nil {
			return nil, huma.Error500InternalServerError("notification could not be opened")
		}
		return &notificationOpenOutput{Location: applyURL}, nil
	})
	huma.Register(api, huma.Operation{OperationID: "job-feedback", Method: "POST", Path: "/api/v1/jobs/{id}/feedback", Summary: "Hide a job or mark it as applied"}, func(ctx context.Context, input *jobFeedbackInput) (*telegramOKOutput, error) {
		if service == nil {
			return nil, huma.Error503ServiceUnavailable("job feedback unavailable")
		}
		id, err := profileUser(input.Authorization, verifier)
		if err != nil {
			return nil, err
		}
		if err := service.ApplyUserFeedback(ctx, id, input.ID, input.Body.Action, input.Body.Reason); err != nil {
			if errors.Is(err, application.ErrFeedbackNotOwned) {
				return nil, huma.Error404NotFound("job match not found")
			}
			if errors.Is(err, application.ErrInvalidCallback) {
				return nil, huma.Error400BadRequest("invalid job feedback")
			}
			return nil, huma.Error500InternalServerError("job feedback failed")
		}
		out := &telegramOKOutput{}
		out.Body.Status = "updated"
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "saved-jobs-list", Method: "GET", Path: "/api/v1/saved-jobs", Summary: "List saved jobs"}, func(ctx context.Context, input *savedJobsInput) (*savedJobsOutput, error) {
		if service == nil {
			return nil, huma.Error503ServiceUnavailable("saved jobs unavailable")
		}
		id, err := profileUser(input.Authorization, verifier)
		if err != nil {
			return nil, err
		}
		items, err := service.SavedJobs(ctx, id, input.Limit)
		if err != nil {
			return nil, huma.Error500InternalServerError("saved jobs lookup failed")
		}
		out := &savedJobsOutput{}
		out.Body.Items = items
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "saved-jobs-delete", Method: "DELETE", Path: "/api/v1/saved-jobs/{id}", Summary: "Remove a saved job"}, func(ctx context.Context, input *savedJobDeleteInput) (*telegramOKOutput, error) {
		if service == nil {
			return nil, huma.Error503ServiceUnavailable("saved jobs unavailable")
		}
		id, err := profileUser(input.Authorization, verifier)
		if err != nil {
			return nil, err
		}
		if err := service.RemoveSavedJob(ctx, id, input.ID); err != nil {
			return nil, huma.Error400BadRequest("invalid saved job ID")
		}
		out := &telegramOKOutput{}
		out.Body.Status = "removed"
		return out, nil
	})
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
		out.Body.Enabled = account.Enabled
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
}

func optionalTime(value time.Time, enabled bool) *time.Time {
	if !enabled {
		return nil
	}
	return &value
}
