package application

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/notification/domain"
	user "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
)

type serviceStore struct {
	linkHash                  []byte
	action                    string
	jobID                     string
	reason                    string
	consume                   int
	applyErr                  error
	openURL                   string
	openScore, openConfidence int
	firstOpen                 bool
	commandLinked             bool
	commandPreferences        domain.NotificationPreferences
	commandJobs               []RecommendedJob
	notificationsEnabled      bool
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
func (*serviceStore) ApplyUserFeedback(context.Context, user.UserID, string, string, string) error {
	return nil
}
func (s *serviceStore) ApplyAction(_ context.Context, _ int64, jobID, action string) (int64, error) {
	s.jobID, s.action = jobID, action
	return 77, s.applyErr
}
func (s *serviceStore) SaveFeedbackReason(_ context.Context, _ int64, jobID, reason string) error {
	s.jobID, s.reason = jobID, reason
	return nil
}
func (s *serviceStore) OpenNotification(context.Context, string) (string, int, int, bool, error) {
	return s.openURL, s.openScore, s.openConfidence, s.firstOpen, nil
}
func (s *serviceStore) AccountByTelegramID(context.Context, int64) (user.UserID, domain.NotificationPreferences, bool, error) {
	prefs := s.commandPreferences
	if prefs.Timezone == "" {
		prefs = domain.DefaultPreferences()
	}
	return "test-user", prefs, s.commandLinked, nil
}
func (s *serviceStore) RecentMatchesForTelegram(context.Context, int64, int) ([]RecommendedJob, error) {
	return s.commandJobs, nil
}
func (s *serviceStore) SetNotificationsEnabled(_ context.Context, _ int64, enabled bool) error {
	s.notificationsEnabled = enabled
	return nil
}

type serviceBot struct {
	chatID  int64
	text    string
	answer  string
	buttons [][]Button
}

type applicationRequester struct {
	telegramID int64
	jobID      string
}

type applicationAnswerer struct {
	telegramID int64
	questionID string
	answer     string
}

func (a *applicationAnswerer) AnswerFromTelegram(_ context.Context, telegramID int64, questionID, answer string) error {
	a.telegramID, a.questionID, a.answer = telegramID, questionID, answer
	return nil
}

func (r *applicationRequester) RequestFromTelegram(_ context.Context, telegramID int64, jobID string) error {
	r.telegramID, r.jobID = telegramID, jobID
	return nil
}

func (b *serviceBot) SendMessage(_ context.Context, chatID int64, text string, buttons [][]Button) error {
	b.chatID, b.text, b.buttons = chatID, text, buttons
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
	if err := service.HandleCallback(context.Background(), 10, "query-id", "job:relevant:"+jobID); err != nil || store.action != "relevant" || !strings.Contains(bot.answer, "look for more") {
		t.Fatalf("positive feedback action=%q answer=%q err=%v", store.action, bot.answer, err)
	}
	if err := service.HandleCallback(context.Background(), 10, "query-id", "job:not_relevant:"+jobID); err != nil || store.action != "not_relevant" || !strings.Contains(bot.answer, "Hidden") {
		t.Fatalf("negative feedback action=%q answer=%q err=%v", store.action, bot.answer, err)
	}
}

func TestHideOffersOptionalReasonAndReasonCallbackPersists(t *testing.T) {
	store, bot := &serviceStore{}, &serviceBot{}
	service := NewService(store, "", time.Now, bot)
	jobID := "b2d45992-9a64-42f3-92a8-b511562184d2"
	if err := service.HandleCallback(context.Background(), 10, "hide-query", "job:hide:"+jobID); err != nil {
		t.Fatal(err)
	}
	if bot.answer != "Hidden from your feed" || bot.chatID != 77 || len(bot.buttons) != 10 || bot.buttons[0][0].CallbackData != "fr:"+jobID+":wrong_stack" {
		t.Fatalf("hide prompt answer=%q chat=%d buttons=%v", bot.answer, bot.chatID, bot.buttons)
	}
	if err := service.HandleCallback(context.Background(), 10, "reason-query", "fr:"+jobID+":wrong_stack"); err != nil {
		t.Fatal(err)
	}
	if store.jobID != jobID || store.reason != "wrong_stack" || bot.answer != "Thanks for the feedback" {
		t.Fatalf("reason was not saved: job=%s reason=%s answer=%s", store.jobID, store.reason, bot.answer)
	}
	if err := service.HandleCallback(context.Background(), 10, "bad-query", "fr:"+jobID+":made_up"); !errors.Is(err, ErrInvalidCallback) {
		t.Fatalf("invalid reason error = %v", err)
	}
}

func TestOpenNotificationValidatesTargetAndRecordsFirstOpen(t *testing.T) {
	store := &serviceStore{openURL: "https://jobs.example/apply", openScore: 87, openConfidence: 93, firstOpen: true}
	service := NewService(store, "", time.Now)
	url, err := service.OpenNotification(context.Background(), "b2d45992-9a64-42f3-92a8-b511562184d2")
	if err != nil || url != store.openURL {
		t.Fatalf("open URL=%q err=%v", url, err)
	}
	store.openURL = "javascript:alert(1)"
	if _, err := service.OpenNotification(context.Background(), "b2d45992-9a64-42f3-92a8-b511562184d2"); !errors.Is(err, ErrNotificationNotFound) {
		t.Fatalf("invalid target error=%v", err)
	}
}

func TestApplyCallbackQueuesApplicationAndAcknowledges(t *testing.T) {
	store, bot, requester := &serviceStore{}, &serviceBot{}, &applicationRequester{}
	service := NewService(store, "", time.Now, bot)
	service.SetApplicationRequester(requester)
	jobID := "b2d45992-9a64-42f3-92a8-b511562184d2"
	if err := service.HandleCallback(context.Background(), 10, "query-id", "job:apply:"+jobID); err != nil {
		t.Fatal(err)
	}
	if requester.telegramID != 10 || requester.jobID != jobID || bot.answer != "Application queued" {
		t.Fatalf("requester=%+v answer=%q", requester, bot.answer)
	}
	if store.action != "" {
		t.Fatalf("apply must not be recorded as completed feedback: %q", store.action)
	}
}

func TestApplicationAnswerCallbackDecodesExplicitAnswer(t *testing.T) {
	bot, answerer := &serviceBot{}, &applicationAnswerer{}
	service := NewService(&serviceStore{}, "", time.Now, bot)
	service.SetApplicationAnswerer(answerer)
	questionID := "b2d45992-9a64-42f3-92a8-b511562184d2"
	encoded := base64.RawURLEncoding.EncodeToString([]byte("Yes"))
	if err := service.HandleCallback(context.Background(), 42, "query-id", "application_answer:"+questionID+":"+encoded); err != nil {
		t.Fatal(err)
	}
	if answerer.telegramID != 42 || answerer.questionID != questionID || answerer.answer != "Yes" || bot.answer != "Answer saved" {
		t.Fatalf("answer callback was not handled: %+v response=%q", answerer, bot.answer)
	}
}

func TestTelegramCommandsRequirePrivateLinkedAccountAndControlNotifications(t *testing.T) {
	store, bot := &serviceStore{commandLinked: true, commandPreferences: domain.DefaultPreferences()}, &serviceBot{}
	service := NewService(store, "", time.Now, bot)
	if err := service.HandleMessage(context.Background(), 10, 10, "private", "candidate", "/status"); err != nil || !strings.Contains(bot.text, "Minimum match score") {
		t.Fatalf("status text=%q err=%v", bot.text, err)
	}
	if err := service.HandleMessage(context.Background(), 10, 10, "private", "candidate", "/stop"); err != nil || store.notificationsEnabled || !strings.Contains(bot.text, "paused") {
		t.Fatalf("stop enabled=%t text=%q err=%v", store.notificationsEnabled, bot.text, err)
	}
	if err := service.HandleMessage(context.Background(), 10, 10, "private", "candidate", "/resume"); err != nil || !store.notificationsEnabled || !strings.Contains(bot.text, "enabled") {
		t.Fatalf("resume enabled=%t text=%q err=%v", store.notificationsEnabled, bot.text, err)
	}
	if err := service.HandleMessage(context.Background(), 10, -100, "group", "candidate", "/stop"); err != nil || !store.notificationsEnabled {
		t.Fatalf("group command changed settings: err=%v", err)
	}
}

func TestTelegramJobsCommandUsesMatchesAndProvidesFeedbackButtons(t *testing.T) {
	jobID := "b2d45992-9a64-42f3-92a8-b511562184d2"
	store := &serviceStore{commandLinked: true, commandJobs: []RecommendedJob{{JobID: jobID, Title: "Senior Go Engineer", Company: "Example", Location: "Worldwide", Score: 87, ApplyURL: "https://jobs.example/apply"}}}
	bot := &serviceBot{}
	service := NewService(store, "", time.Now, bot)
	if err := service.HandleMessage(context.Background(), 42, 42, "private", "alex", "/jobs"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(bot.text, "87%") || len(bot.buttons) != 2 || bot.buttons[0][0].URL != "https://jobs.example/apply" || bot.buttons[1][0].CallbackData != "job:relevant:"+jobID || bot.buttons[1][1].CallbackData != "job:not_relevant:"+jobID {
		t.Fatalf("unexpected recommendation: %q %+v", bot.text, bot.buttons)
	}
}

func TestInvalidTelegramCallbackIsAcknowledged(t *testing.T) {
	bot := &serviceBot{}
	service := NewService(&serviceStore{}, "", time.Now, bot)
	if err := service.HandleCallback(context.Background(), 42, "invalid-query", "job:unknown:bad-id"); !errors.Is(err, ErrInvalidCallback) {
		t.Fatalf("callback error=%v", err)
	}
	if bot.answer != "This action is no longer available" {
		t.Fatalf("callback response=%q", bot.answer)
	}
}

func TestTelegramHelpWorksWithoutLinkAndStartExplainsLinking(t *testing.T) {
	bot := &serviceBot{}
	service := NewService(&serviceStore{}, "", time.Now, bot)
	if err := service.HandleMessage(context.Background(), 42, 42, "private", "alex", "/help"); err != nil || !strings.Contains(bot.text, "/jobs") {
		t.Fatalf("help text=%q err=%v", bot.text, err)
	}
	if err := service.HandleMessage(context.Background(), 42, 42, "private", "alex", "/start"); err != nil || !strings.Contains(bot.text, "one-time link") {
		t.Fatalf("start text=%q err=%v", bot.text, err)
	}
}
