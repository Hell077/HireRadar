package httpapi

import (
	"context"
	"log/slog"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"

	"github.com/Hell077/HireRadar/apps/backend/internal/application/health"
)

type healthOutput struct {
	Body struct {
		Status string `json:"status"`
	}
}

type AuthServices struct {
	Registrar Registrar
	Sessions  Sessions
}

// New builds the Fiber server and registers the Huma HTTP adapter and API.
func New(checker health.Checker, auth ...AuthServices) *fiber.App {
	app := fiber.New(fiber.Config{AppName: "HireRadar API", ReadTimeout: 10 * time.Second})
	app.Use(requestid.New())
	app.Use(func(c fiber.Ctx) error {
		started := time.Now()
		err := c.Next()
		slog.Info("http request", "method", c.Method(), "path", c.Path(), "status", c.Response().StatusCode(), "request_id", requestid.FromContext(c), "duration_ms", time.Since(started).Milliseconds())
		return err
	})
	api := humafiber.New(app, huma.DefaultConfig("HireRadar API", "0.1.0"))

	huma.Register(api, huma.Operation{
		OperationID: "health-check",
		Method:      "GET",
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
	huma.Register(api, huma.Operation{
		OperationID: "readiness-check",
		Method:      "GET",
		Path:        "/api/v1/ready",
		Summary:     "Check required backend dependencies",
	}, func(ctx context.Context, _ *struct{}) (*healthOutput, error) {
		result, err := checker.Ready(ctx)
		if err != nil {
			slog.Warn("backend dependencies unavailable", "error", err)
			return nil, huma.Error503ServiceUnavailable("required dependencies unavailable")
		}
		output := &healthOutput{}
		output.Body.Status = result.Status
		return output, nil
	})
	services := AuthServices{Registrar: unavailableRegistrar{}, Sessions: unavailableSessions{}}
	if len(auth) > 0 {
		if auth[0].Registrar != nil {
			services.Registrar = auth[0].Registrar
		}
		if auth[0].Sessions != nil {
			services.Sessions = auth[0].Sessions
		}
	}
	registerAuth(api, services.Registrar)
	registerSessions(api, services.Sessions)

	return app
}
