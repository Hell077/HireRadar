package httpapi

import (
	"context"
	"errors"

	jobdomain "github.com/Hell077/HireRadar/apps/backend/internal/job/domain"
	matchapp "github.com/Hell077/HireRadar/apps/backend/internal/matching/application"
	"github.com/Hell077/HireRadar/apps/backend/internal/matching/engine"
	user "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/danielgtaylor/huma/v2"
)

type MatchService interface {
	Refresh(context.Context, user.UserID) ([]engine.Result, error)
	List(context.Context, user.UserID, int) ([]engine.Result, error)
	ListPage(context.Context, user.UserID, string, int) (matchapp.MatchPage, error)
}

type matchesGetInput struct {
	Authorization string `header:"Authorization" required:"false"`
	Cursor        string `query:"cursor" maxLength:"512"`
	Limit         int    `query:"limit" default:"20" minimum:"1" maximum:"100"`
}
type matchesRefreshInput struct {
	Authorization string `header:"Authorization" required:"false"`
}
type matchesOutput struct {
	Body struct {
		Matches    []engine.Result `json:"matches"`
		NextCursor string          `json:"next_cursor"`
	}
}

func registerMatches(api huma.API, service MatchService, verifier AccessVerifier) {
	huma.Register(api, huma.Operation{OperationID: "matches-list", Method: "GET", Path: "/api/v1/matches", Summary: "List saved candidate matches"}, func(ctx context.Context, input *matchesGetInput) (*matchesOutput, error) {
		if service == nil {
			return nil, huma.Error503ServiceUnavailable("matching service unavailable")
		}
		id, err := profileUser(input.Authorization, verifier)
		if err != nil {
			return nil, err
		}
		page, err := service.ListPage(ctx, id, input.Cursor, input.Limit)
		if err != nil {
			if errors.Is(err, jobdomain.ErrInvalidCursor) {
				return nil, huma.Error400BadRequest("invalid match cursor")
			}
			return nil, huma.Error500InternalServerError("match lookup failed")
		}
		output := &matchesOutput{}
		output.Body.Matches = page.Matches
		output.Body.NextCursor = page.NextCursor
		return output, nil
	})
	huma.Register(api, huma.Operation{OperationID: "matches-refresh", Method: "POST", Path: "/api/v1/matches/refresh", Summary: "Recalculate and save candidate matches", Description: "Only jobs that pass hard eligibility and preference filters are scored and saved."}, func(ctx context.Context, input *matchesRefreshInput) (*matchesOutput, error) {
		if service == nil {
			return nil, huma.Error503ServiceUnavailable("matching service unavailable")
		}
		id, err := profileUser(input.Authorization, verifier)
		if err != nil {
			return nil, err
		}
		matches, err := service.Refresh(ctx, id)
		if err != nil {
			return nil, huma.Error500InternalServerError("match refresh failed")
		}
		output := &matchesOutput{}
		output.Body.Matches = matches
		return output, nil
	})
}
