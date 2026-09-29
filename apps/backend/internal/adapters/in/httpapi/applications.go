package httpapi

import (
	"context"
	"errors"

	jobapplicationapp "github.com/Hell077/HireRadar/apps/backend/internal/jobapplication/application"
	"github.com/Hell077/HireRadar/apps/backend/internal/jobapplication/domain"
	userdomain "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
)

type ApplicationService interface {
	Request(context.Context, userdomain.UserID, string) (domain.Application, error)
	Get(context.Context, userdomain.UserID, domain.ID) (domain.Application, error)
	List(context.Context, userdomain.UserID, int) ([]domain.Application, error)
	Questions(context.Context, userdomain.UserID, domain.ID) ([]domain.ApplicationQuestion, error)
	AnswerQuestion(context.Context, userdomain.UserID, string, string) error
	Cancel(context.Context, userdomain.UserID, domain.ID) error
	Retry(context.Context, userdomain.UserID, domain.ID) error
}

type applicationRequestInput struct {
	Authorization string `header:"Authorization" required:"false"`
	JobID         string `path:"job_id"`
}

type applicationsGetInput struct {
	Authorization string `header:"Authorization" required:"false"`
	Limit         int    `query:"limit" default:"50" minimum:"1" maximum:"100"`
}

type applicationGetInput struct {
	Authorization string `header:"Authorization" required:"false"`
	ID            string `path:"id"`
}

type applicationQuestionsInput struct {
	Authorization string `header:"Authorization" required:"false"`
	ID            string `path:"id"`
}

type applicationControlInput struct {
	Authorization string `header:"Authorization" required:"false"`
	ID            string `path:"id"`
}

type applicationAnswerInput struct {
	Authorization string `header:"Authorization" required:"false"`
	ID            string `path:"id"`
	Body          struct {
		Answer string `json:"answer" minLength:"1" maxLength:"2000"`
	}
}

type applicationQuestionsOutput struct {
	Body struct {
		Items []domain.ApplicationQuestion `json:"items"`
	}
}

type applicationOutput struct {
	Body struct {
		Application domain.Application `json:"application"`
	}
}

type applicationsOutput struct {
	Body struct {
		Items []domain.Application `json:"items"`
	}
}

