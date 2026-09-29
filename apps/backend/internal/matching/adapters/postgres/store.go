package postgres

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	jobdomain "github.com/Hell077/HireRadar/apps/backend/internal/job/domain"
	"github.com/Hell077/HireRadar/apps/backend/internal/language"
	matchapp "github.com/Hell077/HireRadar/apps/backend/internal/matching/application"
	"github.com/Hell077/HireRadar/apps/backend/internal/matching/engine"
	"github.com/Hell077/HireRadar/apps/backend/internal/observability"
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
	candidates, err := s.LoadCandidates(ctx, []user.UserID{userID})
	if err != nil {
		return engine.Candidate{}, err
	}
	return candidates[userID], nil
}

// LoadCandidates loads one page of profiles with a fixed number of queries.
// Job rematching can therefore scale with pages, not with one query bundle per user.
func (s *Store) LoadCandidates(ctx context.Context, userIDs []user.UserID) (map[user.UserID]engine.Candidate, error) {
	candidates := make(map[user.UserID]engine.Candidate, len(userIDs))
	if len(userIDs) == 0 {
		return candidates, nil
	}
	ids := make([]string, len(userIDs))
	for i, id := range userIDs {
		ids[i] = string(id)
		candidates[id] = engine.Candidate{Profile: profiledomain.Profile{UserID: id}, Preferences: profiledomain.Preferences{
			RemotePolicies: []string{}, EmploymentTypes: []string{}, AllowedRegions: []string{}, ExcludedCountries: []string{}, MaximumJobAgeDays: 30,
		}}
	}
	rows, err := s.pool.Query(ctx, `SELECT user_id::text,country,seniority,match_version FROM user_profiles WHERE user_id=ANY($1::uuid[])`, ids)
	if err != nil {
		return nil, fmt.Errorf("load matching profiles: %w", err)
	}
	for rows.Next() {
		var id string
		var c engine.Candidate
		if err := rows.Scan(&id, &c.Profile.Country, &c.Profile.Seniority, &c.MatchVersion); err != nil {
			rows.Close()
			return nil, err
		}
		existing := candidates[user.UserID(id)]
		existing.Profile.Country = c.Profile.Country
		existing.Profile.Seniority = c.Profile.Seniority
		existing.MatchVersion = c.MatchVersion
		candidates[user.UserID(id)] = existing
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	rows, err = s.pool.Query(ctx, `SELECT us.user_id::text,s.name,us.years::float8,us.level,us.source,us.confirmed FROM user_skills us JOIN skills s ON s.id=us.skill_id WHERE us.user_id=ANY($1::uuid[]) ORDER BY us.user_id,s.normalized_name`, ids)
	if err != nil {
		return nil, fmt.Errorf("load matching skills: %w", err)
	}
	for rows.Next() {
		var id string
		var skill profiledomain.Skill
		if err := rows.Scan(&id, &skill.Name, &skill.Years, &skill.Level, &skill.Source, &skill.Confirmed); err != nil {
			rows.Close()
			return nil, err
		}
		skill.Confidence = candidateSkillConfidence(skill.Source, skill.Confirmed)
		c := candidates[user.UserID(id)]
		c.Skills = append(c.Skills, skill)
		candidates[user.UserID(id)] = c
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	rows, err = s.pool.Query(ctx, `SELECT user_id::text,title FROM user_positions WHERE user_id=ANY($1::uuid[]) ORDER BY user_id,normalized_title`, ids)
	if err != nil {
		return nil, fmt.Errorf("load matching positions: %w", err)
	}
	for rows.Next() {
		var id, title string
		if err := rows.Scan(&id, &title); err != nil {
			rows.Close()
			return nil, err
		}
		c := candidates[user.UserID(id)]
		c.Positions = append(c.Positions, title)
		candidates[user.UserID(id)] = c
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	rows, err = s.pool.Query(ctx, `SELECT DISTINCT ON (r.user_id) r.user_id::text,p.positions,p.languages,p.extracted_text FROM parsed_resumes p JOIN resumes r ON r.id=p.resume_id WHERE r.user_id=ANY($1::uuid[]) AND r.status='processed' ORDER BY r.user_id,p.created_at DESC`, ids)
	if err != nil {
		return nil, fmt.Errorf("load resume matching signals: %w", err)
	}
	for rows.Next() {
		var id string
		var positionsJSON, languagesJSON []byte
		var text string
		if err := rows.Scan(&id, &positionsJSON, &languagesJSON, &text); err != nil {
			rows.Close()
			return nil, err
		}
		c := candidates[user.UserID(id)]
		var positions []struct {
			Title string `json:"title"`
		}
		if err := json.Unmarshal(positionsJSON, &positions); err != nil {
			rows.Close()
			return nil, fmt.Errorf("decode resume positions for matching: %w", err)
		}
		for _, p := range positions {
			if p.Title != "" && !containsPosition(c.Positions, p.Title) {
				c.Positions = append(c.Positions, p.Title)
			}
		}
		if err := json.Unmarshal(languagesJSON, &c.Languages); err != nil {
			rows.Close()
			return nil, fmt.Errorf("decode resume languages for matching: %w", err)
		}
		if len(c.Languages) == 0 {
			c.Languages = language.ResumeLanguages(text)
		}
		candidates[user.UserID(id)] = c
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	rows, err = s.pool.Query(ctx, `SELECT user_id::text,remote_policies,employment_types,allowed_regions,excluded_countries,minimum_salary_amount,minimum_salary_currency,minimum_match_score,maximum_job_age_days FROM job_preferences WHERE user_id=ANY($1::uuid[])`, ids)
	if err != nil {
		return nil, fmt.Errorf("load matching preferences: %w", err)
	}
	for rows.Next() {
		var id, currency string
		var minimum sql.NullInt64
		var prefs profiledomain.Preferences
		if err := rows.Scan(&id, &prefs.RemotePolicies, &prefs.EmploymentTypes, &prefs.AllowedRegions, &prefs.ExcludedCountries, &minimum, &currency, &prefs.MinimumMatchScore, &prefs.MaximumJobAgeDays); err != nil {
			rows.Close()
			return nil, err
		}
		if minimum.Valid {
			prefs.MinimumSalary = &profiledomain.Money{Amount: minimum.Int64, Currency: currency}
		}
		c := candidates[user.UserID(id)]
		c.Preferences = prefs
		candidates[user.UserID(id)] = c
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	return candidates, nil
}

func candidateSkillConfidence(source string, confirmed bool) float64 {
	if !confirmed {
		return 0.45
	}
	switch source {
	case "manual":
		return 1
	case "resume":
		return 0.82
	case "inferred":
		return 0.65
	default:
		return 0.7
	}
}

func containsPosition(values []string, value string) bool {
	for _, item := range values {
		if strings.EqualFold(strings.TrimSpace(item), strings.TrimSpace(value)) {
			return true
		}
	}
	return false
}

func (s *Store) CandidateJobPage(ctx context.Context, userID user.UserID, candidate engine.Candidate, cursor string, limit int) (matchapp.JobPage, error) {
	if limit <= 0 || limit > 1000 {
		limit = 500
	}
	var afterCreated any
	var afterID any
	if cursor != "" {
		created, id, err := decodeJobCursor(cursor)
		if err != nil {
			return matchapp.JobPage{}, err
		}
		afterCreated, afterID = created, id
	}
	query := `SELECT j.id::text,j.match_version,j.company_id::text,c.name,j.title,j.normalized_title,j.seniority,j.job_family,j.job_speciality,j.job_family_confidence,j.seniority_confidence,j.location_confidence,j.salary_confidence,j.description,
		j.salary_min,j.salary_max,j.salary_currency,j.salary_period,j.employment_types,
		COALESCE((SELECT jsonb_agg(jsonb_build_object('id',sk.id::text,'name',sk.name,'required',js.required,'confidence',js.confidence,'minimum_years',js.minimum_years,'minimum_level',js.minimum_level)) FROM job_skills js JOIN skills sk ON sk.id=js.skill_id WHERE js.job_id=j.id),'[]'::jsonb)::text,
		j.remote_policy,j.location,j.location_countries,j.eligibility,j.apply_url,j.published_at,j.first_seen_at,j.last_seen_at,j.status,j.source_priority,j.created_at,j.updated_at
		FROM jobs j JOIN companies c ON c.id=j.company_id
		WHERE j.status='active'
		AND NOT ($2='KZ' AND j.eligibility='not_eligible')
		AND ($2='' OR cardinality(j.location_countries)=0 OR $2=ANY(j.location_countries))
		AND NOT (j.location_countries && $3::text[])
		AND (cardinality($4::text[])=0 OR j.remote_policy=ANY($4::text[])
			OR ('worldwide'=ANY($4::text[]) AND j.remote_policy='remote' AND cardinality(j.location_countries)=0))
		AND (cardinality($5::text[])=0 OR j.employment_types && $5::text[])
		AND (j.published_at IS NULL OR j.published_at >= now()-($6::int*interval '1 day'))
		AND (j.published_at IS NOT NULL OR j.first_seen_at >= now()-($6::int*interval '1 day'))
		AND NOT EXISTS(SELECT 1 FROM user_job_feedback f WHERE f.user_id=$1 AND f.job_id=j.id AND f.feedback_type IN ('hidden','not_interested','applied'))
		AND (NOT EXISTS(SELECT 1 FROM user_source_preferences usp WHERE usp.user_id=$1)
			 OR EXISTS(SELECT 1 FROM job_sources js WHERE js.job_id=j.id AND js.is_active AND COALESCE((SELECT usp.enabled FROM user_source_preferences usp WHERE usp.user_id=$1 AND usp.source_id=js.source_id),true)))
		AND ($7::timestamptz IS NULL OR (j.created_at,j.id)<($7::timestamptz,$8::uuid))
		ORDER BY j.created_at DESC,j.id DESC LIMIT $9`
	rows, err := s.pool.Query(ctx, query, string(userID), candidate.Profile.Country, candidate.Preferences.ExcludedCountries, candidate.Preferences.RemotePolicies, candidate.Preferences.EmploymentTypes, candidate.Preferences.MaximumJobAgeDays, afterCreated, afterID, limit+1)
	if err != nil {
		return matchapp.JobPage{}, fmt.Errorf("preselect candidate jobs: %w", err)
	}
	defer rows.Close()
	jobs := make([]jobdomain.Job, 0, limit+1)
	for rows.Next() {
		var job jobdomain.Job
		var skillsJSON []byte
		var minimum, maximum *float64
		var currency, period *string
		if err := rows.Scan(&job.ID, &job.MatchVersion, &job.CompanyID, &job.Company, &job.Title, &job.NormalizedTitle, &job.Seniority, &job.Family, &job.Speciality, &job.FamilyConfidence, &job.SeniorityConfidence, &job.LocationConfidence, &job.SalaryConfidence, &job.Description, &minimum, &maximum, &currency, &period, &job.EmploymentTypes, &skillsJSON, &job.RemotePolicy, &job.Location, &job.Countries, &job.Eligibility, &job.ApplyURL, &job.PublishedAt, &job.FirstSeenAt, &job.LastSeenAt, &job.Status, &job.SourcePriority, &job.CreatedAt, &job.UpdatedAt); err != nil {
			return matchapp.JobPage{}, err
		}
		if minimum != nil && maximum != nil && currency != nil && period != nil {
			job.Salary = &jobdomain.SalaryRange{Minimum: *minimum, Maximum: *maximum, Currency: *currency, Period: *period}
		}
		if err := json.Unmarshal(skillsJSON, &job.Skills); err != nil {
			return matchapp.JobPage{}, fmt.Errorf("decode preselected job skills: %w", err)
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return matchapp.JobPage{}, err
	}
	page := matchapp.JobPage{}
	if len(jobs) > limit {
		last := jobs[limit-1]
		page.NextCursor = encodeJobCursor(last.CreatedAt, last.ID)
		jobs = jobs[:limit]
	}
	page.Jobs = jobs
	return page, nil
}

func (s *Store) CandidateIDsForJob(ctx context.Context, job jobdomain.Job, cursor string, limit int) ([]user.UserID, string, error) {
	if limit <= 0 || limit > 1000 {
		limit = 250
	}
	var afterID any
	if cursor != "" {
		if uuid.Validate(cursor) != nil {
			return nil, "", jobdomain.ErrInvalidCursor
		}
		afterID = cursor
	}
	var publishedAt any
	if job.PublishedAt != nil {
		publishedAt = *job.PublishedAt
	} else if !job.FirstSeenAt.IsZero() {
		publishedAt = job.FirstSeenAt
	}
	rows, err := s.pool.Query(ctx, `SELECT u.id::text FROM users u
		LEFT JOIN user_profiles p ON p.user_id=u.id
		LEFT JOIN job_preferences jp ON jp.user_id=u.id
		WHERE u.status='active'
		AND (EXISTS(SELECT 1 FROM user_profiles px WHERE px.user_id=u.id)
			OR EXISTS(SELECT 1 FROM user_skills us WHERE us.user_id=u.id)
			OR EXISTS(SELECT 1 FROM user_positions up WHERE up.user_id=u.id)
			OR EXISTS(SELECT 1 FROM job_preferences jpx WHERE jpx.user_id=u.id))
		AND NOT ($1='not_eligible' AND COALESCE(p.country,'')='KZ')
		AND (COALESCE(p.country,'')='' OR cardinality($2::text[])=0 OR p.country=ANY($2::text[]))
		AND NOT ($2::text[] && COALESCE(jp.excluded_countries,'{}'::text[]))
		AND (COALESCE(jp.remote_policies,'{}'::text[])= '{}'::text[] OR $3=ANY(jp.remote_policies)
			OR ('worldwide'=ANY(jp.remote_policies) AND $3='remote' AND cardinality($2::text[])=0))
		AND (COALESCE(jp.employment_types,'{}'::text[])= '{}'::text[] OR jp.employment_types && $4::text[])
		AND (jp.minimum_salary_amount IS NULL OR $5::float8 IS NULL OR jp.minimum_salary_currency<>$6
			OR $7<>'year' OR $5 >= jp.minimum_salary_amount)
		AND (COALESCE(jp.maximum_job_age_days,30)<=0 OR $8::timestamptz IS NULL
			OR $8::timestamptz >= now()-(COALESCE(jp.maximum_job_age_days,30)::int*interval '1 day'))
		AND (NOT EXISTS(SELECT 1 FROM user_source_preferences usp WHERE usp.user_id=u.id)
			OR EXISTS(SELECT 1 FROM job_sources js WHERE js.job_id=$9::uuid AND js.is_active
				AND COALESCE((SELECT usp.enabled FROM user_source_preferences usp WHERE usp.user_id=u.id AND usp.source_id=js.source_id),true)))
		AND ($10::uuid IS NULL OR u.id>$10::uuid)
		ORDER BY u.id LIMIT $11`,
		string(job.Eligibility), job.Countries, string(job.RemotePolicy), job.EmploymentTypes,
		salaryMaximum(job), salaryCurrency(job), salaryPeriod(job), publishedAt, job.ID, afterID, limit+1)
	if err != nil {
		return nil, "", fmt.Errorf("preselect candidates for job: %w", err)
	}
	defer rows.Close()
	ids := make([]user.UserID, 0, limit+1)
	for rows.Next() {
		var id user.UserID
		if err := rows.Scan(&id); err != nil {
			return nil, "", err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	nextCursor := ""
	if len(ids) > limit {
		ids = ids[:limit]
		nextCursor = string(ids[len(ids)-1])
	}
	return ids, nextCursor, nil
}

func salaryMaximum(job jobdomain.Job) *float64 {
	if job.Salary == nil {
		return nil
	}
	return &job.Salary.Maximum
}

func salaryCurrency(job jobdomain.Job) string {
	if job.Salary == nil {
		return ""
	}
	return job.Salary.Currency
}

func salaryPeriod(job jobdomain.Job) string {
	if job.Salary == nil {
		return ""
	}
	return job.Salary.Period
}

func (s *Store) Job(ctx context.Context, id string) (jobdomain.Job, error) {
	var job jobdomain.Job
	var skillsJSON []byte
	var minimum, maximum *float64
	var currency, period *string
	err := s.pool.QueryRow(ctx, `SELECT j.id::text,j.match_version,j.company_id::text,c.name,j.title,j.normalized_title,j.seniority,j.job_family,j.job_speciality,j.job_family_confidence,j.seniority_confidence,j.location_confidence,j.salary_confidence,j.description,
		j.salary_min,j.salary_max,j.salary_currency,j.salary_period,j.employment_types,
		COALESCE((SELECT jsonb_agg(jsonb_build_object('id',sk.id::text,'name',sk.name,'required',js.required,'confidence',js.confidence,'minimum_years',js.minimum_years,'minimum_level',js.minimum_level)) FROM job_skills js JOIN skills sk ON sk.id=js.skill_id WHERE js.job_id=j.id),'[]'::jsonb)::text,
		j.remote_policy,j.location,j.location_countries,j.eligibility,j.apply_url,j.published_at,j.first_seen_at,j.last_seen_at,j.status,j.source_priority,j.created_at,j.updated_at
		FROM jobs j JOIN companies c ON c.id=j.company_id WHERE j.id=$1`, id).Scan(&job.ID, &job.MatchVersion, &job.CompanyID, &job.Company, &job.Title, &job.NormalizedTitle, &job.Seniority, &job.Family, &job.Speciality, &job.FamilyConfidence, &job.SeniorityConfidence, &job.LocationConfidence, &job.SalaryConfidence, &job.Description, &minimum, &maximum, &currency, &period, &job.EmploymentTypes, &skillsJSON, &job.RemotePolicy, &job.Location, &job.Countries, &job.Eligibility, &job.ApplyURL, &job.PublishedAt, &job.FirstSeenAt, &job.LastSeenAt, &job.Status, &job.SourcePriority, &job.CreatedAt, &job.UpdatedAt)
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
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin job match save: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := lockUserJob(ctx, tx, userID, result.JobID); err != nil {
		return err
	}
	var created bool
	var removed int64
	if result.Eligible {
		var dismissed bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM user_job_feedback WHERE user_id=$1 AND job_id=$2 AND feedback_type IN ('hidden','not_interested','applied'))`, string(userID), result.JobID).Scan(&dismissed); err != nil {
			return fmt.Errorf("check job feedback before matching: %w", err)
		}
		result.Eligible = !dismissed
	}
	if !result.Eligible {
		if _, err := tx.Exec(ctx, `UPDATE notifications SET status='cancelled' WHERE user_id=$1 AND job_id=$2 AND status='pending'`, string(userID), result.JobID); err != nil {
			return fmt.Errorf("cancel ineligible match notification: %w", err)
		}
		tag, err := tx.Exec(ctx, `DELETE FROM user_job_matches WHERE user_id=$1 AND job_id=$2`, string(userID), result.JobID)
		if err != nil {
			return fmt.Errorf("remove ineligible job match: %w", err)
		}
		removed = tag.RowsAffected()
	} else {
		var err error
		created, removed, err = upsertMatch(ctx, tx, userID, result)
		if err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit job match: %w", err)
	}
	createdCount := int64(0)
	if created {
		createdCount = 1
	}
	recordMatchChanges(createdCount, removed)
	if created {
		recordMatchQuality(result)
	}
	return nil
}

func (s *Store) SaveMatchBatch(ctx context.Context, userID user.UserID, matches []engine.Result) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin match batch: %w", err)
	}
	defer tx.Rollback(ctx)
	orderedMatches := append([]engine.Result(nil), matches...)
	sort.Slice(orderedMatches, func(i, j int) bool { return orderedMatches[i].JobID < orderedMatches[j].JobID })
	var createdCount, removed int64
	createdMatches := make([]engine.Result, 0)
	for _, match := range orderedMatches {
		newMatch, removedMatches, err := upsertMatch(ctx, tx, userID, match)
		if err != nil {
			return err
		}
		if newMatch {
			createdCount++
			createdMatches = append(createdMatches, match)
		}
		removed += removedMatches
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit match refresh: %w", err)
	}
	recordMatchChanges(createdCount, removed)
	for _, match := range createdMatches {
		recordMatchQuality(match)
	}
	return nil
}

func (s *Store) CompleteProfileRefresh(ctx context.Context, userID user.UserID, candidateVersion int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin profile match cleanup: %w", err)
	}
	defer tx.Rollback(ctx)
	var removed int64
	if err := tx.QueryRow(ctx, `WITH stale AS (
		DELETE FROM user_job_matches WHERE user_id=$1 AND candidate_version < $2 RETURNING job_id
	), cancelled AS (
		UPDATE notifications n SET status='cancelled' FROM stale s
		WHERE n.user_id=$1 AND n.job_id=s.job_id AND n.status='pending' RETURNING n.id
	) SELECT count(*) FROM stale`, string(userID), candidateVersion).Scan(&removed); err != nil {
		return fmt.Errorf("remove stale profile matches: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit profile match cleanup: %w", err)
	}
	recordMatchChanges(0, removed)
	return nil
}

func (s *Store) RemoveJobMatches(ctx context.Context, jobID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin closed job match cleanup: %w", err)
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `DELETE FROM user_job_matches WHERE job_id=$1`, jobID)
	if err != nil {
		return fmt.Errorf("remove closed job matches: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE notifications SET status='cancelled',locked_by=NULL,locked_until=NULL WHERE job_id=$1 AND status IN ('pending','delivering')`, jobID); err != nil {
		return fmt.Errorf("cancel closed job notifications: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit closed job match cleanup: %w", err)
	}
	recordMatchChanges(0, tag.RowsAffected())
	return nil
}

type matchExecer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func upsertMatch(ctx context.Context, exec matchExecer, userID user.UserID, match engine.Result) (bool, int64, error) {
	if err := lockUserJob(ctx, exec, userID, match.JobID); err != nil {
		return false, 0, err
	}
	if !match.Eligible {
		if _, err := exec.Exec(ctx, `UPDATE notifications SET status='cancelled' WHERE user_id=$1 AND job_id=$2 AND status='pending'`, string(userID), match.JobID); err != nil {
			return false, 0, fmt.Errorf("cancel ineligible job notification: %w", err)
		}
		tag, err := exec.Exec(ctx, `DELETE FROM user_job_matches WHERE user_id=$1 AND job_id=$2`, string(userID), match.JobID)
		if err != nil {
			return false, 0, fmt.Errorf("remove ineligible job match: %w", err)
		}
		return false, tag.RowsAffected(), nil
	}
	var dismissed bool
	if err := queryUserJobFeedback(ctx, exec, userID, match.JobID, &dismissed); err != nil {
		return false, 0, fmt.Errorf("check feedback before match save: %w", err)
	}
	if dismissed {
		if _, err := exec.Exec(ctx, `UPDATE notifications SET status='cancelled' WHERE user_id=$1 AND job_id=$2 AND status='pending'`, string(userID), match.JobID); err != nil {
			return false, 0, fmt.Errorf("cancel dismissed job notification: %w", err)
		}
		tag, err := exec.Exec(ctx, `DELETE FROM user_job_matches WHERE user_id=$1 AND job_id=$2`, string(userID), match.JobID)
		if err != nil {
			return false, 0, fmt.Errorf("remove dismissed job match: %w", err)
		}
		return false, tag.RowsAffected(), nil
	}
	components, err := json.Marshal(match.Components)
	if err != nil {
		return false, 0, fmt.Errorf("encode match explanation: %w", err)
	}
	tag, err := exec.Exec(ctx, `INSERT INTO user_job_matches(user_id,job_id,score,components,candidate_version,job_version,confidence)
		VALUES($1,$2,$3,$4::jsonb,$5,$6,$7) ON CONFLICT(user_id,job_id) DO NOTHING`, string(userID), match.JobID, match.Score, components, match.CandidateVersion, match.JobVersion, match.Confidence)
	if err != nil {
		return false, 0, fmt.Errorf("save match: %w", err)
	}
	if tag.RowsAffected() == 0 {
		if _, err := exec.Exec(ctx, `UPDATE user_job_matches SET score=$3,components=$4::jsonb,candidate_version=$5,job_version=$6,confidence=$7,computed_at=now() WHERE user_id=$1 AND job_id=$2`, string(userID), match.JobID, match.Score, components, match.CandidateVersion, match.JobVersion, match.Confidence); err != nil {
			return false, 0, fmt.Errorf("update match: %w", err)
		}
		return false, 0, nil
	}
	if _, err := exec.Exec(ctx, `INSERT INTO outbox_events(id,event_type,aggregate_type,aggregate_id,payload)
		VALUES($1,'match.created','match',$2,jsonb_build_object('user_id',$3::text,'job_id',$2::text,'score',$4::int))`, uuid.NewString(), match.JobID, string(userID), match.Score); err != nil {
		return false, 0, fmt.Errorf("emit job matched event: %w", err)
	}
	return true, 0, nil
}

func recordMatchChanges(created, removed int64) {
	if created > 0 {
		observability.DefaultMetrics.Add("hireradar_matches_created_total", nil, float64(created))
	}
	if removed > 0 {
		observability.DefaultMetrics.Add("hireradar_matches_removed_total", nil, float64(removed))
	}
}

func recordMatchQuality(match engine.Result) {
	observability.DefaultMetrics.Add("hireradar_match_score_bucket_total", map[string]string{"bucket": observability.Bucket100(match.Score)}, 1)
	observability.DefaultMetrics.Add("hireradar_match_confidence_bucket_total", map[string]string{"bucket": observability.Bucket100(match.Confidence)}, 1)
}

type matchQueryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func lockUserJob(ctx context.Context, exec matchExecer, userID user.UserID, jobID string) error {
	if _, err := exec.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1 || ':' || $2,0))`, string(userID), jobID); err != nil {
		return fmt.Errorf("lock user job match: %w", err)
	}
	return nil
}

func queryUserJobFeedback(ctx context.Context, exec matchExecer, userID user.UserID, jobID string, dismissed *bool) error {
	queryer, ok := exec.(matchQueryer)
	if !ok {
		return fmt.Errorf("match transaction cannot query feedback")
	}
	return queryer.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM user_job_feedback WHERE user_id=$1 AND job_id=$2 AND feedback_type IN ('hidden','not_interested','applied'))`, string(userID), jobID).Scan(dismissed)
}

func (s *Store) ListMatches(ctx context.Context, userID user.UserID, limit int) ([]engine.Result, error) {
	page, err := s.ListMatchPage(ctx, userID, "", limit)
	return page.Matches, err
}

type matchCursor struct {
	Score      int       `json:"s"`
	ComputedAt time.Time `json:"t"`
	JobID      string    `json:"j"`
}

type jobCursor struct {
	CreatedAt time.Time `json:"created_at"`
	JobID     string    `json:"job_id"`
}

func encodeJobCursor(createdAt time.Time, jobID string) string {
	data, _ := json.Marshal(jobCursor{CreatedAt: createdAt.UTC(), JobID: jobID})
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeJobCursor(value string) (time.Time, string, error) {
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return time.Time{}, "", jobdomain.ErrInvalidCursor
	}
	var cursor jobCursor
	if err := json.Unmarshal(data, &cursor); err != nil || cursor.CreatedAt.IsZero() || uuid.Validate(cursor.JobID) != nil {
		return time.Time{}, "", jobdomain.ErrInvalidCursor
	}
	return cursor.CreatedAt, cursor.JobID, nil
}

func (s *Store) ListMatchPage(ctx context.Context, userID user.UserID, cursor string, limit int) (matchapp.MatchPage, error) {
	if limit == 0 {
		limit = 25
	}
	if limit < 1 || limit > 100 {
		return matchapp.MatchPage{}, fmt.Errorf("match page limit must be between 1 and 100")
	}
	args := []any{string(userID)}
	conditions := []string{
		"m.user_id=$1",
		"m.candidate_version=COALESCE(p.match_version,0)",
		"m.job_version=j.match_version",
		"NOT EXISTS(SELECT 1 FROM user_job_feedback f WHERE f.user_id=m.user_id AND f.job_id=m.job_id AND f.feedback_type IN ('hidden','not_interested','applied'))",
	}
	if cursor != "" {
		decoded, err := decodeMatchCursor(cursor)
		if err != nil {
			return matchapp.MatchPage{}, err
		}
		args = append(args, decoded.Score, decoded.ComputedAt, decoded.JobID)
		conditions = append(conditions, fmt.Sprintf("(m.score,m.computed_at,m.job_id)<($%d,$%d,$%d::uuid)", len(args)-2, len(args)-1, len(args)))
	}
	args = append(args, limit+1)
	query := `SELECT m.job_id::text,m.score,m.confidence,m.components::text,m.computed_at FROM user_job_matches m
		JOIN jobs j ON j.id=m.job_id LEFT JOIN user_profiles p ON p.user_id=m.user_id WHERE ` + strings.Join(conditions, " AND ") + fmt.Sprintf(" ORDER BY m.score DESC,m.computed_at DESC,m.job_id DESC LIMIT $%d", len(args))
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return matchapp.MatchPage{}, fmt.Errorf("list candidate matches: %w", err)
	}
	defer rows.Close()
	type entry struct {
		result engine.Result
		cursor matchCursor
	}
	items := []entry{}
	for rows.Next() {
		var item engine.Result
		var components []byte
		var computedAt time.Time
		if err := rows.Scan(&item.JobID, &item.Score, &item.Confidence, &components, &computedAt); err != nil {
			return matchapp.MatchPage{}, err
		}
		item.Eligible = true
		if err := json.Unmarshal(components, &item.Components); err != nil {
			return matchapp.MatchPage{}, fmt.Errorf("decode match explanation: %w", err)
		}
		items = append(items, entry{result: item, cursor: matchCursor{Score: item.Score, ComputedAt: computedAt, JobID: item.JobID}})
	}
	if err := rows.Err(); err != nil {
		return matchapp.MatchPage{}, err
	}
	page := matchapp.MatchPage{Matches: make([]engine.Result, 0, limit)}
	if len(items) > limit {
		items = items[:limit]
		page.NextCursor = encodeMatchCursor(items[len(items)-1].cursor)
	}
	for _, item := range items {
		page.Matches = append(page.Matches, item.result)
	}
	return page, nil
}

func encodeMatchCursor(cursor matchCursor) string {
	data, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeMatchCursor(value string) (matchCursor, error) {
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return matchCursor{}, jobdomain.ErrInvalidCursor
	}
	var cursor matchCursor
	if err := json.Unmarshal(data, &cursor); err != nil || cursor.Score < 0 || cursor.Score > 100 || cursor.ComputedAt.IsZero() {
		return matchCursor{}, jobdomain.ErrInvalidCursor
	}
	if _, err := uuid.Parse(cursor.JobID); err != nil {
		return matchCursor{}, jobdomain.ErrInvalidCursor
	}
	return cursor, nil
}
