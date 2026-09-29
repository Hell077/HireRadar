package application

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
	"sync"
	"time"

	jobpostgres "github.com/Hell077/HireRadar/apps/backend/internal/job/adapters/postgres"
	"github.com/Hell077/HireRadar/apps/backend/internal/observability"
	"github.com/Hell077/HireRadar/apps/backend/internal/source/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Worker struct {
	pool    *pgxpool.Pool
	fetcher interface {
		Fetch(context.Context, domain.Source) (domain.FetchResult, error)
	}
	parallelism int
	jobs        *jobpostgres.Ingestor
	reportError func(error)
}

func (w *Worker) SetErrorReporter(report func(error)) { w.reportError = report }

func NewWorker(pool *pgxpool.Pool, fetcher interface {
	Fetch(context.Context, domain.Source) (domain.FetchResult, error)
}, parallelism int) *Worker {
	if parallelism < 1 {
		parallelism = 1
	}
	if parallelism > 32 {
		parallelism = 32
	}
	return &Worker{pool: pool, fetcher: fetcher, parallelism: parallelism, jobs: jobpostgres.NewIngestor()}
}

func (w *Worker) Run(ctx context.Context) error {
	slog.Info("source worker started", "parallelism", w.parallelism)
	for ctx.Err() == nil {
		w.refreshQueueMetrics(ctx)
		sources, err := w.claimDue(ctx)
		if err != nil && ctx.Err() == nil {
			slog.Error("claim due sources failed", "error", err)
			if w.reportError != nil {
				w.reportError(err)
			}
		}
		if len(sources) > 0 {
			var group sync.WaitGroup
			for _, source := range sources {
				source := source
				group.Add(1)
				go func() {
					defer group.Done()
					if err := w.syncSource(ctx, source); err != nil && ctx.Err() == nil {
						slog.Error("source sync failed", "source_id", source.ID, "error", err)
						if w.reportError != nil {
							w.reportError(err)
						}
					} else if w.reportError != nil {
						w.reportError(nil)
					}
				}()
			}
			group.Wait()
			continue
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(2 * time.Second):
		}
	}
	return nil
}

func (w *Worker) refreshQueueMetrics(ctx context.Context) {
	metrics := observability.DefaultMetrics
	if !metrics.ShouldRefresh("source_queue", time.Now(), 30*time.Second) {
		return
	}
	var depth int64
	var oldestAge float64
	err := w.pool.QueryRow(ctx, `SELECT count(*),COALESCE(GREATEST(0,EXTRACT(EPOCH FROM now()-MIN(next_sync_at))),0)
		FROM sources WHERE enabled=true AND next_sync_at<=now()`).Scan(&depth, &oldestAge)
	if err == nil {
		metrics.SetGauge("hireradar_source_queue_depth", nil, float64(depth))
		metrics.SetGauge("hireradar_source_queue_oldest_pending_age_seconds", nil, oldestAge)
	}
}

func (w *Worker) claimDue(ctx context.Context) ([]domain.Source, error) {
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `WITH due AS (
		SELECT id FROM sources WHERE enabled=true AND next_sync_at<=now() ORDER BY next_sync_at,id FOR UPDATE SKIP LOCKED LIMIT $1
	) UPDATE sources s SET next_sync_at=now()+make_interval(secs=>s.sync_interval_seconds),updated_at=now()
	FROM due WHERE s.id=due.id RETURNING s.id,s.name,s.source_type,s.company_name,s.enabled,s.priority,s.sync_interval_seconds,s.config,coalesce(s.cursor,'null'::jsonb),s.last_sync_at`, w.parallelism)
	if err != nil {
		return nil, fmt.Errorf("claim due source rows: %w", err)
	}
	defer rows.Close()
	sources := []domain.Source{}
	for rows.Next() {
		var source domain.Source
		if err := rows.Scan(&source.ID, &source.Name, &source.Type, &source.CompanyName, &source.Enabled, &source.Priority, &source.SyncIntervalSecond, &source.Config, &source.Cursor, &source.LastSyncAt); err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return sources, nil
}

func (w *Worker) syncSource(ctx context.Context, source domain.Source) (retErr error) {
	started := time.Now()
	defer func() {
		observability.DefaultMetrics.Add("hireradar_source_sync_total", nil, 1)
		observability.DefaultMetrics.ObserveDuration("hireradar_source_sync_duration", nil, time.Since(started))
		if retErr != nil {
			observability.DefaultMetrics.Add("hireradar_source_sync_failed_total", nil, 1)
		}
	}()
	runID := uuid.NewString()
	if _, err := w.pool.Exec(ctx, `INSERT INTO source_sync_runs(id,source_id,status) VALUES($1,$2,'running')`, runID, source.ID); err != nil {
		return fmt.Errorf("start sync run: %w", err)
	}
	workCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	result, err := w.fetcher.Fetch(workCtx, source)
	if err != nil {
		_ = w.finishFailed(ctx, source, runID, err)
		return err
	}
	if len(result.Jobs) > 50000 {
		err = errors.New("source returned more than 50000 jobs")
		_ = w.finishFailed(ctx, source, runID, err)
		return err
	}
	observability.DefaultMetrics.Add("hireradar_jobs_fetched_total", nil, float64(len(result.Jobs)))
	newCount, updatedCount, err := w.save(ctx, source, runID, result)
	if err != nil {
		_ = w.finishFailed(ctx, source, runID, err)
		return err
	}
	observability.DefaultMetrics.Add("hireradar_jobs_created_total", nil, float64(newCount))
	observability.DefaultMetrics.Add("hireradar_jobs_updated_total", nil, float64(updatedCount))
	slog.Info("source sync complete", "source_id", source.ID, "fetched", len(result.Jobs), "new", newCount, "updated", updatedCount)
	return nil
}

