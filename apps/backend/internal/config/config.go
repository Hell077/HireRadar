package config

import (
	"fmt"
	"os"
	"strconv"
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
	}
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
	return cfg, nil
}
