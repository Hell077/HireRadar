package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	resumeapp "github.com/Hell077/HireRadar/apps/backend/internal/resume/application"
	"github.com/Hell077/HireRadar/apps/backend/internal/resume/domain"
	"github.com/Hell077/HireRadar/apps/backend/internal/resume/processing"
	user "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProcessingJob = resumeapp.ProcessingJob

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

const resumeColumns = `id::text, user_id::text, file_name, content_type, size, status, created_at, updated_at`

func scanResume(row pgx.Row) (domain.Resume, error) {
	var r domain.Resume
	var userID string
	err := row.Scan(&r.ID, &userID, &r.FileName, &r.ContentType, &r.Size, &r.Status, &r.CreatedAt, &r.UpdatedAt)
	r.UserID = user.UserID(userID)
	return r, err
}

func (s *Store) Create(ctx context.Context, r domain.Resume, key string) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO resumes (id,user_id,file_name,object_key,content_type,size,status,created_at,updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, string(r.ID), string(r.UserID), r.FileName, key, r.ContentType, r.Size, string(r.Status), r.CreatedAt, r.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create resume: %w", err)
	}
	return nil
}

func (s *Store) ByID(ctx context.Context, userID user.UserID, id domain.ID) (domain.Resume, string, error) {
	var r domain.Resume
	var owner, key string
	err := s.pool.QueryRow(ctx, `SELECT `+resumeColumns+`, object_key FROM resumes WHERE user_id=$1 AND id=$2 AND status <> 'deleted'`, string(userID), string(id)).Scan(&r.ID, &owner, &r.FileName, &r.ContentType, &r.Size, &r.Status, &r.CreatedAt, &r.UpdatedAt, &key)
	if err == nil {
		r.UserID = user.UserID(owner)
		return r, key, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Resume{}, "", domain.ErrNotFound
	}
	return domain.Resume{}, "", fmt.Errorf("load resume: %w", err)
}

func (s *Store) List(ctx context.Context, userID user.UserID) ([]domain.Resume, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+resumeColumns+` FROM resumes WHERE user_id=$1 AND status <> 'deleted' ORDER BY created_at DESC`, string(userID))
	if err != nil {
		return nil, fmt.Errorf("list resumes: %w", err)
	}
	defer rows.Close()
	result := make([]domain.Resume, 0)
	for rows.Next() {
		r, err := scanResume(rows)
		if err != nil {
			return nil, fmt.Errorf("scan resume: %w", err)
		}
		result = append(result, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read resumes: %w", err)
	}
	return result, nil
}

func (s *Store) MarkUploaded(ctx context.Context, userID user.UserID, id domain.ID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin resume completion: %w", err)
	}
	defer tx.Rollback(ctx)
	var status domain.Status
	var aggregateID string
	err = tx.QueryRow(ctx, `SELECT status, id::text FROM resumes WHERE user_id=$1 AND id=$2 FOR UPDATE`, string(userID), string(id)).Scan(&status, &aggregateID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock resume: %w", err)
	}
	if status == domain.StatusUploaded || status == domain.StatusProcessing || status == domain.StatusProcessed {
		return nil
	}
	if status != domain.StatusPendingUpload {
		return errors.New("resume is not awaiting upload")
	}
	if _, err := tx.Exec(ctx, `UPDATE resumes SET status='uploaded', updated_at=now() WHERE id=$1`, aggregateID); err != nil {
		return fmt.Errorf("mark resume uploaded: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO outbox_events (id,event_type,aggregate_type,aggregate_id,payload) VALUES ($1,'resume.uploaded','resume',$2,jsonb_build_object('resume_id',$2::text,'user_id',$3::text))`, uuid.NewString(), aggregateID, string(userID)); err != nil {
		return fmt.Errorf("publish resume upload: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit resume completion: %w", err)
	}
	return nil
}

func (s *Store) MarkDeleted(ctx context.Context, userID user.UserID, id domain.ID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin resume deletion: %w", err)
	}
	defer tx.Rollback(ctx)
	var aggregateID string
	err = tx.QueryRow(ctx, `UPDATE resumes SET status='deleted', updated_at=now() WHERE user_id=$1 AND id=$2 AND status <> 'deleted' RETURNING id::text`, string(userID), string(id)).Scan(&aggregateID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("mark resume deleted: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO outbox_events (id,event_type,aggregate_type,aggregate_id,payload) VALUES ($1,'resume.deleted','resume',$2,jsonb_build_object('resume_id',$2::text,'user_id',$3::text))`, uuid.NewString(), aggregateID, string(userID)); err != nil {
		return fmt.Errorf("publish resume deletion: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit resume deletion: %w", err)
	}
	return nil
}

