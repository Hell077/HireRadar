-- +goose Up
CREATE TABLE outbox_events (
    id uuid PRIMARY KEY,
    event_type text NOT NULL,
    aggregate_type text NOT NULL,
    aggregate_id text NOT NULL,
    payload jsonb NOT NULL,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    available_at timestamptz NOT NULL DEFAULT now(),
    processed_at timestamptz,
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0)
);

CREATE INDEX outbox_events_pending_idx
    ON outbox_events (available_at, id)
    WHERE processed_at IS NULL;

-- +goose Down
DROP TABLE outbox_events;
