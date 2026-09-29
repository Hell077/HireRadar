package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestClaimLeaseCanBeReclaimedAfterWorkerCrash(t *testing.T) {
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
	eventID := uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO outbox_events(id,event_type,aggregate_type,aggregate_id,payload) VALUES($1,'test.crash','test',$2,'{}')`, eventID, eventID); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `DELETE FROM outbox_events WHERE id=$1`, eventID)

	first, err := store.Claim(ctx, "worker-one", []string{"test.crash"}, time.Minute, eventID)
	if err != nil || first == nil || first.Attempts != 1 {
		t.Fatalf("first claim=%+v err=%v", first, err)
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM outbox_events WHERE id=$1`, eventID).Scan(&status); err != nil || status != "processing" {
		t.Fatalf("claimed status=%q err=%v", status, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE outbox_events SET locked_until=now()-interval '1 second' WHERE id=$1`, eventID); err != nil {
		t.Fatal(err)
	}
	second, err := store.Claim(ctx, "worker-two", []string{"test.crash"}, time.Minute, eventID)
	if err != nil || second == nil || second.Attempts != 2 {
		t.Fatalf("reclaimed claim=%+v err=%v", second, err)
	}
	if err := store.Complete(ctx, eventID, "worker-one"); err == nil {
		t.Fatal("expired worker unexpectedly completed the event")
	}
	if err := store.Complete(ctx, eventID, "worker-two"); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT status FROM outbox_events WHERE id=$1`, eventID).Scan(&status); err != nil || status != "processed" {
		t.Fatalf("completed status=%q err=%v", status, err)
	}
}
