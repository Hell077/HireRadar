package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/outbox"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const maxAttempts = 12

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func (s *Store) ListFailed(ctx context.Context, limit int) ([]outbox.FailedEvent, error) {
	if limit < 1 || limit > 100 {
		return nil, errors.New("failed outbox limit must be between 1 and 100")
	}
	rows, err := s.pool.Query(ctx, `SELECT id::text,event_type,aggregate_type,aggregate_id,attempts,failed_at
		FROM outbox_events WHERE status='failed' ORDER BY failed_at,id LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list failed outbox events: %w", err)
	}
	defer rows.Close()
	items := make([]outbox.FailedEvent, 0)
	for rows.Next() {
		var item outbox.FailedEvent
		if err := rows.Scan(&item.ID, &item.EventType, &item.AggregateType, &item.AggregateID, &item.Attempts, &item.FailedAt); err != nil {
			return nil, fmt.Errorf("scan failed outbox event: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read failed outbox events: %w", err)
	}
	return items, nil
}

func (s *Store) RetryFailed(ctx context.Context, eventID string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE outbox_events SET status='pending',available_at=now(),attempts=0,failed_at=NULL,last_error=NULL
		WHERE id=$1 AND status='failed'`, eventID)
	if err != nil {
		return fmt.Errorf("retry failed outbox event: %w", err)
	}
	if tag.RowsAffected() > 0 {
		return nil
	}
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM outbox_events WHERE id=$1)`, eventID).Scan(&exists); err != nil {
		return fmt.Errorf("check outbox event before retry: %w", err)
	}
	if !exists {
		return outbox.ErrEventNotFound
	}
	return nil
}

func (s *Store) QueueStats(ctx context.Context, eventTypes []string) (int64, float64, error) {
	var depth int64
	var oldestAge float64
	err := s.pool.QueryRow(ctx, `SELECT count(*),COALESCE(GREATEST(0,EXTRACT(EPOCH FROM now()-MIN(occurred_at) FILTER (WHERE status='pending'))),0)
		FROM outbox_events WHERE event_type=ANY($1::text[]) AND status IN ('pending','processing')`, eventTypes).Scan(&depth, &oldestAge)
	if err != nil {
		return 0, 0, fmt.Errorf("read outbox queue metrics: %w", err)
	}
	return depth, oldestAge, nil
}

func (s *Store) Claim(ctx context.Context, workerID string, eventTypes []string, lease time.Duration, eventID string) (*outbox.Event, error) {
	if lease <= 0 {
		return nil, errors.New("outbox lease must be positive")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin outbox claim: %w", err)
	}
	defer tx.Rollback(ctx)

	event := new(outbox.Event)
	err = tx.QueryRow(ctx, `WITH candidate AS (
		SELECT id FROM outbox_events
		WHERE event_type = ANY($1::text[])
		  AND ($2::uuid IS NULL OR id=$2::uuid)
		  AND ((status='pending' AND available_at<=now()) OR (status='processing' AND locked_until<=now()))
		ORDER BY available_at,id
		FOR UPDATE SKIP LOCKED
		LIMIT 1
	)
	UPDATE outbox_events e
	SET status='processing', locked_by=$3, locked_until=now()+$4::interval,
	    attempts=e.attempts+1
	FROM candidate c
	WHERE e.id=c.id
	RETURNING e.id::text,e.event_type,e.aggregate_type,e.aggregate_id,e.payload,e.attempts,e.available_at,e.locked_by,e.locked_until`,
		eventTypes, nullableID(eventID), workerID, lease.String()).Scan(
		&event.ID, &event.EventType, &event.AggregateType, &event.AggregateID,
		&event.Payload, &event.Attempts, &event.AvailableAt, &event.LockedBy, &event.LockedUntil)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("claim outbox event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit outbox claim: %w", err)
	}
	return event, nil
}

func (s *Store) Complete(ctx context.Context, eventID, workerID string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE outbox_events
		SET status='processed',processed_at=now(),locked_by=NULL,locked_until=NULL,last_error=NULL
		WHERE id=$1 AND status='processing' AND locked_by=$2`, eventID, workerID)
	if err != nil {
		return fmt.Errorf("complete outbox event: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return errors.New("outbox event lease is no longer owned by worker")
	}
	return nil
}

func (s *Store) Retry(ctx context.Context, eventID, workerID string, cause error) error {
	message := errorMessage(cause)
	tag, err := s.pool.Exec(ctx, `UPDATE outbox_events
		SET status=CASE WHEN attempts >= $3 THEN 'failed' ELSE 'pending' END,
		    failed_at=CASE WHEN attempts >= $3 THEN now() ELSE NULL END,
		    available_at=CASE WHEN attempts >= $3 THEN available_at ELSE now()+LEAST(3600,POWER(2,LEAST(attempts,12))::int)*interval '1 second' END,
		    locked_by=NULL,locked_until=NULL,last_error=$4
		WHERE id=$1 AND status='processing' AND locked_by=$2`, eventID, workerID, maxAttempts, message)
	if err != nil {
		return fmt.Errorf("retry outbox event: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return errors.New("outbox event lease is no longer owned by worker")
	}
	return nil
}

func (s *Store) Fail(ctx context.Context, eventID, workerID string, cause error) error {
	tag, err := s.pool.Exec(ctx, `UPDATE outbox_events SET status='failed',failed_at=now(),last_error=$3,locked_by=NULL,locked_until=NULL
		WHERE id=$1 AND status='processing' AND locked_by=$2`, eventID, workerID, errorMessage(cause))
	if err != nil {
		return fmt.Errorf("fail outbox event: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return errors.New("outbox event lease is no longer owned by worker")
	}
	return nil
}

func nullableID(id string) any {
	if id == "" {
		return nil
	}
	return id
}

func errorMessage(err error) string {
	if err == nil {
		return "unknown error"
	}
	message := err.Error()
	if len(message) > 512 {
		return message[:512]
	}
	return message
}
