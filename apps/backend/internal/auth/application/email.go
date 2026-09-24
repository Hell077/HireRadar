package application

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/google/uuid"
)

var (
	ErrInvalidVerificationToken = errors.New("invalid verification token")
	ErrInvalidResetToken        = errors.New("invalid password reset token")
)

type PasswordResetRequest struct {
	UserID           domain.UserID
	TokenID          string
	TokenHash        [sha256.Size]byte
	ExpiresAt        time.Time
	OutboxEventID    string
	OutboxEventBytes []byte
}

type EmailStore interface {
	ByEmail(context.Context, domain.Email) (*domain.User, error)
	VerifyEmail(context.Context, [sha256.Size]byte, time.Time) error
	CreatePasswordReset(context.Context, PasswordResetRequest) error
	ConsumePasswordReset(context.Context, [sha256.Size]byte, string, time.Time) error
}

type EmailService struct {
	store  EmailStore
	hasher PasswordHasher
	now    func() time.Time
}

func NewEmailService(store EmailStore, hasher PasswordHasher, now func() time.Time) *EmailService {
	return &EmailService{store: store, hasher: hasher, now: now}
}

func (s *EmailService) Verify(ctx context.Context, rawToken string) error {
	if !validOpaqueToken(rawToken) {
		return ErrInvalidVerificationToken
	}
	return s.store.VerifyEmail(ctx, sha256.Sum256([]byte(rawToken)), s.now().UTC())
}

func (s *EmailService) RequestReset(ctx context.Context, rawEmail string) error {
	email, err := domain.NewEmail(rawEmail)
	if err != nil {
		return nil
	}
	user, err := s.store.ByEmail(ctx, email)
	if err != nil {
		return fmt.Errorf("lookup password reset user: %w", err)
	}
	if user == nil || user.CanLogin() != nil {
		return nil
	}
	rawToken, hash, err := newRefreshToken()
	if err != nil {
		return err
	}
	payload, err := json.Marshal(struct {
		UserID     domain.UserID `json:"user_id"`
		Email      domain.Email  `json:"email"`
		ResetToken string        `json:"reset_token"`
	}{user.ID, user.Email, rawToken})
	if err != nil {
		return fmt.Errorf("encode password reset event: %w", err)
	}
	return s.store.CreatePasswordReset(ctx, PasswordResetRequest{
		UserID: user.ID, TokenID: uuid.NewString(), TokenHash: hash,
		ExpiresAt: s.now().UTC().Add(time.Hour), OutboxEventID: uuid.NewString(), OutboxEventBytes: payload,
	})
}

func (s *EmailService) ResetPassword(ctx context.Context, rawToken, newPassword string) error {
	if !validOpaqueToken(rawToken) {
		return ErrInvalidResetToken
	}
	if len(newPassword) < 12 || len(newPassword) > 256 {
		return ErrInvalidPassword
	}
	passwordHash, err := s.hasher.Hash(newPassword)
	if err != nil {
		return fmt.Errorf("hash replacement password: %w", err)
	}
	return s.store.ConsumePasswordReset(ctx, sha256.Sum256([]byte(rawToken)), passwordHash, s.now().UTC())
}

func validOpaqueToken(raw string) bool {
	if len(raw) != 43 {
		return false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	return err == nil && len(decoded) == 32
}
