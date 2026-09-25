package postgres

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSourceCatalogReportsLatestRunWithoutConfigOrError(t *testing.T) {
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
	id := "catalog-" + uuid.NewString()
	runID := uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO sources(id,name,source_type,company_name,config) VALUES($1,'Catalog test','greenhouse','Catalog test','{"board":"secret-board"}')`, id); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, "DELETE FROM sources WHERE id=$1", id)
	if _, err := pool.Exec(ctx, `INSERT INTO source_sync_runs(id,source_id,status,fetched_count,new_count,updated_count,error_message) VALUES($1,$2,'failed',7,2,1,'token=secret-value')`, runID, id); err != nil {
		t.Fatal(err)
	}
	items, err := NewCatalog(pool).ListEnabled(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, item := range items {
		if item.ID == id {
			found = true
			if item.LastSyncStatus != "failed" || item.LastFetchedJobs != 7 || item.LastNewJobs != 2 || item.LastUpdatedJobs != 1 || item.LastAttemptAt == nil || item.LastSyncAt != nil {
				t.Fatalf("source run health not mapped: %+v", item)
			}
			encoded, _ := json.Marshal(item)
			if strings.Contains(string(encoded), "secret-board") || strings.Contains(string(encoded), "secret-value") {
				t.Fatalf("source API leaked config or error: %s", encoded)
			}
		}
	}
	if !found {
		t.Fatalf("source %s missing from enabled catalog", id)
	}
}
