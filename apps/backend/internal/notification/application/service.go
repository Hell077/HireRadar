package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/notification/domain"
	"github.com/Hell077/HireRadar/apps/backend/internal/observability"
	user "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/google/uuid"
)

var (
	ErrLinkExpired          = errors.New("telegram link token is invalid or expired")
	ErrTelegramInUse        = errors.New("telegram account is already linked")
	ErrFeedbackNotOwned     = errors.New("Telegram user does not own this match")
	ErrInvalidCallback      = errors.New("invalid Telegram callback")
	ErrNotificationNotFound = errors.New("notification is not available")
)

type Account struct {
	ID                string
	TelegramUserID    int64
	ChatID            int64
	Username          *string
	ConnectedAt       time.Time
	Enabled           bool
	LastInteractionAt *time.Time
}

type SavedJob struct {
	JobID    string    `json:"job_id"`
	Title    string    `json:"title"`
	Company  string    `json:"company"`
	Location string    `json:"location"`
	ApplyURL string    `json:"apply_url"`
	Status   string    `json:"status"`
	SavedAt  time.Time `json:"saved_at"`
}

type RecommendedJob struct {
	JobID        string
	Title        string
	Company      string
	Location     string
	RemotePolicy string
	Employment   []string
	Skills       []string
	ApplyURL     string
	Score        int
}

type CommandStore interface {
	AccountByTelegramID(context.Context, int64) (user.UserID, domain.NotificationPreferences, bool, error)
	RecentMatchesForTelegram(context.Context, int64, int) ([]RecommendedJob, error)
	SetNotificationsEnabled(context.Context, int64, bool) error
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
	ListSavedJobs(context.Context, user.UserID, int) ([]SavedJob, error)
	RemoveSavedJob(context.Context, user.UserID, string) error
	ApplyUserAction(context.Context, user.UserID, string, string) error
	ApplyUserFeedback(context.Context, user.UserID, string, string, string) error
	ApplyAction(context.Context, int64, string, string) (int64, error)
	SaveFeedbackReason(context.Context, int64, string, string) error
	OpenNotification(context.Context, string) (string, int, int, bool, error)
}

type Bot interface {
	SendMessage(context.Context, int64, string, [][]Button) error
	AnswerCallback(context.Context, string, string) error
}

type ApplicationRequester interface {
	RequestFromTelegram(context.Context, int64, string) error
}

type ApplicationAnswerer interface {
	AnswerFromTelegram(context.Context, int64, string, string) error
}

type Button struct {
	Text         string `json:"text"`
	URL          string `json:"url,omitempty"`
	CallbackData string `json:"callback_data,omitempty"`
}

type TelegramAPIError struct {
	StatusCode int
	RetryAfter time.Duration
	Blocked    bool
}

func (e *TelegramAPIError) Error() string {
	return fmt.Sprintf("Telegram API returned HTTP %d", e.StatusCode)
}

type Service struct {
	store        Store
	botUsername  string
	now          func() time.Time
	bot          Bot
	applications ApplicationRequester
	answerer     ApplicationAnswerer
}

func NewService(store Store, botUsername string, now func() time.Time, bot ...Bot) *Service {
	service := &Service{store: store, botUsername: botUsername, now: now}
	if len(bot) > 0 {
		service.bot = bot[0]
	}
	return service
}

func (s *Service) SetApplicationRequester(requester ApplicationRequester) {
	s.applications = requester
}

