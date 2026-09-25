package httpapi

import (
	"context"
	"errors"

	jobpostgres "github.com/Hell077/HireRadar/apps/backend/internal/job/adapters/postgres"
	jobdomain "github.com/Hell077/HireRadar/apps/backend/internal/job/domain"
	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
)

type JobCatalog interface {
	List(context.Context, jobpostgres.ListQuery) (jobpostgres.ListResult, error)
	Get(context.Context, string) (jobdomain.Job, error)
}

type jobsListInput struct {
	Cursor       string `query:"cursor"`
	Status       string `query:"status"`
	RemotePolicy string `query:"remote_policy"`
	Country      string `query:"country"`
	SourceID     string `query:"source"`
	Eligibility  string `query:"eligibility"`
	Limit        int    `query:"limit" default:"25" minimum:"1" maximum:"100"`
}

type jobsListOutput struct{ Body jobpostgres.ListResult }
type jobGetInput struct {
	ID string `path:"id"`
}
type jobGetOutput struct{ Body jobdomain.Job }

func registerJobs(api huma.API, catalog JobCatalog) {
	huma.Register(api, huma.Operation{OperationID: "jobs-list", Method: "GET", Path: "/api/v1/jobs", Summary: "List normalized jobs", Description: "Returns a cursor-paginated list of normalized jobs. Unknown country eligibility is preserved as unknown."}, func(ctx context.Context, input *jobsListInput) (*jobsListOutput, error) {
		if catalog == nil {
			return nil, huma.Error503ServiceUnavailable("job catalog unavailable")
		}
		result, err := catalog.List(ctx, jobpostgres.ListQuery{Cursor: input.Cursor, Status: input.Status, RemotePolicy: input.RemotePolicy, Country: input.Country, SourceID: input.SourceID, Eligibility: input.Eligibility, Limit: input.Limit})
		if errors.Is(err, jobdomain.ErrInvalidCursor) || errors.Is(err, jobdomain.ErrInvalidQuery) {
			return nil, huma.Error400BadRequest("invalid job list query")
		}
		if err != nil {
			return nil, huma.Error500InternalServerError("job catalog lookup failed")
		}
		return &jobsListOutput{Body: result}, nil
	})
	huma.Register(api, huma.Operation{OperationID: "jobs-get", Method: "GET", Path: "/api/v1/jobs/{id}", Summary: "Read a normalized job"}, func(ctx context.Context, input *jobGetInput) (*jobGetOutput, error) {
		if catalog == nil {
			return nil, huma.Error503ServiceUnavailable("job catalog unavailable")
		}
		item, err := catalog.Get(ctx, input.ID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, huma.Error404NotFound("job not found")
		}
		if errors.Is(err, jobdomain.ErrInvalidQuery) {
			return nil, huma.Error400BadRequest("invalid job ID")
		}
		if err != nil {
			return nil, huma.Error500InternalServerError("job lookup failed")
		}
		return &jobGetOutput{Body: item}, nil
	})
}
