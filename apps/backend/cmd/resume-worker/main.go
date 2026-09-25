package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Hell077/HireRadar/apps/backend/internal/adapters/outbound/postgres"
	s3storage "github.com/Hell077/HireRadar/apps/backend/internal/adapters/outbound/s3"
	"github.com/Hell077/HireRadar/apps/backend/internal/config"
	resumepostgres "github.com/Hell077/HireRadar/apps/backend/internal/resume/adapters/postgres"
	resumeapp "github.com/Hell077/HireRadar/apps/backend/internal/resume/application"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("resume worker stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	for _, name := range []string{"DATABASE_URL", "S3_ENDPOINT", "S3_PUBLIC_ENDPOINT", "S3_BUCKET", "S3_ACCESS_KEY", "S3_SECRET_KEY"} {
		if os.Getenv(name) == "" {
			return errors.New(name + " is required")
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	database, err := postgres.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer database.Close()
	cfg := config.Config{S3Endpoint: os.Getenv("S3_ENDPOINT"), S3PublicEndpoint: os.Getenv("S3_PUBLIC_ENDPOINT"), S3Bucket: os.Getenv("S3_BUCKET"), S3AccessKey: os.Getenv("S3_ACCESS_KEY"), S3SecretKey: os.Getenv("S3_SECRET_KEY")}
	objects, err := s3storage.New(ctx, cfg)
	if err != nil {
		return err
	}
	worker := resumeapp.NewWorker(resumepostgres.NewStore(database.Pool()), objects)
	slog.Info("resume worker started")
	if err := worker.Run(ctx); err != nil {
		return err
	}
	slog.Info("resume worker stopped cleanly")
	return nil
}
