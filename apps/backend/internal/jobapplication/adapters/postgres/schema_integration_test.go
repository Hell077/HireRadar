package postgres

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/jobapplication/domain"
	userdomain "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestApplicationSchemaConstraints(t *testing.T) {
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

	userID, companyID, jobID, appID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	defer pool.Exec(ctx, `DELETE FROM outbox_events WHERE aggregate_id=$1`, appID)
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,email,password_hash) VALUES($1,$2,'test')`, userID, userID+"@application.example"); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, userID)
	if _, err := pool.Exec(ctx, `INSERT INTO companies(id,name,normalized_name) VALUES($1,'Application Schema Test',$2)`, companyID, companyID); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `DELETE FROM companies WHERE id=$1`, companyID)
	if _, err := pool.Exec(ctx, `INSERT INTO jobs(id,company_id,title,normalized_title,remote_policy,eligibility,apply_url,fingerprint,status) VALUES($1,$2,'Engineer','engineer','unknown','unknown','https://example.test/apply',$3,'active')`, jobID, companyID, make([]byte, 32)); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `DELETE FROM jobs WHERE id=$1`, jobID)

	if _, err := pool.Exec(ctx, `INSERT INTO user_job_matches(user_id,job_id,score,confidence,candidate_version,job_version) SELECT $1,$2,90,80,0,match_version FROM jobs WHERE id=$2`, userID, jobID); err != nil {
		t.Fatal(err)
	}
	store := NewStore(pool)
	requested := domain.Application{ID: domain.ID(appID), UserID: userID, JobID: jobID, Provider: "unknown", Status: domain.StatusRequested, RequestedAt: time.Now().UTC()}
	created, err := store.Request(ctx, requested)
	if err != nil {
		t.Fatal(err)
	}
	var matchScore, matchConfidence int
	if err := pool.QueryRow(ctx, `SELECT match_score,match_confidence FROM job_applications WHERE id=$1`, appID).Scan(&matchScore, &matchConfidence); err != nil || matchScore != 90 || matchConfidence != 80 {
		t.Fatalf("application match attribution score=%d confidence=%d err=%v", matchScore, matchConfidence, err)
	}
	requested.ID = domain.ID(uuid.NewString())
	duplicate, err := store.Request(ctx, requested)
	if err != nil || duplicate.ID != created.ID {
		t.Fatalf("duplicate request returned %+v, err=%v", duplicate, err)
	}
	var eventCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE event_type='application.requested' AND aggregate_id=$1`, appID).Scan(&eventCount); err != nil || eventCount != 1 {
		t.Fatalf("request event count=%d err=%v", eventCount, err)
	}
	var statusEvents int
	var statusPayloadHasOnlyID bool
	if err := pool.QueryRow(ctx, `SELECT count(*),bool_and((SELECT count(*) FROM jsonb_object_keys(payload))=1) FROM outbox_events WHERE event_type='application.status.requested' AND aggregate_id=$1`, appID).Scan(&statusEvents, &statusPayloadHasOnlyID); err != nil || statusEvents != 1 || !statusPayloadHasOnlyID {
		t.Fatalf("status event count=%d id_only=%v err=%v", statusEvents, statusPayloadHasOnlyID, err)
	}
	form := domain.PreparedForm{Fields: []domain.Field{{Key: "first_name", Type: "input_text", Value: "Ada"}}}
	if err := store.SavePrepared(ctx, created.ID, form, nil); err != nil {
		t.Fatal(err)
	}
	loadedForm, err := store.Prepared(ctx, created.ID)
	if err != nil || len(loadedForm.Fields) != 1 || loadedForm.Fields[0].Value != "Ada" {
		t.Fatalf("prepared form=%+v err=%v", loadedForm, err)
	}
	attempt, begun, err := store.BeginAttempt(ctx, created.ID, "greenhouse")
	if err != nil || !begun || attempt != 1 {
		t.Fatalf("attempt=%d begun=%v err=%v", attempt, begun, err)
	}
	if err := store.MarkUncertain(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	var applicationStatus, attemptStatus string
	if err := pool.QueryRow(ctx, `SELECT status FROM job_applications WHERE id=$1`, appID).Scan(&applicationStatus); err != nil || applicationStatus != string(domain.StatusManualRequired) {
		t.Fatalf("uncertain application status=%q err=%v", applicationStatus, err)
	}
	if err := pool.QueryRow(ctx, `SELECT status FROM job_application_attempts WHERE application_id=$1 AND attempt_number=1`, appID).Scan(&attemptStatus); err != nil || attemptStatus != "uncertain" {
		t.Fatalf("uncertain attempt status=%q err=%v", attemptStatus, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE job_applications SET status='not_a_status' WHERE id=$1`, appID); err == nil {
		t.Fatal("invalid application status was accepted")
	}
	if _, err := pool.Exec(ctx, `INSERT INTO job_application_attempts(id,application_id,attempt_number,status,provider) VALUES($1,$2,1,'running','greenhouse')`, uuid.NewString(), appID); err == nil {
		t.Fatal("duplicate attempt number was accepted")
	}
	questionID := uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO job_application_questions(id,application_id,question,question_type,required,status) VALUES($1,$2,'Do you require sponsorship?','boolean',false,'unanswered')`, questionID, appID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE job_application_questions SET status='answered' WHERE id=$1`, questionID); err == nil {
		t.Fatal("answered question without an answer was accepted")
	}
	var questions int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM job_application_questions WHERE application_id=$1 AND status='unanswered'`, appID).Scan(&questions); err != nil || questions != 1 {
		t.Fatal(fmt.Errorf("unanswered question count=%d err=%v", questions, err))
	}
	if _, err := pool.Exec(ctx, `UPDATE job_applications SET status='needs_input' WHERE id=$1`, appID); err != nil {
		t.Fatal(err)
	}
	questionKey := "work_authorization"
	if err := store.SavePrepared(ctx, created.ID, domain.PreparedForm{Fields: []domain.Field{{Key: questionKey, Type: "select", Required: true}}}, []domain.Question{{ExternalKey: questionKey, Text: "Are you authorized to work?", Type: "select", Required: true, Options: []domain.Option{{Value: float64(1), Label: "Yes"}, {Value: float64(0), Label: "No"}}}}); err != nil {
		t.Fatal(err)
	}
	var answerQuestionID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM job_application_questions WHERE application_id=$1 AND external_key=$2`, appID, questionKey).Scan(&answerQuestionID); err != nil {
		t.Fatal(err)
	}
	if err := store.AnswerQuestion(ctx, userdomain.UserID(userID), answerQuestionID, "1"); err != nil {
		t.Fatal(err)
	}
	if err := store.AnswerQuestion(ctx, userdomain.UserID(userID), answerQuestionID, "1"); err != nil {
		t.Fatalf("idempotent answer failed: %v", err)
	}
	if err := store.SavePrepared(ctx, created.ID, domain.PreparedForm{Fields: []domain.Field{{Key: questionKey, Type: "select", Required: true}}}, []domain.Question{{ExternalKey: questionKey, Text: "Are you authorized to work?", Type: "select", Required: true, Options: []domain.Option{{Value: float64(1), Label: "Yes"}, {Value: float64(0), Label: "No"}}}}); err != nil {
		t.Fatal(err)
	}
	loadedForm, err = store.Prepared(ctx, created.ID)
	if err != nil || loadedForm.Fields[0].Value != float64(1) {
		t.Fatalf("explicit answer was not applied to prepared form: %+v, %v", loadedForm, err)
	}
	if err := pool.QueryRow(ctx, `SELECT status FROM job_applications WHERE id=$1`, appID).Scan(&applicationStatus); err != nil || applicationStatus != string(domain.StatusReady) {
		t.Fatalf("answered application status=%q err=%v", applicationStatus, err)
	}
	var answerEvents int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE event_type='application.input_provided' AND aggregate_id=$1`, appID).Scan(&answerEvents); err != nil || answerEvents != 1 {
		t.Fatalf("answer event count=%d err=%v", answerEvents, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE job_applications SET status='failed' WHERE id=$1`, appID); err != nil {
		t.Fatal(err)
	}
	if err := store.Retry(ctx, userdomain.UserID(userID), created.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.Retry(ctx, userdomain.UserID(userID), created.ID); err != nil {
		t.Fatalf("idempotent retry failed: %v", err)
	}
	var retryEvents int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE event_type='application.retry_requested' AND aggregate_id=$1`, appID).Scan(&retryEvents); err != nil || retryEvents != 1 {
		t.Fatalf("retry event count=%d err=%v", retryEvents, err)
	}
	if err := store.Cancel(ctx, userdomain.UserID(userID), created.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.Cancel(ctx, userdomain.UserID(userID), created.ID); err != nil {
		t.Fatalf("idempotent cancellation failed: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT status FROM job_applications WHERE id=$1`, appID).Scan(&applicationStatus); err != nil || applicationStatus != string(domain.StatusCancelled) {
		t.Fatalf("cancelled application status=%q err=%v", applicationStatus, err)
	}
}
