package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/source/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Catalog struct{ pool *pgxpool.Pool }

func NewCatalog(pool *pgxpool.Pool) *Catalog { return &Catalog{pool: pool} }

func (c *Catalog) ListEnabled(ctx context.Context) ([]domain.Source, error) { return c.list(ctx, true) }
func (c *Catalog) ListAll(ctx context.Context) ([]domain.Source, error)     { return c.list(ctx, false) }

func (c *Catalog) list(ctx context.Context, enabledOnly bool) ([]domain.Source, error) {
	where := ""
	if enabledOnly {
		where = "WHERE s.enabled=true"
	}
	rows, err := c.pool.Query(ctx, `SELECT s.id,s.name,s.source_type,s.company_name,s.enabled,s.priority,s.sync_interval_seconds,s.last_sync_at,r.started_at,coalesce(r.status,''),coalesce(r.fetched_count,0),coalesce(r.new_count,0),coalesce(r.updated_count,0)
		FROM sources s LEFT JOIN LATERAL (SELECT started_at,status,fetched_count,new_count,updated_count FROM source_sync_runs WHERE source_id=s.id ORDER BY started_at DESC,id DESC LIMIT 1) r ON true
		`+where+` ORDER BY s.priority,s.id`)
	if err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}
	defer rows.Close()
	result := []domain.Source{}
	for rows.Next() {
		var item domain.Source
		if err := rows.Scan(&item.ID, &item.Name, &item.Type, &item.CompanyName, &item.Enabled, &item.Priority, &item.SyncIntervalSecond, &item.LastSyncAt, &item.LastAttemptAt, &item.LastSyncStatus, &item.LastFetchedJobs, &item.LastNewJobs, &item.LastUpdatedJobs); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

var operatorSourceID = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,99}$`)

type SourceSettings struct {
	Enabled             bool `json:"enabled"`
	Priority            int  `json:"priority"`
	SyncIntervalSeconds int  `json:"sync_interval_seconds"`
}
type SourceAudit struct {
	ID        int64           `json:"id"`
	SourceID  string          `json:"source_id"`
	Actor     string          `json:"actor"`
	Action    string          `json:"action"`
	Details   json.RawMessage `json:"details"`
	CreatedAt time.Time       `json:"created_at"`
}

func (c *Catalog) UpdateSettings(ctx context.Context, sourceID, actor string, settings SourceSettings) error {
	if !operatorSourceID.MatchString(sourceID) || actor == "" || settings.Priority < 0 || settings.Priority > 1000 || settings.SyncIntervalSeconds < 60 || settings.SyncIntervalSeconds > 604800 {
		return domain.ErrInvalidSource
	}
	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin source settings update: %w", err)
	}
	defer tx.Rollback(ctx)
	var before SourceSettings
	if err := tx.QueryRow(ctx, `SELECT enabled,priority,sync_interval_seconds FROM sources WHERE id=$1 FOR UPDATE`, sourceID).Scan(&before.Enabled, &before.Priority, &before.SyncIntervalSeconds); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE sources SET enabled=$2,priority=$3,sync_interval_seconds=$4,next_sync_at=CASE WHEN $2 THEN LEAST(next_sync_at,now()) ELSE next_sync_at END,updated_at=now() WHERE id=$1`, sourceID, settings.Enabled, settings.Priority, settings.SyncIntervalSeconds); err != nil {
		return fmt.Errorf("update source settings: %w", err)
	}
	details, _ := json.Marshal(map[string]any{"before": before, "after": settings})
	if _, err := tx.Exec(ctx, `INSERT INTO source_admin_audit(source_id,actor,action,details) VALUES($1,$2,'settings_updated',$3)`, sourceID, actor, details); err != nil {
		return fmt.Errorf("audit source settings: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit source settings: %w", err)
	}
	return nil
}

func (c *Catalog) RequestSync(ctx context.Context, sourceID, actor string) error {
	if !operatorSourceID.MatchString(sourceID) || actor == "" {
		return domain.ErrInvalidSource
	}
	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin source sync request: %w", err)
	}
	defer tx.Rollback(ctx)
	result, err := tx.Exec(ctx, `UPDATE sources SET next_sync_at=now(),updated_at=now() WHERE id=$1`, sourceID)
	if err != nil {
		return fmt.Errorf("schedule source sync: %w", err)
	}
	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	if _, err := tx.Exec(ctx, `INSERT INTO source_admin_audit(source_id,actor,action,details) VALUES($1,$2,'sync_requested','{}'::jsonb)`, sourceID, actor); err != nil {
		return fmt.Errorf("audit source sync request: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit source sync request: %w", err)
	}
	return nil
}

func (c *Catalog) ListAudit(ctx context.Context, limit int) ([]SourceAudit, error) {
	if limit < 1 || limit > 200 {
		return nil, domain.ErrInvalidSource
	}
	rows, err := c.pool.Query(ctx, `SELECT id,source_id,actor,action,details,created_at FROM source_admin_audit ORDER BY created_at DESC,id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list source audit: %w", err)
	}
	defer rows.Close()
	items := make([]SourceAudit, 0)
	for rows.Next() {
		var item SourceAudit
		if err := rows.Scan(&item.ID, &item.SourceID, &item.Actor, &item.Action, &item.Details, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
