package httpapi

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
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

type requestMetric struct {
	count    uint64
	duration float64
}

var requestMetrics = struct {
	sync.Mutex
	values map[string]requestMetric
}{values: make(map[string]requestMetric)}

type AuthServices struct {
	Registrar             Registrar
	Sessions              Sessions
	Email                 EmailActions
	Limiter               AuthLimiter
	Profile               ProfileService
	Skills                SkillService
	Positions             PositionService
	Preferences           PreferencesService
	Sources               SourcePreferenceService
	Resumes               ResumeService
	SourceCatalog         SourceCatalog
	Jobs                  JobCatalog
	Matches               MatchService
	Telegram              TelegramService
	TelegramWebhookSecret string
	Verifier              AccessVerifier
}

// New builds the Fiber server and registers the Huma HTTP adapter and API.
func New(checker health.Checker, auth ...AuthServices) *fiber.App {
	app := fiber.New(fiber.Config{AppName: "HireRadar API", ReadTimeout: 10 * time.Second})
	app.Use(requestid.New())
	app.Use(func(c fiber.Ctx) error {
		started := time.Now()
		err := c.Next()
		elapsed := time.Since(started)
		slog.Info("http request", "method", c.Method(), "path", c.Path(), "status", c.Response().StatusCode(), "request_id", requestid.FromContext(c), "duration_ms", elapsed.Milliseconds())
		key := fmt.Sprintf("%s|%d", c.Method(), c.Response().StatusCode())
		requestMetrics.Lock()
		metric := requestMetrics.values[key]
		metric.count++
		metric.duration += elapsed.Seconds()
		requestMetrics.values[key] = metric
		requestMetrics.Unlock()
		return err
	})
	app.Get("/metrics", func(c fiber.Ctx) error {
		requestMetrics.Lock()
		keys := make([]string, 0, len(requestMetrics.values))
		for key := range requestMetrics.values {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		var output strings.Builder
		output.WriteString("# HELP hireradar_http_requests_total Total HTTP requests by method and status.\n# TYPE hireradar_http_requests_total counter\n")
		for _, key := range keys {
			parts := strings.Split(key, "|")
			metric := requestMetrics.values[key]
			fmt.Fprintf(&output, "hireradar_http_requests_total{method=%q,status=%q} %d\n", parts[0], parts[1], metric.count)
		}
		output.WriteString("# HELP hireradar_http_request_duration_seconds_sum Cumulative HTTP request duration in seconds.\n# TYPE hireradar_http_request_duration_seconds_sum counter\n")
		for _, key := range keys {
			parts := strings.Split(key, "|")
			metric := requestMetrics.values[key]
			fmt.Fprintf(&output, "hireradar_http_request_duration_seconds_sum{method=%q,status=%q} %g\n", parts[0], parts[1], metric.duration)
		}
		requestMetrics.Unlock()
		c.Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		return c.SendString(output.String())
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
		services.Telegram = auth[0].Telegram
		services.TelegramWebhookSecret = auth[0].TelegramWebhookSecret
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
	registerTelegram(api, services.Telegram, services.Verifier, services.TelegramWebhookSecret)

	return app
}
