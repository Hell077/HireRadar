package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var ErrEventNotFound = errors.New("outbox event not found")

type Event struct {
	ID            string
	EventType     string
	AggregateType string
	AggregateID   string
	Payload       json.RawMessage
	Attempts      int
	AvailableAt   time.Time
	LockedBy      *string
	LockedUntil   *time.Time
}

type FailedEvent struct {
	ID            string    `json:"id"`
	EventType     string    `json:"event_type"`
	AggregateType string    `json:"aggregate_type"`
	AggregateID   string    `json:"aggregate_id"`
	Attempts      int       `json:"attempts"`
	FailedAt      time.Time `json:"failed_at"`
}

type Store interface {
	Claim(ctx context.Context, workerID string, eventTypes []string, lease time.Duration, eventID string) (*Event, error)
	Complete(ctx context.Context, eventID, workerID string) error
	Retry(ctx context.Context, eventID, workerID string, cause error) error
	Fail(ctx context.Context, eventID, workerID string, cause error) error
}
