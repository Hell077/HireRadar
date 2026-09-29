package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	jobapplicationpostgres "github.com/Hell077/HireRadar/apps/backend/internal/jobapplication/adapters/postgres"
	jobapplication "github.com/Hell077/HireRadar/apps/backend/internal/jobapplication/application"
	notificationpostgres "github.com/Hell077/HireRadar/apps/backend/internal/notification/adapters/postgres"
	telegram "github.com/Hell077/HireRadar/apps/backend/internal/notification/adapters/telegram"
	notification "github.com/Hell077/HireRadar/apps/backend/internal/notification/application"
	"github.com/Hell077/HireRadar/apps/backend/internal/notification/telegramwebhook"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := run(); err != nil {
		slog.Error("Telegram bot stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	token := strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN"))
	username := strings.TrimPrefix(strings.TrimSpace(os.Getenv("TELEGRAM_BOT_USERNAME")), "@")
	secret := strings.TrimSpace(os.Getenv("TELEGRAM_WEBHOOK_SECRET"))
	webhookURL := strings.TrimSpace(os.Getenv("TELEGRAM_WEBHOOK_URL"))
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	port := strings.TrimSpace(os.Getenv("PORT"))
	if token == "" || username == "" || secret == "" || databaseURL == "" || webhookURL == "" {
		return errors.New("TELEGRAM_BOT_TOKEN, TELEGRAM_BOT_USERNAME, TELEGRAM_WEBHOOK_SECRET, TELEGRAM_WEBHOOK_URL, and DATABASE_URL are required")
	}
	parsed, err := url.ParseRequestURI(webhookURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.Path != "/telegram/webhook" {
		return errors.New("TELEGRAM_WEBHOOK_URL must be an HTTPS URL ending in /telegram/webhook")
	}
	if port == "" {
		port = "8080"
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("configure PostgreSQL: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("connect PostgreSQL: %w", err)
	}
	client := telegram.NewClient(token)
	if err := client.SetWebhook(ctx, webhookURL, secret); err != nil {
		return fmt.Errorf("register Telegram webhook: %w", err)
	}
	notifications := notification.NewService(notificationpostgres.NewStore(pool), username, time.Now, client)
	applications := jobapplicationpostgres.NewStore(pool)
	requests := jobapplication.NewRequestService(applications, uuid.NewString, time.Now)
	notifications.SetApplicationRequester(requests)
	notifications.SetApplicationAnswerer(requests)
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           telegramwebhook.New(secret, notifications, pool),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	listenErr := make(chan error, 1)
	go func() { listenErr <- server.ListenAndServe() }()
	slog.Info("Telegram webhook service listening", "port", port, "path", "/telegram/webhook")
	select {
	case err := <-listenErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve Telegram webhook: %w", err)
		}
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			return fmt.Errorf("shutdown Telegram webhook: %w", err)
		}
		if err := <-listenErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve Telegram webhook: %w", err)
		}
	}
	return nil
}
