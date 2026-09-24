package httpapi

import (
	"context"
	"errors"

	profile "github.com/Hell077/HireRadar/apps/backend/internal/profile/domain"
	user "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/danielgtaylor/huma/v2"
)

type SkillService interface {
	GetSkills(context.Context, user.UserID) ([]profile.Skill, error)
	SaveSkills(context.Context, user.UserID, []profile.Skill) error
}

type skillsGetInput struct {
	Authorization string `header:"Authorization" required:"false"`
}
type skillsPutInput struct {
	Authorization string `header:"Authorization" required:"false"`
	Body          struct {
		Skills []profile.Skill `json:"skills" required:"true" maxItems:"100"`
	}
}
type skillsOutput struct {
	Body struct {
		Skills []profile.Skill `json:"skills"`
	}
}

func registerSkills(api huma.API, service SkillService, verifier AccessVerifier) {
	huma.Register(api, huma.Operation{
		OperationID: "profile-skills-get", Method: "GET", Path: "/api/v1/profile/skills", Summary: "Get candidate skills",
	}, func(ctx context.Context, input *skillsGetInput) (*skillsOutput, error) {
		if service == nil {
			return nil, huma.Error503ServiceUnavailable("profile unavailable")
		}
		userID, err := profileUser(input.Authorization, verifier)
		if err != nil {
			return nil, err
		}
		skills, err := service.GetSkills(ctx, userID)
		if err != nil {
			return nil, huma.Error500InternalServerError("skill lookup failed")
		}
		output := &skillsOutput{}
		output.Body.Skills = skills
		return output, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "profile-skills-put", Method: "PUT", Path: "/api/v1/profile/skills", Summary: "Replace candidate skills",
	}, func(ctx context.Context, input *skillsPutInput) (*skillsOutput, error) {
		if service == nil {
			return nil, huma.Error503ServiceUnavailable("profile unavailable")
		}
		userID, err := profileUser(input.Authorization, verifier)
		if err != nil {
			return nil, err
		}
		if err := service.SaveSkills(ctx, userID, input.Body.Skills); err != nil {
			if errors.Is(err, profile.ErrInvalidSkills) {
				return nil, huma.Error400BadRequest("invalid skills")
			}
			return nil, huma.Error500InternalServerError("skill update failed")
		}
		skills, err := service.GetSkills(ctx, userID)
		if err != nil {
			return nil, huma.Error500InternalServerError("skill lookup failed")
		}
		output := &skillsOutput{}
		output.Body.Skills = skills
		return output, nil
	})
}