func (s *Service) SetApplicationAnswerer(answerer ApplicationAnswerer) {
	s.answerer = answerer
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

// HandleMessage processes private Telegram commands while keeping account and
// matching data access in the existing notification store.
func (s *Service) HandleMessage(ctx context.Context, telegramUserID, chatID int64, chatType, username, text string) error {
	if telegramUserID <= 0 || chatID <= 0 || chatType != "private" || telegramUserID != chatID || len(text) > 4096 {
		return nil
	}
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return nil
	}
	command := strings.SplitN(strings.TrimPrefix(parts[0], "/"), "@", 2)[0]
	argument := ""
	if len(parts) > 1 {
		argument = parts[1]
	}
	if command == "start" && argument != "" {
		if err := s.HandleStart(ctx, telegramUserID, chatID, username, argument); err != nil {
			if errors.Is(err, ErrLinkExpired) || errors.Is(err, ErrTelegramInUse) {
				return s.send(ctx, chatID, "That connection link is invalid or expired. Create a new link from HireRadar settings.", nil)
			}
			return err
		}
		return nil
	}
	if command == "help" {
		return s.send(ctx, chatID, "HireRadar commands:\n/status — connection and notification status\n/jobs — recent recommended jobs\n/settings — notification settings\n/stop — pause notifications\n/resume — enable notifications", nil)
	}
	store, ok := s.store.(CommandStore)
	if !ok {
		return errors.New("Telegram command store is unavailable")
	}
	_, preferences, linked, err := store.AccountByTelegramID(ctx, telegramUserID)
	if err != nil {
		return err
	}
	if command == "start" {
		if linked {
			return s.send(ctx, chatID, "Your HireRadar account is already connected. Use /help to see available commands.", nil)
		}
		return s.send(ctx, chatID, "Connect Telegram from HireRadar Settings using the one-time link. Then use /help here.", nil)
	}
	if !linked {
		return s.send(ctx, chatID, "Connect Telegram from HireRadar Settings first. The one-time link expires after 15 minutes.", nil)
	}
	switch command {
	case "status":
		state := "enabled"
		if !preferences.Enabled {
			state = "paused"
		}
		return s.send(ctx, chatID, fmt.Sprintf("HireRadar account: connected\nJob notifications: %s\nMinimum match score: %d%%", state, preferences.MinimumScore), nil)
	case "settings":
		return s.send(ctx, chatID, fmt.Sprintf("Notifications are %s. Minimum match score: %d%%. Use /stop to pause or /resume to enable.", map[bool]string{true: "enabled", false: "paused"}[preferences.Enabled], preferences.MinimumScore), nil)
	case "stop", "resume":
		enabled := command == "resume"
		if err := store.SetNotificationsEnabled(ctx, telegramUserID, enabled); err != nil {
			return err
		}
		message := "Notifications paused. Your HireRadar account remains connected. Use /resume to enable them again."
		if enabled {
			message = "Notifications enabled. New qualifying matches will be sent here."
		}
		return s.send(ctx, chatID, message, nil)
	case "jobs":
		jobs, err := store.RecentMatchesForTelegram(ctx, telegramUserID, 5)
		if err != nil {
			return err
		}
		if len(jobs) == 0 {
			return s.send(ctx, chatID, "No recent matches yet. Check your HireRadar profile and preferences; new jobs are checked automatically.", nil)
		}
		for _, job := range jobs {
			message := fmt.Sprintf("<b>%s</b>\n%s\nMatch: %d%%", html.EscapeString(job.Title), html.EscapeString(job.Company), job.Score)
			if job.Location != "" {
				message += "\n" + html.EscapeString(job.Location)
			}
			if len(job.Employment) > 0 {
				message += "\n" + html.EscapeString(strings.Join(job.Employment, " · "))
			}
			if len(job.Skills) > 0 {
				message += "\n" + html.EscapeString(strings.Join(job.Skills, " · "))
			}
			buttons := [][]Button{{{Text: "View job", URL: job.ApplyURL}, {Text: "Apply", CallbackData: "job:apply:" + job.JobID}}, {{Text: "Relevant", CallbackData: "job:relevant:" + job.JobID}, {Text: "Not relevant", CallbackData: "job:not_relevant:" + job.JobID}}}
			if err := s.send(ctx, chatID, message, buttons); err != nil {
				return err
			}
		}
		return nil
	default:
		return s.send(ctx, chatID, "Unknown command. Use /help for available commands.", nil)
	}
}

func (s *Service) send(ctx context.Context, chatID int64, message string, buttons [][]Button) error {
	if s.bot == nil {
		return nil
	}
	return s.bot.SendMessage(ctx, chatID, message, buttons)
}

