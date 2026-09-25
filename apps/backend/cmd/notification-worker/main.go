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
	notificationpostgres "github.com/Hell077/HireRadar/apps/backend/internal/notification/adapters/postgres"
	notificationtelegram "github.com/Hell077/HireRadar/apps/backend/internal/notification/adapters/telegram"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("notification worker stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	databaseURL, botToken := os.Getenv("DATABASE_URL"), os.Getenv("TELEGRAM_BOT_TOKEN")
	if databaseURL == "" || botToken == "" {
		return errors.New("DATABASE_URL and TELEGRAM_BOT_TOKEN are required")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	database, err := postgres.New(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer database.Close()
	worker := notificationpostgres.NewWorker(database.Pool(), notificationtelegram.NewClient(botToken), time.Now)
	slog.Info("notification worker started")
	for ctx.Err() == nil {
		found, err := worker.ProcessNext(ctx)
		if err != nil && ctx.Err() == nil {
			slog.Error("notification processing failed", "error", err)
		}
		if found && err == nil {
			continue
		}
		select {
		case <-ctx.Done():
		case <-time.After(2 * time.Second):
		}
	}
	slog.Info("notification worker stopped cleanly")
	return nil
}
