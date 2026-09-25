package application

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/job/normalization"
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
	testCompany := "Test " + uuid.NewString()
	testTitle := "Engineer " + uuid.NewString()
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM outbox_events WHERE aggregate_id IN (SELECT id::text FROM jobs WHERE id IN (SELECT job_id FROM job_sources WHERE source_id=ANY($1)))`, []string{sourceID, badID})
		_, _ = pool.Exec(ctx, `DELETE FROM jobs WHERE id IN (SELECT job_id FROM job_sources WHERE source_id=ANY($1))`, []string{sourceID, badID})
		_, _ = pool.Exec(ctx, `DELETE FROM sources WHERE id=ANY($1)`, []string{sourceID, badID})
		_, _ = pool.Exec(ctx, `DELETE FROM companies WHERE normalized_name=$1`, normalization.CompanyKey(testCompany))
	}()
	job := domain.ExternalJob{ExternalID: "posting-1", CompanyName: testCompany, Title: testTitle, Description: "Initial", Location: "Remote", ApplyURL: "https://jobs.example/" + uuid.NewString(), Raw: json.RawMessage(`{"id":"posting-1","description":"Initial"}`)}
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

func TestCatalogDeduplicatesSourcesAndDelaysClosure(t *testing.T) {
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
	primaryID, secondaryID := "catalog-"+uuid.NewString(), "catalog-"+uuid.NewString()
	for _, id := range []string{primaryID, secondaryID} {
		if _, err := pool.Exec(ctx, `INSERT INTO sources(id,name,source_type,company_name,config,enabled,priority) VALUES($1,$1,'greenhouse','Acme','{"board":"acme"}',false,100)`, id); err != nil {
			t.Fatal(err)
		}
	}
	title := "Senior Go Engineer " + uuid.NewString()
	companyName := "Acme " + uuid.NewString()
	jobURL := "https://jobs.example/" + uuid.NewString() + "/role"
	defer func() {
		ids := []string{primaryID, secondaryID}
		_, _ = pool.Exec(ctx, `DELETE FROM outbox_events WHERE aggregate_id IN (SELECT id::text FROM jobs WHERE id IN (SELECT job_id FROM job_sources WHERE source_id=ANY($1)))`, ids)
		_, _ = pool.Exec(ctx, `DELETE FROM jobs WHERE id IN (SELECT job_id FROM job_sources WHERE source_id=ANY($1))`, ids)
		_, _ = pool.Exec(ctx, `DELETE FROM sources WHERE id=ANY($1)`, ids)
		_, _ = pool.Exec(ctx, `DELETE FROM companies WHERE normalized_name=$1`, normalization.CompanyKey(companyName))
	}()
	primary := domain.Source{ID: primaryID, Name: "Acme careers", CompanyName: companyName + " Inc.", Priority: 10}
	secondary := domain.Source{ID: secondaryID, Name: "Board mirror", CompanyName: companyName + ", LLC", Priority: 50}
	first := domain.ExternalJob{ExternalID: "posting-1", CompanyName: primary.CompanyName, Title: title, Description: "Build things", Location: "Worldwide remote", ApplyURL: jobURL + "?utm_source=board"}
	w := NewWorker(pool, fixtureFetcher{result: domain.FetchResult{Jobs: []domain.ExternalJob{first}}}, 1)
	if err := w.syncSource(ctx, primary); err != nil {
		t.Fatal(err)
	}
	second := first
	second.ExternalID = "posting-mirror"
	second.CompanyName = secondary.CompanyName
	second.ApplyURL = "https://mirror.example/role"
	second.Description = "Lower priority content"
	w.fetcher = fixtureFetcher{result: domain.FetchResult{Jobs: []domain.ExternalJob{second}}}
	if err := w.syncSource(ctx, secondary); err != nil {
		t.Fatal(err)
	}
	var jobs, references, companies int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM jobs WHERE title=$1", title).Scan(&jobs); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM job_sources WHERE source_id=ANY($1)", []string{primaryID, secondaryID}).Scan(&references); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM companies WHERE normalized_name=$1", normalization.CompanyKey(companyName)).Scan(&companies); err != nil {
		t.Fatal(err)
	}
	if jobs != 1 || references != 2 || companies != 1 {
		t.Fatalf("jobs=%d refs=%d companies=%d", jobs, references, companies)
	}
	var canonicalURL, description string
	if err := pool.QueryRow(ctx, "SELECT apply_url,description FROM jobs WHERE title=$1", title).Scan(&canonicalURL, &description); err != nil {
		t.Fatal(err)
	}
	if canonicalURL != jobURL+"?utm_source=board" || description != "Build things" {
		t.Fatalf("lower-priority mirror replaced official data: url=%q description=%q", canonicalURL, description)
	}
	// One successful absence does not close a job; the other source still lists it.
	w.fetcher = fixtureFetcher{result: domain.FetchResult{Jobs: []domain.ExternalJob{}}}
	for range 2 {
		if err := w.syncSource(ctx, primary); err != nil {
			t.Fatal(err)
		}
	}
	var status string
	if err := pool.QueryRow(ctx, "SELECT status FROM jobs WHERE title=$1", title).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "active" {
		t.Fatalf("job status after one source stops listing it = %q", status)
	}
	for range 2 {
		if err := w.syncSource(ctx, secondary); err != nil {
			t.Fatal(err)
		}
	}
	if err := pool.QueryRow(ctx, "SELECT status FROM jobs WHERE title=$1", title).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "closed" {
		t.Fatalf("job status after both sources miss twice = %q", status)
	}
	var events int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM outbox_events WHERE event_type='job.closed' AND aggregate_id=(SELECT id::text FROM jobs WHERE title=$1)", title).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if events != 1 {
		t.Fatalf("job.closed events = %d, want 1", events)
	}
}
