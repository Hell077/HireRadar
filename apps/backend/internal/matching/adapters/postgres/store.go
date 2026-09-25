package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	jobdomain "github.com/Hell077/HireRadar/apps/backend/internal/job/domain"
	"github.com/Hell077/HireRadar/apps/backend/internal/matching/engine"
	profiledomain "github.com/Hell077/HireRadar/apps/backend/internal/profile/domain"
	user "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func (s *Store) LoadCandidate(ctx context.Context, userID user.UserID) (engine.Candidate, error) {
	candidate := engine.Candidate{Profile: profiledomain.Profile{UserID: userID}, Preferences: profiledomain.Preferences{
		RemotePolicies: []string{}, EmploymentTypes: []string{}, AllowedRegions: []string{}, ExcludedCountries: []string{},
		MaximumJobAgeDays: 30,
	}}
	err := s.pool.QueryRow(ctx, `SELECT country,seniority FROM user_profiles WHERE user_id=$1`, string(userID)).Scan(&candidate.Profile.Country, &candidate.Profile.Seniority)
	if err != nil && err != pgx.ErrNoRows {
		return engine.Candidate{}, fmt.Errorf("load matching profile: %w", err)
	}
	rows, err := s.pool.Query(ctx, `SELECT s.name,us.years::float8,us.level FROM user_skills us JOIN skills s ON s.id=us.skill_id WHERE us.user_id=$1 ORDER BY s.normalized_name`, string(userID))
	if err != nil {
		return engine.Candidate{}, fmt.Errorf("load matching skills: %w", err)
	}
	for rows.Next() {
		var skill profiledomain.Skill
		if err := rows.Scan(&skill.Name, &skill.Years, &skill.Level); err != nil {
			rows.Close()
			return engine.Candidate{}, err
		}
		candidate.Skills = append(candidate.Skills, skill)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return engine.Candidate{}, err
	}
	rows.Close()
	rows, err = s.pool.Query(ctx, `SELECT title FROM user_positions WHERE user_id=$1 ORDER BY normalized_title`, string(userID))
	if err != nil {
		return engine.Candidate{}, fmt.Errorf("load matching positions: %w", err)
	}
	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err != nil {
			rows.Close()
			return engine.Candidate{}, err
		}
		candidate.Positions = append(candidate.Positions, title)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return engine.Candidate{}, err
	}
	rows.Close()
	var minimum sql.NullInt64
	var currency string
	err = s.pool.QueryRow(ctx, `SELECT remote_policies,employment_types,allowed_regions,excluded_countries,minimum_salary_amount,minimum_salary_currency,minimum_match_score,maximum_job_age_days FROM job_preferences WHERE user_id=$1`, string(userID)).Scan(&candidate.Preferences.RemotePolicies, &candidate.Preferences.EmploymentTypes, &candidate.Preferences.AllowedRegions, &candidate.Preferences.ExcludedCountries, &minimum, &currency, &candidate.Preferences.MinimumMatchScore, &candidate.Preferences.MaximumJobAgeDays)
	if err != nil && err != pgx.ErrNoRows {
		return engine.Candidate{}, fmt.Errorf("load matching preferences: %w", err)
	}
	if minimum.Valid {
		candidate.Preferences.MinimumSalary = &profiledomain.Money{Amount: minimum.Int64, Currency: currency}
	}
	return candidate, nil
}

