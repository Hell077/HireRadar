package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/observability"
	"github.com/Hell077/HireRadar/apps/backend/internal/outbox"
	outboxpostgres "github.com/Hell077/HireRadar/apps/backend/internal/outbox/adapters/postgres"
	userdomain "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OutboxWorker struct {
	pool        *pgxpool.Pool
	store       outbox.Store
	workerID    string
	refreshUser func(context.Context, userdomain.UserID) error
	refreshJob  func(context.Context, string) error
}

func NewOutboxWorker(pool *pgxpool.Pool, refreshUser func(context.Context, userdomain.UserID) error, refreshJob func(context.Context, string) error) *OutboxWorker {
	return &OutboxWorker{pool: pool, store: outboxpostgres.NewStore(pool), workerID: uuid.NewString(), refreshUser: refreshUser, refreshJob: refreshJob}
}

// ProcessNext claims one profile or job event. The claim is committed before
// refreshing matches, so the potentially long matching pass holds no row lock.
func (w *OutboxWorker) ProcessNext(ctx context.Context) (bool, error) {
	return w.process(ctx, "")
}

func (w *OutboxWorker) ProcessEvent(ctx context.Context, eventID string) (bool, error) {
	return w.process(ctx, eventID)
}

func (w *OutboxWorker) process(ctx context.Context, onlyEvent string) (found bool, retErr error) {
	w.refreshQueueMetrics(ctx)
	started := time.Now()
	eventType := "unknown"
	defer func() {
		if !found {
			return
		}
		observability.DefaultMetrics.Add("hireradar_matching_events_total", map[string]string{"event_type": eventType}, 1)
		observability.DefaultMetrics.ObserveDuration("hireradar_matching_duration", map[string]string{"event_type": eventType}, time.Since(started))
	}()
	event, err := w.store.Claim(ctx, w.workerID, []string{"profile.changed", "job.created", "job.updated", "job.closed"}, 5*time.Minute, onlyEvent)
	if err != nil || event == nil {
		return event != nil, err
	}
	eventType = event.EventType
	validAggregate := (event.EventType == "profile.changed" && event.AggregateType == "user") ||
		(event.EventType != "profile.changed" && event.AggregateType == "job")
	if _, err := uuid.Parse(event.AggregateID); err != nil || !validAggregate {
		cause := fmt.Errorf("matching event %s has invalid aggregate", event.ID)
		if failErr := w.store.Fail(ctx, event.ID, w.workerID, cause); failErr != nil {
			return true, fmt.Errorf("%v; mark event failed: %w", cause, failErr)
		}
		observability.DefaultMetrics.Add("hireradar_matching_failed_total", map[string]string{"event_type": eventType}, 1)
		return true, cause
	}

	var refreshErr error
	if event.EventType == "profile.changed" {
		if w.refreshUser == nil {
			refreshErr = errors.New("profile rematcher is unavailable")
		} else {
			refreshErr = w.refreshUser(ctx, userdomain.UserID(event.AggregateID))
		}
	} else if w.refreshJob == nil {
		refreshErr = errors.New("job rematcher is unavailable")
	} else {
		refreshErr = w.refreshJob(ctx, event.AggregateID)
	}
	if refreshErr != nil {
		if err := w.store.Retry(ctx, event.ID, w.workerID, refreshErr); err != nil {
			return true, fmt.Errorf("refresh matches: %v; schedule retry: %w", refreshErr, err)
		}
		observability.DefaultMetrics.Add("hireradar_matching_retry_total", map[string]string{"event_type": eventType}, 1)
		return true, fmt.Errorf("refresh matches for %s: %w", event.AggregateID, refreshErr)
	}
	if err := w.store.Complete(ctx, event.ID, w.workerID); err != nil {
		return true, fmt.Errorf("complete matching event: %w", err)
	}
	return true, nil
}

func (w *OutboxWorker) refreshQueueMetrics(ctx context.Context) {
	metrics := observability.DefaultMetrics
	if !metrics.ShouldRefresh("matching_queue", time.Now(), 30*time.Second) {
		return
	}
	var depth int64
	var oldestAge float64
	err := w.pool.QueryRow(ctx, `SELECT count(*),COALESCE(GREATEST(0,EXTRACT(EPOCH FROM now()-MIN(occurred_at) FILTER (WHERE status='pending'))),0)
		FROM outbox_events WHERE event_type IN ('profile.changed','job.created','job.updated','job.closed')
		AND status IN ('pending','processing')`).Scan(&depth, &oldestAge)
	if err == nil {
		metrics.SetGauge("hireradar_matching_queue_depth", nil, float64(depth))
		metrics.SetGauge("hireradar_matching_queue_oldest_pending_age_seconds", nil, oldestAge)
	}
}
