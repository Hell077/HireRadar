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
	pool        *pgxpool.Pool
	refreshUser func(context.Context, domain.UserID) error
	refreshJob  func(context.Context, string) error
}

func NewOutboxWorker(pool *pgxpool.Pool, refreshUser func(context.Context, domain.UserID) error, refreshJob func(context.Context, string) error) *OutboxWorker {
	return &OutboxWorker{pool: pool, refreshUser: refreshUser, refreshJob: refreshJob}
}

// ProcessNext claims one profile or job event at a time. Refresh writes are
// safe to repeat, so a crash before acknowledgement recalculates the same data.
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
	var eventID, eventType, aggregateType, aggregateID string
	err = tx.QueryRow(ctx, `SELECT id::text,event_type,aggregate_type,aggregate_id FROM outbox_events
		WHERE event_type IN ('profile.changed','job.created','job.updated','job.closed') AND processed_at IS NULL AND available_at<=now()
		AND ($1::uuid IS NULL OR id=$1::uuid)
		ORDER BY available_at,id FOR UPDATE SKIP LOCKED LIMIT 1`, eventFilter).Scan(&eventID, &eventType, &aggregateType, &aggregateID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim profile change: %w", err)
	}
	validAggregate := (eventType == "profile.changed" && aggregateType == "user") || (eventType != "profile.changed" && aggregateType == "job")
	if _, err := uuid.Parse(aggregateID); err != nil || !validAggregate {
		if _, updateErr := tx.Exec(ctx, `UPDATE outbox_events SET processed_at=now() WHERE id=$1`, eventID); updateErr != nil {
			return true, fmt.Errorf("discard malformed matching event: %w", updateErr)
		}
		if err := tx.Commit(ctx); err != nil {
			return true, err
		}
		return true, fmt.Errorf("profile change event %s has invalid user id", eventID)
	}
	var refreshErr error
	if eventType == "profile.changed" {
		if w.refreshUser == nil {
			refreshErr = errors.New("profile rematcher is unavailable")
		} else {
			refreshErr = w.refreshUser(ctx, domain.UserID(aggregateID))
		}
	} else if w.refreshJob == nil {
		refreshErr = errors.New("job rematcher is unavailable")
	} else {
		refreshErr = w.refreshJob(ctx, aggregateID)
	}
	if refreshErr != nil {
		if _, updateErr := tx.Exec(ctx, `UPDATE outbox_events SET attempts=attempts+1,available_at=now()+LEAST(3600,POWER(2,LEAST(attempts+1,12))::int)*interval '1 second' WHERE id=$1`, eventID); updateErr != nil {
			return true, fmt.Errorf("schedule matching retry: %w", updateErr)
		}
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return true, fmt.Errorf("commit matching retry: %w", commitErr)
		}
		return true, fmt.Errorf("refresh matches for %s: %w", aggregateID, refreshErr)
	}
	if _, err := tx.Exec(ctx, `UPDATE outbox_events SET processed_at=now() WHERE id=$1`, eventID); err != nil {
		return true, fmt.Errorf("acknowledge matching event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return true, fmt.Errorf("commit matching event: %w", err)
	}
	return true, nil
}
