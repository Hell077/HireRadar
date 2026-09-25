package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	jobdomain "github.com/Hell077/HireRadar/apps/backend/internal/job/domain"
	"github.com/Hell077/HireRadar/apps/backend/internal/job/normalization"
	sourcedomain "github.com/Hell077/HireRadar/apps/backend/internal/source/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestJobCatalogCursorAndFilters(t *testing.T) {
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
	sourceID := "jobs-" + uuid.NewString()
	company := "Catalog Test " + uuid.NewString()
	urlBase := "https://catalog.example/" + uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO sources(id,name,source_type,company_name,enabled) VALUES($1,$1,'greenhouse',$2,false)`, sourceID, company); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM jobs WHERE company_id IN(SELECT id FROM companies WHERE normalized_name=$1)`, normalization.CompanyKey(company))
		_, _ = pool.Exec(ctx, `DELETE FROM companies WHERE normalized_name=$1`, normalization.CompanyKey(company))
		_, _ = pool.Exec(ctx, `DELETE FROM sources WHERE id=$1`, sourceID)
	}()
	runID := uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO source_sync_runs(id,source_id,status) VALUES($1,$2,'running')`, runID, sourceID); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	ingestor := NewIngestor()
	for n, location := range []string{"Remote — Kazakhstan", "Remote — US only"} {
		external := sourcedomain.ExternalJob{ExternalID: fmt.Sprintf("%d", n), CompanyName: company, Title: fmt.Sprintf("Engineer %d", n), Description: "Uses Go for backend engineering.", Location: location, ApplyURL: fmt.Sprintf("%s/%d", urlBase, n)}
		vocabulary, err := ingestor.LoadSkillVocabulary(ctx, tx)
		if err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		if _, _, err := ingestor.Save(ctx, tx, sourcedomain.Source{ID: sourceID, CompanyName: company, Priority: 10}, runID, external, vocabulary); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	catalog := NewCatalog(pool)
	first, err := catalog.List(ctx, ListQuery{Limit: 1, SourceID: sourceID})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 1 || first.NextCursor == "" {
		t.Fatalf("unexpected first page: %+v", first)
	}
	second, err := catalog.List(ctx, ListQuery{Limit: 1, Cursor: first.NextCursor, SourceID: sourceID})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Items) != 1 || second.Items[0].ID == first.Items[0].ID || second.NextCursor != "" {
		t.Fatalf("unexpected second page: %+v", second)
	}
	filtered, err := catalog.List(ctx, ListQuery{Limit: 1, Country: "KZ", SourceID: sourceID, Eligibility: string(jobdomain.Eligible)})
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered.Items) != 1 || filtered.Items[0].Eligibility != jobdomain.Eligible || filtered.Items[0].Location != "Remote — Kazakhstan" {
		t.Fatalf("country filter did not isolate Kazakhstan job: %+v", filtered)
	}
	if len(filtered.Items[0].Skills) == 0 || filtered.Items[0].Skills[0].Name != "Go" {
		t.Fatalf("job skills were not extracted from the catalog: %+v", filtered.Items[0].Skills)
	}
	if got, err := catalog.Get(ctx, filtered.Items[0].ID); err != nil || len(got.Skills) == 0 {
		t.Fatalf("get normalized job skills = %+v, %v", got, err)
	}
	if _, err := catalog.List(ctx, ListQuery{Cursor: "broken", Limit: 1}); !errors.Is(err, jobdomain.ErrInvalidCursor) {
		t.Fatalf("invalid cursor error = %v", err)
	}
}
