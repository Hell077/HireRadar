package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/adapters/in/httpapi"
	"github.com/Hell077/HireRadar/apps/backend/internal/adapters/outbound/password"
	"github.com/Hell077/HireRadar/apps/backend/internal/adapters/outbound/postgres"
	rediscache "github.com/Hell077/HireRadar/apps/backend/internal/adapters/outbound/redis"
	s3storage "github.com/Hell077/HireRadar/apps/backend/internal/adapters/outbound/s3"
	"github.com/Hell077/HireRadar/apps/backend/internal/application/health"
	authpostgres "github.com/Hell077/HireRadar/apps/backend/internal/auth/adapters/postgres"
	authredis "github.com/Hell077/HireRadar/apps/backend/internal/auth/adapters/redis"
	smtpadapter "github.com/Hell077/HireRadar/apps/backend/internal/auth/adapters/smtp"
	"github.com/Hell077/HireRadar/apps/backend/internal/auth/adapters/token"
	"github.com/Hell077/HireRadar/apps/backend/internal/auth/application"
	"github.com/Hell077/HireRadar/apps/backend/internal/config"
	jobpostgres "github.com/Hell077/HireRadar/apps/backend/internal/job/adapters/postgres"
	"github.com/Hell077/HireRadar/apps/backend/internal/lifecycle"
	matchpostgres "github.com/Hell077/HireRadar/apps/backend/internal/matching/adapters/postgres"
	matchapp "github.com/Hell077/HireRadar/apps/backend/internal/matching/application"
	notificationpostgres "github.com/Hell077/HireRadar/apps/backend/internal/notification/adapters/postgres"
	notificationtelegram "github.com/Hell077/HireRadar/apps/backend/internal/notification/adapters/telegram"
	notificationapp "github.com/Hell077/HireRadar/apps/backend/internal/notification/application"
	"github.com/Hell077/HireRadar/apps/backend/internal/observability"
	profilepostgres "github.com/Hell077/HireRadar/apps/backend/internal/profile/adapters/postgres"
	profileapp "github.com/Hell077/HireRadar/apps/backend/internal/profile/application"
	resumepostgres "github.com/Hell077/HireRadar/apps/backend/internal/resume/adapters/postgres"
	resumeapp "github.com/Hell077/HireRadar/apps/backend/internal/resume/application"
	ats "github.com/Hell077/HireRadar/apps/backend/internal/source/adapters/ats"
	sourcepostgres "github.com/Hell077/HireRadar/apps/backend/internal/source/adapters/postgres"
	sourceapp "github.com/Hell077/HireRadar/apps/backend/internal/source/application"
	userdomain "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/Hell077/HireRadar/apps/backend/migrations"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type unavailable struct{}

