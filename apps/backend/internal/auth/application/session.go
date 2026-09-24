package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/google/uuid"
)

var (
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
)

type Session struct {
	ID               string
	UserID           domain.UserID
	RefreshTokenHash [sha256.Size]byte
	UserAgent        string
	IPAddress        string
	ExpiresAt        time.Time
	CreatedAt        time.Time
}

type SessionStore interface {
	ByEmail(context.Context, domain.Email) (*domain.User, error)
	CreateSession(context.Context, Session) error
	RotateSession(context.Context, [sha256.Size]byte, [sha256.Size]byte, time.Time, time.Time) (domain.UserID, error)
	RevokeSession(context.Context, [sha256.Size]byte, time.Time) error
}

type AccessSigner interface {
	Issue(domain.UserID, time.Time) (string, time.Time, error)
}

type Tokens struct {
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
}

type SessionService struct {
	store     SessionStore
	hasher    PasswordHasher
	signer    AccessSigner
	dummyHash string
	now       func() time.Time
}

func NewSessionService(store SessionStore, hasher PasswordHasher, signer AccessSigner, now func() time.Time) (*SessionService, error) {
	dummyHash, err := hasher.Hash("dummy password for timing consistency")
	if err != nil {
		return nil, fmt.Errorf("prepare login timing hash: %w", err)
	}
	return &SessionService{store: store, hasher: hasher, signer: signer, dummyHash: dummyHash, now: now}, nil
}

func (s *SessionService) Login(ctx context.Context, rawEmail, password, userAgent, ipAddress string) (Tokens, error) {
	email, err := domain.NewEmail(rawEmail)
	if err != nil {
		return Tokens{}, ErrInvalidCredentials
	}
	user, err := s.store.ByEmail(ctx, email)
	if err != nil {
		return Tokens{}, fmt.Errorf("load user: %w", err)
	}
	if user == nil {
		_ = s.hasher.Compare(s.dummyHash, password)
		return Tokens{}, ErrInvalidCredentials
	}
	if s.hasher.Compare(user.PasswordHash, password) != nil || user.CanLogin() != nil {
		return Tokens{}, ErrInvalidCredentials
	}
	now := s.now().UTC()
	tokens, hash, err := s.newTokens(user.ID, now)
	if err != nil {
		return Tokens{}, err
	}
	if len(userAgent) > 512 {
		userAgent = userAgent[:512]
	}
	if err := s.store.CreateSession(ctx, Session{
		ID: uuid.NewString(), UserID: user.ID, RefreshTokenHash: hash,
		UserAgent: userAgent, IPAddress: ipAddress,
		ExpiresAt: tokens.RefreshExpiresAt, CreatedAt: now,
	}); err != nil {
		return Tokens{}, fmt.Errorf("create session: %w", err)
	}
	return tokens, nil
}

func (s *SessionService) Refresh(ctx context.Context, oldRefreshToken string) (Tokens, error) {
	if len(oldRefreshToken) != 43 {
		return Tokens{}, ErrInvalidRefreshToken
	}
	oldHash := sha256.Sum256([]byte(oldRefreshToken))
	now := s.now().UTC()
	raw, newHash, err := newRefreshToken()
	if err != nil {
		return Tokens{}, err
	}
	expiresAt := now.Add(30 * 24 * time.Hour)
	userID, err := s.store.RotateSession(ctx, oldHash, newHash, now, expiresAt)
	if err != nil {
		return Tokens{}, err
	}
	access, accessExpiry, err := s.signer.Issue(userID, now)
	if err != nil {
		return Tokens{}, err
	}
	return Tokens{AccessToken: access, AccessExpiresAt: accessExpiry, RefreshToken: raw, RefreshExpiresAt: expiresAt}, nil
}

func (s *SessionService) Logout(ctx context.Context, refreshToken string) error {
	if len(refreshToken) != 43 {
		return ErrInvalidRefreshToken
	}
	return s.store.RevokeSession(ctx, sha256.Sum256([]byte(refreshToken)), s.now().UTC())
}

func (s *SessionService) newTokens(userID domain.UserID, now time.Time) (Tokens, [sha256.Size]byte, error) {
	access, accessExpiry, err := s.signer.Issue(userID, now)
	if err != nil {
		return Tokens{}, [sha256.Size]byte{}, err
	}
	raw, hash, err := newRefreshToken()
	if err != nil {
		return Tokens{}, [sha256.Size]byte{}, err
	}
	return Tokens{
		AccessToken: access, AccessExpiresAt: accessExpiry,
		RefreshToken: raw, RefreshExpiresAt: now.Add(30 * 24 * time.Hour),
	}, hash, nil
}

func newRefreshToken() (string, [sha256.Size]byte, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", [sha256.Size]byte{}, fmt.Errorf("generate refresh token: %w", err)
	}
	raw := base64.RawURLEncoding.EncodeToString(secret)
	return raw, sha256.Sum256([]byte(raw)), nil
}