func (w *Worker) save(ctx context.Context, source domain.Source, runID string, result domain.FetchResult) (int, int, error) {
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback(ctx)
	var newCount, updatedCount int
	var closedCount int
	vocabulary, err := w.jobs.LoadSkillVocabulary(ctx, tx)
	if err != nil {
		return 0, 0, err
	}
	for _, job := range result.Jobs {
		job.ExternalID = strings.TrimSpace(job.ExternalID)
		job.Title = strings.TrimSpace(job.Title)
		job.ApplyURL = strings.TrimSpace(job.ApplyURL)
		if job.ExternalID == "" || job.Title == "" || job.ApplyURL == "" {
			return 0, 0, fmt.Errorf("source %s returned job with missing external ID, title, or apply URL", source.ID)
		}
		payload, err := json.Marshal(job)
		if err != nil {
			return 0, 0, err
		}
		hash := sha256.Sum256(payload)
		_, err = tx.Exec(ctx, `INSERT INTO raw_jobs(source_id,external_id,content_hash,payload) VALUES($1,$2,$3,$4::jsonb) ON CONFLICT(source_id,external_id,content_hash) DO NOTHING`, source.ID, job.ExternalID, hash[:], payload)
		if err != nil {
			return 0, 0, fmt.Errorf("persist raw job %s: %w", job.ExternalID, err)
		}
		created, updated, err := w.jobs.Save(ctx, tx, source, runID, job, vocabulary)
		if err != nil {
			return 0, 0, fmt.Errorf("normalize source job %s: %w", job.ExternalID, err)
		}
		if created {
			newCount++
		} else if updated {
			updatedCount++
		}
	}
	if result.Mode == domain.SyncSnapshot {
		closedCount, err = w.jobs.CloseMissing(ctx, tx, source.ID, runID)
		if err != nil {
			return 0, 0, err
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE sources SET cursor=$2::jsonb,last_sync_at=now(),next_sync_at=now()+make_interval(secs=>sync_interval_seconds),updated_at=now() WHERE id=$1`, source.ID, nonNullJSON(result.NextCursor)); err != nil {
		return 0, 0, err
	}
	if _, err := tx.Exec(ctx, `UPDATE source_sync_runs SET status='succeeded',finished_at=now(),fetched_count=$2,new_count=$3,updated_count=$4 WHERE id=$1`, runID, len(result.Jobs), newCount, updatedCount); err != nil {
		return 0, 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, 0, fmt.Errorf("commit source sync: %w", err)
	}
	observability.DefaultMetrics.Add("hireradar_jobs_closed_total", nil, float64(closedCount))
	return newCount, updatedCount, nil
}

func (w *Worker) finishFailed(ctx context.Context, source domain.Source, runID string, cause error) error {
	message := cause.Error()
	if len(message) > 2000 {
		message = message[:2000]
	}
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE source_sync_runs SET status='failed',finished_at=now(),error_message=$2 WHERE id=$1`, runID, message); err != nil {
		return err
	}
	var consecutive int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM source_sync_runs
		WHERE source_id=$1 AND status='failed' AND started_at > COALESCE(
			(SELECT max(started_at) FROM source_sync_runs WHERE source_id=$1 AND status='succeeded'), '-infinity'::timestamptz)`, source.ID).Scan(&consecutive); err != nil {
		return err
	}
	delay := retryDelay(consecutive, cause)
	if _, err := tx.Exec(ctx, `UPDATE sources SET next_sync_at=now()+make_interval(secs=>$2),updated_at=now() WHERE id=$1`, source.ID, delay.Seconds()); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	observability.DefaultMetrics.Add("hireradar_source_sync_retry_total", nil, 1)
	return nil
}

func retryDelay(consecutive int, cause error) time.Duration {
	delays := [...]time.Duration{time.Minute, 2 * time.Minute, 5 * time.Minute, 15 * time.Minute, 30 * time.Minute, time.Hour}
	if consecutive < 1 {
		consecutive = 1
	}
	base := delays[min(consecutive-1, len(delays)-1)]
	var httpErr *domain.HTTPError
	// Jitter by +/- 20% to keep many sources from retrying together.
	jitter := time.Duration(float64(base) * (0.8 + rand.Float64()*0.4))
	if errors.As(cause, &httpErr) && httpErr.RetryAfter > jitter {
		jitter = httpErr.RetryAfter
	}
	if jitter > 6*time.Hour {
		jitter = 6 * time.Hour
	}
	return jitter
}

func nonNullJSON(data json.RawMessage) string {
	if len(data) == 0 || string(data) == "null" {
		return "null"
	}
	return string(data)
}
