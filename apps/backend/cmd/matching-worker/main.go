package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/adapters/outbound/postgres"
	matchpostgres "github.com/Hell077/HireRadar/apps/backend/internal/matching/adapters/postgres"
	matchapp "github.com/Hell077/HireRadar/apps/backend/internal/matching/application"
	user "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("matching worker stopped", "error", err)
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
	service := matchapp.NewService(matchpostgres.NewStore(database.Pool()), time.Now)
	worker := matchpostgres.NewOutboxWorker(database.Pool(), func(ctx context.Context, id user.UserID) error {
		_, err := service.Refresh(ctx, id)
		return err
	})
	slog.Info("matching worker started")
	for ctx.Err() == nil {
		found, err := worker.ProcessNext(ctx)
		if err != nil && ctx.Err() == nil {
			slog.Error("profile rematch failed", "error", err)
		}
		if found && err == nil {
			continue
		}
		select {
		case <-ctx.Done():
		case <-time.After(2 * time.Second):
		}
	}
	slog.Info("matching worker stopped cleanly")
	return nil
}
