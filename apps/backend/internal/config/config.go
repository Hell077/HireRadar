package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Environment           string
	Port                  string
	DatabaseURL           string
	RedisURL              string
	JWTPrivateKey         string
	S3Endpoint            string
	S3PublicEndpoint      string
	S3Bucket              string
	S3AccessKey           string
	S3SecretKey           string
	TelegramBotToken      string
	TelegramBotUsername   string
	TelegramWebhookSecret string
	TelegramWebhookURL    string
	OperatorAPIToken      string
}

// Load validates process configuration before opening external connections.
// Development can run without infrastructure so OpenAPI types can be generated;
// readiness remains unavailable until both dependencies are configured.
func Load() (Config, error) {
	cfg := Config{
		Environment:           os.Getenv("APP_ENV"),
		Port:                  os.Getenv("PORT"),
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		RedisURL:              os.Getenv("REDIS_URL"),
		JWTPrivateKey:         os.Getenv("JWT_PRIVATE_KEY"),
		S3Endpoint:            os.Getenv("S3_ENDPOINT"),
		S3PublicEndpoint:      os.Getenv("S3_PUBLIC_ENDPOINT"),
		S3Bucket:              os.Getenv("S3_BUCKET"),
		S3AccessKey:           os.Getenv("S3_ACCESS_KEY"),
		S3SecretKey:           os.Getenv("S3_SECRET_KEY"),
		TelegramBotToken:      os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramBotUsername:   os.Getenv("TELEGRAM_BOT_USERNAME"),
		TelegramWebhookSecret: os.Getenv("TELEGRAM_WEBHOOK_SECRET"),
		TelegramWebhookURL:    os.Getenv("TELEGRAM_WEBHOOK_URL"),
		OperatorAPIToken:      os.Getenv("OPERATOR_API_TOKEN"),
	}
	cfg.TelegramBotUsername = strings.TrimPrefix(cfg.TelegramBotUsername, "@")
	telegramValues := []string{cfg.TelegramBotToken, cfg.TelegramBotUsername, cfg.TelegramWebhookSecret}
	configuredTelegram := 0
	for _, value := range telegramValues {
		if value != "" {
			configuredTelegram++
		}
	}
	if configuredTelegram != 0 && configuredTelegram != len(telegramValues) {
		return Config{}, fmt.Errorf("TELEGRAM_BOT_TOKEN, TELEGRAM_BOT_USERNAME, and TELEGRAM_WEBHOOK_SECRET must be configured together")
	}
	if configuredTelegram == len(telegramValues) {
		if len(cfg.TelegramBotUsername) < 5 || len(cfg.TelegramBotUsername) > 32 || !strings.HasSuffix(strings.ToLower(cfg.TelegramBotUsername), "bot") {
			return Config{}, fmt.Errorf("TELEGRAM_BOT_USERNAME must be a 5–32 character Telegram bot username ending in bot")
		}
		for _, char := range cfg.TelegramBotUsername {
			if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '_') {
				return Config{}, fmt.Errorf("TELEGRAM_BOT_USERNAME contains an invalid character")
			}
		}
		if len(cfg.TelegramWebhookSecret) < 1 || len(cfg.TelegramWebhookSecret) > 256 {
			return Config{}, fmt.Errorf("TELEGRAM_WEBHOOK_SECRET must be between 1 and 256 characters")
		}
		for _, char := range cfg.TelegramWebhookSecret {
			if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '_' || char == '-') {
				return Config{}, fmt.Errorf("TELEGRAM_WEBHOOK_SECRET contains an invalid character")
			}
		}
	}
	if cfg.TelegramWebhookURL != "" {
		parsed, err := url.ParseRequestURI(cfg.TelegramWebhookURL)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || configuredTelegram != len(telegramValues) {
			return Config{}, fmt.Errorf("TELEGRAM_WEBHOOK_URL requires complete Telegram settings and an HTTPS URL")
		}
	}
	if cfg.Environment == "" {
		cfg.Environment = "development"
	}
	if cfg.Environment != "development" && cfg.Environment != "production" && cfg.Environment != "test" {
		return Config{}, fmt.Errorf("APP_ENV must be development, test, or production")
	}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	port, err := strconv.Atoi(cfg.Port)
	if err != nil || port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("PORT must be an integer between 1 and 65535")
	}
	if cfg.Environment == "production" {
		if cfg.DatabaseURL == "" {
			return Config{}, fmt.Errorf("DATABASE_URL is required in production")
		}
		if cfg.RedisURL == "" {
			return Config{}, fmt.Errorf("REDIS_URL is required in production")
		}
		if cfg.JWTPrivateKey == "" {
			return Config{}, fmt.Errorf("JWT_PRIVATE_KEY is required in production")
		}
		if cfg.S3Endpoint == "" || cfg.S3PublicEndpoint == "" || cfg.S3Bucket == "" || cfg.S3AccessKey == "" || cfg.S3SecretKey == "" {
			return Config{}, fmt.Errorf("S3_ENDPOINT, S3_PUBLIC_ENDPOINT, S3_BUCKET, S3_ACCESS_KEY, and S3_SECRET_KEY are required in production")
		}
	}
	if cfg.OperatorAPIToken != "" && len(cfg.OperatorAPIToken) < 32 {
		return Config{}, fmt.Errorf("OPERATOR_API_TOKEN must contain at least 32 characters when configured")
	}
	return cfg, nil
}