func (s *Store) Claim(ctx context.Context) (resumeapp.ProcessingJob, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return resumeapp.ProcessingJob{}, false, fmt.Errorf("begin resume claim: %w", err)
	}
	defer tx.Rollback(ctx)
	var job resumeapp.ProcessingJob
	var userID string
	err = tx.QueryRow(ctx, `SELECT id::text,user_id::text,file_name,content_type,size,status,created_at,updated_at,object_key,processing_attempts
		FROM resumes WHERE (status='uploaded' AND next_processing_at<=now()) OR (status='processing' AND processing_started_at<now()-interval '15 minutes' AND processing_attempts<3)
		ORDER BY next_processing_at,created_at FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&job.Resume.ID, &userID, &job.Resume.FileName, &job.Resume.ContentType, &job.Resume.Size, &job.Resume.Status, &job.Resume.CreatedAt, &job.Resume.UpdatedAt, &job.ObjectKey, &job.Attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return resumeapp.ProcessingJob{}, false, nil
	}
	if err != nil {
		return resumeapp.ProcessingJob{}, false, fmt.Errorf("claim uploaded resume: %w", err)
	}
	job.Resume.UserID = user.UserID(userID)
	if _, err := tx.Exec(ctx, `UPDATE resumes SET status='processing',processing_started_at=now(),updated_at=now() WHERE id=$1`, string(job.Resume.ID)); err != nil {
		return resumeapp.ProcessingJob{}, false, fmt.Errorf("mark resume processing: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return resumeapp.ProcessingJob{}, false, fmt.Errorf("commit resume claim: %w", err)
	}
	return job, true, nil
}

func (s *Store) SkillVocabulary(ctx context.Context) ([]processing.SkillTerm, error) {
	rows, err := s.pool.Query(ctx, `SELECT s.name,s.normalized_name FROM skills s UNION ALL SELECT s.name,a.normalized_alias FROM skill_aliases a JOIN skills s ON s.id=a.skill_id ORDER BY 1`)
	if err != nil {
		return nil, fmt.Errorf("load skill vocabulary: %w", err)
	}
	defer rows.Close()
	result := make([]processing.SkillTerm, 0)
	for rows.Next() {
		var term processing.SkillTerm
		if err := rows.Scan(&term.Name, &term.Normalized); err != nil {
			return nil, fmt.Errorf("scan skill term: %w", err)
		}
		result = append(result, term)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read skill vocabulary: %w", err)
	}
	return result, nil
}

func (s *Store) SaveAnalysis(ctx context.Context, job resumeapp.ProcessingJob, parsed domain.ParsedResume) error {
	skills, err := json.Marshal(parsed.Skills)
	if err != nil {
		return fmt.Errorf("encode parsed skills: %w", err)
	}
	positions, err := json.Marshal(parsed.Positions)
	if err != nil {
		return fmt.Errorf("encode parsed positions: %w", err)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin parsed resume save: %w", err)
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `INSERT INTO parsed_resumes (resume_id,extracted_text,skills,positions,total_experience_months)
		VALUES ($1,$2,$3,$4,$5) ON CONFLICT (resume_id) DO UPDATE SET extracted_text=EXCLUDED.extracted_text,skills=EXCLUDED.skills,positions=EXCLUDED.positions,total_experience_months=EXCLUDED.total_experience_months,created_at=now()`, string(job.Resume.ID), parsed.Text, skills, positions, parsed.TotalExperienceMonths)
	if err != nil {
		return fmt.Errorf("store parsed resume: %w", err)
	}
	for _, skill := range parsed.Skills {
		if _, err := tx.Exec(ctx, `INSERT INTO resume_suggestions (id,resume_id,user_id,kind,value,normalized_value,confidence) VALUES ($1,$2,$3,'skill',$4,$5,$6) ON CONFLICT (resume_id,kind,normalized_value) DO NOTHING`, uuid.NewString(), string(job.Resume.ID), string(job.Resume.UserID), skill.Name, strings.ToLower(skill.Name), skill.Confidence); err != nil {
			return fmt.Errorf("store skill suggestion: %w", err)
		}
	}
	for _, position := range parsed.Positions {
		normalized := normalizeSuggestion(position.Title)
		if normalized == "" {
			continue
		}
		if _, err := tx.Exec(ctx, `INSERT INTO resume_suggestions (id,resume_id,user_id,kind,value,normalized_value,confidence) VALUES ($1,$2,$3,'position',$4,$5,$6) ON CONFLICT (resume_id,kind,normalized_value) DO NOTHING`, uuid.NewString(), string(job.Resume.ID), string(job.Resume.UserID), position.Title, normalized, position.Confidence); err != nil {
			return fmt.Errorf("store position suggestion: %w", err)
		}
	}
	updated, err := tx.Exec(ctx, `UPDATE resumes SET status='processed',processing_started_at=NULL,processing_error='',updated_at=now() WHERE id=$1 AND status='processing'`, string(job.Resume.ID))
	if err != nil {
		return fmt.Errorf("finish resume processing: %w", err)
	}
	if updated.RowsAffected() != 1 {
		return fmt.Errorf("resume %s is no longer being processed", job.Resume.ID)
	}
	if _, err := tx.Exec(ctx, `UPDATE outbox_events SET processed_at=now() WHERE event_type='resume.uploaded' AND aggregate_id=$1 AND processed_at IS NULL`, string(job.Resume.ID)); err != nil {
		return fmt.Errorf("ack resume uploaded event: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO outbox_events (id,event_type,aggregate_type,aggregate_id,payload) VALUES ($1,'resume.processed','resume',$2,jsonb_build_object('resume_id',$2::text,'user_id',$3::text))`, uuid.NewString(), string(job.Resume.ID), string(job.Resume.UserID)); err != nil {
		return fmt.Errorf("publish resume processed: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit parsed resume: %w", err)
	}
	return nil
}