func (unavailable) Ping(context.Context) error { return errors.New("not configured") }

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("backend stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	manager := lifecycle.New()
	runners := map[string]lifecycle.Runner{}
	disabledReasons := map[string]string{}
	shutdownTracing, err := observability.InitTracing(ctx, "hireradar-api")
	if err != nil {
		return fmt.Errorf("configure distributed tracing: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTracing(shutdownCtx); err != nil {
			slog.Warn("flush distributed traces", "error", err)
		}
	}()

	database := health.Pinger(unavailable{})
	services := httpapi.AuthServices{}
	if cfg.DatabaseURL != "" {
		client, err := postgres.New(ctx, cfg.DatabaseURL)
		if err != nil {
			return fmt.Errorf("configure PostgreSQL: %w", err)
		}
		defer client.Close()
		if err := migrate(ctx, cfg.DatabaseURL); err != nil {
			return fmt.Errorf("migrate PostgreSQL: %w", err)
		}
		database = client
		store := authpostgres.NewRegistrationStore(client.Pool())
		services.Registrar = application.NewRegisterService(store, password.Argon2id{}, time.Now)
		services.Email = application.NewEmailService(store, password.Argon2id{}, time.Now)
		profileService := profileapp.NewService(profilepostgres.NewStore(client.Pool()))
		services.Profile = profileService
		services.Skills = profileService
		services.Positions = profileService
		services.Preferences = profileService
		services.Sources = profileService
		sourceCatalog := sourcepostgres.NewCatalog(client.Pool())
		services.SourceCatalog = sourceCatalog
		services.SourceOperations = sourceCatalog
		services.OperatorAPIToken = cfg.OperatorAPIToken
		services.Jobs = jobpostgres.NewCatalog(client.Pool())
		services.Matches = matchapp.NewService(matchpostgres.NewStore(client.Pool()), time.Now)
		if value := os.Getenv("SOURCE_WORKER_PARALLELISM"); value != "" {
			parallelism, err := strconv.Atoi(value)
			if err != nil || parallelism < 1 || parallelism > 32 {
				return errors.New("SOURCE_WORKER_PARALLELISM must be between 1 and 32")
			}
			sourceWorker := sourceapp.NewWorker(client.Pool(), ats.NewRegistry(), parallelism)
			sourceWorker.SetErrorReporter(serviceReporter(manager, "source"))
			runners["source"] = sourceWorker.Run
		} else {
			sourceWorker := sourceapp.NewWorker(client.Pool(), ats.NewRegistry(), 4)
			sourceWorker.SetErrorReporter(serviceReporter(manager, "source"))
			runners["source"] = sourceWorker.Run
		}
		matcher := matchapp.NewService(matchpostgres.NewStore(client.Pool()), time.Now)
		matchWorker := matchpostgres.NewOutboxWorker(client.Pool(), func(ctx context.Context, id userdomain.UserID) error {
			_, err := matcher.Refresh(ctx, id)
			return err
		}, func(ctx context.Context, jobID string) error { return matcher.RefreshJob(ctx, jobID) })
		runners["matching"] = func(ctx context.Context) error {
			return runProcessor(ctx, "matching", manager, matchWorker.ProcessNext)
		}
		if address, from := os.Getenv("SMTP_ADDRESS"), os.Getenv("SMTP_FROM"); address != "" && from != "" {
			publicURL := os.Getenv("APP_PUBLIC_URL")
			if publicURL == "" {
				publicURL = "http://localhost:3000"
			}
			sender := smtpadapter.Sender{Address: address, From: from, Username: os.Getenv("SMTP_USERNAME"), Password: os.Getenv("SMTP_PASSWORD")}
			store := authpostgres.NewDeliveryStore(client.Pool(), sender, publicURL)
			runners["email"] = func(ctx context.Context) error { return runProcessor(ctx, "email", manager, store.DeliverNext) }
		} else {
			disabledReasons["email"] = "SMTP_ADDRESS and SMTP_FROM are not configured"
		}
		var telegramBot notificationapp.Bot
		if cfg.TelegramBotToken != "" {
			client := notificationtelegram.NewClient(cfg.TelegramBotToken)
			telegramBot = client
			if cfg.TelegramWebhookURL != "" {
				if err := client.SetWebhook(ctx, cfg.TelegramWebhookURL, cfg.TelegramWebhookSecret); err != nil {
					slog.Warn("configure Telegram webhook", "error", err)
				}
			}
		}
		services.Telegram = notificationapp.NewService(notificationpostgres.NewStore(client.Pool()), cfg.TelegramBotUsername, time.Now, telegramBot)
		services.TelegramWebhookSecret = cfg.TelegramWebhookSecret
		if cfg.S3Endpoint != "" && cfg.S3PublicEndpoint != "" && cfg.S3Bucket != "" && cfg.S3AccessKey != "" && cfg.S3SecretKey != "" {
			storage, err := s3storage.New(ctx, cfg)
			if err != nil {
				return fmt.Errorf("configure resume storage: %w", err)
			}
			services.Resumes = resumeapp.NewService(resumepostgres.NewStore(client.Pool()), storage, time.Now)
			resumeWorker := resumeapp.NewWorker(resumepostgres.NewStore(client.Pool()), storage)
			resumeWorker.SetErrorReporter(serviceReporter(manager, "resume"))
			runners["resume"] = resumeWorker.Run
		} else {
			disabledReasons["resume"] = "resume storage is not configured"
		}
		runners["notification"] = func(ctx context.Context) error {
			if cfg.TelegramBotToken == "" {
				return errors.New("TELEGRAM_BOT_TOKEN is not configured")
			}
			worker := notificationpostgres.NewWorker(client.Pool(), notificationtelegram.NewClient(cfg.TelegramBotToken), time.Now)
			return runProcessor(ctx, "notification", manager, worker.ProcessNext)
		}
		if cfg.JWTPrivateKey != "" {
			signer, err := token.NewSigner(cfg.JWTPrivateKey)
			if err != nil {
				return fmt.Errorf("configure access tokens: %w", err)
			}
			sessions, err := application.NewSessionService(store, password.Argon2id{}, signer, time.Now)
			if err != nil {
				return fmt.Errorf("configure sessions: %w", err)
			}
			services.Sessions = sessions
			services.Verifier = signer
		}
	} else {
		for _, name := range []string{"source", "matching", "notification", "email", "resume"} {
			disabledReasons[name] = "DATABASE_URL is not configured"
		}
	}
	for _, name := range []string{"source", "matching", "notification", "email", "resume"} {
		runner, enabled := runners[name]
		reason := disabledReasons[name]
		if name == "notification" && cfg.TelegramBotToken == "" {
			enabled = false
			reason = "TELEGRAM_BOT_TOKEN is not configured"
		}
		if err := manager.Register(name, enabled, reason, runner); err != nil {
			return err
		}
	}
	services.Services = manager
	cache := health.Pinger(unavailable{})
	if cfg.RedisURL != "" {
		client, err := rediscache.New(cfg.RedisURL)
		if err != nil {
			return fmt.Errorf("configure Redis: %w", err)
		}
		defer client.Close()
		cache = client
		services.Limiter = authredis.NewLimiter(client.Redis())
	}

	checker := health.NewService(
		health.Dependency{Name: "postgres", Pinger: database},
		health.Dependency{Name: "redis", Pinger: cache},
	)
	app := httpapi.New(checker, services)
	manager.StartAll(ctx)
	listenErr := make(chan error, 1)
	go func() { listenErr <- app.Listen(":" + cfg.Port) }()
	slog.Info("backend listening", "port", cfg.Port, "environment", cfg.Environment)

	select {
	case err := <-listenErr:
		stop()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = manager.StopAll(shutdownCtx)
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := manager.StopAll(shutdownCtx); err != nil {
			slog.Error("background service shutdown incomplete", "error", err)
		}
		if err := app.ShutdownWithContext(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown HTTP server: %w", err)
		}
		if err := <-listenErr; err != nil {
			return err
		}
		slog.Info("backend stopped cleanly")
		return nil
	}
}

func runProcessor(ctx context.Context, name string, manager *lifecycle.Manager, process func(context.Context) (bool, error)) error {
	slog.Info("worker loop started", "service", name)
	for ctx.Err() == nil {
		found, err := process(ctx)
		if err != nil && ctx.Err() == nil {
			manager.ReportError(name, err)
			slog.Error("worker iteration failed", "service", name, "error", err)
		} else if err == nil {
			manager.ReportSuccess(name)
		}
		if found && err == nil {
			continue
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(2 * time.Second):
		}
	}
	return nil
}

func serviceReporter(manager *lifecycle.Manager, name string) func(error) {
	return func(err error) {
		if err != nil {
			manager.ReportError(name, err)
		} else {
			manager.ReportSuccess(name)
		}
	}
}

func migrate(ctx context.Context, databaseURL string) error {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	return migrations.Up(ctx, db)
}
