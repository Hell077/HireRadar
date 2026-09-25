-- +goose Up
CREATE TABLE user_job_matches (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    job_id uuid NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    score smallint NOT NULL CHECK (score BETWEEN 0 AND 100),
    components jsonb NOT NULL DEFAULT '[]'::jsonb,
    computed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(user_id,job_id)
);
CREATE INDEX user_job_matches_feed_idx ON user_job_matches(user_id,score DESC,computed_at DESC,job_id);

-- +goose Down
DROP TABLE user_job_matches;
