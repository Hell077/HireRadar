-- +goose Up
CREATE TABLE companies (
    id uuid PRIMARY KEY,
    name text NOT NULL,
    normalized_name text NOT NULL UNIQUE,
    website_url text,
    careers_url text,
    remote_policy text NOT NULL DEFAULT 'unknown' CHECK (remote_policy IN ('worldwide','remote','remote_region','remote_country','hybrid','onsite','unknown')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE jobs (
    id uuid PRIMARY KEY,
    company_id uuid NOT NULL REFERENCES companies(id),
    title text NOT NULL,
    normalized_title text NOT NULL,
    description text NOT NULL DEFAULT '',
    employment_types text[] NOT NULL DEFAULT '{}',
    remote_policy text NOT NULL CHECK (remote_policy IN ('worldwide','remote','remote_region','remote_country','hybrid','onsite','unknown')),
    location text NOT NULL DEFAULT '',
    location_countries text[] NOT NULL DEFAULT '{}',
    eligibility text NOT NULL CHECK (eligibility IN ('eligible','not_eligible','unknown')),
    apply_url text NOT NULL,
    canonical_url text,
    published_at timestamptz,
    fingerprint bytea NOT NULL CHECK (octet_length(fingerprint)=32),
    status text NOT NULL CHECK (status IN ('active','closed','expired','unknown')),
    source_priority integer NOT NULL DEFAULT 100,
    first_seen_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    closed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX jobs_feed_cursor_idx ON jobs(created_at DESC,id DESC) WHERE status='active';
CREATE INDEX jobs_canonical_url_idx ON jobs(canonical_url) WHERE canonical_url IS NOT NULL;
CREATE INDEX jobs_fingerprint_idx ON jobs(fingerprint);
CREATE INDEX jobs_eligibility_idx ON jobs(eligibility,status);

CREATE TABLE job_sources (
    source_id text NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    external_id text NOT NULL,
    job_id uuid NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    url text NOT NULL,
    first_seen_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    last_seen_run_id uuid REFERENCES source_sync_runs(id),
    missing_count integer NOT NULL DEFAULT 0 CHECK (missing_count >= 0),
    is_active boolean NOT NULL DEFAULT true,
    PRIMARY KEY(source_id,external_id)
);
CREATE INDEX job_sources_job_idx ON job_sources(job_id) WHERE is_active;
CREATE INDEX job_sources_missing_idx ON job_sources(source_id,last_seen_run_id) WHERE is_active;

-- +goose Down
DROP TABLE job_sources;
DROP TABLE jobs;
DROP TABLE companies;
