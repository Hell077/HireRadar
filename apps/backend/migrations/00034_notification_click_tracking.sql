-- +goose Up
ALTER TABLE notifications
    ADD COLUMN opened_at timestamptz,
    ADD COLUMN match_score smallint NOT NULL DEFAULT 0 CHECK (match_score BETWEEN 0 AND 100),
    ADD COLUMN match_confidence smallint NOT NULL DEFAULT 0 CHECK (match_confidence BETWEEN 0 AND 100);

-- +goose Down
ALTER TABLE notifications
    DROP COLUMN match_confidence,
    DROP COLUMN match_score,
    DROP COLUMN opened_at;
