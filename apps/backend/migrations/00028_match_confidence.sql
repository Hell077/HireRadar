-- +goose Up
ALTER TABLE user_job_matches
    ADD COLUMN confidence smallint NOT NULL DEFAULT 0 CHECK (confidence BETWEEN 0 AND 100);

-- +goose Down
ALTER TABLE user_job_matches DROP COLUMN confidence;
