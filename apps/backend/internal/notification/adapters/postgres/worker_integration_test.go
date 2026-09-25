package postgres

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	matchingpostgres "github.com/Hell077/HireRadar/apps/backend/internal/matching/adapters/postgres"
	"github.com/Hell077/HireRadar/apps/backend/internal/matching/engine"
	"github.com/Hell077/HireRadar/apps/backend/internal/notification/application"
	"github.com/Hell077/HireRadar/apps/backend/internal/notification/domain"
	user "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type fakeBot struct {
	messages int
	buttons  [][]application.Button
	err      error
}

func (f *fakeBot) SendMessage(_ context.Context, _ int64, _ string, buttons [][]application.Button) error {
	f.messages++
	f.buttons = buttons
	return f.err
}
func (f *fakeBot) AnswerCallback(context.Context, string, string) error { return f.err }

func TestNotificationOutboxDeliveryFeedbackAndDeadLetter(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to a migrated PostgreSQL test database")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	store := NewStore(pool)
	userID, companyID := uuid.NewString(), uuid.NewString()
	firstJobID, retryJobID := uuid.NewString(), uuid.NewString()
	companyName := "Notification Test " + uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,email,password_hash) VALUES($1,$2,'test')`, userID, userID+"@example.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO companies(id,name,normalized_name) VALUES($1,$2,$3)`, companyID, companyName, companyName); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM outbox_events WHERE payload->>'user_id'=$1 OR aggregate_id=ANY($2::text[])`, userID, []string{firstJobID, retryJobID})
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, userID)
		_, _ = pool.Exec(ctx, `DELETE FROM companies WHERE id=$1`, companyID)
	}()
	for _, item := range []struct{ id, title string }{{firstJobID, "Go Engineer"}, {retryJobID, "Platform Engineer"}} {
		if _, err := pool.Exec(ctx, `INSERT INTO jobs(id,company_id,title,normalized_title,remote_policy,eligibility,apply_url,fingerprint,status)
			VALUES($1,$2,$3,$3,'worldwide','eligible',$4,$5,'active')`, item.id, companyID, item.title, "https://example.test/"+item.id, make([]byte, 32)); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO user_job_matches(user_id,job_id,score,components) VALUES($1,$2,92,'[]'::jsonb)`, userID, item.id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO telegram_accounts(id,user_id,telegram_user_id,chat_id,username) VALUES($1,$2,501,7001,'tester')`, uuid.NewString(), userID); err != nil {
		t.Fatal(err)
	}
	if err := store.SavePreferences(ctx, userIDValue(userID), domain.DefaultPreferences()); err != nil {
		t.Fatal(err)
	}
	eventID := uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO outbox_events(id,event_type,aggregate_type,aggregate_id,payload) VALUES($1,'match.created','match',$2,jsonb_build_object('user_id',$3::text,'job_id',$2::text,'score',92))`, eventID, firstJobID, userID); err != nil {
		t.Fatal(err)
	}
	bot := &fakeBot{}
	worker := NewWorker(pool, bot, time.Now)
	found, err := worker.ProcessEvent(ctx, eventID)
	if err != nil || !found {
		t.Fatalf("schedule match found=%v err=%v", found, err)
	}
	found, err = worker.deliverNext(ctx)
	if err != nil || !found || bot.messages != 1 || len(bot.buttons) != 2 || bot.buttons[1][0].CallbackData != "job:save:"+firstJobID {
		t.Fatalf("delivery found=%v err=%v bot=%+v", found, err, bot)
	}
	var notificationCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE user_id=$1 AND job_id=$2`, userID, firstJobID).Scan(&notificationCount); err != nil || notificationCount != 1 {
		t.Fatalf("notification count=%d err=%v", notificationCount, err)
	}
	duplicateEvent := uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO outbox_events(id,event_type,aggregate_type,aggregate_id,payload) VALUES($1,'match.created','match',$2,jsonb_build_object('user_id',$3::text,'job_id',$2::text,'score',92))`, duplicateEvent, firstJobID, userID); err != nil {
		t.Fatal(err)
	}
	if found, err := worker.ProcessEvent(ctx, duplicateEvent); err != nil || !found {
		t.Fatalf("duplicate event found=%v err=%v", found, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE user_id=$1 AND job_id=$2`, userID, firstJobID).Scan(&notificationCount); err != nil || notificationCount != 1 {
		t.Fatalf("duplicate notification count=%d err=%v", notificationCount, err)
	}
	if err := store.ApplyAction(ctx, 501, firstJobID, "save"); err != nil {
		t.Fatal(err)
	}
	if err := store.ApplyAction(ctx, 999, firstJobID, "hide"); !errors.Is(err, application.ErrFeedbackNotOwned) {
		t.Fatalf("non-owner feedback error = %v", err)
	}
	if err := store.ApplyAction(ctx, 501, firstJobID, "hide"); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM user_job_matches WHERE user_id=$1 AND job_id=$2`, userID, firstJobID).Scan(&notificationCount); err != nil || notificationCount != 0 {
		t.Fatalf("hidden match count=%d err=%v", notificationCount, err)
	}
	if err := matchingpostgres.NewStore(pool).SaveJobMatch(ctx, userIDValue(userID), engine.Result{JobID: firstJobID, Eligible: true, Score: 92}); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM user_job_matches WHERE user_id=$1 AND job_id=$2`, userID, firstJobID).Scan(&notificationCount); err != nil || notificationCount != 0 {
		t.Fatalf("dismissed match was recreated: count=%d err=%v", notificationCount, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM saved_jobs WHERE user_id=$1 AND job_id=$2`, userID, firstJobID).Scan(&notificationCount); err != nil || notificationCount != 1 {
		t.Fatalf("saved job count=%d err=%v", notificationCount, err)
	}
	prefs := domain.DefaultPreferences()
	maxPerDay := 1
	prefs.MaxPerDay = &maxPerDay
	if err := store.SavePreferences(ctx, userIDValue(userID), prefs); err != nil {
		t.Fatal(err)
	}
	retryEvent := uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO outbox_events(id,event_type,aggregate_type,aggregate_id,payload) VALUES($1,'match.created','match',$2,jsonb_build_object('user_id',$3::text,'job_id',$2::text,'score',92))`, retryEvent, retryJobID, userID); err != nil {
		t.Fatal(err)
	}
	if found, err := worker.ProcessEvent(ctx, retryEvent); err != nil || !found {
		t.Fatalf("process retry event found=%v err=%v", found, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE notifications SET scheduled_at=now()-interval '1 second' WHERE user_id=$1 AND job_id=$2`, userID, retryJobID); err != nil {
		t.Fatal(err)
	}
	if found, err := worker.deliverNext(ctx); err != nil || !found || bot.messages != 1 {
		t.Fatalf("daily delivery cap found=%v err=%v messages=%d", found, err, bot.messages)
	}
	var nextDelivery time.Time
	if err := pool.QueryRow(ctx, `SELECT scheduled_at FROM notifications WHERE user_id=$1 AND job_id=$2`, userID, retryJobID).Scan(&nextDelivery); err != nil || nextDelivery.Before(time.Now().UTC().Truncate(24*time.Hour).Add(24*time.Hour).Add(-time.Second)) {
		t.Fatalf("daily-capped schedule=%s err=%v", nextDelivery, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE notifications SET sent_at=now()-interval '1 day' WHERE user_id=$1 AND job_id=$2`, userID, firstJobID); err != nil {
		t.Fatal(err)
	}
	bot.err = errors.New("telegram unavailable")
	for attempt := 1; attempt <= maxDeliveryAttempts; attempt++ {
		if _, err := pool.Exec(ctx, `UPDATE notifications SET scheduled_at=now()-interval '1 second' WHERE user_id=$1 AND job_id=$2`, userID, retryJobID); err != nil {
			t.Fatal(err)
		}
		found, err := worker.deliverNext(ctx)
		if !found || err == nil {
			t.Fatalf("failed attempt %d found=%v err=%v", attempt, found, err)
		}
	}
	var status string
	var attempts int
	if err := pool.QueryRow(ctx, `SELECT status,attempts FROM notifications WHERE user_id=$1 AND job_id=$2`, userID, retryJobID).Scan(&status, &attempts); err != nil || status != "failed" || attempts != maxDeliveryAttempts {
		t.Fatalf("dead-letter status=%q attempts=%d err=%v", status, attempts, err)
	}
}

func TestLinkTokensAreHashedSingleUseAndPreferencesRoundTrip(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to a migrated PostgreSQL test database")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	userID := uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,email,password_hash) VALUES($1,$2,'test')`, userID, userID+"@example.test"); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, userID)
	store := NewStore(pool)
	hash := []byte(strings.Repeat("h", 32))
	if err := store.CreateLink(ctx, userIDValue(userID), uuid.NewString(), hash, time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := store.ConsumeLink(ctx, hash, application.Account{ID: uuid.NewString(), TelegramUserID: 777, ChatID: 888}); err != nil {
		t.Fatal(err)
	}
	if err := store.ConsumeLink(ctx, hash, application.Account{ID: uuid.NewString(), TelegramUserID: 777, ChatID: 888}); !errors.Is(err, application.ErrLinkExpired) {
		t.Fatalf("reused link error = %v", err)
	}
	prefs := domain.DefaultPreferences()
	start, end, limit := "23:00", "08:00", 5
	prefs.Timezone, prefs.QuietStart, prefs.QuietEnd, prefs.MaxPerDay = "Asia/Almaty", &start, &end, &limit
	if err := store.SavePreferences(ctx, userIDValue(userID), prefs); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetPreferences(ctx, userIDValue(userID))
	if err != nil || got.Timezone != prefs.Timezone || got.QuietStart == nil || *got.QuietStart != start || got.QuietEnd == nil || *got.QuietEnd != end || got.MaxPerDay == nil || *got.MaxPerDay != limit {
		t.Fatalf("preferences=%+v err=%v", got, err)
	}
	if err := store.Disconnect(ctx, userIDValue(userID)); err != nil {
		t.Fatal(err)
	}
	account, err := store.Connection(ctx, userIDValue(userID))
	if err != nil || account.ID != "" {
		t.Fatalf("disconnected account=%+v err=%v", account, err)
	}
}

func userIDValue(id string) user.UserID { return user.UserID(id) }
