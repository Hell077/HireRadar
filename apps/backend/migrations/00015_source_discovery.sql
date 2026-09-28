-- +goose Up
CREATE TABLE discovery_sources (
    id text PRIMARY KEY CHECK (id ~ '^[a-z0-9][a-z0-9_-]{1,99}$'),
    name text NOT NULL CHECK (length(trim(name)) BETWEEN 1 AND 200),
    repo_owner text NOT NULL,
    repo_name text NOT NULL,
    parser_key text NOT NULL CHECK (parser_key IN ('remoteintech','established_remote','global_hiring','awesome_remote_job','european_remote','remote_by_default','remote_freelancer','remote_developer_directory','github_issue_boards')),
    enabled boolean NOT NULL DEFAULT true,
    interval_seconds integer NOT NULL DEFAULT 86400 CHECK (interval_seconds BETWEEN 3600 AND 2592000),
    next_run_at timestamptz NOT NULL DEFAULT now(),
    etag text,
    last_commit_sha text,
    last_run_at timestamptz,
    last_status text CHECK (last_status IS NULL OR last_status IN ('succeeded','not_modified','failed')),
    last_error text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(repo_owner,repo_name)
);
CREATE INDEX discovery_sources_due_idx ON discovery_sources(next_run_at,id) WHERE enabled;

CREATE TABLE discovery_runs (
    id uuid PRIMARY KEY,
    discovery_source_id text NOT NULL REFERENCES discovery_sources(id) ON DELETE CASCADE,
    status text NOT NULL CHECK (status IN ('running','succeeded','not_modified','failed')),
    started_at timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz,
    targets_seen integer NOT NULL DEFAULT 0 CHECK (targets_seen >= 0),
    candidates_created integer NOT NULL DEFAULT 0 CHECK (candidates_created >= 0),
    candidates_reused integer NOT NULL DEFAULT 0 CHECK (candidates_reused >= 0),
    provenance_added integer NOT NULL DEFAULT 0 CHECK (provenance_added >= 0),
    error_message text
);
CREATE INDEX discovery_runs_source_started_idx ON discovery_runs(discovery_source_id,started_at DESC);

CREATE TABLE source_candidates (
    id uuid PRIMARY KEY,
    kind text NOT NULL CHECK (kind IN ('company','job_board','freelance_platform','github_jobs')),
    name text NOT NULL CHECK (length(trim(name)) BETWEEN 1 AND 300),
    normalized_name text NOT NULL,
    official_domain text,
    website_url text,
    careers_url text,
    regions text[] NOT NULL DEFAULT '{}',
    technologies text[] NOT NULL DEFAULT '{}',
    discovery_metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    status text NOT NULL DEFAULT 'discovered' CHECK (status IN ('discovered','resolving','verified','source_created','manual_review','unsupported','temporarily_unavailable','invalid')),
    detected_provider text,
    provider_key text,
    provider_url text,
    provider_confidence real,
    provider_evidence jsonb NOT NULL DEFAULT '[]'::jsonb,
    source_id text REFERENCES sources(id) ON DELETE SET NULL,
    last_checked_at timestamptz,
    next_check_at timestamptz NOT NULL DEFAULT now(),
    last_error text,
    first_seen_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX source_candidates_domain_uniq ON source_candidates(kind,official_domain) WHERE official_domain IS NOT NULL;
CREATE INDEX source_candidates_name_idx ON source_candidates(kind,normalized_name);
CREATE INDEX source_candidates_due_idx ON source_candidates(next_check_at,id) WHERE status IN ('discovered','resolving','temporarily_unavailable');
CREATE INDEX source_candidates_provider_idx ON source_candidates(detected_provider,status);

CREATE TABLE candidate_provenance (
    discovery_source_id text NOT NULL REFERENCES discovery_sources(id) ON DELETE CASCADE,
    external_key text NOT NULL,
    candidate_id uuid NOT NULL REFERENCES source_candidates(id) ON DELETE CASCADE,
    source_url text NOT NULL DEFAULT '',
    raw_metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    first_seen_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(discovery_source_id,external_key)
);
CREATE INDEX candidate_provenance_candidate_idx ON candidate_provenance(candidate_id);

-- A provider board may be discovered from multiple catalogs. Treat that as one JobSource.
CREATE UNIQUE INDEX sources_provider_identity_uniq
    ON sources(source_type, (COALESCE(config->>'board', (config->>'owner') || '/' || (config->>'repo'))))
    WHERE config ? 'board' OR (config ? 'owner' AND config ? 'repo');

-- +goose Down
DROP INDEX sources_provider_identity_uniq;
DROP TABLE candidate_provenance;
DROP TABLE source_candidates;
DROP TABLE discovery_runs;
DROP TABLE discovery_sources;
