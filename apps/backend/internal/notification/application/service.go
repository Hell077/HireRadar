package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/notification/domain"
	user "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/google/uuid"
)

var (
	ErrLinkExpired   = errors.New("telegram link token is invalid or expired")
	ErrTelegramInUse = errors.New("telegram account is already linked")
)

type Account struct {
	ID             string
	TelegramUserID int64
	ChatID         int64
	Username       *string
	ConnectedAt    time.Time
}

type Link struct {
	Token     string
	ExpiresAt time.Time
	URL       string
}

type Store interface {
	CreateLink(context.Context, user.UserID, string, []byte, time.Time) error
	Connection(context.Context, user.UserID) (Account, error)
	ConsumeLink(context.Context, []byte, Account) error
	Disconnect(context.Context, user.UserID) error
	GetPreferences(context.Context, user.UserID) (domain.NotificationPreferences, error)
	SavePreferences(context.Context, user.UserID, domain.NotificationPreferences) error
}

type Service struct {
	store       Store
	botUsername string
	now         func() time.Time
}

func NewService(store Store, botUsername string, now func() time.Time) *Service {
	return &Service{store: store, botUsername: botUsername, now: now}
}

func (s *Service) Connection(ctx context.Context, userID user.UserID) (Account, error) {
	return s.store.Connection(ctx, userID)
}

func (s *Service) CreateLink(ctx context.Context, userID user.UserID) (Link, error) {
	if s.botUsername == "" {
		return Link{}, errors.New("telegram bot username is not configured")
	}
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return Link{}, fmt.Errorf("generate Telegram link token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw[:])
	hash := sha256.Sum256([]byte(token))
	expiresAt := s.now().UTC().Add(15 * time.Minute)
	if err := s.store.CreateLink(ctx, userID, uuid.NewString(), hash[:], expiresAt); err != nil {
		return Link{}, err
	}
	return Link{Token: token, ExpiresAt: expiresAt, URL: "https://t.me/" + s.botUsername + "?start=" + token}, nil
}

func (s *Service) HandleStart(ctx context.Context, telegramUserID, chatID int64, username, token string) error {
	if token == "" || len(token) > 128 || telegramUserID <= 0 || chatID <= 0 {
		return ErrLinkExpired
	}
	hash := sha256.Sum256([]byte(token))
	var name *string
	if username != "" {
		name = &username
	}
	err := s.store.ConsumeLink(ctx, hash[:], Account{ID: uuid.NewString(), TelegramUserID: telegramUserID, ChatID: chatID, Username: name})
	if errors.Is(err, ErrTelegramInUse) {
		return ErrTelegramInUse
	}
	if err != nil {
		return fmt.Errorf("link Telegram account: %w", err)
	}
	return nil
}

func (s *Service) Preferences(ctx context.Context, userID user.UserID) (domain.NotificationPreferences, error) {
	return s.store.GetPreferences(ctx, userID)
}

func (s *Service) SavePreferences(ctx context.Context, userID user.UserID, preferences domain.NotificationPreferences) error {
	if err := preferences.Validate(); err != nil {
		return err
	}
	return s.store.SavePreferences(ctx, userID, preferences)
}

func (s *Service) Disconnect(ctx context.Context, userID user.UserID) error {
	return s.store.Disconnect(ctx, userID)
}
