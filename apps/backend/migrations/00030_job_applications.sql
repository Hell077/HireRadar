-- +goose Up
CREATE TABLE job_applications (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    job_id uuid NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    provider text NOT NULL CHECK (length(trim(provider)) > 0),
    status text NOT NULL CHECK (status IN (
        'requested', 'preparing', 'needs_input', 'ready', 'submitting',
        'submitted', 'manual_required', 'failed', 'cancelled'
    )),
    requested_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    submitted_at timestamptz,
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    locked_by text,
    locked_until timestamptz,
    last_error_code text,
    last_error_message text,
    form_snapshot jsonb,
    result jsonb,
    UNIQUE (user_id, job_id),
    CHECK ((locked_by IS NULL) = (locked_until IS NULL)),
    CHECK ((status = 'submitted') = (submitted_at IS NOT NULL))
);

CREATE INDEX job_applications_pending_idx
    ON job_applications (requested_at, id)
    WHERE status IN ('requested', 'preparing', 'ready', 'submitting');
CREATE INDEX job_applications_lease_idx
    ON job_applications (locked_until, id)
    WHERE locked_until IS NOT NULL;
CREATE INDEX job_applications_user_idx
    ON job_applications (user_id, requested_at DESC);

CREATE TABLE job_application_attempts (
    id uuid PRIMARY KEY,
    application_id uuid NOT NULL REFERENCES job_applications(id) ON DELETE CASCADE,
    attempt_number integer NOT NULL CHECK (attempt_number > 0),
    status text NOT NULL CHECK (status IN (
        'running', 'succeeded', 'retryable', 'failed', 'needs_input', 'manual_required', 'uncertain'
    )),
    provider text NOT NULL CHECK (length(trim(provider)) > 0),
    started_at timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz,
    error_code text,
    error_message text,
    result jsonb,
    UNIQUE (application_id, attempt_number),
    CHECK ((status = 'running') = (finished_at IS NULL))
);

CREATE INDEX job_application_attempts_history_idx
    ON job_application_attempts (application_id, attempt_number DESC);

CREATE TABLE job_application_questions (
    id uuid PRIMARY KEY,
    application_id uuid NOT NULL REFERENCES job_applications(id) ON DELETE CASCADE,
    external_key text,
    question text NOT NULL CHECK (length(trim(question)) > 0),
    question_type text NOT NULL CHECK (length(trim(question_type)) > 0),
    required boolean NOT NULL DEFAULT true,
    options jsonb,
    answer jsonb,
    answer_source text CHECK (answer_source IS NULL OR answer_source IN (
        'profile', 'resume', 'saved_answer', 'user', 'generated'
    )),
    status text NOT NULL CHECK (status IN ('unanswered', 'answered', 'skipped', 'invalid')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((status = 'answered') = (answer IS NOT NULL))
);

CREATE UNIQUE INDEX job_application_questions_external_idx
    ON job_application_questions (application_id, external_key)
    WHERE external_key IS NOT NULL;
CREATE INDEX job_application_questions_unanswered_idx
    ON job_application_questions (application_id, id)
    WHERE status = 'unanswered' AND required;

-- +goose Down
DROP TABLE job_application_questions;
DROP TABLE job_application_attempts;
DROP TABLE job_applications;
