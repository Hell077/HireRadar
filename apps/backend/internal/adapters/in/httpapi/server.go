package httpapi

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v3"

	"github.com/Hell077/HireRadar/apps/backend/internal/application/health"
)

type healthOutput struct {
	Body struct {
		Status string `json:"status"`
	}
}

// New builds the Fiber server and registers the Huma HTTP adapter and API.
func New(checker health.Checker) *fiber.App {
	app := fiber.New(fiber.Config{AppName: "HireRadar API"})
	api := humafiber.New(app, huma.DefaultConfig("HireRadar API", "0.1.0"))

	huma.Register(api, huma.Operation{
		OperationID: "health-check",
		Method:      http.MethodGet,
		Path:        "/api/v1/health",
		Summary:     "Check API health",
	}, func(ctx context.Context, _ *struct{}) (*healthOutput, error) {
		result, err := checker.Check(ctx)
		if err != nil {
			return nil, err
		}

		output := &healthOutput{}
		output.Body.Status = result.Status
		return output, nil
	})

	return app
}
