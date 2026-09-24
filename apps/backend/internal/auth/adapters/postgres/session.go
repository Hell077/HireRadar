package postgres

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/auth/application"
	"github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/jackc/pgx/v5"
)

func (s *RegistrationStore) ByEmail(ctx context.Context, email domain.Email) (*domain.User, error) {
	var user domain.User
	var id, address, status string
	err := s.pool.QueryRow(ctx, `
		SELECT id, email, password_hash, status, email_verified, created_at, updated_at
		FROM users WHERE lower(email) = $1`, string(email)).Scan(
		&id, &address, &user.PasswordHash, &status, &user.EmailVerified, &user.CreatedAt, &user.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query user by email: %w", err)
	}
	user.ID = domain.UserID(id)
	user.Email = domain.Email(address)
	user.Status = domain.Status(status)
	return &user, nil
}

func (s *RegistrationStore) CreateSession(ctx context.Context, session application.Session) error {
	var ipAddress *string
	if session.IPAddress != "" {
		ipAddress = &session.IPAddress
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO sessions (id, user_id, refresh_token_hash, user_agent, ip_address, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		session.ID, string(session.UserID), session.RefreshTokenHash[:], session.UserAgent,
		ipAddress, session.ExpiresAt, session.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	return nil
}

func (s *RegistrationStore) RotateSession(ctx context.Context, oldHash, newHash [sha256.Size]byte, now, expiresAt time.Time) (domain.UserID, error) {
	var userID string
	err := s.pool.QueryRow(ctx, `
		UPDATE sessions AS s
		SET refresh_token_hash = $2, expires_at = $4
		FROM users AS u
		WHERE s.refresh_token_hash = $1 AND s.user_id = u.id AND u.status = 'active'
		  AND s.revoked_at IS NULL AND s.expires_at > $3
		RETURNING s.user_id`, oldHash[:], newHash[:], now, expiresAt).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", application.ErrInvalidRefreshToken
	}
	if err != nil {
		return "", fmt.Errorf("rotate session: %w", err)
	}
	return domain.UserID(userID), nil
}

func (s *RegistrationStore) RevokeSession(ctx context.Context, tokenHash [sha256.Size]byte, now time.Time) error {
	result, err := s.pool.Exec(ctx, `
		UPDATE sessions SET revoked_at = $2
		WHERE refresh_token_hash = $1 AND revoked_at IS NULL`, tokenHash[:], now)
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	if result.RowsAffected() == 0 {
		return application.ErrInvalidRefreshToken
	}
	return nil
}
