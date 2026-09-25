package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
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
	"github.com/Hell077/HireRadar/apps/backend/internal/auth/adapters/token"
	"github.com/Hell077/HireRadar/apps/backend/internal/auth/application"
	"github.com/Hell077/HireRadar/apps/backend/internal/config"
	profilepostgres "github.com/Hell077/HireRadar/apps/backend/internal/profile/adapters/postgres"
	profileapp "github.com/Hell077/HireRadar/apps/backend/internal/profile/application"
	resumepostgres "github.com/Hell077/HireRadar/apps/backend/internal/resume/adapters/postgres"
	resumeapp "github.com/Hell077/HireRadar/apps/backend/internal/resume/application"
	sourcepostgres "github.com/Hell077/HireRadar/apps/backend/internal/source/adapters/postgres"
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
		services.SourceCatalog = sourcepostgres.NewCatalog(client.Pool())
		if cfg.S3Endpoint != "" && cfg.S3PublicEndpoint != "" && cfg.S3Bucket != "" && cfg.S3AccessKey != "" && cfg.S3SecretKey != "" {
			storage, err := s3storage.New(ctx, cfg)
			if err != nil {
				return fmt.Errorf("configure resume storage: %w", err)
			}
			services.Resumes = resumeapp.NewService(resumepostgres.NewStore(client.Pool()), storage, time.Now)
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
	}
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
	listenErr := make(chan error, 1)
	go func() { listenErr <- app.Listen(":" + cfg.Port) }()
	slog.Info("backend listening", "port", cfg.Port, "environment", cfg.Environment)

	select {
	case err := <-listenErr:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
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

func migrate(ctx context.Context, databaseURL string) error {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	return migrations.Up(ctx, db)
}