func registerApplications(api huma.API, service ApplicationService, verifier AccessVerifier) {
	huma.Register(api, huma.Operation{OperationID: "job-apply-request", Method: "POST", Path: "/api/v1/jobs/{job_id}/apply", Summary: "Queue a job application"}, func(ctx context.Context, input *applicationRequestInput) (*applicationOutput, error) {
		if service == nil {
			return nil, huma.Error503ServiceUnavailable("applications unavailable")
		}
		userID, err := profileUser(input.Authorization, verifier)
		if err != nil {
			return nil, err
		}
		created, err := service.Request(ctx, userID, input.JobID)
		if err != nil {
			if errors.Is(err, jobapplicationapp.ErrApplicationUnavailable) {
				return nil, huma.Error404NotFound("job match is not available")
			}
			return nil, huma.Error500InternalServerError("application could not be queued")
		}
		out := &applicationOutput{}
		out.Body.Application = created
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "applications-list", Method: "GET", Path: "/api/v1/applications", Summary: "List applications"}, func(ctx context.Context, input *applicationsGetInput) (*applicationsOutput, error) {
		if service == nil {
			return nil, huma.Error503ServiceUnavailable("applications unavailable")
		}
		userID, err := profileUser(input.Authorization, verifier)
		if err != nil {
			return nil, err
		}
		items, err := service.List(ctx, userID, input.Limit)
		if err != nil {
			return nil, huma.Error500InternalServerError("applications lookup failed")
		}
		out := &applicationsOutput{}
		out.Body.Items = items
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "applications-get", Method: "GET", Path: "/api/v1/applications/{id}", Summary: "Get application status"}, func(ctx context.Context, input *applicationGetInput) (*applicationOutput, error) {
		if service == nil {
			return nil, huma.Error503ServiceUnavailable("applications unavailable")
		}
		userID, err := profileUser(input.Authorization, verifier)
		if err != nil {
			return nil, err
		}
		item, err := service.Get(ctx, userID, domain.ID(input.ID))
		if err != nil {
			if errors.Is(err, jobapplicationapp.ErrApplicationUnavailable) {
				return nil, huma.Error404NotFound("application not found")
			}
			return nil, huma.Error500InternalServerError("application lookup failed")
		}
		out := &applicationOutput{}
		out.Body.Application = item
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "application-questions-list", Method: "GET", Path: "/api/v1/applications/{id}/questions", Summary: "List application questions"}, func(ctx context.Context, input *applicationQuestionsInput) (*applicationQuestionsOutput, error) {
		if service == nil {
			return nil, huma.Error503ServiceUnavailable("applications unavailable")
		}
		userID, err := profileUser(input.Authorization, verifier)
		if err != nil {
			return nil, err
		}
		if uuid.Validate(input.ID) != nil {
			return nil, huma.Error404NotFound("application not found")
		}
		items, err := service.Questions(ctx, userID, domain.ID(input.ID))
		if err != nil {
			if errors.Is(err, jobapplicationapp.ErrApplicationUnavailable) {
				return nil, huma.Error404NotFound("application not found")
			}
			return nil, huma.Error500InternalServerError("application questions lookup failed")
		}
		out := &applicationQuestionsOutput{}
		out.Body.Items = items
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "application-question-answer", Method: "POST", Path: "/api/v1/application-questions/{id}/answer", Summary: "Answer an application question"}, func(ctx context.Context, input *applicationAnswerInput) (*struct{}, error) {
		if service == nil {
			return nil, huma.Error503ServiceUnavailable("applications unavailable")
		}
		userID, err := profileUser(input.Authorization, verifier)
		if err != nil {
			return nil, err
		}
		if uuid.Validate(input.ID) != nil {
			return nil, huma.Error404NotFound("application question not found")
		}
		if err := service.AnswerQuestion(ctx, userID, input.ID, input.Body.Answer); err != nil {
			if errors.Is(err, jobapplicationapp.ErrApplicationUnavailable) {
				return nil, huma.Error404NotFound("application question not found")
			}
			if errors.Is(err, jobapplicationapp.ErrInvalidApplicationAnswer) {
				return nil, huma.Error400BadRequest("invalid application answer")
			}
			return nil, huma.Error500InternalServerError("application answer could not be saved")
		}
		return &struct{}{}, nil
	})
	huma.Register(api, huma.Operation{OperationID: "application-cancel", Method: "POST", Path: "/api/v1/applications/{id}/cancel", Summary: "Cancel an application"}, func(ctx context.Context, input *applicationControlInput) (*struct{}, error) {
		if service == nil {
			return nil, huma.Error503ServiceUnavailable("applications unavailable")
		}
		userID, err := profileUser(input.Authorization, verifier)
		if err != nil {
			return nil, err
		}
		if uuid.Validate(input.ID) != nil {
			return nil, huma.Error404NotFound("application not found")
		}
		if err := service.Cancel(ctx, userID, domain.ID(input.ID)); err != nil {
			return nil, applicationOperationError(err)
		}
		return &struct{}{}, nil
	})
	huma.Register(api, huma.Operation{OperationID: "application-retry", Method: "POST", Path: "/api/v1/applications/{id}/retry", Summary: "Retry a failed application"}, func(ctx context.Context, input *applicationControlInput) (*struct{}, error) {
		if service == nil {
			return nil, huma.Error503ServiceUnavailable("applications unavailable")
		}
		userID, err := profileUser(input.Authorization, verifier)
		if err != nil {
			return nil, err
		}
		if uuid.Validate(input.ID) != nil {
			return nil, huma.Error404NotFound("application not found")
		}
		if err := service.Retry(ctx, userID, domain.ID(input.ID)); err != nil {
			return nil, applicationOperationError(err)
		}
		return &struct{}{}, nil
	})
}

func applicationOperationError(err error) error {
	if errors.Is(err, jobapplicationapp.ErrApplicationUnavailable) {
		return huma.Error404NotFound("application not found")
	}
	if errors.Is(err, jobapplicationapp.ErrInvalidApplicationOperation) {
		return huma.Error409Conflict("application cannot perform this operation")
	}
	return huma.Error500InternalServerError("application operation failed")
}
