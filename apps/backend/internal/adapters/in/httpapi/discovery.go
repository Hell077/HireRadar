package httpapi

import (
	"context"
	"crypto/subtle"
	"fmt"

	"github.com/Hell077/HireRadar/apps/backend/internal/discovery/domain"
	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
)

type DiscoveryOperations interface {
	Overview(context.Context) (domain.Overview, error)
	ScheduleRun(context.Context, string) error
}

type adminDiscoveryInput struct {
	Token string `header:"X-Operator-Token" required:"false"`
}

type adminDiscoveryRunInput struct {
	Token string `header:"X-Operator-Token" required:"false"`
	ID    string `path:"id"`
}

type adminDiscoveryOverviewOutput struct {
	Body domain.Overview
}

type adminDiscoveryRunOutput struct {
	Body struct {
		Status string `json:"status"`
		ID     string `json:"id"`
	}
}

func registerDiscoveryOperations(api huma.API, operations DiscoveryOperations, token string) {
	validToken := func(value string) bool {
		return token != "" && len(value) == len(token) && subtle.ConstantTimeCompare([]byte(value), []byte(token)) == 1
	}
	unauthorized := func(value string) error {
		if validToken(value) {
			return nil
		}
		if token == "" {
			return huma.Error503ServiceUnavailable("discovery operations unavailable")
		}
		return huma.Error401Unauthorized("operator token required")
	}
	huma.Register(api, huma.Operation{OperationID: "admin-discovery-overview", Method: "GET", Path: "/api/v1/admin/discovery", Summary: "Read source discovery status"}, func(ctx context.Context, input *adminDiscoveryInput) (*adminDiscoveryOverviewOutput, error) {
		if err := unauthorized(input.Token); err != nil {
			return nil, err
		}
		if operations == nil {
			return nil, huma.Error503ServiceUnavailable("discovery operations unavailable")
		}
		result, err := operations.Overview(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("discovery status lookup failed")
		}
		return &adminDiscoveryOverviewOutput{Body: result}, nil
	})
	huma.Register(api, huma.Operation{OperationID: "admin-discovery-run", Method: "POST", Path: "/api/v1/admin/discovery/{id}/run", Summary: "Schedule a source discovery run"}, func(ctx context.Context, input *adminDiscoveryRunInput) (*adminDiscoveryRunOutput, error) {
		if err := unauthorized(input.Token); err != nil {
			return nil, err
		}
		if operations == nil {
			return nil, huma.Error503ServiceUnavailable("discovery operations unavailable")
		}
		if err := operations.ScheduleRun(ctx, input.ID); err != nil {
			if err == pgx.ErrNoRows {
				return nil, huma.Error404NotFound("discovery source not found or disabled")
			}
			return nil, huma.Error400BadRequest(fmt.Sprintf("discovery run could not be scheduled: %v", err))
		}
		output := &adminDiscoveryRunOutput{}
		output.Body.Status = "scheduled"
		output.Body.ID = input.ID
		return output, nil
	})
}
