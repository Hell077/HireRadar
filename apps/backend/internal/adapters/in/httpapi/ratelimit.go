package httpapi

import (
	"context"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
)

type AuthLimiter interface {
	Allow(context.Context, string, string, int64, time.Duration) (bool, error)
}

type limitPolicy struct {
	limit  int64
	window time.Duration
}

var authLimits = map[string]limitPolicy{
	"/api/v1/auth/register":        {5, time.Hour},
	"/api/v1/auth/login":           {10, time.Minute},
	"/api/v1/auth/refresh":         {30, time.Minute},
	"/api/v1/auth/logout":          {30, time.Minute},
	"/api/v1/auth/verify-email":    {10, time.Minute},
	"/api/v1/auth/forgot-password": {5, time.Hour},
	"/api/v1/auth/reset-password":  {10, time.Minute},
}

func rateLimitAuth(limiter AuthLimiter) fiber.Handler {
	return func(c fiber.Ctx) error {
		if c.Method() != "POST" {
			return c.Next()
		}
		policy, ok := authLimits[c.Path()]
		if !ok {
			return c.Next()
		}
		allowed, err := limiter.Allow(c.Context(), c.Path(), c.IP(), policy.limit, policy.window)
		if err != nil {
			slog.Error("auth rate limiter unavailable", "error", err)
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "authentication unavailable"})
		}
		if !allowed {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "too many requests"})
		}
		return c.Next()
	}
}
