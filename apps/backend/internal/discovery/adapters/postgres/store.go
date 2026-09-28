package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/discovery/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func (s *Store) ClaimDueSources(ctx context.Context, limit int) ([]domain.DiscoverySource, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `WITH due AS (
		SELECT id FROM discovery_sources WHERE enabled AND next_run_at<=now()
		ORDER BY next_run_at,id FOR UPDATE SKIP LOCKED LIMIT $1
	) UPDATE discovery_sources d SET next_run_at=now()+make_interval(secs=>d.interval_seconds),updated_at=now()
		FROM due WHERE d.id=due.id RETURNING d.id,d.name,d.repo_owner,d.repo_name,d.parser_key,coalesce(d.etag,''),coalesce(d.last_commit_sha,''),d.interval_seconds`, limit)
	if err != nil {
		return nil, fmt.Errorf("claim due discovery sources: %w", err)
	}
	defer rows.Close()
	sources := make([]domain.DiscoverySource, 0)
	for rows.Next() {
		var item domain.DiscoverySource
		if err := rows.Scan(&item.ID, &item.Name, &item.Owner, &item.Repo, &item.Parser, &item.ETag, &item.LastCommitSHA, &item.IntervalSeconds); err != nil {
			return nil, err
		}
		sources = append(sources, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return sources, nil
}

func (s *Store) StartRun(ctx context.Context, sourceID string) (string, error) {
	id := uuid.NewString()
	_, err := s.pool.Exec(ctx, `INSERT INTO discovery_runs(id,discovery_source_id,status) VALUES($1,$2,'running')`, id, sourceID)
	if err != nil {
		return "", fmt.Errorf("start discovery run: %w", err)
	}
	return id, nil
}

func (s *Store) SaveTargets(ctx context.Context, sourceID string, targets []domain.DiscoveredTarget) (domain.RunStats, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.RunStats{}, err
	}
	defer tx.Rollback(ctx)
	stats := domain.RunStats{TargetsSeen: len(targets)}
	for _, target := range targets {
		normalizedName := domain.NameKey(target.Name)
		if normalizedName == "" || strings.TrimSpace(target.Name) == "" {
			continue
		}
		lockKey := target.Kind.String() + ":name:" + normalizedName
		if target.OfficialDomain != "" {
			lockKey = target.Kind.String() + ":domain:" + target.OfficialDomain
		}
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, lockKey); err != nil {
			return stats, err
		}
		id, created, err := upsertCandidate(ctx, tx, target, normalizedName)
		if err != nil {
			return stats, err
		}
		if created {
			stats.CandidatesCreated++
		} else {
			stats.CandidatesReused++
		}
		metadata := target.DiscoveryFields
		if !json.Valid(metadata) {
			metadata = json.RawMessage(`{}`)
		}
		tag, err := tx.Exec(ctx, `INSERT INTO candidate_provenance(discovery_source_id,external_key,candidate_id,source_url,raw_metadata)
			VALUES($1,$2,$3,$4,$5::jsonb) ON CONFLICT(discovery_source_id,external_key) DO NOTHING`, sourceID, target.ExternalKey, id, target.WebsiteURL, metadata)
		if err != nil {
			return stats, fmt.Errorf("save candidate provenance %s/%s: %w", sourceID, target.ExternalKey, err)
		}
		stats.ProvenanceAdded += int(tag.RowsAffected())
		if tag.RowsAffected() == 0 {
			if _, err := tx.Exec(ctx, `UPDATE candidate_provenance SET candidate_id=$3,source_url=$4,raw_metadata=$5::jsonb,last_seen_at=now()
				WHERE discovery_source_id=$1 AND external_key=$2`, sourceID, target.ExternalKey, id, target.WebsiteURL, metadata); err != nil {
				return stats, fmt.Errorf("refresh candidate provenance %s/%s: %w", sourceID, target.ExternalKey, err)
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return stats, fmt.Errorf("commit discovery targets: %w", err)
	}
	return stats, nil
}

func upsertCandidate(ctx context.Context, tx pgx.Tx, target domain.DiscoveredTarget, normalizedName string) (string, bool, error) {
	var id string
	var existingDomain *string
	err := tx.QueryRow(ctx, `SELECT id::text,official_domain FROM source_candidates
		WHERE kind=$1 AND (($2<>'' AND official_domain=$2) OR (normalized_name=$3 AND (official_domain IS NULL OR $2='' OR official_domain=$2)))
		ORDER BY CASE WHEN $2<>'' AND official_domain=$2 THEN 0 ELSE 1 END,id LIMIT 1 FOR UPDATE`, target.Kind, target.OfficialDomain, normalizedName).Scan(&id, &existingDomain)
	if err == nil {
		_, err = tx.Exec(ctx, `UPDATE source_candidates SET name=CASE WHEN $2='' THEN name ELSE $2 END,
			official_domain=coalesce(official_domain,NULLIF($3,'')),website_url=coalesce(website_url,NULLIF($4,'')),careers_url=coalesce(careers_url,NULLIF($5,'')),
			regions=(SELECT coalesce(array_agg(DISTINCT x),ARRAY[]::text[]) FROM unnest(regions||coalesce($6::text[],ARRAY[]::text[])) x),
			technologies=(SELECT coalesce(array_agg(DISTINCT x),ARRAY[]::text[]) FROM unnest(technologies||coalesce($7::text[],ARRAY[]::text[])) x),
			discovery_metadata=discovery_metadata||$8::jsonb,last_seen_at=now(),updated_at=now() WHERE id=$1`, id, target.Name, target.OfficialDomain, target.WebsiteURL, target.CareersURL, target.Regions, target.Technologies, validJSON(target.DiscoveryFields))
		if err != nil {
			return "", false, fmt.Errorf("update discovered candidate: %w", err)
		}
		return id, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", false, err
	}
	id = uuid.NewString()
	_, err = tx.Exec(ctx, `INSERT INTO source_candidates(id,kind,name,normalized_name,official_domain,website_url,careers_url,regions,technologies,discovery_metadata)
		VALUES($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,''),NULLIF($7,''),coalesce($8::text[],ARRAY[]::text[]),coalesce($9::text[],ARRAY[]::text[]),$10::jsonb)`, id, target.Kind, target.Name, normalizedName, target.OfficialDomain, target.WebsiteURL, target.CareersURL, target.Regions, target.Technologies, validJSON(target.DiscoveryFields))
	if err != nil {
		return "", false, fmt.Errorf("insert discovered candidate: %w", err)
	}
	return id, true, nil
}

func validJSON(value json.RawMessage) string {
	if !json.Valid(value) {
		return `{}`
	}
	return string(value)
}

func (s *Store) FinishRun(ctx context.Context, source domain.DiscoverySource, runID string, snapshot domain.RepositorySnapshot, stats domain.RunStats, runErr error) error {
	status := "succeeded"
	var errorMessage any
	var retryAfter time.Duration
	if runErr != nil {
		status = "failed"
		errorMessage = runErr.Error()
		// A failed parse/persist must not advance the GitHub cursor. Otherwise
		// the next conditional request can return 304 and permanently skip targets.
		snapshot.ETag = ""
		snapshot.CommitSHA = ""
		var retry interface{ RetryDelay(time.Time) time.Duration }
		if errors.As(runErr, &retry) {
			retryAfter = retry.RetryDelay(time.Now())
		}
		if retryAfter < time.Minute {
			retryAfter = time.Minute
		}
		if retryAfter > 24*time.Hour {
			retryAfter = 24 * time.Hour
		}
	} else if snapshot.NotChanged {
		status = "not_modified"
	}
	seconds := int64(source.IntervalSeconds)
	if runErr != nil {
		seconds = int64(retryAfter.Seconds())
	}
	_, err := s.pool.Exec(ctx, `UPDATE discovery_sources SET etag=coalesce(NULLIF($2,''),etag),last_commit_sha=coalesce(NULLIF($3,''),last_commit_sha),
		last_run_at=now(),last_status=$4,last_error=$5,next_run_at=now()+make_interval(secs=>$6),updated_at=now() WHERE id=$1`, source.ID, snapshot.ETag, snapshot.CommitSHA, status, errorMessage, seconds)
	if err != nil {
		return fmt.Errorf("finish discovery source state: %w", err)
	}
	_, err = s.pool.Exec(ctx, `UPDATE discovery_runs SET status=$2,finished_at=now(),targets_seen=$3,candidates_created=$4,candidates_reused=$5,provenance_added=$6,error_message=$7 WHERE id=$1`, runID, status, stats.TargetsSeen, stats.CandidatesCreated, stats.CandidatesReused, stats.ProvenanceAdded, errorMessage)
	return err
}

func (s *Store) ClaimCandidates(ctx context.Context, limit int) ([]domain.Candidate, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `WITH due AS (
		SELECT id FROM source_candidates WHERE status IN ('discovered','temporarily_unavailable','resolving') AND next_check_at<=now()
		ORDER BY next_check_at,id FOR UPDATE SKIP LOCKED LIMIT $1
	) UPDATE source_candidates c SET status='resolving',next_check_at=now()+interval '15 minutes',updated_at=now()
	FROM due WHERE c.id=due.id RETURNING c.id::text,c.kind,c.name,c.normalized_name,coalesce(c.official_domain,''),coalesce(c.website_url,''),coalesce(c.careers_url,''),c.regions,c.technologies,c.discovery_metadata,c.status,coalesce(c.detected_provider,''),coalesce(c.provider_key,''),coalesce(c.provider_url,''),coalesce(c.provider_confidence,0)::double precision,c.provider_evidence,coalesce(c.source_id,''),c.next_check_at,coalesce(c.last_error,'')`, limit)
	if err != nil {
		return nil, fmt.Errorf("claim source candidates: %w", err)
	}
	defer rows.Close()
	items := make([]domain.Candidate, 0)
	for rows.Next() {
		var item domain.Candidate
		if err := rows.Scan(&item.ID, &item.Kind, &item.Name, &item.NormalizedName, &item.OfficialDomain, &item.WebsiteURL, &item.CareersURL, &item.Regions, &item.Technologies, &item.DiscoveryMetadata, &item.Status, &item.DetectedProvider, &item.ProviderKey, &item.ProviderURL, &item.ProviderConfidence, &item.ProviderEvidence, &item.SourceID, &item.NextCheckAt, &item.LastError); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) CandidateUnavailable(ctx context.Context, id string, cause error) error {
	message := cause.Error()
	if len(message) > 2000 {
		message = message[:2000]
	}
	_, err := s.pool.Exec(ctx, `UPDATE source_candidates SET status='temporarily_unavailable',last_error=$2,last_checked_at=now(),next_check_at=now()+interval '1 hour',updated_at=now() WHERE id=$1`, id, message)
	return err
}

func (s *Store) CandidateInvalid(ctx context.Context, id string, provider domain.DetectedProvider, cause error) error {
	message := cause.Error()
	if len(message) > 2000 {
		message = message[:2000]
	}
	evidence, _ := json.Marshal(provider.Evidence)
	_, err := s.pool.Exec(ctx, `UPDATE source_candidates SET status='invalid',detected_provider=NULLIF($2,''),provider_key=NULLIF($3,''),provider_url=NULLIF($4,''),provider_confidence=NULLIF($5,0),provider_evidence=$6::jsonb,last_error=$7,last_checked_at=now(),next_check_at=now()+interval '90 days',updated_at=now() WHERE id=$1`, id, provider.Type, provider.Key, provider.URL, provider.Confidence, evidence, message)
	return err
}

func (s *Store) CandidateReview(ctx context.Context, id string, provider domain.DetectedProvider, reason string) error {
	metadata, _ := json.Marshal(provider.Evidence)
	state := candidateState(provider.Type)
	if provider.Type == "github" && reason != "" {
		state = "manual_review"
	}
	_, err := s.pool.Exec(ctx, `UPDATE source_candidates SET status=$2,detected_provider=NULLIF($3,''),provider_key=NULLIF($4,''),provider_url=NULLIF($5,''),provider_confidence=NULLIF($6,0),provider_evidence=$7::jsonb,last_error=NULLIF($8,''),last_checked_at=now(),next_check_at=now()+interval '30 days',updated_at=now() WHERE id=$1`, id, state, provider.Type, provider.Key, provider.URL, provider.Confidence, metadata, reason)
	return err
}

func candidateState(provider string) string {
	if provider == "" {
		return "manual_review"
	}
	switch provider {
	case "greenhouse", "lever", "ashby":
		return "verified"
	case "github":
		return "verified"
	default:
		return "unsupported"
	}
}

func (s *Store) CreateOrLinkSource(ctx context.Context, candidateID string) (string, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", false, err
	}
	defer tx.Rollback(ctx)
	var item struct {
		Name, Provider, Key, URL string
	}
	if err := tx.QueryRow(ctx, `SELECT name,detected_provider,provider_key,provider_url FROM source_candidates WHERE id=$1 AND status='verified' FOR UPDATE`, candidateID).Scan(&item.Name, &item.Provider, &item.Key, &item.URL); err != nil {
		return "", false, err
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, item.Provider+":"+item.Key); err != nil {
		return "", false, err
	}
	var sourceID string
	if item.Provider == "github" {
		owner, repo, ok := strings.Cut(item.Key, "/")
		if !ok {
			return "", false, fmt.Errorf("invalid verified GitHub repository key %q", item.Key)
		}
		err = tx.QueryRow(ctx, `SELECT id FROM sources WHERE source_type='github' AND config->>'owner'=$1 AND config->>'repo'=$2 FOR UPDATE`, owner, repo).Scan(&sourceID)
	} else {
		err = tx.QueryRow(ctx, `SELECT id FROM sources WHERE source_type=$1 AND config->>'board'=$2 FOR UPDATE`, item.Provider, item.Key).Scan(&sourceID)
	}
	created := false
	if errors.Is(err, pgx.ErrNoRows) {
		var config []byte
		if item.Provider == "github" {
			owner, repo, _ := strings.Cut(item.Key, "/")
			config, _ = json.Marshal(map[string]any{"owner": owner, "repo": repo, "page_size": 100})
		} else {
			config, _ = json.Marshal(map[string]string{"board": item.Key})
		}
		sourceID = sourceIDFor(item.Provider, item.Key)
		err = tx.QueryRow(ctx, `INSERT INTO sources(id,name,source_type,company_name,config,sync_interval_seconds)
			VALUES($1,$2,$3,$4,$5::jsonb,3600) ON CONFLICT DO NOTHING RETURNING id`, sourceID, item.Name+" jobs", item.Provider, item.Name, config).Scan(&sourceID)
		if errors.Is(err, pgx.ErrNoRows) {
			if item.Provider == "github" {
				owner, repo, _ := strings.Cut(item.Key, "/")
				err = tx.QueryRow(ctx, `SELECT id FROM sources WHERE source_type='github' AND config->>'owner'=$1 AND config->>'repo'=$2`, owner, repo).Scan(&sourceID)
			} else {
				err = tx.QueryRow(ctx, `SELECT id FROM sources WHERE source_type=$1 AND config->>'board'=$2`, item.Provider, item.Key).Scan(&sourceID)
			}
		} else if err == nil {
			created = true
		}
	}
	if err != nil {
		return "", false, fmt.Errorf("register verified source %s/%s: %w", item.Provider, item.Key, err)
	}
	if _, err := tx.Exec(ctx, `UPDATE source_candidates SET status='source_created',source_id=$2,last_checked_at=now(),next_check_at=now()+interval '30 days',last_error=NULL,updated_at=now() WHERE id=$1`, candidateID, sourceID); err != nil {
		return "", false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", false, err
	}
	return sourceID, created, nil
}