func (s *Service) HandleCallback(ctx context.Context, telegramUserID int64, callbackID, data string) error {
	invalid := func() error {
		if s.bot != nil && callbackID != "" {
			_ = s.bot.AnswerCallback(ctx, callbackID, "This action is no longer available")
		}
		return ErrInvalidCallback
	}
	if strings.HasPrefix(data, "application_answer:") {
		parts := strings.Split(data, ":")
		if len(parts) != 3 || uuid.Validate(parts[1]) != nil || s.answerer == nil {
			return invalid()
		}
		answer, err := base64.RawURLEncoding.DecodeString(parts[2])
		if err != nil || len(answer) == 0 || len(answer) > 2000 {
			return invalid()
		}
		if err := s.answerer.AnswerFromTelegram(ctx, telegramUserID, parts[1], string(answer)); err != nil {
			if s.bot != nil && callbackID != "" {
				_ = s.bot.AnswerCallback(ctx, callbackID, "Answer could not be saved")
			}
			return err
		}
		if s.bot != nil && callbackID != "" {
			return s.bot.AnswerCallback(ctx, callbackID, "Answer saved")
		}
		return nil
	}
	parts := strings.Split(data, ":")
	if len(parts) == 3 && parts[0] == "fr" && uuid.Validate(parts[1]) == nil && validFeedbackReason(parts[2]) && parts[2] != "" {
		if err := s.store.SaveFeedbackReason(ctx, telegramUserID, parts[1], parts[2]); err != nil {
			if s.bot != nil && callbackID != "" {
				_ = s.bot.AnswerCallback(ctx, callbackID, "This action is no longer available")
			}
			return err
		}
		if s.bot != nil && callbackID != "" {
			return s.bot.AnswerCallback(ctx, callbackID, "Thanks for the feedback")
		}
		return nil
	}
	if len(parts) != 3 || parts[0] != "job" || uuid.Validate(parts[2]) != nil {
		return invalid()
	}
	action := parts[1]
	if action == "apply" {
		if s.applications == nil {
			return invalid()
		}
		if err := s.applications.RequestFromTelegram(ctx, telegramUserID, parts[2]); err != nil {
			if errors.Is(err, ErrFeedbackNotOwned) && s.bot != nil && callbackID != "" {
				_ = s.bot.AnswerCallback(ctx, callbackID, "This match is no longer available")
			} else if s.bot != nil && callbackID != "" {
				_ = s.bot.AnswerCallback(ctx, callbackID, "Application could not be queued")
			}
			return err
		}
		if s.bot != nil && callbackID != "" {
			return s.bot.AnswerCallback(ctx, callbackID, "Application queued")
		}
		return nil
	}
	if action != "save" && action != "hide" && action != "applied" && action != "relevant" && action != "not_relevant" {
		return invalid()
	}
	chatID, err := s.store.ApplyAction(ctx, telegramUserID, parts[2], action)
	if err != nil {
		if s.bot != nil && callbackID != "" {
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
		} else if action == "relevant" {
			response = "Thanks — we’ll look for more like this"
		} else if action == "not_relevant" {
			response = "Hidden from your feed"
		}
		if err := s.bot.AnswerCallback(ctx, callbackID, response); err != nil {
			return err
		}
	}
	if action == "hide" && s.bot != nil && chatID != 0 {
		return s.bot.SendMessage(ctx, chatID, "Why was this job a mismatch? (optional)", feedbackReasonButtons(parts[2]))
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

func (s *Service) SavedJobs(ctx context.Context, userID user.UserID, limit int) ([]SavedJob, error) {
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > 100 {
		return nil, errors.New("saved job limit must be between 1 and 100")
	}
	return s.store.ListSavedJobs(ctx, userID, limit)
}

func (s *Service) RemoveSavedJob(ctx context.Context, userID user.UserID, jobID string) error {
	if uuid.Validate(jobID) != nil {
		return errors.New("invalid job ID")
	}
	return s.store.RemoveSavedJob(ctx, userID, jobID)
}

func (s *Service) ApplyUserAction(ctx context.Context, userID user.UserID, jobID, action string) error {
	if uuid.Validate(jobID) != nil || (action != "hide" && action != "applied") {
		return ErrInvalidCallback
	}
	return s.store.ApplyUserAction(ctx, userID, jobID, action)
}

func feedbackReasonButtons(jobID string) [][]Button {
	values := []struct{ label, value string }{
		{"Wrong stack", "wrong_stack"}, {"Wrong role", "wrong_role"},
		{"Seniority", "wrong_seniority"}, {"Location", "wrong_location"},
		{"Salary", "wrong_salary"}, {"Company", "wrong_company"},
		{"Duplicate", "duplicate"}, {"Already seen", "already_seen"},
		{"Not interested", "not_interested"}, {"Other", "other"},
	}
	buttons := make([][]Button, 0, len(values))
	for _, item := range values {
		buttons = append(buttons, []Button{{Text: item.label, CallbackData: "fr:" + jobID + ":" + item.value}})
	}
	return buttons
}

func (s *Service) ApplyUserFeedback(ctx context.Context, userID user.UserID, jobID, action, reason string) error {
	if uuid.Validate(jobID) != nil || (action != "hide" && action != "applied") || !validFeedbackReason(reason) || (reason != "" && action != "hide") {
		return ErrInvalidCallback
	}
	return s.store.ApplyUserFeedback(ctx, userID, jobID, action, reason)
}

func (s *Service) OpenNotification(ctx context.Context, notificationID string) (string, error) {
	if uuid.Validate(notificationID) != nil {
		return "", ErrNotificationNotFound
	}
	applyURL, score, confidence, firstOpen, err := s.store.OpenNotification(ctx, notificationID)
	if err != nil {
		return "", err
	}
	parsed, err := url.ParseRequestURI(applyURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
		return "", ErrNotificationNotFound
	}
	if firstOpen {
		observability.DefaultMetrics.Add("hireradar_product_notification_open_total", map[string]string{
			"score_bucket": observability.Bucket100(score), "confidence_bucket": observability.Bucket100(confidence),
		}, 1)
	}
	return parsed.String(), nil
}

func validFeedbackReason(reason string) bool {
	switch reason {
	case "", "wrong_stack", "wrong_role", "wrong_seniority", "wrong_location", "wrong_salary", "wrong_company", "duplicate", "already_seen", "not_interested", "other":
		return true
	default:
		return false
	}
}

func (s *Service) Disconnect(ctx context.Context, userID user.UserID) error {
	return s.store.Disconnect(ctx, userID)
}
