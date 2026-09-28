package postgres

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/Hell077/HireRadar/apps/backend/internal/discovery/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCandidatesMergeByOfficialDomainAndKeepAllProvenance(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()
	store := NewStore(pool)
	discoverySourceIDs := []string{"itest-a-" + uuid.NewString(), "itest-b-" + uuid.NewString()}
	for _, id := range discoverySourceIDs {
		if _, err := pool.Exec(ctx, `INSERT INTO discovery_sources(id,name,repo_owner,repo_name,parser_key) VALUES($1,$1,'owner',$1,'remoteintech')`, id); err != nil {
			t.Fatal(err)
		}
	}
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM source_candidates WHERE id IN (SELECT candidate_id FROM candidate_provenance WHERE discovery_source_id=ANY($1))`, discoverySourceIDs)
		_, _ = pool.Exec(ctx, `DELETE FROM discovery_sources WHERE id=ANY($1)`, discoverySourceIDs)
	}()
	target := domain.DiscoveredTarget{Kind: domain.CompanyTarget, ExternalKey: "company-a", Name: "Example Co", OfficialDomain: "example.test", WebsiteURL: "https://example.test", DiscoveryFields: json.RawMessage(`{"region":"worldwide"}`)}
	first, err := store.SaveTargets(ctx, discoverySourceIDs[0], []domain.DiscoveredTarget{target})
	if err != nil {
		t.Fatal(err)
	}
	alias := target
	alias.ExternalKey, alias.Name = "company-alias", "Example Incorporated"
	second, err := store.SaveTargets(ctx, discoverySourceIDs[0], []domain.DiscoveredTarget{alias})
	if err != nil {
		t.Fatal(err)
	}
	otherCatalog := target
	otherCatalog.ExternalKey = "example"
	third, err := store.SaveTargets(ctx, discoverySourceIDs[1], []domain.DiscoveredTarget{otherCatalog})
	if err != nil {
		t.Fatal(err)
	}
	if first.CandidatesCreated != 1 || second.CandidatesReused != 1 || third.CandidatesReused != 1 {
		t.Fatalf("candidate stats first=%+v alias=%+v second catalog=%+v", first, second, third)
	}
	var candidates, provenance int
	if err := pool.QueryRow(ctx, `SELECT count(DISTINCT c.id),count(*) FROM source_candidates c JOIN candidate_provenance p ON p.candidate_id=c.id WHERE c.official_domain='example.test'`).Scan(&candidates, &provenance); err != nil {
		t.Fatal(err)
	}
	if candidates != 1 || provenance != 3 {
		t.Fatalf("deduplication candidates=%d provenance=%d", candidates, provenance)
	}
}

func TestVerifiedSourceCreationIsIdempotentAndPreservesOperatorSettings(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()
	store := NewStore(pool)
	discoverySourceID := "itest-source-" + uuid.NewString()
	board := "discovery-test-" + uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO discovery_sources(id,name,repo_owner,repo_name,parser_key) VALUES($1,$1,'owner',$1,'remoteintech')`, discoverySourceID); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM sources WHERE source_type='greenhouse' AND config->>'board'=$1`, board)
		_, _ = pool.Exec(ctx, `DELETE FROM source_candidates WHERE id IN (SELECT candidate_id FROM candidate_provenance WHERE discovery_source_id=$1)`, discoverySourceID)
		_, _ = pool.Exec(ctx, `DELETE FROM discovery_sources WHERE id=$1`, discoverySourceID)
	}()
	targets := []domain.DiscoveredTarget{
		{Kind: domain.CompanyTarget, ExternalKey: "first", Name: "Candidate First", OfficialDomain: "first.test", WebsiteURL: "https://first.test"},
		{Kind: domain.CompanyTarget, ExternalKey: "second", Name: "Candidate Second", OfficialDomain: "second.test", WebsiteURL: "https://second.test"},
	}
	if _, err := store.SaveTargets(ctx, discoverySourceID, targets); err != nil {
		t.Fatal(err)
	}
	rows, err := pool.Query(ctx, `SELECT candidate_id::text FROM candidate_provenance WHERE discovery_source_id=$1 ORDER BY external_key`, discoverySourceID)
	if err != nil {
		t.Fatal(err)
	}
	var candidateIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		candidateIDs = append(candidateIDs, id)
	}
	rows.Close()
	if len(candidateIDs) != 2 {
		t.Fatalf("candidate IDs=%v", candidateIDs)
	}
	for _, id := range candidateIDs {
		if _, err := pool.Exec(ctx, `UPDATE source_candidates SET status='verified',detected_provider='greenhouse',provider_key=$2,provider_url='https://boards.greenhouse.io/'||$2,provider_confidence=.99 WHERE id=$1`, id, board); err != nil {
			t.Fatal(err)
		}
	}
	createdID, created, err := store.CreateOrLinkSource(ctx, candidateIDs[0])
	if err != nil || !created {
		t.Fatalf("first register source=%s created=%v err=%v", createdID, created, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE sources SET enabled=false,priority=17,sync_interval_seconds=7200 WHERE id=$1`, createdID); err != nil {
		t.Fatal(err)
	}
	linkedID, createdAgain, err := store.CreateOrLinkSource(ctx, candidateIDs[1])
	if err != nil || createdAgain || linkedID != createdID {
		t.Fatalf("second register source=%s created=%v err=%v", linkedID, createdAgain, err)
	}
	var enabled bool
	var priority, interval int
	if err := pool.QueryRow(ctx, `SELECT enabled,priority,sync_interval_seconds FROM sources WHERE id=$1`, createdID).Scan(&enabled, &priority, &interval); err != nil {
		t.Fatal(err)
	}
	if enabled || priority != 17 || interval != 7200 {
		t.Fatalf("operator settings changed: enabled=%v priority=%d interval=%d", enabled, priority, interval)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM sources WHERE source_type='greenhouse' AND config->>'board'=$1`, board).Scan(&count); err != nil || count != 1 {
		t.Fatalf("sources for provider key=%d err=%v", count, err)
	}
}

func TestVerifiedGitHubIssuesSourceUsesRepositoryIdentity(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()
	store := NewStore(pool)
	discoverySourceID := "itest-github-" + uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO discovery_sources(id,name,repo_owner,repo_name,parser_key) VALUES($1,$1,'owner',$1,'github_issue_boards')`, discoverySourceID); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM sources WHERE source_type='github' AND config->>'owner'='pythonjobs' AND config->>'repo'='jobs'`)
		_, _ = pool.Exec(ctx, `DELETE FROM source_candidates WHERE id IN (SELECT candidate_id FROM candidate_provenance WHERE discovery_source_id=$1)`, discoverySourceID)
		_, _ = pool.Exec(ctx, `DELETE FROM discovery_sources WHERE id=$1`, discoverySourceID)
	}()
	if _, err := store.SaveTargets(ctx, discoverySourceID, []domain.DiscoveredTarget{{Kind: domain.GitHubJobsTarget, ExternalKey: "pythonjobs/jobs", Name: "pythonjobs/jobs", WebsiteURL: "https://github.com/pythonjobs/jobs", OfficialDomain: "github.com/pythonjobs/jobs"}}); err != nil {
		t.Fatal(err)
	}
	var candidateID string
	if err := pool.QueryRow(ctx, `SELECT candidate_id::text FROM candidate_provenance WHERE discovery_source_id=$1`, discoverySourceID).Scan(&candidateID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE source_candidates SET status='verified',detected_provider='github',provider_key='pythonjobs/jobs',provider_url='https://github.com/pythonjobs/jobs/issues',provider_confidence=.99 WHERE id=$1`, candidateID); err != nil {
		t.Fatal(err)
	}
	sourceID, created, err := store.CreateOrLinkSource(ctx, candidateID)
	if err != nil || !created {
		t.Fatalf("source=%s created=%v err=%v", sourceID, created, err)
	}
	var owner, repo string
	if err := pool.QueryRow(ctx, `SELECT config->>'owner',config->>'repo' FROM sources WHERE id=$1`, sourceID).Scan(&owner, &repo); err != nil || owner != "pythonjobs" || repo != "jobs" {
		t.Fatalf("GitHub identity owner=%q repo=%q err=%v", owner, repo, err)
	}
}

