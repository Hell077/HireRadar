-- +goose Up
ALTER TABLE resumes
    ADD COLUMN processing_started_at timestamptz,
    ADD COLUMN processing_attempts integer NOT NULL DEFAULT 0 CHECK (processing_attempts BETWEEN 0 AND 3),
    ADD COLUMN processing_error text NOT NULL DEFAULT '',
    ADD COLUMN next_processing_at timestamptz NOT NULL DEFAULT now();

CREATE TABLE parsed_resumes (
    resume_id uuid PRIMARY KEY REFERENCES resumes(id) ON DELETE CASCADE,
    extracted_text text NOT NULL,
    skills jsonb NOT NULL DEFAULT '[]'::jsonb,
    positions jsonb NOT NULL DEFAULT '[]'::jsonb,
    total_experience_months integer NOT NULL DEFAULT 0 CHECK (total_experience_months BETWEEN 0 AND 840),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE resume_suggestions (
    id uuid PRIMARY KEY,
    resume_id uuid NOT NULL REFERENCES resumes(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('skill', 'position')),
    value text NOT NULL CHECK (length(value) BETWEEN 1 AND 120),
    normalized_value text NOT NULL CHECK (length(normalized_value) BETWEEN 1 AND 120),
    confidence real NOT NULL CHECK (confidence BETWEEN 0 AND 1),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'rejected')),
    created_at timestamptz NOT NULL DEFAULT now(),
    reviewed_at timestamptz,
    UNIQUE (resume_id, kind, normalized_value)
);

CREATE INDEX resume_suggestions_pending_idx ON resume_suggestions (user_id, created_at DESC) WHERE status='pending';

-- +goose Down
DROP TABLE resume_suggestions;
DROP TABLE parsed_resumes;
ALTER TABLE resumes DROP COLUMN next_processing_at, DROP COLUMN processing_error, DROP COLUMN processing_attempts, DROP COLUMN processing_started_at;
