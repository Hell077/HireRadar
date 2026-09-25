package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/notification/domain"
	user "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/google/uuid"
)

var (
	ErrLinkExpired      = errors.New("telegram link token is invalid or expired")
	ErrTelegramInUse    = errors.New("telegram account is already linked")
	ErrFeedbackNotOwned = errors.New("Telegram user does not own this match")
	ErrInvalidCallback  = errors.New("invalid Telegram callback")
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
	ApplyAction(context.Context, int64, string, string) error
}

type Bot interface {
	SendMessage(context.Context, int64, string, [][]Button) error
	AnswerCallback(context.Context, string, string) error
}

type Button struct {
	Text         string `json:"text"`
	URL          string `json:"url,omitempty"`
	CallbackData string `json:"callback_data,omitempty"`
}

type Service struct {
	store       Store
	botUsername string
	now         func() time.Time
	bot         Bot
}

func NewService(store Store, botUsername string, now func() time.Time, bot ...Bot) *Service {
	service := &Service{store: store, botUsername: botUsername, now: now}
	if len(bot) > 0 {
		service.bot = bot[0]
	}
	return service
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
	if s.bot != nil {
		if err := s.bot.SendMessage(ctx, chatID, "Telegram successfully connected to HireRadar. Suitable jobs will arrive here.", nil); err != nil {
			slog.Warn("Telegram linked but confirmation message failed", "error", err)
		}
	}
	return nil
}

func (s *Service) HandleCallback(ctx context.Context, telegramUserID int64, callbackID, data string) error {
	parts := strings.Split(data, ":")
	if len(parts) != 3 || parts[0] != "job" || uuid.Validate(parts[2]) != nil {
		return ErrInvalidCallback
	}
	action := parts[1]
	if action != "save" && action != "hide" && action != "applied" {
		return ErrInvalidCallback
	}
	if err := s.store.ApplyAction(ctx, telegramUserID, parts[2], action); err != nil {
		if errors.Is(err, ErrFeedbackNotOwned) && s.bot != nil && callbackID != "" {
			_ = s.bot.AnswerCallback(ctx, callbackID, "This match is no longer available")
		}
		return err
	}
	if s.bot != nil && callbackID != "" {
		response := "Saved"
		if action == "hide" {
			response = "Hidden from your feed"
		} else if action == "applied" {
			response = "Marked as applied"
		}
		return s.bot.AnswerCallback(ctx, callbackID, response)
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
