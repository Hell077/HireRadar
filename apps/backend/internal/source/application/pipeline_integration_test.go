package application

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/job/normalization"
	matchingpostgres "github.com/Hell077/HireRadar/apps/backend/internal/matching/adapters/postgres"
	matchingapp "github.com/Hell077/HireRadar/apps/backend/internal/matching/application"
	notificationpostgres "github.com/Hell077/HireRadar/apps/backend/internal/notification/adapters/postgres"
	"github.com/Hell077/HireRadar/apps/backend/internal/notification/application"
	"github.com/Hell077/HireRadar/apps/backend/internal/source/domain"
	userdomain "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pipelineBot struct{ sends int }

func (b *pipelineBot) SendMessage(context.Context, int64, string, [][]application.Button) error {
	b.sends++
	return nil
}
func (*pipelineBot) AnswerCallback(context.Context, string, string) error { return nil }

// This gate crosses the persisted ingestion, matching outbox, and notification
// outbox boundaries. A fresh notification worker instance resumes queued work.
func TestSeededSourcePipelineSurvivesWorkerRestartAndDeliversOnce(t *testing.T) {
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
	sourceID := "pipeline-" + uuid.NewString()
	userID := uuid.NewString()
	title := "Senior Backend Engineer " + uuid.NewString()
	companyName := "Pipeline Company " + uuid.NewString()
	jobURL := "https://jobs.example/" + uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO sources(id,name,source_type,company_name,config,enabled) VALUES($1,'Pipeline fixture','greenhouse',$2,'{"board":"fixture"}',false)`, sourceID, companyName); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,email,password_hash) VALUES($1,$2,'test')`, userID, userID+"@example.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO user_profiles(user_id,country,seniority) VALUES($1,'KZ','senior')`, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO job_preferences(user_id,minimum_match_score,maximum_job_age_days) VALUES($1,0,30)`, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO user_positions(id,user_id,title,normalized_title) VALUES($1,$2,$3,$4)`, uuid.NewString(), userID, title, strings.ToLower(title)); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM outbox_events WHERE payload->>'user_id'=$1 OR payload->>'job_id' IN (SELECT id::text FROM jobs WHERE title=$2) OR aggregate_id IN (SELECT id::text FROM jobs WHERE title=$2)`, userID, title)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, userID)
		_, _ = pool.Exec(ctx, `DELETE FROM jobs WHERE title=$1`, title)
		_, _ = pool.Exec(ctx, `DELETE FROM sources WHERE id=$1`, sourceID)
		_, _ = pool.Exec(ctx, `DELETE FROM companies WHERE normalized_name=$1`, normalization.CompanyKey(companyName))
	}()
	job := domain.ExternalJob{ExternalID: "seed-1", CompanyName: companyName, Title: title, Description: "Build backend services with Go. Go is used throughout the team.", Location: "Almaty, Kazakhstan", ApplyURL: jobURL, Raw: json.RawMessage(`{"id":"seed-1","title":"Senior Backend Engineer"}`)}
	if err := NewWorker(pool, fixtureFetcher{result: domain.FetchResult{Jobs: []domain.ExternalJob{job}}}, 1).syncSource(ctx, domain.Source{ID: sourceID, SyncIntervalSecond: 900}); err != nil {
		t.Fatal(err)
	}
	var jobID, eventID string
	if err := pool.QueryRow(ctx, `SELECT j.id::text FROM jobs j JOIN job_sources js ON js.job_id=j.id WHERE js.source_id=$1`, sourceID).Scan(&jobID); err != nil {
		t.Fatal(err)
	}
	var rawCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM raw_jobs WHERE source_id=$1`, sourceID).Scan(&rawCount); err != nil || rawCount != 1 {
		t.Fatalf("raw snapshots=%d err=%v", rawCount, err)
	}
	if err := pool.QueryRow(ctx, `SELECT id::text FROM outbox_events WHERE event_type='job.created' AND aggregate_id=$1 ORDER BY occurred_at DESC LIMIT 1`, jobID).Scan(&eventID); err != nil {
		t.Fatal(err)
	}
	matcher := matchingapp.NewService(matchingpostgres.NewStore(pool), time.Now)
	matchingWorker := matchingpostgres.NewOutboxWorker(pool, func(ctx context.Context, id userdomain.UserID) error { _, err := matcher.Refresh(ctx, id); return err }, func(ctx context.Context, id string) error { return matcher.RefreshJob(ctx, id) })
	if found, err := matchingWorker.ProcessEvent(ctx, eventID); err != nil || !found {
		t.Fatalf("matching event found=%v err=%v", found, err)
	}
	var score int
	if err := pool.QueryRow(ctx, `SELECT score FROM user_job_matches WHERE user_id=$1 AND job_id=$2`, userID, jobID).Scan(&score); err != nil {
		t.Fatal(err)
	}
	if score < 70 {
		t.Fatalf("match score %d is below notification threshold", score)
	}
	telegramID := int64(918273645001)
	if _, err := pool.Exec(ctx, `INSERT INTO telegram_accounts(id,user_id,telegram_user_id,chat_id) VALUES($1,$2,$3,$3)`, uuid.NewString(), userID, telegramID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT id::text FROM outbox_events WHERE event_type='match.created' AND payload->>'user_id'=$1 AND payload->>'job_id'=$2 ORDER BY occurred_at DESC LIMIT 1`, userID, jobID).Scan(&eventID); err != nil {
		t.Fatal(err)
	}
	bot := &pipelineBot{}
	firstProcess := notificationpostgres.NewWorker(pool, bot, time.Now)
	if found, err := firstProcess.ProcessEvent(ctx, eventID); err != nil || !found {
		t.Fatalf("notification schedule found=%v err=%v", found, err)
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM notifications WHERE user_id=$1 AND job_id=$2`, userID, jobID).Scan(&status); err != nil || status != "pending" {
		t.Fatalf("queued notification status=%q err=%v", status, err)
	}
	// Reconstruct the worker after enqueue, as after a process restart.
	restarted := notificationpostgres.NewWorker(pool, bot, time.Now)
	if found, err := restarted.ProcessNext(ctx); err != nil || !found {
		t.Fatalf("resumed notification found=%v err=%v", found, err)
	}
	if found, err := restarted.ProcessEvent(ctx, eventID); err != nil || found {
		t.Fatalf("duplicate outbox processing found=%v err=%v", found, err)
	}
	if err := pool.QueryRow(ctx, `SELECT status FROM notifications WHERE user_id=$1 AND job_id=$2`, userID, jobID).Scan(&status); err != nil || status != "sent" || bot.sends != 1 {
		t.Fatalf("final notification status=%q sends=%d err=%v", status, bot.sends, err)
	}
}
