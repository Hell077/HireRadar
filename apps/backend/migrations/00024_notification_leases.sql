-- +goose Up
ALTER TABLE notifications
    DROP CONSTRAINT notifications_status_check,
    ADD CONSTRAINT notifications_status_check
        CHECK (status IN ('pending', 'delivering', 'sent', 'failed', 'cancelled')),
    ADD COLUMN locked_by text,
    ADD COLUMN locked_until timestamptz;

CREATE INDEX notifications_delivery_lease_idx
    ON notifications (locked_until, id)
    WHERE status = 'delivering';

-- +goose Down
DROP INDEX notifications_delivery_lease_idx;
ALTER TABLE notifications
    DROP COLUMN locked_until,
    DROP COLUMN locked_by,
    DROP CONSTRAINT notifications_status_check,
    ADD CONSTRAINT notifications_status_check
        CHECK (status IN ('pending', 'sent', 'failed', 'cancelled'));