func TestGitHubRepositoryWithoutJobLikeIssuesStaysInManualReview(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()
	store := NewStore(pool)
	discoverySourceID := "itest-review-" + uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO discovery_sources(id,name,repo_owner,repo_name,parser_key) VALUES($1,$1,'owner',$1,'github_issue_boards')`, discoverySourceID); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM source_candidates WHERE id IN (SELECT candidate_id FROM candidate_provenance WHERE discovery_source_id=$1)`, discoverySourceID)
		_, _ = pool.Exec(ctx, `DELETE FROM discovery_sources WHERE id=$1`, discoverySourceID)
	}()
	if _, err := store.SaveTargets(ctx, discoverySourceID, []domain.DiscoveredTarget{{Kind: domain.GitHubJobsTarget, ExternalKey: "owner/bugs", Name: "owner/bugs", WebsiteURL: "https://github.com/owner/bugs", OfficialDomain: "github.com/owner/bugs"}}); err != nil {
		t.Fatal(err)
	}
	var candidateID string
	if err := pool.QueryRow(ctx, `SELECT candidate_id::text FROM candidate_provenance WHERE discovery_source_id=$1`, discoverySourceID).Scan(&candidateID); err != nil {
		t.Fatal(err)
	}
	if err := store.CandidateReview(ctx, candidateID, domain.DetectedProvider{Type: "github", Key: "owner/bugs"}, "sample issues are not job postings"); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM source_candidates WHERE id=$1`, candidateID).Scan(&status); err != nil || status != "manual_review" {
		t.Fatalf("candidate status=%q err=%v", status, err)
	}
}

func integrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to a migrated PostgreSQL test database")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}
