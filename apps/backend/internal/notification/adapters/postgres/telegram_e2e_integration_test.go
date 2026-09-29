package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	matchingpostgres "github.com/Hell077/HireRadar/apps/backend/internal/matching/adapters/postgres"
	matchingapp "github.com/Hell077/HireRadar/apps/backend/internal/matching/application"
	telegramclient "github.com/Hell077/HireRadar/apps/backend/internal/notification/adapters/telegram"
	"github.com/Hell077/HireRadar/apps/backend/internal/notification/application"
	"github.com/Hell077/HireRadar/apps/backend/internal/notification/domain"
	"github.com/Hell077/HireRadar/apps/backend/internal/notification/telegramwebhook"
	user "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestTelegramEndToEndLinkMatchNotificationAndFeedback(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to a fresh migrated PostgreSQL test database")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	userID, companyID, jobID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	telegramID := int64(time.Now().UnixNano()%1_000_000_000 + 1)
	chatID := telegramID // Telegram private chat IDs equal the user's numeric ID.
	companyName := "Telegram E2E " + uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,email,password_hash) VALUES($1,$2,'test')`, userID, userID+"@example.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO user_profiles(user_id,country,seniority) VALUES($1,'KZ','senior')`, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO job_preferences(user_id,remote_policies,employment_types,minimum_match_score,maximum_job_age_days) VALUES($1,'{worldwide}','{full_time}',0,30)`, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO user_positions(id,user_id,title,normalized_title) VALUES($1,$2,'Backend Engineer','backend engineer')`, uuid.NewString(), userID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO companies(id,name,normalized_name) VALUES($1,$2,$3)`, companyID, companyName, companyName); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO jobs(id,company_id,title,normalized_title,seniority,employment_types,remote_policy,location,eligibility,apply_url,fingerprint,status,first_seen_at) VALUES($1,$2,'Senior Go Backend Engineer','senior go backend engineer','senior','{full_time}','worldwide','Worldwide','eligible','https://jobs.example.test/e2e',$3,'active',now())`, jobID, companyID, make([]byte, 32)); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM outbox_events WHERE payload->>'user_id'=$1 OR aggregate_id=ANY($2::text[])`, userID, []string{jobID})
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, userID)
		_, _ = pool.Exec(ctx, `DELETE FROM jobs WHERE id=$1`, jobID)
		_, _ = pool.Exec(ctx, `DELETE FROM companies WHERE id=$1`, companyID)
	}()

	type telegramRequest struct {
		Method string `json:"method"`
		Text   string `json:"text"`
	}
	var requests []telegramRequest
	var requestsMu sync.Mutex
	telegramAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request telegramRequest
		request.Method = strings.TrimPrefix(r.URL.Path, "/bottest-token/")
		_ = json.NewDecoder(r.Body).Decode(&request)
		requestsMu.Lock()
		requests = append(requests, request)
		requestsMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	defer telegramAPI.Close()
	bot := telegramclient.NewClientWithBaseURL("test-token", telegramAPI.URL, telegramAPI.Client())
	store := NewStore(pool)
	service := application.NewService(store, "hire_radar_bot", time.Now, bot)
	webhook := httptest.NewServer(telegramwebhook.New("fixture-webhook-secret", service, pool))
	defer webhook.Close()

	link, err := service.CreateLink(ctx, user.UserID(userID))
	if err != nil || !strings.HasPrefix(link.URL, "https://t.me/hire_radar_bot?start=") {
		t.Fatalf("create link url=%q err=%v", link.URL, err)
	}
	startUpdate := fmt.Sprintf(`{"message":{"text":"/start %s","from":{"id":%d,"username":"e2e"},"chat":{"id":%d,"type":"private"}}}`, link.Token, telegramID, chatID)
	postTelegramUpdate(t, webhook.URL, "fixture-webhook-secret", startUpdate)
	var linked int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM telegram_accounts WHERE user_id=$1 AND telegram_user_id=$2 AND chat_id=$3 AND enabled`, userID, telegramID, chatID).Scan(&linked); err != nil || linked != 1 {
		t.Fatalf("linked accounts=%d err=%v", linked, err)
	}
	if err := store.SavePreferences(ctx, user.UserID(userID), domain.NotificationPreferences{Enabled: true, MinimumScore: 0, Immediate: true, Timezone: "UTC"}); err != nil {
		t.Fatal(err)
	}

	matcher := matchingapp.NewService(matchingpostgres.NewStore(pool), time.Now)
	results, err := matcher.Refresh(ctx, user.UserID(userID))
	if err != nil || len(results) != 1 || results[0].JobID != jobID {
		t.Fatalf("matching results=%+v err=%v", results, err)
	}
	var matchEventID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM outbox_events WHERE event_type='match.created' AND aggregate_id=$1 AND payload->>'user_id'=$2 ORDER BY occurred_at DESC LIMIT 1`, jobID, userID).Scan(&matchEventID); err != nil {
		t.Fatal(err)
	}
	notifications := NewWorker(pool, bot, time.Now)
	if found, err := notifications.ProcessEvent(ctx, matchEventID); err != nil || !found {
		t.Fatalf("schedule match notification found=%v err=%v", found, err)
	}
	if found, err := notifications.deliverNext(ctx); err != nil || !found {
		t.Fatalf("deliver match notification found=%v err=%v", found, err)
	}
	var notificationCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE user_id=$1 AND job_id=$2 AND status='sent'`, userID, jobID).Scan(&notificationCount); err != nil || notificationCount != 1 {
		t.Fatalf("sent notifications=%d err=%v", notificationCount, err)
	}
	requestsMu.Lock()
	matchedMessageSent := false
	for _, request := range requests {
		if request.Method == "sendMessage" && strings.Contains(request.Text, "Senior Go Backend Engineer") {
			matchedMessageSent = true
		}
	}
	requestsMu.Unlock()
	if !matchedMessageSent {
		t.Fatal("fake Telegram API did not receive the matched job notification")
	}

	callback := fmt.Sprintf(`{"callback_query":{"id":"feedback-e2e","data":"job:relevant:%s","from":{"id":%d}}}`, jobID, telegramID)
	postTelegramUpdate(t, webhook.URL, "fixture-webhook-secret", callback)
	var feedbackCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM user_job_feedback WHERE user_id=$1 AND job_id=$2 AND feedback_type='relevant'`, userID, jobID).Scan(&feedbackCount); err != nil || feedbackCount != 1 {
		t.Fatalf("relevant feedback count=%d err=%v", feedbackCount, err)
	}
	requestsMu.Lock()
	callbackAnswered := false
	for _, request := range requests {
		if request.Method == "answerCallbackQuery" {
			callbackAnswered = true
		}
	}
	requestsMu.Unlock()
	if !callbackAnswered {
		t.Fatal("fake Telegram API did not receive a callback acknowledgement")
	}
}

func postTelegramUpdate(t *testing.T, endpoint, secret, body string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, endpoint+"/telegram/webhook", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", secret)
	req.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("Telegram fixture update status=%d", response.StatusCode)
	}
}
