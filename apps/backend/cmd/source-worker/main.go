package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/Hell077/HireRadar/apps/backend/internal/adapters/outbound/postgres"
	ats "github.com/Hell077/HireRadar/apps/backend/internal/source/adapters/ats"
	"github.com/Hell077/HireRadar/apps/backend/internal/source/application"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("source worker stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	database, err := postgres.New(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer database.Close()
	parallelism := 4
	if value := os.Getenv("SOURCE_WORKER_PARALLELISM"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 32 {
			return errors.New("SOURCE_WORKER_PARALLELISM must be between 1 and 32")
		}
		parallelism = parsed
	}
	return application.NewWorker(database.Pool(), ats.NewRegistry(), parallelism).Run(ctx)
}
