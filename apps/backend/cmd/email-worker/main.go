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
	authpostgres "github.com/Hell077/HireRadar/apps/backend/internal/auth/adapters/postgres"
	"github.com/Hell077/HireRadar/apps/backend/internal/auth/adapters/smtp"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("email worker stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	for _, name := range []string{"DATABASE_URL", "SMTP_ADDRESS", "SMTP_FROM", "APP_PUBLIC_URL"} {
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
	sender := smtp.Sender{Address: os.Getenv("SMTP_ADDRESS"), From: os.Getenv("SMTP_FROM"), Username: os.Getenv("SMTP_USERNAME"), Password: os.Getenv("SMTP_PASSWORD")}
	store := authpostgres.NewDeliveryStore(database.Pool(), sender, os.Getenv("APP_PUBLIC_URL"))
	slog.Info("email worker started")
	for ctx.Err() == nil {
		found, err := store.DeliverNext(ctx)
		if err != nil && ctx.Err() == nil {
			slog.Error("email delivery failed", "error", err)
		}
		if found && err == nil {
			continue
		}
		select {
		case <-ctx.Done():
		case <-time.After(2 * time.Second):
		}
	}
	slog.Info("email worker stopped cleanly")
	return nil
}
