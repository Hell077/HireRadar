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
	rows, err := c.pool.Query(ctx, `SELECT id,name,source_type,company_name,enabled,priority,sync_interval_seconds,last_sync_at FROM sources WHERE enabled=true ORDER BY priority,id`)
	if err != nil {
		return nil, fmt.Errorf("list enabled sources: %w", err)
	}
	defer rows.Close()
	result := []domain.Source{}
	for rows.Next() {
		var item domain.Source
		if err := rows.Scan(&item.ID, &item.Name, &item.Type, &item.CompanyName, &item.Enabled, &item.Priority, &item.SyncIntervalSecond, &item.LastSyncAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
