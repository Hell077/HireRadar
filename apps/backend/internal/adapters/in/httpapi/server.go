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
	Registrar     Registrar
	Sessions      Sessions
	Email         EmailActions
	Limiter       AuthLimiter
	Profile       ProfileService
	Skills        SkillService
	Positions     PositionService
	Preferences   PreferencesService
	Sources       SourcePreferenceService
	Resumes       ResumeService
	SourceCatalog SourceCatalog
	Jobs          JobCatalog
	Matches       MatchService
	Verifier      AccessVerifier
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
	if len(auth) > 0 && auth[0].Limiter != nil {
		app.Use("/api/v1/auth", rateLimitAuth(auth[0].Limiter))
	}
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
	services := AuthServices{Registrar: unavailableRegistrar{}, Sessions: unavailableSessions{}, Email: unavailableEmail{}}
	if len(auth) > 0 {
		if auth[0].Registrar != nil {
			services.Registrar = auth[0].Registrar
		}
		if auth[0].Sessions != nil {
			services.Sessions = auth[0].Sessions
		}
		if auth[0].Email != nil {
			services.Email = auth[0].Email
		}
		services.Profile = auth[0].Profile
		services.Skills = auth[0].Skills
		services.Positions = auth[0].Positions
		services.Preferences = auth[0].Preferences
		services.Sources = auth[0].Sources
		services.Resumes = auth[0].Resumes
		services.SourceCatalog = auth[0].SourceCatalog
		services.Jobs = auth[0].Jobs
		services.Matches = auth[0].Matches
		services.Verifier = auth[0].Verifier
	}
	registerAuth(api, services.Registrar)
	registerSessions(api, services.Sessions)
	registerEmail(api, services.Email)
	registerProfile(api, services.Profile, services.Verifier)
	registerSkills(api, services.Skills, services.Verifier)
	registerPositions(api, services.Positions, services.Verifier)
	registerPreferences(api, services.Preferences, services.Verifier)
	registerSourcePreferences(api, services.Sources, services.Verifier)
	registerResumes(api, services.Resumes, services.Verifier)
	registerSources(api, services.SourceCatalog)
	registerJobs(api, services.Jobs)
	registerMatches(api, services.Matches, services.Verifier)

	return app
}
