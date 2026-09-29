-- +goose Up
ALTER TABLE user_profiles ADD COLUMN match_version bigint NOT NULL DEFAULT 1;
ALTER TABLE jobs ADD COLUMN match_version bigint NOT NULL DEFAULT 1;
ALTER TABLE user_job_matches
    ADD COLUMN candidate_version bigint NOT NULL DEFAULT 0,
    ADD COLUMN job_version bigint NOT NULL DEFAULT 0;

UPDATE user_job_matches m
SET candidate_version=COALESCE((SELECT p.match_version FROM user_profiles p WHERE p.user_id=m.user_id),0),
    job_version=(SELECT j.match_version FROM jobs j WHERE j.id=m.job_id);

-- +goose StatementBegin
CREATE FUNCTION bump_profile_match_version() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    owner_id uuid;
BEGIN
    IF TG_TABLE_NAME = 'parsed_resumes' THEN
        SELECT user_id INTO owner_id FROM resumes WHERE id=COALESCE(NEW.resume_id, OLD.resume_id);
    ELSE
        owner_id := COALESCE(NEW.user_id, OLD.user_id);
    END IF;
    INSERT INTO user_profiles(user_id,match_version)
    VALUES(owner_id,1)
    ON CONFLICT(user_id) DO UPDATE
    SET match_version=user_profiles.match_version+1,updated_at=now();
    RETURN COALESCE(NEW,OLD);
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION version_profile_row() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF ROW(NEW.country,NEW.seniority,NEW.experience_years,NEW.desired_salary_amount,NEW.desired_salary_currency)
       IS DISTINCT FROM ROW(OLD.country,OLD.seniority,OLD.experience_years,OLD.desired_salary_amount,OLD.desired_salary_currency) THEN
        NEW.match_version := OLD.match_version+1;
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER user_profiles_match_version
BEFORE UPDATE ON user_profiles FOR EACH ROW EXECUTE FUNCTION version_profile_row();
CREATE TRIGGER user_skills_match_version
AFTER INSERT OR UPDATE OR DELETE ON user_skills FOR EACH ROW EXECUTE FUNCTION bump_profile_match_version();
CREATE TRIGGER user_positions_match_version
AFTER INSERT OR UPDATE OR DELETE ON user_positions FOR EACH ROW EXECUTE FUNCTION bump_profile_match_version();
CREATE TRIGGER job_preferences_match_version
AFTER INSERT OR UPDATE OR DELETE ON job_preferences FOR EACH ROW EXECUTE FUNCTION bump_profile_match_version();
CREATE TRIGGER source_preferences_match_version
AFTER INSERT OR UPDATE OR DELETE ON user_source_preferences FOR EACH ROW EXECUTE FUNCTION bump_profile_match_version();
CREATE TRIGGER parsed_resumes_match_version
AFTER INSERT OR UPDATE OR DELETE ON parsed_resumes FOR EACH ROW EXECUTE FUNCTION bump_profile_match_version();

-- +goose StatementBegin
CREATE FUNCTION version_job_row() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF ROW(NEW.title,NEW.normalized_title,NEW.description,NEW.seniority,NEW.salary_min,NEW.salary_max,
           NEW.salary_currency,NEW.salary_period,NEW.employment_types,NEW.remote_policy,NEW.location,
           NEW.location_countries,NEW.eligibility,NEW.status)
       IS DISTINCT FROM ROW(OLD.title,OLD.normalized_title,OLD.description,OLD.seniority,OLD.salary_min,OLD.salary_max,
           OLD.salary_currency,OLD.salary_period,OLD.employment_types,OLD.remote_policy,OLD.location,
           OLD.location_countries,OLD.eligibility,OLD.status) THEN
        NEW.match_version := OLD.match_version+1;
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION bump_job_match_version() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    UPDATE jobs SET match_version=match_version+1 WHERE id=COALESCE(NEW.job_id,OLD.job_id);
    RETURN COALESCE(NEW,OLD);
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER jobs_match_version
BEFORE UPDATE ON jobs FOR EACH ROW EXECUTE FUNCTION version_job_row();
CREATE TRIGGER job_skills_match_version
AFTER INSERT OR UPDATE OR DELETE ON job_skills FOR EACH ROW EXECUTE FUNCTION bump_job_match_version();

-- +goose Down
DROP TRIGGER job_skills_match_version ON job_skills;
DROP TRIGGER jobs_match_version ON jobs;
DROP FUNCTION bump_job_match_version();
DROP FUNCTION version_job_row();
DROP TRIGGER parsed_resumes_match_version ON parsed_resumes;
DROP TRIGGER source_preferences_match_version ON user_source_preferences;
DROP TRIGGER job_preferences_match_version ON job_preferences;
DROP TRIGGER user_positions_match_version ON user_positions;
DROP TRIGGER user_skills_match_version ON user_skills;
DROP TRIGGER user_profiles_match_version ON user_profiles;
DROP FUNCTION version_profile_row();
DROP FUNCTION bump_profile_match_version();
ALTER TABLE user_job_matches DROP COLUMN job_version,DROP COLUMN candidate_version;
ALTER TABLE jobs DROP COLUMN match_version;
ALTER TABLE user_profiles DROP COLUMN match_version;
