package config

import (
	"strings"
	"testing"
)

func TestLoadDevelopmentDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("PORT", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("REDIS_URL", "")
	t.Setenv("JWT_PRIVATE_KEY", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Environment != "development" || cfg.Port != "8080" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestLoadRejectsProductionWithoutDependencies(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("REDIS_URL", "")
	t.Setenv("JWT_PRIVATE_KEY", "")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("Load() error = %v, want missing DATABASE_URL", err)
	}
	t.Setenv("DATABASE_URL", "postgres://localhost/hireradar")
	_, err = Load()
	if err == nil || !strings.Contains(err.Error(), "REDIS_URL") {
		t.Fatalf("Load() error = %v, want missing REDIS_URL", err)
	}
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	_, err = Load()
	if err == nil || !strings.Contains(err.Error(), "JWT_PRIVATE_KEY") {
		t.Fatalf("Load() error = %v, want missing JWT_PRIVATE_KEY", err)
	}
}

func TestLoadRejectsInvalidPort(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("PORT", "70000")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "PORT") {
		t.Fatalf("Load() error = %v, want invalid PORT", err)
	}
}
