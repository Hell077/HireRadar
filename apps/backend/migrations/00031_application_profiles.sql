-- +goose Up
ALTER TABLE resumes ADD CONSTRAINT resumes_id_user_id_unique UNIQUE (id, user_id);

CREATE TABLE application_profiles (
    user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    first_name text NOT NULL DEFAULT '',
    last_name text NOT NULL DEFAULT '',
    email text NOT NULL DEFAULT '',
    phone text NOT NULL DEFAULT '',
    country text NOT NULL DEFAULT '',
    city text NOT NULL DEFAULT '',
    linkedin_url text NOT NULL DEFAULT '',
    github_url text NOT NULL DEFAULT '',
    website_url text NOT NULL DEFAULT '',
    resume_id uuid,
    expected_salary_amount bigint,
    expected_salary_currency text,
    notice_period_days integer,
    work_authorization jsonb NOT NULL DEFAULT '[]'::jsonb,
    custom_answers jsonb NOT NULL DEFAULT '{}'::jsonb,
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (resume_id, user_id) REFERENCES resumes(id, user_id) ON DELETE SET NULL (resume_id),
    CHECK (length(first_name) <= 100 AND length(last_name) <= 100),
    CHECK (length(email) <= 320 AND length(phone) <= 40),
    CHECK (length(country) <= 2 AND length(city) <= 120),
    CHECK (length(linkedin_url) <= 2048 AND length(github_url) <= 2048 AND length(website_url) <= 2048),
    CHECK ((expected_salary_amount IS NULL) = (expected_salary_currency IS NULL)),
    CHECK (expected_salary_amount IS NULL OR expected_salary_amount >= 0),
    CHECK (expected_salary_currency IS NULL OR expected_salary_currency ~ '^[A-Z]{3}$'),
    CHECK (notice_period_days IS NULL OR notice_period_days BETWEEN 0 AND 365),
    CHECK (jsonb_typeof(work_authorization) = 'array'),
    CHECK (jsonb_typeof(custom_answers) = 'object')
);

-- +goose Down
DROP TABLE application_profiles;
ALTER TABLE resumes DROP CONSTRAINT resumes_id_user_id_unique;
