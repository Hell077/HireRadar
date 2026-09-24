-- +goose Up
CREATE TABLE user_profiles (
    user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    first_name text NOT NULL DEFAULT '',
    last_name text NOT NULL DEFAULT '',
    country text NOT NULL DEFAULT '',
    city text NOT NULL DEFAULT '',
    timezone text NOT NULL DEFAULT '',
    experience_years integer NOT NULL DEFAULT 0 CHECK (experience_years BETWEEN 0 AND 70),
    seniority text NOT NULL DEFAULT '' CHECK (seniority IN ('', 'intern', 'junior', 'middle', 'senior', 'staff', 'principal', 'lead')),
    desired_salary_amount bigint CHECK (desired_salary_amount >= 0),
    desired_salary_currency text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (country = '' OR country ~ '^[A-Z]{2}$'),
    CHECK (desired_salary_currency = '' OR desired_salary_currency ~ '^[A-Z]{3}$'),
    CHECK ((desired_salary_amount IS NULL) = (desired_salary_currency = ''))
);

CREATE TABLE skills (
    id uuid PRIMARY KEY,
    name text NOT NULL,
    normalized_name text NOT NULL UNIQUE,
    CHECK (length(name) BETWEEN 1 AND 100),
    CHECK (length(normalized_name) BETWEEN 1 AND 100)
);

CREATE TABLE skill_aliases (
    id uuid PRIMARY KEY,
    skill_id uuid NOT NULL REFERENCES skills(id) ON DELETE CASCADE,
    alias text NOT NULL,
    normalized_alias text NOT NULL UNIQUE,
    CHECK (length(alias) BETWEEN 1 AND 100)
);

CREATE TABLE user_skills (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    skill_id uuid NOT NULL REFERENCES skills(id),
    years numeric(4,1) CHECK (years BETWEEN 0 AND 70),
    level text NOT NULL DEFAULT '' CHECK (level IN ('', 'beginner', 'intermediate', 'advanced', 'expert')),
    source text NOT NULL DEFAULT 'manual' CHECK (source IN ('manual', 'resume', 'inferred')),
    confirmed boolean NOT NULL DEFAULT true,
    PRIMARY KEY (user_id, skill_id)
);

CREATE TABLE user_positions (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title text NOT NULL,
    normalized_title text NOT NULL,
    UNIQUE (user_id, normalized_title),
    CHECK (length(title) BETWEEN 1 AND 120)
);

CREATE TABLE job_preferences (
    user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    remote_policies text[] NOT NULL DEFAULT '{}',
    employment_types text[] NOT NULL DEFAULT '{}',
    allowed_regions text[] NOT NULL DEFAULT '{}',
    excluded_countries text[] NOT NULL DEFAULT '{}',
    minimum_salary_amount bigint CHECK (minimum_salary_amount >= 0),
    minimum_salary_currency text NOT NULL DEFAULT '',
    minimum_match_score integer NOT NULL DEFAULT 0 CHECK (minimum_match_score BETWEEN 0 AND 100),
    maximum_job_age_days integer NOT NULL DEFAULT 30 CHECK (maximum_job_age_days BETWEEN 1 AND 365),
    notifications_enabled boolean NOT NULL DEFAULT true,
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((minimum_salary_amount IS NULL) = (minimum_salary_currency = ''))
);

CREATE TABLE user_source_preferences (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source_id text NOT NULL,
    enabled boolean NOT NULL,
    PRIMARY KEY (user_id, source_id),
    CHECK (source_id ~ '^[a-z0-9][a-z0-9_-]{0,99}$')
);

-- +goose Down
DROP TABLE user_source_preferences;
DROP TABLE job_preferences;
DROP TABLE user_positions;
DROP TABLE user_skills;
DROP TABLE skill_aliases;
DROP TABLE skills;
DROP TABLE user_profiles;
