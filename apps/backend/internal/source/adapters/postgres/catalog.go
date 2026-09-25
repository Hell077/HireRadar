package postgres

import (
	"context"
	"fmt"

	"github.com/Hell077/HireRadar/apps/backend/internal/source/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Catalog struct{ pool *pgxpool.Pool }

func NewCatalog(pool *pgxpool.Pool) *Catalog { return &Catalog{pool: pool} }

func (c *Catalog) ListEnabled(ctx context.Context) ([]domain.Source, error) {
	rows, err := c.pool.Query(ctx, `SELECT s.id,s.name,s.source_type,s.company_name,s.enabled,s.priority,s.sync_interval_seconds,s.last_sync_at,r.started_at,coalesce(r.status,''),coalesce(r.fetched_count,0),coalesce(r.new_count,0),coalesce(r.updated_count,0)
		FROM sources s LEFT JOIN LATERAL (SELECT started_at,status,fetched_count,new_count,updated_count FROM source_sync_runs WHERE source_id=s.id ORDER BY started_at DESC,id DESC LIMIT 1) r ON true
		WHERE s.enabled=true ORDER BY s.priority,s.id`)
	if err != nil {
		return nil, fmt.Errorf("list enabled sources: %w", err)
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
