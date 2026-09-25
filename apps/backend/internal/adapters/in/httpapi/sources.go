package httpapi

import (
	"context"

	"github.com/Hell077/HireRadar/apps/backend/internal/source/domain"
	"github.com/danielgtaylor/huma/v2"
)

type SourceCatalog interface {
	ListEnabled(context.Context) ([]domain.Source, error)
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
