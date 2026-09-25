package application

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	jobpostgres "github.com/Hell077/HireRadar/apps/backend/internal/job/adapters/postgres"
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
}

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
		sources, err := w.claimDue(ctx)
		if err != nil && ctx.Err() == nil {
			slog.Error("claim due sources failed", "error", err)
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

func (w *Worker) syncSource(ctx context.Context, source domain.Source) error {
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
	newCount, updatedCount, err := w.save(ctx, source, runID, result)
	if err != nil {
		_ = w.finishFailed(ctx, source, runID, err)
		return err
	}
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
		created, updated, err := w.jobs.Save(ctx, tx, source, runID, job)
		if err != nil {
			return 0, 0, fmt.Errorf("normalize source job %s: %w", job.ExternalID, err)
		}
		if created {
			newCount++
		} else if updated {
			updatedCount++
		}
	}
	if err := w.jobs.CloseMissing(ctx, tx, source.ID, runID); err != nil {
		return 0, 0, err
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
	if _, err := tx.Exec(ctx, `UPDATE sources SET next_sync_at=now()+interval '1 minute',updated_at=now() WHERE id=$1`, source.ID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func nonNullJSON(data json.RawMessage) string {
	if len(data) == 0 || string(data) == "null" {
		return "null"
	}
	return string(data)
}
