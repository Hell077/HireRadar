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
