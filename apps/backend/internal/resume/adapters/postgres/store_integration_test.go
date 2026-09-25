package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/resume/domain"
	user "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestResumeStorePostgresIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	owner, stranger, id := uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, uid := range []string{owner, stranger} {
		if _, err := pool.Exec(ctx, `INSERT INTO users (id,email,password_hash,email_verified) VALUES ($1,$2,'integration-hash',true)`, uid, uid+"@resume.example"); err != nil {
			t.Fatal(err)
		}
	}
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM outbox_events WHERE aggregate_id=$1`, id)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id IN ($1,$2)`, owner, stranger)
	}()
	store := NewStore(pool)
	resume := domain.Resume{ID: domain.ID(id), UserID: user.UserID(owner), FileName: "cv.pdf", ContentType: "application/pdf", Size: 100, Status: domain.StatusPendingUpload, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	key := "users/" + owner + "/resumes/" + id + "/original.pdf"
	if err := store.Create(ctx, resume, key); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.ByID(ctx, user.UserID(stranger), domain.ID(id)); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("foreign resume error = %v", err)
	}
	if err := store.MarkUploaded(ctx, user.UserID(owner), domain.ID(id)); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkUploaded(ctx, user.UserID(owner), domain.ID(id)); err != nil {
		t.Fatal(err)
	}
	var events int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE event_type='resume.uploaded' AND aggregate_id=$1`, id).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if events != 1 {
		t.Fatalf("resume.uploaded events = %d, want 1", events)
	}
	got, gotKey, err := store.ByID(ctx, user.UserID(owner), domain.ID(id))
	if err != nil || got.Status != domain.StatusUploaded || gotKey != key {
		t.Fatalf("resume = %+v key=%q err=%v", got, gotKey, err)
	}
	items, err := store.List(ctx, user.UserID(owner))
	if err != nil || len(items) != 1 {
		t.Fatalf("list = %v, %v", items, err)
	}
	if err := store.MarkDeleted(ctx, user.UserID(owner), domain.ID(id)); err != nil {
		t.Fatal(err)
	}
	items, err = store.List(ctx, user.UserID(owner))
	if err != nil || len(items) != 0 {
		t.Fatalf("list after delete = %v, %v", items, err)
	}
}

func TestResumeAnalysisAndSuggestionReviewIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	userID, resumeID := uuid.NewString(), uuid.NewString()
	var skillID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM skills WHERE normalized_name='go'`).Scan(&skillID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO users (id,email,password_hash,email_verified) VALUES ($1,$2,'integration-hash',true)`, userID, userID+"@resume-analysis.example"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM outbox_events WHERE aggregate_id=$1 OR aggregate_id=$2`, userID, resumeID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, userID)
	}()
	store := NewStore(pool)
	now := time.Now().UTC()
	resume := domain.Resume{ID: domain.ID(resumeID), UserID: user.UserID(userID), FileName: "cv.pdf", ContentType: "application/pdf", Size: 100, Status: domain.StatusUploaded, CreatedAt: now, UpdatedAt: now}
	if err := store.Create(ctx, resume, "users/"+userID+"/resumes/"+resumeID+"/original.pdf"); err != nil {
		t.Fatal(err)
	}
	job, found, err := store.Claim(ctx)
	if err != nil || !found || job.Resume.ID != domain.ID(resumeID) {
		t.Fatalf("claim = %+v found=%v err=%v", job, found, err)
	}
	parsed := domain.ParsedResume{ResumeID: domain.ID(resumeID), Text: "Senior Go developer with 5 years experience", Skills: []domain.DetectedSkill{{Name: "Go", Confidence: .92}}, Positions: []domain.DetectedPosition{{Title: "Senior Go developer", Confidence: .62}}, TotalExperienceMonths: 60}
	if err := store.SaveAnalysis(ctx, job, parsed); err != nil {
		t.Fatal(err)
	}
	loaded, suggestions, err := store.Analysis(ctx, user.UserID(userID), domain.ID(resumeID))
	if err != nil || loaded.Text != parsed.Text || len(suggestions) != 2 {
		t.Fatalf("analysis=%+v suggestions=%+v err=%v", loaded, suggestions, err)
	}
	var skillSuggestion, positionSuggestion string
	for _, item := range suggestions {
		if item.Kind == "skill" {
			skillSuggestion = item.ID
		} else {
			positionSuggestion = item.ID
		}
	}
	if err := store.ReviewSuggestion(ctx, user.UserID(userID), domain.ID(resumeID), skillSuggestion, true); err != nil {
		t.Fatal(err)
	}
	if err := store.ReviewSuggestion(ctx, user.UserID(userID), domain.ID(resumeID), positionSuggestion, false); err != nil {
		t.Fatal(err)
	}
	if err := store.ReviewSuggestion(ctx, user.UserID(userID), domain.ID(resumeID), skillSuggestion, true); !errors.Is(err, domain.ErrSuggestionReviewed) {
		t.Fatalf("review replay error = %v", err)
	}
	var skills, positions, profileEvents int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM user_skills WHERE user_id=$1 AND skill_id=$2 AND source='resume' AND confirmed`, userID, skillID).Scan(&skills); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM user_positions WHERE user_id=$1`, userID).Scan(&positions); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE event_type='profile.changed' AND aggregate_id=$1`, userID).Scan(&profileEvents); err != nil {
		t.Fatal(err)
	}
	if skills != 1 || positions != 0 || profileEvents != 1 {
		t.Fatalf("accepted skills=%d positions=%d profile events=%d", skills, positions, profileEvents)
	}
}
