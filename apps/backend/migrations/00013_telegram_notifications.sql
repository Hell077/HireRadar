-- +goose Up
CREATE TABLE telegram_accounts (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    telegram_user_id bigint NOT NULL UNIQUE,
    chat_id bigint NOT NULL,
    username text,
    connected_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE telegram_link_tokens (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash bytea NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    used_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX telegram_link_tokens_pending_idx ON telegram_link_tokens(expires_at) WHERE used_at IS NULL;

CREATE TABLE notification_preferences (
    user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    enabled boolean NOT NULL DEFAULT true,
    minimum_score smallint NOT NULL DEFAULT 70 CHECK (minimum_score BETWEEN 0 AND 100),
    immediate boolean NOT NULL DEFAULT true,
    digest_enabled boolean NOT NULL DEFAULT false,
    timezone text NOT NULL DEFAULT 'UTC',
    quiet_start time,
    quiet_end time,
    max_per_day smallint CHECK (max_per_day IS NULL OR max_per_day BETWEEN 1 AND 100),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((quiet_start IS NULL) = (quiet_end IS NULL))
);

CREATE TABLE notifications (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    job_id uuid NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    notification_type text NOT NULL DEFAULT 'job_match' CHECK (notification_type IN ('job_match')),
    channel text NOT NULL DEFAULT 'telegram' CHECK (channel IN ('telegram')),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','sent','failed','cancelled')),
    scheduled_at timestamptz NOT NULL DEFAULT now(),
    sent_at timestamptz,
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    last_error text,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(user_id,job_id,notification_type)
);
CREATE INDEX notifications_due_idx ON notifications(scheduled_at,id) WHERE status='pending';

CREATE TABLE saved_jobs (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    job_id uuid NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(user_id,job_id)
);

CREATE TABLE user_job_feedback (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    job_id uuid NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    feedback_type text NOT NULL CHECK (feedback_type IN ('hidden','not_interested','applied')),
    reason text,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(user_id,job_id,feedback_type)
);

-- +goose Down
DROP TABLE user_job_feedback;
DROP TABLE saved_jobs;
DROP TABLE notifications;
DROP TABLE notification_preferences;
DROP TABLE telegram_link_tokens;
DROP TABLE telegram_accounts;
