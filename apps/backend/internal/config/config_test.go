package config

import (
	"strings"
	"testing"
)

func TestLoadDevelopmentDefaults(t *testing.T) {
	t.Setenv("HIRERADAR_MODE", "")
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
	t.Setenv("OPERATOR_API_TOKEN", "")
	t.Setenv("GREENHOUSE_API_KEYS_JSON", "")
	t.Setenv("LEVER_API_KEYS_JSON", "")
	t.Setenv("ASHBY_API_KEYS_JSON", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Environment != "development" || cfg.Port != "8080" || cfg.RuntimeMode != "all" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestLoadRuntimeMode(t *testing.T) {
	for _, mode := range []string{"api", "worker", "all"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("HIRERADAR_MODE", mode)
			cfg, err := Load()
			if err != nil || cfg.RuntimeMode != mode {
				t.Fatalf("mode=%q config=%+v err=%v", mode, cfg, err)
			}
		})
	}
	t.Setenv("HIRERADAR_MODE", "invalid")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "HIRERADAR_MODE") {
		t.Fatalf("invalid mode error=%v", err)
	}
}

func TestLoadValidatesLeverAPIKeysWithoutReturningSecrets(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("LEVER_API_KEYS_JSON", `{"acme":"employer-secret"}`)
	cfg, err := Load()
	if err != nil || cfg.LeverAPIKeys["acme"] != "employer-secret" {
		t.Fatalf("Lever credentials were not loaded: %+v, %v", cfg.LeverAPIKeys, err)
	}
	t.Setenv("LEVER_API_KEYS_JSON", `{"../acme":"employer-secret"}`)
	if _, err := Load(); err == nil || strings.Contains(err.Error(), "employer-secret") {
		t.Fatalf("invalid Lever credentials leaked or were accepted: %v", err)
	}
}

func TestLoadRejectsWeakOperatorToken(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("OPERATOR_API_TOKEN", "short")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "OPERATOR_API_TOKEN") {
		t.Fatalf("Load() error=%v, want weak operator token rejection", err)
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
