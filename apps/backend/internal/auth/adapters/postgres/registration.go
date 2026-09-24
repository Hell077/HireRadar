package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/Hell077/HireRadar/apps/backend/internal/auth/application"
	"github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RegistrationStore struct {
	pool *pgxpool.Pool
}

func NewRegistrationStore(pool *pgxpool.Pool) *RegistrationStore {
	return &RegistrationStore{pool: pool}
}

func (s *RegistrationStore) ExistsByEmail(ctx context.Context, email domain.Email) (bool, error) {
	var exists bool
	if err := s.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE lower(email) = $1)", string(email)).Scan(&exists); err != nil {
		return false, fmt.Errorf("query email: %w", err)
	}
	return exists, nil
}

func (s *RegistrationStore) CreateRegistration(ctx context.Context, registration application.Registration) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin registration transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	user := registration.User
	_, err = tx.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, status, email_verified, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		string(user.ID), string(user.Email), user.PasswordHash, string(user.Status),
		user.EmailVerified, user.CreatedAt, user.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.ConstraintName == "users_email_normalized_idx" {
			return application.ErrEmailTaken
		}
		return fmt.Errorf("insert user: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO email_verification_tokens (id, user_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)`,
		registration.TokenID, string(user.ID), registration.TokenHash[:], registration.TokenExpiresAt)
	if err != nil {
		return fmt.Errorf("insert verification token: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO outbox_events (id, event_type, aggregate_type, aggregate_id, payload)
		VALUES ($1, $2, 'user', $3, $4::jsonb)`,
		registration.OutboxEventID, registration.OutboxEventType, string(user.ID), string(registration.OutboxEventBytes))
	if err != nil {
		return fmt.Errorf("insert registration event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit registration: %w", err)
	}
	return nil
}