func sourceIDFor(provider, key string) string {
	slug := strings.ToLower(key)
	slug = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > 55 {
		slug = slug[:55]
	}
	digest := sha256.Sum256([]byte(provider + ":" + key))
	return "discovered-" + provider + "-" + slug + "-" + hex.EncodeToString(digest[:4])
}

func (s *Store) ScheduleRun(ctx context.Context, id string) error {
	result, err := s.pool.Exec(ctx, `UPDATE discovery_sources SET next_run_at=now(),updated_at=now() WHERE id=$1 AND enabled`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) Overview(ctx context.Context) (domain.Overview, error) {
	out := domain.Overview{Sources: []domain.DiscoverySourceStatus{}, CandidateStates: []domain.Count{}, Providers: []domain.Count{}}
	rows, err := s.pool.Query(ctx, `SELECT d.id,d.name,d.repo_owner,d.repo_name,coalesce(d.last_status,''),coalesce(d.last_error,''),d.last_run_at,
		coalesce(r.targets_seen,0),coalesce(r.candidates_created,0),coalesce(r.candidates_reused,0),coalesce(r.provenance_added,0),d.next_run_at
		FROM discovery_sources d LEFT JOIN LATERAL (SELECT targets_seen,candidates_created,candidates_reused,provenance_added FROM discovery_runs WHERE discovery_source_id=d.id ORDER BY started_at DESC LIMIT 1) r ON true ORDER BY d.id`)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var item domain.DiscoverySourceStatus
		if err := rows.Scan(&item.ID, &item.Name, &item.RepoOwner, &item.RepoName, &item.Status, &item.LastError, &item.LastRunAt, &item.TargetsSeen, &item.CandidatesCreated, &item.CandidatesReused, &item.ProvenanceAdded, &item.NextRunAt); err != nil {
			rows.Close()
			return out, err
		}
		out.Sources = append(out.Sources, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return out, err
	}
	rows.Close()
	for _, query := range []struct {
		dest *[]domain.Count
		sql  string
	}{{&out.CandidateStates, `SELECT status,count(*) FROM source_candidates GROUP BY status ORDER BY status`}, {&out.Providers, `SELECT detected_provider,count(*) FROM source_candidates WHERE detected_provider IS NOT NULL GROUP BY detected_provider ORDER BY detected_provider`}} {
		rows, err := s.pool.Query(ctx, query.sql)
		if err != nil {
			return out, err
		}
		for rows.Next() {
			var item domain.Count
			if err := rows.Scan(&item.Key, &item.Count); err != nil {
				rows.Close()
				return out, err
			}
			*query.dest = append(*query.dest, item)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return out, err
		}
		rows.Close()
	}
	return out, nil
}
