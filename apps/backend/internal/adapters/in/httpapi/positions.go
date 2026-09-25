package httpapi

import (
	"context"
	"errors"

	profile "github.com/Hell077/HireRadar/apps/backend/internal/profile/domain"
	user "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/danielgtaylor/huma/v2"
)

type PositionService interface {
	GetPositions(context.Context, user.UserID) ([]string, error)
	SavePositions(context.Context, user.UserID, []string) error
}

type positionsGetInput struct {
	Authorization string `header:"Authorization" required:"false"`
}
type positionsPutInput struct {
	Authorization string `header:"Authorization" required:"false"`
	Body          struct {
		Positions []string `json:"positions" required:"true" maxItems:"20"`
	}
}
type positionsOutput struct {
	Body struct {
		Positions []string `json:"positions"`
	}
}

func registerPositions(api huma.API, service PositionService, verifier AccessVerifier) {
	huma.Register(api, huma.Operation{
		OperationID: "profile-positions-get", Method: "GET", Path: "/api/v1/profile/positions", Summary: "Get desired positions",
	}, func(ctx context.Context, input *positionsGetInput) (*positionsOutput, error) {
		if service == nil {
			return nil, huma.Error503ServiceUnavailable("profile unavailable")
		}
		userID, err := profileUser(input.Authorization, verifier)
		if err != nil {
			return nil, err
		}
		positions, err := service.GetPositions(ctx, userID)
		if err != nil {
			return nil, huma.Error500InternalServerError("position lookup failed")
		}
		output := &positionsOutput{}
		output.Body.Positions = positions
		return output, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "profile-positions-put", Method: "PUT", Path: "/api/v1/profile/positions", Summary: "Replace desired positions",
	}, func(ctx context.Context, input *positionsPutInput) (*positionsOutput, error) {
		if service == nil {
			return nil, huma.Error503ServiceUnavailable("profile unavailable")
		}
		userID, err := profileUser(input.Authorization, verifier)
		if err != nil {
			return nil, err
		}
		if err := service.SavePositions(ctx, userID, input.Body.Positions); err != nil {
			if errors.Is(err, profile.ErrInvalidPositions) {
				return nil, huma.Error400BadRequest("invalid positions")
			}
			return nil, huma.Error500InternalServerError("position update failed")
		}
		positions, err := service.GetPositions(ctx, userID)
		if err != nil {
			return nil, huma.Error500InternalServerError("position lookup failed")
		}
		output := &positionsOutput{}
		output.Body.Positions = positions
		return output, nil
	})
}
