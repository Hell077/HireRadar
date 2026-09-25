package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/matching/application"
	"github.com/Hell077/HireRadar/apps/backend/internal/matching/engine"
	user "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMatchingRefreshPreselectsAndPersistsIdempotently(t *testing.T) {
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
	userID, companyID := uuid.NewString(), uuid.NewString()
	companyName := "Match Integration " + uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,email,password_hash) VALUES($1,$2,'test')`, userID, userID+"@example.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO user_profiles(user_id,country,seniority) VALUES($1,'KZ','senior')`, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO job_preferences(user_id,remote_policies,employment_types,minimum_match_score,maximum_job_age_days) VALUES($1,'{remote}','{full_time}',60,30)`, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO companies(id,name,normalized_name) VALUES($1,$2,$3)`, companyID, companyName, companyName); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM outbox_events WHERE aggregate_id=$1`, userID)
		_, _ = pool.Exec(ctx, `DELETE FROM companies WHERE id=$1`, companyID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, userID)
	}()
	if _, err := pool.Exec(ctx, `INSERT INTO skills(id,name,normalized_name) VALUES($1,'Go','go') ON CONFLICT(normalized_name) DO NOTHING`, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	var skillID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM skills WHERE normalized_name='go'`).Scan(&skillID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO user_skills(user_id,skill_id,years,level) VALUES($1,$2,5,'advanced')`, userID, skillID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO user_positions(id,user_id,title,normalized_title) VALUES($1,$2,'Backend Engineer','backend engineer')`, uuid.NewString(), userID); err != nil {
		t.Fatal(err)
	}
	goodID, rejectedID := uuid.NewString(), uuid.NewString()
	insertJob := func(id, title, country, eligibility string) {
		t.Helper()
		_, err := pool.Exec(ctx, `INSERT INTO jobs(id,company_id,title,normalized_title,seniority,description,salary_min,salary_max,salary_currency,salary_period,employment_types,remote_policy,location,location_countries,eligibility,apply_url,fingerprint,status) VALUES($1,$2,$3,$4,'senior','Go backend',120000,150000,'USD','year','{full_time}','remote',$5,$6,$7,$8,$9,'active')`, id, companyID, title, title, "Remote — "+country, []string{country}, eligibility, "https://example.test/"+id, make([]byte, 32))
		if err != nil {
			t.Fatal(err)
		}
	}
	insertJob(goodID, "Senior Backend Engineer", "KZ", "eligible")
	insertJob(rejectedID, "Senior Backend Engineer", "US", "not_eligible")
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM jobs WHERE id=ANY($1::uuid[])`, []string{goodID, rejectedID})
	}()
	if _, err := pool.Exec(ctx, `INSERT INTO job_skills(job_id,skill_id,required,confidence) VALUES($1,$2,true,0.95)`, goodID, skillID); err != nil {
		t.Fatal(err)
	}
	service := application.NewService(NewStore(pool), time.Now)
	results, err := service.Refresh(ctx, user.UserID(userID))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].JobID != goodID || results[0].Score < 60 {
		t.Fatalf("unexpected matching result: %+v", results)
	}
	if _, err := service.Refresh(ctx, user.UserID(userID)); err != nil {
		t.Fatal(err)
	}
	listed, err := service.List(ctx, user.UserID(userID), 100)
	if err != nil || len(listed) != 1 || listed[0].JobID != goodID || !listed[0].Eligible || len(listed[0].Components) != 5 {
		t.Fatalf("saved match list=%+v err=%v", listed, err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM user_job_matches WHERE user_id=$1`, userID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("persisted match count=%d err=%v", count, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE event_type='match.created' AND payload->>'user_id'=$1 AND payload->>'job_id'=$2`, userID, goodID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("match event count=%d err=%v", count, err)
	}
	eventID := uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO outbox_events(id,event_type,aggregate_type,aggregate_id,payload) VALUES($1,'profile.changed','user',$2,jsonb_build_object('user_id',$2::text))`, eventID, userID); err != nil {
		t.Fatal(err)
	}
	worker := NewOutboxWorker(pool, func(ctx context.Context, id user.UserID) error {
		_, err := service.Refresh(ctx, id)
		return err
	}, func(ctx context.Context, id string) error {
		return service.RefreshJob(ctx, id)
	})
	found, err := worker.ProcessEvent(ctx, eventID)
	if err != nil || !found {
		t.Fatalf("process profile change found=%v err=%v", found, err)
	}
	var processed bool
	if err := pool.QueryRow(ctx, `SELECT processed_at IS NOT NULL FROM outbox_events WHERE id=$1`, eventID).Scan(&processed); err != nil || !processed {
		t.Fatalf("event acknowledgement processed=%v err=%v", processed, err)
	}
	found, err = worker.ProcessEvent(ctx, eventID)
	if err != nil || found {
		t.Fatalf("duplicate event processing found=%v err=%v", found, err)
	}
	if err := NewStore(pool).SaveMatches(ctx, user.UserID(userID), []engine.Result{}); err != nil {
		t.Fatal(err)
	}
	jobEventID := uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO outbox_events(id,event_type,aggregate_type,aggregate_id,payload) VALUES($1,'job.created','job',$2,jsonb_build_object('job_id',$2::text))`, jobEventID, goodID); err != nil {
		t.Fatal(err)
	}
	found, err = worker.ProcessEvent(ctx, jobEventID)
	if err != nil || !found {
		t.Fatalf("process new job event found=%v err=%v", found, err)
	}
	listed, err = service.List(ctx, user.UserID(userID), 100)
	if err != nil || len(listed) != 1 || listed[0].JobID != goodID {
		t.Fatalf("job event did not fan out to candidate: %+v err=%v", listed, err)
	}
	closedEventID := uuid.NewString()
	if _, err := pool.Exec(ctx, `UPDATE jobs SET status='closed' WHERE id=$1`, goodID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO outbox_events(id,event_type,aggregate_type,aggregate_id,payload) VALUES($1,'job.closed','job',$2,jsonb_build_object('job_id',$2::text))`, closedEventID, goodID); err != nil {
		t.Fatal(err)
	}
	found, err = worker.ProcessEvent(ctx, closedEventID)
	if err != nil || !found {
		t.Fatalf("process closed job event found=%v err=%v", found, err)
	}
	listed, err = service.List(ctx, user.UserID(userID), 100)
	if err != nil || len(listed) != 0 {
		t.Fatalf("closed job match was not removed: %+v err=%v", listed, err)
	}
	retryEventID := uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO outbox_events(id,event_type,aggregate_type,aggregate_id,payload) VALUES($1,'profile.changed','user',$2,jsonb_build_object('user_id',$2::text))`, retryEventID, userID); err != nil {
		t.Fatal(err)
	}
	failingWorker := NewOutboxWorker(pool, func(context.Context, user.UserID) error { return errors.New("temporary matching failure") }, nil)
	found, err = failingWorker.ProcessEvent(ctx, retryEventID)
	if !found || err == nil {
		t.Fatalf("failed refresh found=%v err=%v", found, err)
	}
	var attempts int
	if err := pool.QueryRow(ctx, `SELECT attempts FROM outbox_events WHERE id=$1`, retryEventID).Scan(&attempts); err != nil || attempts != 1 {
		t.Fatalf("retry attempts=%d err=%v", attempts, err)
	}
	if err := NewStore(pool).SaveMatches(ctx, user.UserID(userID), []engine.Result{}); err != nil {
		t.Fatal(err)
	}
	listed, err = service.List(ctx, user.UserID(userID), 100)
	if err != nil || len(listed) != 0 {
		t.Fatalf("stale match was not cleared: %+v err=%v", listed, err)
	}
}
