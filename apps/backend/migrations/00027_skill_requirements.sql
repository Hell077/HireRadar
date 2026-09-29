-- +goose Up
ALTER TABLE job_skills
    ADD COLUMN minimum_years numeric(4,1) CHECK (minimum_years BETWEEN 0 AND 70),
    ADD COLUMN minimum_level text NOT NULL DEFAULT ''
        CHECK (minimum_level IN ('','beginner','intermediate','advanced','expert'));

-- +goose Down
ALTER TABLE job_skills DROP COLUMN minimum_level,DROP COLUMN minimum_years;
