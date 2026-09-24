package postgres

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/auth/application"
	"github.com/jackc/pgx/v5"
)

func (s *RegistrationStore) VerifyEmail(ctx context.Context, tokenHash [sha256.Size]byte, now time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin email verification: %w", err)
	}
	defer tx.Rollback(ctx)
	var userID string
	err = tx.QueryRow(ctx, `
		UPDATE email_verification_tokens SET used_at = $2
		WHERE token_hash = $1 AND used_at IS NULL AND expires_at > $2
		RETURNING user_id`, tokenHash[:], now).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return application.ErrInvalidVerificationToken
	}
	if err != nil {
		return fmt.Errorf("consume verification token: %w", err)
	}
	result, err := tx.Exec(ctx, `
		UPDATE users SET email_verified = true, updated_at = $2
		WHERE id = $1 AND status = 'active' AND email_verified = false`, userID, now)
	if err != nil {
		return fmt.Errorf("verify user email: %w", err)
	}
	if result.RowsAffected() == 0 {
		return application.ErrInvalidVerificationToken
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit email verification: %w", err)
	}
	return nil
}

func (s *RegistrationStore) CreatePasswordReset(ctx context.Context, request application.PasswordResetRequest) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin password reset request: %w", err)
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `
		INSERT INTO password_reset_tokens (id, user_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)`, request.TokenID, string(request.UserID), request.TokenHash[:], request.ExpiresAt)
	if err != nil {
		return fmt.Errorf("insert password reset token: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO outbox_events (id, event_type, aggregate_type, aggregate_id, payload)
		VALUES ($1, 'auth.password_reset_requested', 'user', $2, $3::jsonb)`,
		request.OutboxEventID, string(request.UserID), string(request.OutboxEventBytes))
	if err != nil {
		return fmt.Errorf("insert password reset event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit password reset request: %w", err)
	}
	return nil
}

func (s *RegistrationStore) ConsumePasswordReset(ctx context.Context, tokenHash [sha256.Size]byte, passwordHash string, now time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin password reset: %w", err)
	}
	defer tx.Rollback(ctx)
	var userID string
	err = tx.QueryRow(ctx, `
		UPDATE password_reset_tokens SET used_at = $2
		WHERE token_hash = $1 AND used_at IS NULL AND expires_at > $2
		RETURNING user_id`, tokenHash[:], now).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return application.ErrInvalidResetToken
	}
	if err != nil {
		return fmt.Errorf("consume password reset token: %w", err)
	}
	result, err := tx.Exec(ctx, `
		UPDATE users SET password_hash = $2, updated_at = $3
		WHERE id = $1 AND status = 'active'`, userID, passwordHash, now)
	if err != nil {
		return fmt.Errorf("replace user password: %w", err)
	}
	if result.RowsAffected() == 0 {
		return application.ErrInvalidResetToken
	}
	if _, err := tx.Exec(ctx, `UPDATE sessions SET revoked_at = $2 WHERE user_id = $1 AND revoked_at IS NULL`, userID, now); err != nil {
		return fmt.Errorf("revoke password reset sessions: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE password_reset_tokens SET used_at = $2 WHERE user_id = $1 AND used_at IS NULL`, userID, now); err != nil {
		return fmt.Errorf("invalidate password reset tokens: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit password reset: %w", err)
	}
	return nil
}
