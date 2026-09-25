-- +goose Up
ALTER TABLE jobs ADD COLUMN seniority text NOT NULL DEFAULT 'unknown'
    CHECK (seniority IN ('intern','junior','mid','senior','staff','principal','lead','manager','director','executive','unknown'));

CREATE TABLE job_skills (
    job_id uuid NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    skill_id uuid NOT NULL REFERENCES skills(id),
    required boolean NOT NULL DEFAULT false,
    confidence real NOT NULL CHECK (confidence BETWEEN 0 AND 1),
    PRIMARY KEY(job_id,skill_id)
);
CREATE INDEX job_skills_skill_idx ON job_skills(skill_id,job_id);

-- +goose Down
DROP TABLE job_skills;
ALTER TABLE jobs DROP COLUMN seniority;
