-- +goose Up
ALTER TABLE job_applications
    ADD COLUMN match_score smallint CHECK (match_score IS NULL OR match_score BETWEEN 0 AND 100),
    ADD COLUMN match_confidence smallint CHECK (match_confidence IS NULL OR match_confidence BETWEEN 0 AND 100);

-- +goose Down
ALTER TABLE job_applications
    DROP COLUMN match_confidence,
    DROP COLUMN match_score;
