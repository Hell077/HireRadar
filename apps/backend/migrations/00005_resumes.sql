-- +goose Up
CREATE TABLE resumes (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    file_name text NOT NULL,
    object_key text NOT NULL UNIQUE,
    content_type text NOT NULL DEFAULT 'application/pdf',
    size bigint NOT NULL CHECK (size BETWEEN 1 AND 10485760),
    sha256 text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'pending_upload'
        CHECK (status IN ('pending_upload', 'uploaded', 'processing', 'processed', 'failed', 'deleted')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (length(file_name) BETWEEN 1 AND 255),
    CHECK (content_type = 'application/pdf')
);

CREATE INDEX resumes_user_active_idx ON resumes (user_id, created_at DESC)
    WHERE status <> 'deleted';

-- +goose Down
DROP TABLE resumes;
