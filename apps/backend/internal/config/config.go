package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Environment string
	Port        string
	DatabaseURL string
	RedisURL    string
}

// Load validates process configuration before opening external connections.
// Development can run without infrastructure so OpenAPI types can be generated;
// readiness remains unavailable until both dependencies are configured.
func Load() (Config, error) {
	cfg := Config{
		Environment: os.Getenv("APP_ENV"),
		Port:        os.Getenv("PORT"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:    os.Getenv("REDIS_URL"),
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
	}
	return cfg, nil
}
