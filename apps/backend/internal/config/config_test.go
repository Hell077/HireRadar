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
	t.Setenv("S3_ENDPOINT", "")
	t.Setenv("S3_PUBLIC_ENDPOINT", "")
	t.Setenv("S3_BUCKET", "")
	t.Setenv("S3_ACCESS_KEY", "")
	t.Setenv("S3_SECRET_KEY", "")
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	t.Setenv("TELEGRAM_BOT_USERNAME", "")
	t.Setenv("TELEGRAM_WEBHOOK_SECRET", "")
	t.Setenv("TELEGRAM_WEBHOOK_URL", "")
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
	t.Setenv("S3_ENDPOINT", "")
	t.Setenv("S3_PUBLIC_ENDPOINT", "")
	t.Setenv("S3_BUCKET", "")
	t.Setenv("S3_ACCESS_KEY", "")
	t.Setenv("S3_SECRET_KEY", "")
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	t.Setenv("TELEGRAM_BOT_USERNAME", "")
	t.Setenv("TELEGRAM_WEBHOOK_SECRET", "")
	t.Setenv("TELEGRAM_WEBHOOK_URL", "")
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
	t.Setenv("JWT_PRIVATE_KEY", "test")
	_, err = Load()
	if err == nil || !strings.Contains(err.Error(), "S3_ENDPOINT") {
		t.Fatalf("Load() error = %v, want missing S3 settings", err)
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

func TestLoadValidatesTelegramSettingsTogetherAndHTTPSWebhook(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("TELEGRAM_BOT_TOKEN", "token")
	t.Setenv("TELEGRAM_BOT_USERNAME", "@hire_radar_bot")
	t.Setenv("TELEGRAM_WEBHOOK_SECRET", "secret")
	t.Setenv("TELEGRAM_WEBHOOK_URL", "http://example.test/webhooks/telegram")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("Load() error = %v, want insecure webhook rejection", err)
	}
	t.Setenv("TELEGRAM_WEBHOOK_URL", "https://example.test/webhooks/telegram")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TelegramBotUsername != "hire_radar_bot" {
		t.Fatalf("bot username = %q", cfg.TelegramBotUsername)
	}
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	if _, err := Load(); err == nil {
		t.Fatal("partial Telegram settings were accepted")
	}
}
