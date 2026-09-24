package application

import (
	"context"
	"crypto/rand"
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
	ErrInvalidPassword = errors.New("password must be 12 to 256 bytes")
	ErrEmailTaken      = errors.New("email already registered")
)

type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash string, password string) error
}

type Registration struct {
	User             *domain.User
	TokenID          string
	TokenHash        [sha256.Size]byte
	TokenExpiresAt   time.Time
	OutboxEventID    string
	OutboxEventType  string
	OutboxEventBytes []byte
}

type RegistrationStore interface {
	ExistsByEmail(context.Context, domain.Email) (bool, error)
	CreateRegistration(context.Context, Registration) error
}

type RegisterService struct {
	store  RegistrationStore
	hasher PasswordHasher
	now    func() time.Time
}

func NewRegisterService(store RegistrationStore, hasher PasswordHasher, now func() time.Time) *RegisterService {
	return &RegisterService{store: store, hasher: hasher, now: now}
}

func (s *RegisterService) Register(ctx context.Context, rawEmail, password string) (domain.UserID, error) {
	email, err := domain.NewEmail(rawEmail)
	if err != nil {
		return "", err
	}
	if len(password) < 12 || len(password) > 256 {
		return "", ErrInvalidPassword
	}
	exists, err := s.store.ExistsByEmail(ctx, email)
	if err != nil {
		return "", fmt.Errorf("check existing email: %w", err)
	}
	if exists {
		return "", ErrEmailTaken
	}
	passwordHash, err := s.hasher.Hash(password)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	now := s.now().UTC()
	user, err := domain.NewUser(domain.UserID(uuid.NewString()), email, passwordHash, now)
	if err != nil {
		return "", err
	}
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return "", fmt.Errorf("generate verification token: %w", err)
	}
	rawToken := base64.RawURLEncoding.EncodeToString(token)
	payload, err := json.Marshal(struct {
		UserID            domain.UserID `json:"user_id"`
		Email             domain.Email  `json:"email"`
		VerificationToken string        `json:"verification_token"`
	}{UserID: user.ID, Email: user.Email, VerificationToken: rawToken})
	if err != nil {
		return "", fmt.Errorf("encode verification event: %w", err)
	}
	registration := Registration{
		User:             user,
		TokenID:          uuid.NewString(),
		TokenHash:        sha256.Sum256([]byte(rawToken)),
		TokenExpiresAt:   now.Add(24 * time.Hour),
		OutboxEventID:    uuid.NewString(),
		OutboxEventType:  "auth.verification_requested",
		OutboxEventBytes: payload,
	}
	if err := s.store.CreateRegistration(ctx, registration); err != nil {
		return "", err
	}
	return user.ID, nil
}
