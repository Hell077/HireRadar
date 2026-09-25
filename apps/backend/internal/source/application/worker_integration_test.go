package application

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/source/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type fixtureFetcher struct {
	result domain.FetchResult
	err    error
}

func (f fixtureFetcher) Fetch(context.Context, domain.Source) (domain.FetchResult, error) {
	return f.result, f.err
}

func TestSourceWorkerPersistsReplayableSnapshotsAndIsolatesFailures(t *testing.T) {
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
	sourceID := "integration-" + uuid.NewString()
	badID := "integration-" + uuid.NewString()
	for _, id := range []string{sourceID, badID} {
		if _, err := pool.Exec(ctx, `INSERT INTO sources(id,name,source_type,company_name,config,enabled) VALUES($1,$1,'greenhouse','Test','{"board":"test"}',false)`, id); err != nil {
			t.Fatal(err)
		}
	}
	defer pool.Exec(ctx, "DELETE FROM sources WHERE id=ANY($1)", []string{sourceID, badID})
	job := domain.ExternalJob{ExternalID: "posting-1", CompanyName: "Test", Title: "Engineer", Description: "Initial", Location: "Remote", ApplyURL: "https://jobs.example/1", Raw: json.RawMessage(`{"id":"posting-1","description":"Initial"}`)}
	w := NewWorker(pool, fixtureFetcher{result: domain.FetchResult{Jobs: []domain.ExternalJob{job}, NextCursor: json.RawMessage(`{"page":2}`)}}, 1)
	if err := w.syncSource(ctx, domain.Source{ID: sourceID, SyncIntervalSecond: 900}); err != nil {
		t.Fatal(err)
	}
	job.Description = "Updated"
	job.Raw = json.RawMessage(`{"id":"posting-1","description":"Updated"}`)
	w.fetcher = fixtureFetcher{result: domain.FetchResult{Jobs: []domain.ExternalJob{job}}}
	if err := w.syncSource(ctx, domain.Source{ID: sourceID, SyncIntervalSecond: 900}); err != nil {
		t.Fatal(err)
	}
	var rawCount, newCount, updatedCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM raw_jobs WHERE source_id=$1", sourceID).Scan(&rawCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT new_count,updated_count FROM source_sync_runs WHERE source_id=$1 ORDER BY started_at DESC LIMIT 1", sourceID).Scan(&newCount, &updatedCount); err != nil {
		t.Fatal(err)
	}
	if rawCount != 2 || newCount != 0 || updatedCount != 1 {
		t.Fatalf("raw=%d latest new=%d updated=%d", rawCount, newCount, updatedCount)
	}
	var payload string
	if err := pool.QueryRow(ctx, "SELECT payload::text FROM raw_jobs WHERE source_id=$1 ORDER BY fetched_at DESC LIMIT 1", sourceID).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	if !json.Valid([]byte(payload)) {
		t.Fatalf("stored payload is not replayable JSON: %q", payload)
	}
	bad := NewWorker(pool, fixtureFetcher{err: context.DeadlineExceeded}, 1)
	if err := bad.syncSource(ctx, domain.Source{ID: badID}); err == nil {
		t.Fatal("expected source fetch failure")
	}
	var failed int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM source_sync_runs WHERE source_id=$1 AND status='failed'", badID).Scan(&failed); err != nil {
		t.Fatal(err)
	}
	var retryAt time.Time
	if err := pool.QueryRow(ctx, "SELECT next_sync_at FROM sources WHERE id=$1", badID).Scan(&retryAt); err != nil {
		t.Fatal(err)
	}
	if failed != 1 || !retryAt.After(time.Now()) {
		t.Fatalf("failed runs=%d retry_at=%v", failed, retryAt)
	}
}
