package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/Hell077/HireRadar/apps/backend/internal/auth/application"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type EmailSender interface {
	Send(context.Context, application.EmailMessage) error
}

type DeliveryStore struct {
	pool      *pgxpool.Pool
	sender    EmailSender
	publicURL string
}

func NewDeliveryStore(pool *pgxpool.Pool, sender EmailSender, publicURL string) *DeliveryStore {
	return &DeliveryStore{pool: pool, sender: sender, publicURL: publicURL}
}

// DeliverNext locks one pending auth event while sending. A failed send is
// retried with exponential backoff; a successful send scrubs the raw token.
func (s *DeliveryStore) DeliverNext(ctx context.Context) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin email delivery: %w", err)
	}
	defer tx.Rollback(ctx)
	var id, eventType string
	var payload []byte
	err = tx.QueryRow(ctx, `
		SELECT id, event_type, payload FROM outbox_events
		WHERE event_type IN ('auth.verification_requested', 'auth.password_reset_requested')
		  AND processed_at IS NULL AND available_at <= now()
		ORDER BY available_at, id FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&id, &eventType, &payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim email event: %w", err)
	}
	message, err := application.MessageForEvent(eventType, payload, s.publicURL)
	if err == nil {
		err = s.sender.Send(ctx, message)
	}
	if err != nil {
		if _, updateErr := tx.Exec(ctx, `
			UPDATE outbox_events SET attempts = attempts + 1,
			available_at = now() + LEAST(3600, POWER(2, LEAST(attempts + 1, 12))::int) * interval '1 second'
			WHERE id = $1`, id); updateErr != nil {
			return true, fmt.Errorf("schedule email retry: %w", updateErr)
		}
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return true, fmt.Errorf("commit email retry: %w", commitErr)
		}
		return true, fmt.Errorf("deliver email event: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE outbox_events SET processed_at = now(), payload = payload - 'verification_token' - 'reset_token'
		WHERE id = $1`, id); err != nil {
		return true, fmt.Errorf("finish email event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return true, fmt.Errorf("commit email delivery: %w", err)
	}
	return true, nil
}