func (s *Store) CandidateJobs(ctx context.Context, userID user.UserID, candidate engine.Candidate) ([]jobdomain.Job, error) {
	query := `SELECT j.id::text,j.company_id::text,c.name,j.title,j.normalized_title,j.seniority,j.description,
		j.salary_min,j.salary_max,j.salary_currency,j.salary_period,j.employment_types,
		COALESCE((SELECT jsonb_agg(jsonb_build_object('id',sk.id::text,'name',sk.name,'required',js.required,'confidence',js.confidence)) FROM job_skills js JOIN skills sk ON sk.id=js.skill_id WHERE js.job_id=j.id),'[]'::jsonb)::text,
		j.remote_policy,j.location,j.location_countries,j.eligibility,j.apply_url,j.published_at,j.first_seen_at,j.last_seen_at,j.status,j.source_priority,j.created_at,j.updated_at
		FROM jobs j JOIN companies c ON c.id=j.company_id
		WHERE j.status='active'
		AND NOT ($2='KZ' AND j.eligibility='not_eligible')
		AND ($2='' OR cardinality(j.location_countries)=0 OR $2=ANY(j.location_countries))
		AND NOT (j.location_countries && $3::text[])
		AND (cardinality($4::text[])=0 OR j.remote_policy=ANY($4::text[]))
		AND (cardinality($5::text[])=0 OR j.employment_types && $5::text[])
		AND (j.published_at IS NULL OR j.published_at >= now()-($6::int*interval '1 day'))
		AND (j.published_at IS NOT NULL OR j.first_seen_at >= now()-($6::int*interval '1 day'))
		AND (NOT EXISTS(SELECT 1 FROM user_source_preferences usp WHERE usp.user_id=$1)
		 OR EXISTS(SELECT 1 FROM job_sources js WHERE js.job_id=j.id AND js.is_active AND COALESCE((SELECT usp.enabled FROM user_source_preferences usp WHERE usp.user_id=$1 AND usp.source_id=js.source_id),true)))
		ORDER BY j.created_at DESC,j.id DESC LIMIT 5000`
	rows, err := s.pool.Query(ctx, query, string(userID), candidate.Profile.Country, candidate.Preferences.ExcludedCountries, candidate.Preferences.RemotePolicies, candidate.Preferences.EmploymentTypes, candidate.Preferences.MaximumJobAgeDays)
	if err != nil {
		return nil, fmt.Errorf("preselect candidate jobs: %w", err)
	}
	defer rows.Close()
	jobs := []jobdomain.Job{}
	for rows.Next() {
		var job jobdomain.Job
		var skillsJSON []byte
		var minimum, maximum *float64
		var currency, period *string
		if err := rows.Scan(&job.ID, &job.CompanyID, &job.Company, &job.Title, &job.NormalizedTitle, &job.Seniority, &job.Description, &minimum, &maximum, &currency, &period, &job.EmploymentTypes, &skillsJSON, &job.RemotePolicy, &job.Location, &job.Countries, &job.Eligibility, &job.ApplyURL, &job.PublishedAt, &job.FirstSeenAt, &job.LastSeenAt, &job.Status, &job.SourcePriority, &job.CreatedAt, &job.UpdatedAt); err != nil {
			return nil, err
		}
		if minimum != nil && maximum != nil && currency != nil && period != nil {
			job.Salary = &jobdomain.SalaryRange{Minimum: *minimum, Maximum: *maximum, Currency: *currency, Period: *period}
		}
		if err := json.Unmarshal(skillsJSON, &job.Skills); err != nil {
			return nil, fmt.Errorf("decode preselected job skills: %w", err)
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (s *Store) CandidateIDs(ctx context.Context) ([]user.UserID, error) {
	rows, err := s.pool.Query(ctx, `SELECT u.id::text FROM users u WHERE u.status='active' AND (
		EXISTS(SELECT 1 FROM user_profiles p WHERE p.user_id=u.id)
		OR EXISTS(SELECT 1 FROM user_skills us WHERE us.user_id=u.id)
		OR EXISTS(SELECT 1 FROM user_positions up WHERE up.user_id=u.id)
		OR EXISTS(SELECT 1 FROM job_preferences jp WHERE jp.user_id=u.id)) ORDER BY u.id`)
	if err != nil {
		return nil, fmt.Errorf("list candidate accounts: %w", err)
	}
	defer rows.Close()
	ids := []user.UserID{}
	for rows.Next() {
		var id user.UserID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) Job(ctx context.Context, id string) (jobdomain.Job, error) {
	var job jobdomain.Job
	var skillsJSON []byte
	var minimum, maximum *float64
	var currency, period *string
	err := s.pool.QueryRow(ctx, `SELECT j.id::text,j.company_id::text,c.name,j.title,j.normalized_title,j.seniority,j.description,
		j.salary_min,j.salary_max,j.salary_currency,j.salary_period,j.employment_types,
		COALESCE((SELECT jsonb_agg(jsonb_build_object('id',sk.id::text,'name',sk.name,'required',js.required,'confidence',js.confidence)) FROM job_skills js JOIN skills sk ON sk.id=js.skill_id WHERE js.job_id=j.id),'[]'::jsonb)::text,
		j.remote_policy,j.location,j.location_countries,j.eligibility,j.apply_url,j.published_at,j.first_seen_at,j.last_seen_at,j.status,j.source_priority,j.created_at,j.updated_at
		FROM jobs j JOIN companies c ON c.id=j.company_id WHERE j.id=$1`, id).Scan(&job.ID, &job.CompanyID, &job.Company, &job.Title, &job.NormalizedTitle, &job.Seniority, &job.Description, &minimum, &maximum, &currency, &period, &job.EmploymentTypes, &skillsJSON, &job.RemotePolicy, &job.Location, &job.Countries, &job.Eligibility, &job.ApplyURL, &job.PublishedAt, &job.FirstSeenAt, &job.LastSeenAt, &job.Status, &job.SourcePriority, &job.CreatedAt, &job.UpdatedAt)
	if err != nil {
		return jobdomain.Job{}, fmt.Errorf("load job for rematching: %w", err)
	}
	if minimum != nil && maximum != nil && currency != nil && period != nil {
		job.Salary = &jobdomain.SalaryRange{Minimum: *minimum, Maximum: *maximum, Currency: *currency, Period: *period}
	}
	if err := json.Unmarshal(skillsJSON, &job.Skills); err != nil {
		return jobdomain.Job{}, fmt.Errorf("decode rematching job skills: %w", err)
	}
	return job, nil
}

func (s *Store) SaveJobMatch(ctx context.Context, userID user.UserID, result engine.Result) error {
	if !result.Eligible {
		if _, err := s.pool.Exec(ctx, `DELETE FROM user_job_matches WHERE user_id=$1 AND job_id=$2`, string(userID), result.JobID); err != nil {
			return fmt.Errorf("remove ineligible job match: %w", err)
		}
		return nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin job match save: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := upsertMatch(ctx, tx, userID, result); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit job match: %w", err)
	}
	return nil
}

func (s *Store) SaveMatches(ctx context.Context, userID user.UserID, matches []engine.Result) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin match refresh: %w", err)
	}
	defer tx.Rollback(ctx)
	ids := make([]string, 0, len(matches))
	for _, match := range matches {
		ids = append(ids, match.JobID)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM user_job_matches WHERE user_id=$1 AND NOT(job_id=ANY($2::uuid[]))`, string(userID), ids); err != nil {
		return fmt.Errorf("remove stale matches: %w", err)
	}
	for _, match := range matches {
		if err := upsertMatch(ctx, tx, userID, match); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit match refresh: %w", err)
	}
	return nil
}

type matchExecer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func upsertMatch(ctx context.Context, exec matchExecer, userID user.UserID, match engine.Result) error {
	components, err := json.Marshal(match.Components)
	if err != nil {
		return fmt.Errorf("encode match explanation: %w", err)
	}
	tag, err := exec.Exec(ctx, `INSERT INTO user_job_matches(user_id,job_id,score,components) VALUES($1,$2,$3,$4::jsonb)
		ON CONFLICT(user_id,job_id) DO NOTHING`, string(userID), match.JobID, match.Score, components)
	if err != nil {
		return fmt.Errorf("save match: %w", err)
	}
	if tag.RowsAffected() == 0 {
		if _, err := exec.Exec(ctx, `UPDATE user_job_matches SET score=$3,components=$4::jsonb,computed_at=now() WHERE user_id=$1 AND job_id=$2`, string(userID), match.JobID, match.Score, components); err != nil {
			return fmt.Errorf("update match: %w", err)
		}
		return nil
	}
	if _, err := exec.Exec(ctx, `INSERT INTO outbox_events(id,event_type,aggregate_type,aggregate_id,payload)
		VALUES($1,'match.created','match',$2,jsonb_build_object('user_id',$3::text,'job_id',$2::text,'score',$4::int))`, uuid.NewString(), match.JobID, string(userID), match.Score); err != nil {
		return fmt.Errorf("emit job matched event: %w", err)
	}
	return nil
}

func (s *Store) ListMatches(ctx context.Context, userID user.UserID, limit int) ([]engine.Result, error) {
	rows, err := s.pool.Query(ctx, `SELECT job_id::text,score,components::text FROM user_job_matches WHERE user_id=$1 ORDER BY score DESC,computed_at DESC,job_id LIMIT $2`, string(userID), limit)
	if err != nil {
		return nil, fmt.Errorf("list candidate matches: %w", err)
	}
	defer rows.Close()
	result := []engine.Result{}
	for rows.Next() {
		var item engine.Result
		var components []byte
		if err := rows.Scan(&item.JobID, &item.Score, &components); err != nil {
			return nil, err
		}
		item.Eligible = true
		if err := json.Unmarshal(components, &item.Components); err != nil {
			return nil, fmt.Errorf("decode match explanation: %w", err)
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
