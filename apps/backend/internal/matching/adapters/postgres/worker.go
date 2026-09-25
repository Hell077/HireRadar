package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OutboxWorker struct {
	pool    *pgxpool.Pool
	refresh func(context.Context, domain.UserID) error
}

func NewOutboxWorker(pool *pgxpool.Pool, refresh func(context.Context, domain.UserID) error) *OutboxWorker {
	return &OutboxWorker{pool: pool, refresh: refresh}
}

// ProcessNext claims one profile change at a time. Refresh writes are safe to
// repeat, so a crash before acknowledgement simply recalculates the same user.
func (w *OutboxWorker) ProcessNext(ctx context.Context) (bool, error) {
	return w.process(ctx, "")
}

func (w *OutboxWorker) ProcessEvent(ctx context.Context, eventID string) (bool, error) {
	return w.process(ctx, eventID)
}

func (w *OutboxWorker) process(ctx context.Context, onlyEvent string) (bool, error) {
	var eventFilter any
	if onlyEvent != "" {
		eventFilter = onlyEvent
	}
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin matching event: %w", err)
	}
	defer tx.Rollback(ctx)
	var eventID, aggregateID string
	err = tx.QueryRow(ctx, `SELECT id::text,aggregate_id FROM outbox_events
		WHERE event_type='profile.changed' AND aggregate_type='user' AND processed_at IS NULL AND available_at<=now()
		AND ($1::uuid IS NULL OR id=$1::uuid)
		ORDER BY available_at,id FOR UPDATE SKIP LOCKED LIMIT 1`, eventFilter).Scan(&eventID, &aggregateID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim profile change: %w", err)
	}
	if _, err := uuid.Parse(aggregateID); err != nil {
		if _, updateErr := tx.Exec(ctx, `UPDATE outbox_events SET processed_at=now() WHERE id=$1`, eventID); updateErr != nil {
			return true, fmt.Errorf("discard malformed matching event: %w", updateErr)
		}
		if err := tx.Commit(ctx); err != nil {
			return true, err
		}
		return true, fmt.Errorf("profile change event %s has invalid user id", eventID)
	}
	if err := w.refresh(ctx, domain.UserID(aggregateID)); err != nil {
		if _, updateErr := tx.Exec(ctx, `UPDATE outbox_events SET attempts=attempts+1,available_at=now()+LEAST(3600,POWER(2,LEAST(attempts+1,12))::int)*interval '1 second' WHERE id=$1`, eventID); updateErr != nil {
			return true, fmt.Errorf("schedule matching retry: %w", updateErr)
		}
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return true, fmt.Errorf("commit matching retry: %w", commitErr)
		}
		return true, fmt.Errorf("refresh matches for %s: %w", aggregateID, err)
	}
	if _, err := tx.Exec(ctx, `UPDATE outbox_events SET processed_at=now() WHERE id=$1`, eventID); err != nil {
		return true, fmt.Errorf("acknowledge matching event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return true, fmt.Errorf("commit matching event: %w", err)
	}
	return true, nil
}
