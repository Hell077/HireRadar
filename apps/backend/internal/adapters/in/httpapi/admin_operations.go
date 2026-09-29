package httpapi

import (
	"context"
	"crypto/subtle"
	"errors"

	jobapplicationapp "github.com/Hell077/HireRadar/apps/backend/internal/jobapplication/application"
	jobapplication "github.com/Hell077/HireRadar/apps/backend/internal/jobapplication/domain"
	"github.com/Hell077/HireRadar/apps/backend/internal/outbox"
	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
)

type AdminOutboxOperations interface {
	ListFailed(context.Context, int) ([]outbox.FailedEvent, error)
	RetryFailed(context.Context, string) error
}

type AdminApplicationOperations interface {
	ListFailed(context.Context, int) ([]jobapplication.FailedApplication, error)
	RetryFailed(context.Context, jobapplication.ID) error
}

type adminFailedLimitInput struct {
	Token string `header:"X-Operator-Token" required:"false"`
	Limit int    `query:"limit" default:"50" minimum:"1" maximum:"100"`
}

type adminRetryOutboxInput struct {
	Token string `header:"X-Operator-Token" required:"false"`
	ID    string `path:"id"`
}

type adminRetryApplicationInput struct {
	Token string `header:"X-Operator-Token" required:"false"`
	ID    string `path:"id"`
}

type adminFailedOutboxOutput struct {
	Body struct {
		Items []outbox.FailedEvent `json:"items"`
	}
}

type adminFailedApplicationsOutput struct {
	Body struct {
		Items []jobapplication.FailedApplication `json:"items"`
	}
}

type adminRetryOutput struct {
	Body struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
}

func registerAdminOperations(api huma.API, outboxOperations AdminOutboxOperations, applicationOperations AdminApplicationOperations, token string) {
	unauthorized := func(value string) error {
		if token != "" && len(value) == len(token) && subtle.ConstantTimeCompare([]byte(value), []byte(token)) == 1 {
			return nil
		}
		if token == "" {
			return huma.Error503ServiceUnavailable("admin recovery is unavailable")
		}
		return huma.Error401Unauthorized("operator token required")
	}
	huma.Register(api, huma.Operation{OperationID: "admin-outbox-failed-list", Method: "GET", Path: "/api/v1/admin/outbox/failed", Summary: "List failed outbox events"}, func(ctx context.Context, input *adminFailedLimitInput) (*adminFailedOutboxOutput, error) {
		if err := unauthorized(input.Token); err != nil {
			return nil, err
		}
		if outboxOperations == nil {
			return nil, huma.Error503ServiceUnavailable("outbox recovery is unavailable")
		}
		items, err := outboxOperations.ListFailed(ctx, input.Limit)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed outbox lookup failed")
		}
		out := &adminFailedOutboxOutput{}
		out.Body.Items = items
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "admin-outbox-retry", Method: "POST", Path: "/api/v1/admin/outbox/{id}/retry", Summary: "Retry a failed outbox event"}, func(ctx context.Context, input *adminRetryOutboxInput) (*adminRetryOutput, error) {
		if err := unauthorized(input.Token); err != nil {
			return nil, err
		}
		if outboxOperations == nil {
			return nil, huma.Error503ServiceUnavailable("outbox recovery is unavailable")
		}
		if uuid.Validate(input.ID) != nil {
			return nil, huma.Error400BadRequest("invalid outbox event ID")
		}
		if err := outboxOperations.RetryFailed(ctx, input.ID); err != nil {
			if errors.Is(err, outbox.ErrEventNotFound) {
				return nil, huma.Error404NotFound("outbox event not found")
			}
			return nil, huma.Error500InternalServerError("outbox event could not be retried")
		}
		return queuedAdminRetry(input.ID), nil
	})
	huma.Register(api, huma.Operation{OperationID: "admin-applications-failed-list", Method: "GET", Path: "/api/v1/admin/applications/failed", Summary: "List failed applications"}, func(ctx context.Context, input *adminFailedLimitInput) (*adminFailedApplicationsOutput, error) {
		if err := unauthorized(input.Token); err != nil {
			return nil, err
		}
		if applicationOperations == nil {
			return nil, huma.Error503ServiceUnavailable("application recovery is unavailable")
		}
		items, err := applicationOperations.ListFailed(ctx, input.Limit)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed application lookup failed")
		}
		out := &adminFailedApplicationsOutput{}
		out.Body.Items = items
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "admin-application-retry", Method: "POST", Path: "/api/v1/admin/applications/{id}/retry", Summary: "Retry a failed application"}, func(ctx context.Context, input *adminRetryApplicationInput) (*adminRetryOutput, error) {
		if err := unauthorized(input.Token); err != nil {
			return nil, err
		}
		if applicationOperations == nil {
			return nil, huma.Error503ServiceUnavailable("application recovery is unavailable")
		}
		if uuid.Validate(input.ID) != nil {
			return nil, huma.Error400BadRequest("invalid application ID")
		}
		if err := applicationOperations.RetryFailed(ctx, jobapplication.ID(input.ID)); err != nil {
			if errors.Is(err, jobapplicationapp.ErrApplicationUnavailable) {
				return nil, huma.Error404NotFound("application not found")
			}
			if errors.Is(err, jobapplicationapp.ErrInvalidApplicationOperation) {
				return nil, huma.Error400BadRequest("application is not eligible for retry")
			}
			return nil, huma.Error500InternalServerError("application could not be retried")
		}
		return queuedAdminRetry(input.ID), nil
	})
}

func queuedAdminRetry(id string) *adminRetryOutput {
	out := &adminRetryOutput{}
	out.Body.ID = id
	out.Body.Status = "queued"
	return out
}
