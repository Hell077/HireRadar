-- +goose Up
ALTER TABLE telegram_accounts
    ADD COLUMN enabled boolean NOT NULL DEFAULT true,
    ADD COLUMN last_interaction_at timestamptz;
UPDATE telegram_accounts SET last_interaction_at=updated_at;

-- +goose Down
ALTER TABLE telegram_accounts
    DROP COLUMN last_interaction_at,
    DROP COLUMN enabled;
