package application

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/notification/domain"
	user "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
)

type serviceStore struct {
	linkHash []byte
	action   string
	jobID    string
	consume  int
	applyErr error
}

func (s *serviceStore) CreateLink(_ context.Context, _ user.UserID, _ string, hash []byte, _ time.Time) error {
	s.linkHash = append([]byte(nil), hash...)
	return nil
}
func (*serviceStore) Connection(context.Context, user.UserID) (Account, error) { return Account{}, nil }
func (s *serviceStore) ConsumeLink(context.Context, []byte, Account) error     { s.consume++; return nil }
func (*serviceStore) Disconnect(context.Context, user.UserID) error            { return nil }
func (*serviceStore) GetPreferences(context.Context, user.UserID) (domain.NotificationPreferences, error) {
	return domain.DefaultPreferences(), nil
}
func (*serviceStore) SavePreferences(context.Context, user.UserID, domain.NotificationPreferences) error {
	return nil
}
func (*serviceStore) ListSavedJobs(context.Context, user.UserID, int) ([]SavedJob, error) {
	return nil, nil
}
func (*serviceStore) RemoveSavedJob(context.Context, user.UserID, string) error          { return nil }
func (*serviceStore) ApplyUserAction(context.Context, user.UserID, string, string) error { return nil }
func (s *serviceStore) ApplyAction(_ context.Context, _ int64, jobID, action string) error {
	s.jobID, s.action = jobID, action
	return s.applyErr
}

type serviceBot struct {
	chatID int64
	text   string
	answer string
}

func (b *serviceBot) SendMessage(_ context.Context, chatID int64, text string, _ [][]Button) error {
	b.chatID, b.text = chatID, text
	return nil
}
func (b *serviceBot) AnswerCallback(_ context.Context, _ string, text string) error {
	b.answer = text
	return nil
}

func TestCreateLinkStoresOnlyHashAndExpiresInFifteenMinutes(t *testing.T) {
	now := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	store := &serviceStore{}
	service := NewService(store, "hire_radar_bot", func() time.Time { return now })
	link, err := service.CreateLink(context.Background(), "user-id")
	if err != nil {
		t.Fatal(err)
	}
	if len(store.linkHash) != 32 || strings.Contains(link.URL, " ") || !strings.HasPrefix(link.URL, "https://t.me/hire_radar_bot?start=") {
		t.Fatalf("unexpected link=%+v stored hash length=%d", link, len(store.linkHash))
	}
	if !link.ExpiresAt.Equal(now.Add(15 * time.Minute)) {
		t.Fatalf("expiry=%s", link.ExpiresAt)
	}
}

func TestCallbackChecksActionAndAcknowledgesOwnedFeedback(t *testing.T) {
	store, bot := &serviceStore{}, &serviceBot{}
	service := NewService(store, "", time.Now, bot)
	jobID := "b2d45992-9a64-42f3-92a8-b511562184d2"
	if err := service.HandleCallback(context.Background(), 10, "query-id", "job:save:"+jobID); err != nil {
		t.Fatal(err)
	}
	if store.jobID != jobID || store.action != "save" || bot.answer != "Saved" {
		t.Fatalf("action=%s job=%s answer=%s", store.action, store.jobID, bot.answer)
	}
	if err := service.HandleCallback(context.Background(), 10, "query-id", "job:delete:"+jobID); !errors.Is(err, ErrInvalidCallback) {
		t.Fatalf("invalid callback error=%v", err)
	}
}
