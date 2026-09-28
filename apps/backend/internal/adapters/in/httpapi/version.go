package httpapi

import (
	"context"

	"github.com/Hell077/HireRadar/apps/backend/internal/buildinfo"
	"github.com/danielgtaylor/huma/v2"
)

type versionOutput struct {
	Body struct {
		Version   string `json:"version"`
		Commit    string `json:"commit"`
		BuildTime string `json:"build_time"`
	}
}

func registerVersion(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "version",
		Method:      "GET",
		Path:        "/api/v1/version",
		Summary:     "Read backend build identity",
	}, func(context.Context, *struct{}) (*versionOutput, error) {
		output := &versionOutput{}
		output.Body.Version = buildinfo.Version
		output.Body.Commit = buildinfo.Commit
		output.Body.BuildTime = buildinfo.BuildTime
		return output, nil
	})
}
