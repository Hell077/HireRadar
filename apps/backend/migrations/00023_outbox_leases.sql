-- +goose Up
ALTER TABLE outbox_events
    ADD COLUMN status text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'processed', 'failed')),
    ADD COLUMN locked_by text,
    ADD COLUMN locked_until timestamptz,
    ADD COLUMN last_error text,
    ADD COLUMN failed_at timestamptz;

UPDATE outbox_events
SET status = 'processed'
WHERE processed_at IS NOT NULL;

CREATE INDEX outbox_events_pending_available_idx
    ON outbox_events (available_at, id)
    WHERE status = 'pending';
CREATE INDEX outbox_events_processing_lease_idx
    ON outbox_events (locked_until, id)
    WHERE status = 'processing';
CREATE INDEX outbox_events_failed_idx
    ON outbox_events (failed_at, id)
    WHERE status = 'failed';

-- +goose Down
DROP INDEX outbox_events_failed_idx;
DROP INDEX outbox_events_processing_lease_idx;
DROP INDEX outbox_events_pending_available_idx;
ALTER TABLE outbox_events
    DROP COLUMN failed_at,
    DROP COLUMN last_error,
    DROP COLUMN locked_until,
    DROP COLUMN locked_by,
    DROP COLUMN status;
