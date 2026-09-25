package httpapi

import (
	"context"
	"crypto/subtle"
	"errors"

	sourcepostgres "github.com/Hell077/HireRadar/apps/backend/internal/source/adapters/postgres"
	"github.com/Hell077/HireRadar/apps/backend/internal/source/domain"
	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
)

type SourceCatalog interface {
	ListEnabled(context.Context) ([]domain.Source, error)
}

type SourceOperations interface {
	ListAll(context.Context) ([]domain.Source, error)
	UpdateSettings(context.Context, string, string, sourcepostgres.SourceSettings) error
	RequestSync(context.Context, string, string) error
	ListAudit(context.Context, int) ([]sourcepostgres.SourceAudit, error)
}
type adminSourcesInput struct {
	Token string `header:"X-Operator-Token" required:"false"`
}
type adminAuditInput struct {
	Token string `header:"X-Operator-Token" required:"false"`
	Limit int    `query:"limit" default:"50" minimum:"1" maximum:"200"`
}
type adminSourceUpdateInput struct {
	Token string `header:"X-Operator-Token" required:"false"`
	ID    string `path:"id"`
	Body  sourcepostgres.SourceSettings
}
type adminSourceSyncInput struct {
	Token string `header:"X-Operator-Token" required:"false"`
	ID    string `path:"id"`
}
type adminSourcesOutput struct {
	Body struct {
		Sources []domain.Source `json:"sources"`
	}
}
type adminAuditOutput struct {
	Body struct {
		Events []sourcepostgres.SourceAudit `json:"events"`
	}
}
type adminOperationOutput struct {
	Body struct {
		Status string `json:"status"`
	}
}

func registerSourceOperations(api huma.API, operations SourceOperations, token string) {
	validToken := func(value string) bool {
		return token != "" && len(value) == len(token) && subtle.ConstantTimeCompare([]byte(value), []byte(token)) == 1
	}
	unauthorized := func(value string) error {
		if !validToken(value) {
			if token == "" {
				return huma.Error503ServiceUnavailable("source operations unavailable")
			}
			return huma.Error401Unauthorized("operator token required")
		}
		return nil
	}
	huma.Register(api, huma.Operation{OperationID: "admin-sources-list", Method: "GET", Path: "/api/v1/admin/sources", Summary: "List all sources and sync health"}, func(ctx context.Context, input *adminSourcesInput) (*adminSourcesOutput, error) {
		if err := unauthorized(input.Token); err != nil {
			return nil, err
		}
		if operations == nil {
			return nil, huma.Error503ServiceUnavailable("source operations unavailable")
		}
		items, err := operations.ListAll(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("source list failed")
		}
		out := &adminSourcesOutput{}
		out.Body.Sources = items
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "admin-source-settings", Method: "PUT", Path: "/api/v1/admin/sources/{id}", Summary: "Update source polling settings"}, func(ctx context.Context, input *adminSourceUpdateInput) (*adminOperationOutput, error) {
		if err := unauthorized(input.Token); err != nil {
			return nil, err
		}
		if operations == nil {
			return nil, huma.Error503ServiceUnavailable("source operations unavailable")
		}
		if err := operations.UpdateSettings(ctx, input.ID, "operator", input.Body); err != nil {
			if errors.Is(err, domain.ErrInvalidSource) {
				return nil, huma.Error400BadRequest("invalid source settings")
			}
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, huma.Error404NotFound("source not found")
			}
			return nil, huma.Error500InternalServerError("source settings update failed")
		}
		out := &adminOperationOutput{}
		out.Body.Status = "updated"
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "admin-source-sync", Method: "POST", Path: "/api/v1/admin/sources/{id}/sync", Summary: "Schedule a source sync"}, func(ctx context.Context, input *adminSourceSyncInput) (*adminOperationOutput, error) {
		if err := unauthorized(input.Token); err != nil {
			return nil, err
		}
		if operations == nil {
			return nil, huma.Error503ServiceUnavailable("source operations unavailable")
		}
		if err := operations.RequestSync(ctx, input.ID, "operator"); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, huma.Error404NotFound("source not found")
			}
			if errors.Is(err, domain.ErrInvalidSource) {
				return nil, huma.Error400BadRequest("invalid source ID")
			}
			return nil, huma.Error500InternalServerError("source sync request failed")
		}
		out := &adminOperationOutput{}
		out.Body.Status = "scheduled"
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "admin-source-audit", Method: "GET", Path: "/api/v1/admin/sources/audit", Summary: "Read source operator audit trail"}, func(ctx context.Context, input *adminAuditInput) (*adminAuditOutput, error) {
		if err := unauthorized(input.Token); err != nil {
			return nil, err
		}
		if operations == nil {
			return nil, huma.Error503ServiceUnavailable("source operations unavailable")
		}
		items, err := operations.ListAudit(ctx, input.Limit)
		if err != nil {
			return nil, huma.Error500InternalServerError("source audit lookup failed")
		}
		out := &adminAuditOutput{}
		out.Body.Events = items
		return out, nil
	})
}

type sourceCatalogOutput struct {
	Body struct {
		Sources []domain.Source `json:"sources"`
	}
}

func registerSources(api huma.API, catalog SourceCatalog) {
	huma.Register(api, huma.Operation{OperationID: "sources-list", Method: "GET", Path: "/api/v1/sources", Summary: "List enabled job sources"}, func(ctx context.Context, _ *struct{}) (*sourceCatalogOutput, error) {
		if catalog == nil {
			return nil, huma.Error503ServiceUnavailable("source catalog unavailable")
		}
		sources, err := catalog.ListEnabled(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("source catalog lookup failed")
		}
		output := &sourceCatalogOutput{}
		output.Body.Sources = sources
		return output, nil
	})
}
