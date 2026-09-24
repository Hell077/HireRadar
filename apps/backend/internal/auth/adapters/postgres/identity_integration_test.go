package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/adapters/outbound/password"
	"github.com/Hell077/HireRadar/apps/backend/internal/auth/adapters/token"
	"github.com/Hell077/HireRadar/apps/backend/internal/auth/application"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestIdentityPostgresIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}

	store := NewRegistrationStore(pool)
	hasher := password.Argon2id{}
	now := time.Now().UTC()
	clock := func() time.Time { return now }
	register := application.NewRegisterService(store, hasher, clock)
	email := "integration-" + uuid.NewString() + "@example.com"
	userID, err := register.Register(ctx, email, "original password")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM outbox_events WHERE aggregate_id = $1", string(userID))
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", string(userID))
	}()
	if _, err := register.Register(ctx, email, "original password"); !errors.Is(err, application.ErrEmailTaken) {
		t.Fatalf("duplicate registration = %v", err)
	}

	verification := opaqueToken(1)
	insertToken(t, pool, "email_verification_tokens", string(userID), verification, now.Add(time.Hour), now)
	emailService := application.NewEmailService(store, hasher, clock)
	if err := emailService.Verify(ctx, verification); err != nil {
		t.Fatal(err)
	}
	if err := emailService.Verify(ctx, verification); !errors.Is(err, application.ErrInvalidVerificationToken) {
		t.Fatalf("verification replay = %v", err)
	}
	expiredVerification := opaqueToken(2)
	insertToken(t, pool, "email_verification_tokens", string(userID), expiredVerification, now.Add(-time.Hour), now.Add(-2*time.Hour))
	if err := emailService.Verify(ctx, expiredVerification); !errors.Is(err, application.ErrInvalidVerificationToken) {
		t.Fatalf("expired verification = %v", err)
	}

	signer, err := token.NewSigner(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	sessions, err := application.NewSessionService(store, hasher, signer, clock)
	if err != nil {
		t.Fatal(err)
	}
	login, err := sessions.Login(ctx, email, "original password", "integration", "")
	if err != nil {
		t.Fatal(err)
	}
	rotated, err := sessions.Refresh(ctx, login.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sessions.Refresh(ctx, login.RefreshToken); !errors.Is(err, application.ErrInvalidRefreshToken) {
		t.Fatalf("refresh replay = %v", err)
	}
	if err := sessions.Logout(ctx, rotated.RefreshToken); err != nil {
		t.Fatal(err)
	}
	if _, err := sessions.Refresh(ctx, rotated.RefreshToken); !errors.Is(err, application.ErrInvalidRefreshToken) {
		t.Fatalf("refresh after logout = %v", err)
	}
	expiredRefresh := opaqueToken(3)
	expiredHash := sha256.Sum256([]byte(expiredRefresh))
	_, err = pool.Exec(ctx, `INSERT INTO sessions (id, user_id, refresh_token_hash, expires_at, created_at) VALUES ($1, $2, $3, $4, $5)`,
		uuid.NewString(), string(userID), expiredHash[:], now.Add(-time.Hour), now.Add(-2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sessions.Refresh(ctx, expiredRefresh); !errors.Is(err, application.ErrInvalidRefreshToken) {
		t.Fatalf("expired refresh = %v", err)
	}

	active, err := sessions.Login(ctx, email, "original password", "integration", "")
	if err != nil {
		t.Fatal(err)
	}
	reset := opaqueToken(4)
	insertToken(t, pool, "password_reset_tokens", string(userID), reset, now.Add(time.Hour), now)
	if err := emailService.ResetPassword(ctx, reset, "replacement password"); err != nil {
		t.Fatal(err)
	}
	if err := emailService.ResetPassword(ctx, reset, "replacement password"); !errors.Is(err, application.ErrInvalidResetToken) {
		t.Fatalf("reset replay = %v", err)
	}
	if _, err := sessions.Refresh(ctx, active.RefreshToken); !errors.Is(err, application.ErrInvalidRefreshToken) {
		t.Fatalf("refresh after reset = %v", err)
	}
	if _, err := sessions.Login(ctx, email, "original password", "integration", ""); !errors.Is(err, application.ErrInvalidCredentials) {
		t.Fatalf("old password = %v", err)
	}
	if _, err := sessions.Login(ctx, email, "replacement password", "integration", ""); err != nil {
		t.Fatal(err)
	}
}

func opaqueToken(fill byte) string {
	secret := make([]byte, 32)
	for i := range secret {
		secret[i] = fill
	}
	return base64.RawURLEncoding.EncodeToString(secret)
}

func insertToken(t *testing.T, pool *pgxpool.Pool, table, userID, raw string, expiry, created time.Time) {
	t.Helper()
	hash := sha256.Sum256([]byte(raw))
	if table != "email_verification_tokens" && table != "password_reset_tokens" {
		t.Fatal("invalid token table")
	}
	_, err := pool.Exec(context.Background(), "INSERT INTO "+table+" (id, user_id, token_hash, expires_at, created_at) VALUES ($1, $2, $3, $4, $5)",
		uuid.NewString(), userID, hash[:], expiry, created)
	if err != nil {
		t.Fatal(err)
	}
}
