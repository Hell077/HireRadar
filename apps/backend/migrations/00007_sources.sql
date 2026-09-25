-- +goose Up
CREATE TABLE sources (
    id text PRIMARY KEY CHECK (id ~ '^[a-z0-9][a-z0-9_-]{1,99}$'),
    name text NOT NULL CHECK (length(trim(name)) BETWEEN 1 AND 200),
    source_type text NOT NULL CHECK (source_type IN ('greenhouse','lever','ashby')),
    company_name text NOT NULL CHECK (length(trim(company_name)) BETWEEN 1 AND 200),
    enabled boolean NOT NULL DEFAULT true,
    priority smallint NOT NULL DEFAULT 100 CHECK (priority BETWEEN 0 AND 1000),
    sync_interval_seconds integer NOT NULL DEFAULT 900 CHECK (sync_interval_seconds BETWEEN 60 AND 604800),
    config jsonb NOT NULL DEFAULT '{}'::jsonb,
    cursor jsonb,
    last_sync_at timestamptz,
    next_sync_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sources_due_idx ON sources(next_sync_at, id) WHERE enabled;

CREATE TABLE source_sync_runs (
    id uuid PRIMARY KEY,
    source_id text NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    status text NOT NULL CHECK (status IN ('running','succeeded','failed')),
    started_at timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz,
    fetched_count integer NOT NULL DEFAULT 0 CHECK (fetched_count >= 0),
    new_count integer NOT NULL DEFAULT 0 CHECK (new_count >= 0),
    updated_count integer NOT NULL DEFAULT 0 CHECK (updated_count >= 0),
    error_message text
);
CREATE INDEX source_sync_runs_source_started_idx ON source_sync_runs(source_id, started_at DESC);

CREATE TABLE raw_jobs (
    id bigserial PRIMARY KEY,
    source_id text NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    external_id text NOT NULL CHECK (length(external_id) BETWEEN 1 AND 500),
    content_hash bytea NOT NULL CHECK (octet_length(content_hash) = 32),
    payload jsonb NOT NULL,
    fetched_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(source_id, external_id, content_hash)
);
CREATE INDEX raw_jobs_latest_idx ON raw_jobs(source_id, external_id, fetched_at DESC);

-- +goose Down
DROP TABLE raw_jobs;
DROP TABLE source_sync_runs;
DROP TABLE sources;
