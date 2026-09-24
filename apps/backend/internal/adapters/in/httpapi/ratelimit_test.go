package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/application/health"
)

type fakeLimiter struct {
	calls int
	err   error
}

func (f *fakeLimiter) Allow(_ context.Context, route, ip string, limit int64, window time.Duration) (bool, error) {
	f.calls++
	if route != "/api/v1/auth/forgot-password" || ip == "" || limit != 5 || window != time.Hour {
		return false, errors.New("wrong rate limit dimensions")
	}
	return f.calls <= 1, f.err
}

func TestAuthRateLimit(t *testing.T) {
	limiter := &fakeLimiter{}
	app := New(health.NewService(), AuthServices{Limiter: limiter})
	request := func() int {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password", strings.NewReader(`{"email":"person@example.com"}`))
		req.Header.Set("Content-Type", "application/json")
		res, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		return res.StatusCode
	}
	if got := request(); got != http.StatusServiceUnavailable {
		t.Fatalf("first status = %d", got)
	}
	if got := request(); got != http.StatusTooManyRequests {
		t.Fatalf("second status = %d", got)
	}
	limiter.err = errors.New("redis unavailable")
	if got := request(); got != http.StatusServiceUnavailable {
		t.Fatalf("redis failure status = %d", got)
	}
}