func (s *Store) FailProcessing(ctx context.Context, job resumeapp.ProcessingJob, cause error) error {
	errorText := cause.Error()
	if len(errorText) > 500 {
		errorText = errorText[:500]
	}
	_, err := s.pool.Exec(ctx, `UPDATE resumes SET status=CASE WHEN processing_attempts+1>=3 THEN 'failed' ELSE 'uploaded' END,
		processing_attempts=processing_attempts+1,processing_started_at=NULL,processing_error=$2,next_processing_at=now()+LEAST(300,POWER(2,processing_attempts+1)::int)*interval '1 second',updated_at=now() WHERE id=$1 AND status='processing'`, string(job.Resume.ID), errorText)
	if err != nil {
		return fmt.Errorf("persist resume processing failure: %w", err)
	}
	return nil
}

func normalizeSuggestion(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func (s *Store) Analysis(ctx context.Context, userID user.UserID, id domain.ID) (domain.ParsedResume, []domain.Suggestion, error) {
	var result domain.ParsedResume
	result.ResumeID = id
	var skillsJSON, positionsJSON []byte
	err := s.pool.QueryRow(ctx, `SELECT p.extracted_text,p.skills,p.positions,p.total_experience_months FROM parsed_resumes p JOIN resumes r ON r.id=p.resume_id WHERE r.id=$1 AND r.user_id=$2 AND r.status<>'deleted'`, string(id), string(userID)).Scan(&result.Text, &skillsJSON, &positionsJSON, &result.TotalExperienceMonths)
	if errors.Is(err, pgx.ErrNoRows) {
		var exists bool
		if checkErr := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM resumes WHERE id=$1 AND user_id=$2 AND status<>'deleted')`, string(id), string(userID)).Scan(&exists); checkErr != nil {
			return domain.ParsedResume{}, nil, fmt.Errorf("check resume analysis owner: %w", checkErr)
		}
		if !exists {
			return domain.ParsedResume{}, nil, domain.ErrNotFound
		}
		return domain.ParsedResume{}, nil, domain.ErrAnalysisNotReady
	}
	if err != nil {
		return domain.ParsedResume{}, nil, fmt.Errorf("load resume analysis: %w", err)
	}
	if err := json.Unmarshal(skillsJSON, &result.Skills); err != nil {
		return domain.ParsedResume{}, nil, fmt.Errorf("decode parsed skills: %w", err)
	}
	if err := json.Unmarshal(positionsJSON, &result.Positions); err != nil {
		return domain.ParsedResume{}, nil, fmt.Errorf("decode parsed positions: %w", err)
	}
	rows, err := s.pool.Query(ctx, `SELECT id::text,resume_id::text,kind,value,confidence,status FROM resume_suggestions WHERE resume_id=$1 AND user_id=$2 ORDER BY kind,normalized_value`, string(id), string(userID))
	if err != nil {
		return domain.ParsedResume{}, nil, fmt.Errorf("list resume suggestions: %w", err)
	}
	defer rows.Close()
	suggestions := make([]domain.Suggestion, 0)
	for rows.Next() {
		var item domain.Suggestion
		if err := rows.Scan(&item.ID, &item.ResumeID, &item.Kind, &item.Value, &item.Confidence, &item.Status); err != nil {
			return domain.ParsedResume{}, nil, fmt.Errorf("scan resume suggestion: %w", err)
		}
		suggestions = append(suggestions, item)
	}
	if err := rows.Err(); err != nil {
		return domain.ParsedResume{}, nil, fmt.Errorf("read resume suggestions: %w", err)
	}
	return result, suggestions, nil
}

func (s *Store) ReviewSuggestion(ctx context.Context, userID user.UserID, resumeID domain.ID, suggestionID string, accept bool) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin suggestion review: %w", err)
	}
	defer tx.Rollback(ctx)
	var kind, value, status string
	err = tx.QueryRow(ctx, `SELECT s.kind,s.value,s.status FROM resume_suggestions s JOIN resumes r ON r.id=s.resume_id WHERE s.id=$1 AND s.resume_id=$2 AND s.user_id=$3 AND r.status<>'deleted' FOR UPDATE OF s`, suggestionID, string(resumeID), string(userID)).Scan(&kind, &value, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock resume suggestion: %w", err)
	}
	if status != "pending" {
		return domain.ErrSuggestionReviewed
	}
	changed := false
	if accept {
		switch kind {
		case "skill":
			var skillID string
			err = tx.QueryRow(ctx, `SELECT id::text FROM skills WHERE normalized_name=$1 UNION ALL SELECT skill_id::text FROM skill_aliases WHERE normalized_alias=$1 LIMIT 1`, strings.ToLower(value)).Scan(&skillID)
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("resolve suggested skill %q: %w", value, err)
			}
			if err != nil {
				return fmt.Errorf("resolve suggested skill: %w", err)
			}
			tag, execErr := tx.Exec(ctx, `INSERT INTO user_skills (user_id,skill_id,source,confirmed) VALUES ($1,$2,'resume',true) ON CONFLICT (user_id,skill_id) DO NOTHING`, string(userID), skillID)
			if execErr != nil {
				return fmt.Errorf("accept skill suggestion: %w", execErr)
			}
			changed = tag.RowsAffected() > 0
		case "position":
			tag, execErr := tx.Exec(ctx, `INSERT INTO user_positions (id,user_id,title,normalized_title) VALUES ($1,$2,$3,$4) ON CONFLICT (user_id,normalized_title) DO NOTHING`, uuid.NewString(), string(userID), value, normalizeSuggestion(value))
			if execErr != nil {
				return fmt.Errorf("accept position suggestion: %w", execErr)
			}
			changed = tag.RowsAffected() > 0
		default:
			return fmt.Errorf("unsupported suggestion kind %q", kind)
		}
	}
	newStatus := "rejected"
	if accept {
		newStatus = "accepted"
	}
	if _, err := tx.Exec(ctx, `UPDATE resume_suggestions SET status=$2,reviewed_at=now() WHERE id=$1`, suggestionID, newStatus); err != nil {
		return fmt.Errorf("save suggestion review: %w", err)
	}
	if changed {
		if _, err := tx.Exec(ctx, `INSERT INTO outbox_events (id,event_type,aggregate_type,aggregate_id,payload) VALUES ($1,'profile.changed','user',$2,jsonb_build_object('user_id',$2::text))`, uuid.NewString(), string(userID)); err != nil {
			return fmt.Errorf("publish profile change: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO outbox_events (id,event_type,aggregate_type,aggregate_id,payload) VALUES ($1,'resume.suggestion_reviewed','resume',$2,jsonb_build_object('resume_id',$2::text,'user_id',$3::text,'suggestion_id',$4::text,'status',$5::text))`, uuid.NewString(), string(resumeID), string(userID), suggestionID, newStatus); err != nil {
		return fmt.Errorf("publish suggestion review: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit suggestion review: %w", err)
	}
	return nil
}
