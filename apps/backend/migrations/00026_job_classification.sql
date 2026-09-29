-- +goose Up
ALTER TABLE jobs
    ADD COLUMN job_family text NOT NULL DEFAULT 'unknown'
        CHECK (job_family IN ('software_engineering','data','devops','security','qa','product','design','sales','marketing','support','hr','finance','operations','management','other','unknown')),
    ADD COLUMN job_speciality text NOT NULL DEFAULT '' CHECK (length(job_speciality) <= 100),
    ADD COLUMN job_family_confidence real NOT NULL DEFAULT 0 CHECK (job_family_confidence BETWEEN 0 AND 1),
    ADD COLUMN seniority_confidence real NOT NULL DEFAULT 0 CHECK (seniority_confidence BETWEEN 0 AND 1),
    ADD COLUMN location_confidence real NOT NULL DEFAULT 0 CHECK (location_confidence BETWEEN 0 AND 1),
    ADD COLUMN salary_confidence real NOT NULL DEFAULT 0 CHECK (salary_confidence BETWEEN 0 AND 1);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION version_job_row() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF ROW(NEW.title,NEW.normalized_title,NEW.description,NEW.seniority,NEW.salary_min,NEW.salary_max,
           NEW.salary_currency,NEW.salary_period,NEW.employment_types,NEW.remote_policy,NEW.location,
           NEW.location_countries,NEW.eligibility,NEW.status,NEW.job_family,NEW.job_speciality,NEW.job_family_confidence,
           NEW.seniority_confidence,NEW.location_confidence,NEW.salary_confidence)
       IS DISTINCT FROM ROW(OLD.title,OLD.normalized_title,OLD.description,OLD.seniority,OLD.salary_min,OLD.salary_max,
           OLD.salary_currency,OLD.salary_period,OLD.employment_types,OLD.remote_policy,OLD.location,
           OLD.location_countries,OLD.eligibility,OLD.status,OLD.job_family,OLD.job_speciality,OLD.job_family_confidence,
           OLD.seniority_confidence,OLD.location_confidence,OLD.salary_confidence) THEN
        NEW.match_version := OLD.match_version+1;
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- +goose Down
ALTER TABLE jobs DROP COLUMN salary_confidence,DROP COLUMN location_confidence,DROP COLUMN seniority_confidence,
    DROP COLUMN job_family_confidence,DROP COLUMN job_speciality,DROP COLUMN job_family;
