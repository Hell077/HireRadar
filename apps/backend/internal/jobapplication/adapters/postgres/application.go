package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	jobpostgres "github.com/Hell077/HireRadar/apps/backend/internal/job/adapters/postgres"
	jobdomain "github.com/Hell077/HireRadar/apps/backend/internal/job/domain"
	"github.com/Hell077/HireRadar/apps/backend/internal/jobapplication/application"
	"github.com/Hell077/HireRadar/apps/backend/internal/jobapplication/domain"
	"github.com/Hell077/HireRadar/apps/backend/internal/observability"
	userdomain "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func (s *Store) Request(ctx context.Context, requested domain.Application) (domain.Application, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Application{}, fmt.Errorf("begin application request: %w", err)
	}
	defer tx.Rollback(ctx)

	var existing domain.Application
	err = tx.QueryRow(ctx, `
		SELECT id::text,user_id::text,job_id::text,provider,status,attempts,requested_at,updated_at,submitted_at
		FROM job_applications WHERE user_id=$1 AND job_id=$2`, requested.UserID, requested.JobID).
		Scan(&existing.ID, &existing.UserID, &existing.JobID, &existing.Provider, &existing.Status, &existing.Attempts, &existing.RequestedAt, &existing.UpdatedAt, &existing.SubmittedAt)
	if err == nil {
		if err := tx.Commit(ctx); err != nil {
			return domain.Application{}, fmt.Errorf("commit existing application lookup: %w", err)
		}
		return existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.Application{}, fmt.Errorf("find existing application: %w", err)
	}

	var matchScore, matchConfidence int
	err = tx.QueryRow(ctx, `
		SELECT m.score,m.confidence
			FROM user_job_matches m
			JOIN jobs j ON j.id=m.job_id AND j.status='active'
			LEFT JOIN user_profiles p ON p.user_id=m.user_id
			WHERE m.user_id=$1 AND m.job_id=$2
			  AND m.candidate_version=COALESCE(p.match_version,0)
			  AND m.job_version=j.match_version`, requested.UserID, requested.JobID).Scan(&matchScore, &matchConfidence)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Application{}, application.ErrApplicationUnavailable
	}
	if err != nil {
		return domain.Application{}, fmt.Errorf("validate application match: %w", err)
	}

	var result domain.Application
	var created bool
	err = tx.QueryRow(ctx, `
		INSERT INTO job_applications(id,user_id,job_id,provider,status,requested_at,updated_at,match_score,match_confidence)
		VALUES($1,$2,$3,$4,$5,$6,$6,$7,$8)
		ON CONFLICT(user_id,job_id) DO NOTHING
		RETURNING id::text,user_id::text,job_id::text,provider,status,attempts,requested_at,updated_at,submitted_at`,
		string(requested.ID), requested.UserID, requested.JobID, string(requested.Provider), string(requested.Status), requested.RequestedAt, matchScore, matchConfidence).
		Scan(&result.ID, &result.UserID, &result.JobID, &result.Provider, &result.Status, &result.Attempts, &result.RequestedAt, &result.UpdatedAt, &result.SubmittedAt)
	if err == nil {
		created = true
	} else if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `
			SELECT id::text,user_id::text,job_id::text,provider,status,attempts,requested_at,updated_at,submitted_at
			FROM job_applications WHERE user_id=$1 AND job_id=$2`, requested.UserID, requested.JobID).
			Scan(&result.ID, &result.UserID, &result.JobID, &result.Provider, &result.Status, &result.Attempts, &result.RequestedAt, &result.UpdatedAt, &result.SubmittedAt)
	}
	if err != nil {
		return domain.Application{}, fmt.Errorf("persist application request: %w", err)
	}
	if created {
		if _, err := tx.Exec(ctx, `
			INSERT INTO outbox_events(id,event_type,aggregate_type,aggregate_id,payload)
			VALUES($1,'application.requested','job_application',$2,jsonb_build_object('application_id',$2::text))`,
			uuid.NewString(), string(result.ID)); err != nil {
			return domain.Application{}, fmt.Errorf("publish application request: %w", err)
		}
		if err := insertStatusEvent(ctx, tx, string(result.ID)); err != nil {
			return domain.Application{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Application{}, fmt.Errorf("commit application request: %w", err)
	}
	if created {
		observability.DefaultMetrics.Add("hireradar_applications_requested_total", nil, 1)
		recordApplicationConversion("requested", matchScore, matchConfidence)
	}
	return result, nil
}

func (s *Store) UserForTelegram(ctx context.Context, telegramUserID int64) (userdomain.UserID, error) {
	var userID userdomain.UserID
	err := s.pool.QueryRow(ctx, `SELECT user_id::text FROM telegram_accounts WHERE telegram_user_id=$1`, telegramUserID).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", application.ErrTelegramAccountNotFound
	}
	if err != nil {
		return "", fmt.Errorf("resolve Telegram account owner: %w", err)
	}
	return userID, nil
}

func (s *Store) Get(ctx context.Context, userID userdomain.UserID, id domain.ID) (domain.Application, error) {
	var result domain.Application
	err := s.pool.QueryRow(ctx, `
		SELECT id::text,user_id::text,job_id::text,provider,status,attempts,requested_at,updated_at,submitted_at
		FROM job_applications WHERE user_id=$1 AND id=$2`, string(userID), string(id)).Scan(
		&result.ID, &result.UserID, &result.JobID, &result.Provider, &result.Status, &result.Attempts,
		&result.RequestedAt, &result.UpdatedAt, &result.SubmittedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Application{}, application.ErrApplicationUnavailable
	}
	if err != nil {
		return domain.Application{}, fmt.Errorf("load application: %w", err)
	}
	return result, nil
}

func (s *Store) List(ctx context.Context, userID userdomain.UserID, limit int) ([]domain.Application, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text,user_id::text,job_id::text,provider,status,attempts,requested_at,updated_at,submitted_at
		FROM job_applications WHERE user_id=$1 ORDER BY requested_at DESC,id DESC LIMIT $2`, string(userID), limit)
	if err != nil {
		return nil, fmt.Errorf("list applications: %w", err)
	}
	defer rows.Close()
	applications := make([]domain.Application, 0)
	for rows.Next() {
		var app domain.Application
		if err := rows.Scan(&app.ID, &app.UserID, &app.JobID, &app.Provider, &app.Status, &app.Attempts, &app.RequestedAt, &app.UpdatedAt, &app.SubmittedAt); err != nil {
			return nil, fmt.Errorf("scan application: %w", err)
		}
		applications = append(applications, app)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read applications: %w", err)
	}
	return applications, nil
}

func (s *Store) Questions(ctx context.Context, userID userdomain.UserID, id domain.ID) ([]domain.ApplicationQuestion, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT q.id::text,q.application_id::text,COALESCE(q.external_key,''),q.question,q.question_type,q.required,
		       COALESCE(q.options,'[]'::jsonb),COALESCE(q.answer,'null'::jsonb),q.status
		FROM job_application_questions q
	JOIN job_applications a ON a.id=q.application_id
	WHERE a.user_id=$1 AND a.id=$2
	ORDER BY q.created_at,q.id`, string(userID), string(id))
	if err != nil {
		return nil, fmt.Errorf("list application questions: %w", err)
	}
	defer rows.Close()
	questions := make([]domain.ApplicationQuestion, 0)
	for rows.Next() {
		var question domain.ApplicationQuestion
		var options, answer []byte
		if err := rows.Scan(&question.ID, &question.Application, &question.ExternalKey, &question.Text, &question.Type, &question.Required, &options, &answer, &question.Status); err != nil {
			return nil, fmt.Errorf("scan application question: %w", err)
		}
		if err := json.Unmarshal(options, &question.Options); err != nil {
			return nil, fmt.Errorf("decode application question options: %w", err)
		}
		if string(answer) != "null" {
			if err := json.Unmarshal(answer, &question.Answer); err != nil {
				return nil, fmt.Errorf("decode application question answer: %w", err)
			}
		}
		questions = append(questions, question)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read application questions: %w", err)
	}
	if len(questions) == 0 {
		if _, err := s.Get(ctx, userID, id); err != nil {
			return nil, err
		}
	}
	return questions, nil
}

func (s *Store) AnswerQuestion(ctx context.Context, userID userdomain.UserID, questionID, answer string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin application answer: %w", err)
	}
	defer tx.Rollback(ctx)
	var appID string
	var status domain.Status
	if err := tx.QueryRow(ctx, `
		SELECT a.id::text,a.status FROM job_applications a
		JOIN job_application_questions q ON q.application_id=a.id
		WHERE a.user_id=$1 AND q.id=$2 FOR UPDATE OF a`, string(userID), questionID).Scan(&appID, &status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrApplicationUnavailable
		}
		return fmt.Errorf("load application for answer: %w", err)
	}
	var optionsJSON []byte
	var existingAnswer []byte
	var questionStatus string
	if err := tx.QueryRow(ctx, `SELECT COALESCE(options,'[]'::jsonb),answer,status FROM job_application_questions WHERE id=$1`, questionID).Scan(&optionsJSON, &existingAnswer, &questionStatus); err != nil {
		return fmt.Errorf("load application question: %w", err)
	}
	var options []domain.Option
	if err := json.Unmarshal(optionsJSON, &options); err != nil {
		return fmt.Errorf("decode application question options: %w", err)
	}
	if questionStatus == "answered" {
		var previous any
		if json.Unmarshal(existingAnswer, &previous) == nil && fmt.Sprint(previous) == answer {
			return tx.Commit(ctx)
		}
	}
	if status != domain.StatusNeedsInput && status != domain.StatusPreparing {
		return application.ErrInvalidApplicationAnswer
	}
	answerValue := any(answer)
	if len(options) > 0 {
		valid := false
		for _, option := range options {
			if fmt.Sprint(option.Value) == answer {
				valid = true
				answerValue = option.Value
				break
			}
		}
		if !valid {
			return application.ErrInvalidApplicationAnswer
		}
	}
	encoded, err := json.Marshal(answerValue)
	if err != nil {
		return fmt.Errorf("encode application answer: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE job_application_questions SET answer=$2::jsonb,answer_source='user',status='answered',updated_at=now() WHERE id=$1`, questionID, encoded); err != nil {
		return fmt.Errorf("save application answer: %w", err)
	}
	if status == domain.StatusNeedsInput {
		var unanswered bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM job_application_questions WHERE application_id=$1 AND required AND status='unanswered')`, appID).Scan(&unanswered); err != nil {
			return fmt.Errorf("check unanswered application questions: %w", err)
		}
		if !unanswered {
			if _, err := tx.Exec(ctx, `UPDATE job_applications SET status='preparing',updated_at=now() WHERE id=$1 AND status='needs_input'`, appID); err != nil {
				return fmt.Errorf("resume application after answers: %w", err)
			}
			if _, err := tx.Exec(ctx, `INSERT INTO outbox_events(id,event_type,aggregate_type,aggregate_id,payload) VALUES($1,'application.input_provided','job_application',$2,jsonb_build_object('application_id',$2::text))`, uuid.NewString(), appID); err != nil {
				return fmt.Errorf("publish application answers: %w", err)
			}
		}
	}
	if err := insertStatusEvent(ctx, tx, appID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit application answer: %w", err)
	}
	return nil
}

func (s *Store) Cancel(ctx context.Context, userID userdomain.UserID, id domain.ID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin application cancellation: %w", err)
	}
	defer tx.Rollback(ctx)
	var status domain.Status
	err = tx.QueryRow(ctx, `SELECT status FROM job_applications WHERE id=$1 AND user_id=$2 FOR UPDATE`, string(id), string(userID)).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return application.ErrApplicationUnavailable
	}
	if err != nil {
		return fmt.Errorf("load application to cancel: %w", err)
	}
	if status != domain.StatusCancelled {
		app := domain.Application{Status: status}
		if err := app.Transition(domain.StatusCancelled, time.Now().UTC()); err != nil {
			return application.ErrInvalidApplicationOperation
		}
		if _, err := tx.Exec(ctx, `UPDATE job_applications SET status='cancelled',updated_at=now(),locked_by=NULL,locked_until=NULL WHERE id=$1`, string(id)); err != nil {
			return fmt.Errorf("cancel application: %w", err)
		}
		if err := insertStatusEvent(ctx, tx, string(id)); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit application cancellation: %w", err)
	}
	return nil
}

func (s *Store) Retry(ctx context.Context, userID userdomain.UserID, id domain.ID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin application retry: %w", err)
	}
	defer tx.Rollback(ctx)
	var status domain.Status
	var attempts int
	err = tx.QueryRow(ctx, `SELECT status,attempts FROM job_applications WHERE id=$1 AND user_id=$2 FOR UPDATE`, string(id), string(userID)).Scan(&status, &attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return application.ErrApplicationUnavailable
	}
	if err != nil {
		return fmt.Errorf("load application to retry: %w", err)
	}
	if status == domain.StatusRequested || status == domain.StatusPreparing || status == domain.StatusReady || status == domain.StatusSubmitting {
		return tx.Commit(ctx)
	}
	if status != domain.StatusFailed || attempts >= 5 {
		return application.ErrInvalidApplicationOperation
	}
	if _, err := tx.Exec(ctx, `UPDATE job_applications SET status='requested',last_error_code=NULL,last_error_message=NULL,updated_at=now() WHERE id=$1`, string(id)); err != nil {
		return fmt.Errorf("reset failed application: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO outbox_events(id,event_type,aggregate_type,aggregate_id,payload) VALUES($1,'application.retry_requested','job_application',$2,jsonb_build_object('application_id',$2::text))`, uuid.NewString(), string(id)); err != nil {
		return fmt.Errorf("publish application retry: %w", err)
	}
	if err := insertStatusEvent(ctx, tx, string(id)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit application retry: %w", err)
	}
	return nil
}

func (s *Store) ListFailed(ctx context.Context, limit int) ([]domain.FailedApplication, error) {
	if limit < 1 || limit > 100 {
		return nil, errors.New("failed application limit must be between 1 and 100")
	}
	rows, err := s.pool.Query(ctx, `SELECT a.id::text,a.job_id::text,j.title,c.name,a.provider,a.status,a.attempts,COALESCE(a.last_error_code,''),a.updated_at
		FROM job_applications a JOIN jobs j ON j.id=a.job_id JOIN companies c ON c.id=j.company_id
		WHERE a.status='failed' ORDER BY a.updated_at,a.id LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list failed applications: %w", err)
	}
	defer rows.Close()
	items := make([]domain.FailedApplication, 0)
	for rows.Next() {
		var item domain.FailedApplication
		if err := rows.Scan(&item.ID, &item.JobID, &item.Title, &item.Company, &item.Provider, &item.Status, &item.Attempts, &item.ErrorCode, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan failed application: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read failed applications: %w", err)
	}
	return items, nil
}

func (s *Store) RetryFailed(ctx context.Context, id domain.ID) error {
	var userID userdomain.UserID
	if err := s.pool.QueryRow(ctx, `SELECT user_id::text FROM job_applications WHERE id=$1`, string(id)).Scan(&userID); errors.Is(err, pgx.ErrNoRows) {
		return application.ErrApplicationUnavailable
	} else if err != nil {
		return fmt.Errorf("find application owner for admin retry: %w", err)
	}
	return s.Retry(ctx, userID, id)
}

func (s *Store) Application(ctx context.Context, id domain.ID) (domain.Application, error) {
	var result domain.Application
	err := s.pool.QueryRow(ctx, `
		SELECT id::text,user_id::text,job_id::text,provider,status,attempts,requested_at,updated_at,submitted_at
		FROM job_applications WHERE id=$1`, string(id)).Scan(
		&result.ID, &result.UserID, &result.JobID, &result.Provider, &result.Status, &result.Attempts,
		&result.RequestedAt, &result.UpdatedAt, &result.SubmittedAt)
	if err != nil {
		return domain.Application{}, fmt.Errorf("load application: %w", err)
	}
	return result, nil
}

func (s *Store) Job(ctx context.Context, id string) (jobdomain.Job, error) {
	return jobpostgres.NewCatalog(s.pool).Get(ctx, id)
}

func (s *Store) Profile(ctx context.Context, userID userdomain.UserID) (domain.ApplicationProfile, error) {
	return NewProfileStore(s.pool).GetProfile(ctx, string(userID))
}

func (s *Store) Prepared(ctx context.Context, id domain.ID) (domain.PreparedForm, error) {
	var raw []byte
	if err := s.pool.QueryRow(ctx, `SELECT form_snapshot FROM job_applications WHERE id=$1`, string(id)).Scan(&raw); err != nil {
		return domain.PreparedForm{}, fmt.Errorf("load prepared application form: %w", err)
	}
	var form domain.PreparedForm
	if len(raw) == 0 || json.Unmarshal(raw, &form) != nil {
		return domain.PreparedForm{}, errors.New("prepared application form is unavailable")
	}
	return form, nil
}

func (s *Store) SavePrepared(ctx context.Context, id domain.ID, form domain.PreparedForm, questions []domain.Question) error {
	answers, err := s.pool.Query(ctx, `SELECT external_key,answer FROM job_application_questions WHERE application_id=$1 AND status='answered' AND external_key IS NOT NULL`, string(id))
	if err != nil {
		return fmt.Errorf("load saved application answers: %w", err)
	}
	values := make(map[string]any)
	for answers.Next() {
		var key string
		var raw []byte
		var value any
		if err := answers.Scan(&key, &raw); err != nil {
			answers.Close()
			return fmt.Errorf("read saved application answer: %w", err)
		}
		if err := json.Unmarshal(raw, &value); err != nil {
			answers.Close()
			return fmt.Errorf("decode saved application answer: %w", err)
		}
		values[key] = value
	}
	if err := answers.Err(); err != nil {
		answers.Close()
		return fmt.Errorf("read saved application answers: %w", err)
	}
	answers.Close()
	for i := range form.Fields {
		if value, ok := values[form.Fields[i].Key]; ok {
			form.Fields[i].Value = value
		}
	}
	missingQuestions := questions[:0]
	for _, question := range questions {
		if question.ExternalKey != "" {
			if _, answered := values[question.ExternalKey]; answered {
				continue
			}
		}
		missingQuestions = append(missingQuestions, question)
	}
	raw, err := json.Marshal(form)
	if err != nil {
		return fmt.Errorf("encode prepared application form: %w", err)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin application preparation: %w", err)
	}
	defer tx.Rollback(ctx)
	var previousStatus domain.Status
	if err := tx.QueryRow(ctx, `SELECT status FROM job_applications WHERE id=$1 FOR UPDATE`, string(id)).Scan(&previousStatus); err != nil {
		return fmt.Errorf("load application before saving prepared form: %w", err)
	}
	status := domain.StatusReady
	if len(missingQuestions) > 0 {
		status = domain.StatusNeedsInput
	}
	command, err := tx.Exec(ctx, `
		UPDATE job_applications SET form_snapshot=$2::jsonb,status=$3,updated_at=now()
		WHERE id=$1 AND status IN ('requested','preparing','ready','needs_input')`, string(id), raw, string(status))
	if err != nil {
		return fmt.Errorf("save prepared form: %w", err)
	}
	if command.RowsAffected() != 1 {
		return errors.New("application is not available for preparation")
	}
	if status != previousStatus {
		if err := insertStatusEvent(ctx, tx, string(id)); err != nil {
			return err
		}
	}
	for _, question := range missingQuestions {
		key := question.ExternalKey
		if key == "" {
			key = uuid.NewString()
		}
		options, err := json.Marshal(question.Options)
		if err != nil {
			return fmt.Errorf("encode application question options: %w", err)
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO job_application_questions(id,application_id,external_key,question,question_type,required,options,status)
			VALUES($1,$2,$3,$4,$5,$6,$7::jsonb,'unanswered')
			ON CONFLICT(application_id,external_key) WHERE external_key IS NOT NULL DO UPDATE SET
				question=EXCLUDED.question,question_type=EXCLUDED.question_type,required=EXCLUDED.required,
				options=EXCLUDED.options,updated_at=now()`, uuid.NewString(), string(id), key, question.Text, question.Type, question.Required, options)
		if err != nil {
			return fmt.Errorf("save application question: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit application preparation: %w", err)
	}
	if status != previousStatus {
		if status == domain.StatusReady {
			observability.DefaultMetrics.Add("hireradar_applications_prepared_total", nil, 1)
		}
		recordApplicationOutcome(status)
	}
	return nil
}

func (s *Store) BeginAttempt(ctx context.Context, id domain.ID, provider domain.Provider) (int, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, false, fmt.Errorf("begin application attempt: %w", err)
	}
	defer tx.Rollback(ctx)
	var status domain.Status
	var attempts int
	if err := tx.QueryRow(ctx, `SELECT status,attempts FROM job_applications WHERE id=$1 FOR UPDATE`, string(id)).Scan(&status, &attempts); err != nil {
		return 0, false, fmt.Errorf("load application for attempt: %w", err)
	}
	if status != domain.StatusReady {
		return attempts, false, nil
	}
	if attempts >= 5 {
		return attempts, false, errors.New("application submission retry limit reached")
	}
	attempt := attempts + 1
	if _, err := tx.Exec(ctx, `INSERT INTO job_application_attempts(id,application_id,attempt_number,status,provider) VALUES($1,$2,$3,'running',$4)`, uuid.NewString(), string(id), attempt, string(provider)); err != nil {
		return 0, false, fmt.Errorf("create application attempt: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE job_applications SET status='submitting',provider=$2,attempts=$3,updated_at=now() WHERE id=$1`, string(id), string(provider), attempt); err != nil {
		return 0, false, fmt.Errorf("mark application submitting: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, false, fmt.Errorf("commit application attempt: %w", err)
	}
	return attempt, true, nil
}

func (s *Store) FinishAttempt(ctx context.Context, id domain.ID, attempt int, status domain.Status, errorCode string, result *domain.SubmissionResult) error {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode application result: %w", err)
	}
	attemptStatus := string(status)
	switch status {
	case domain.StatusSubmitted:
		attemptStatus = "succeeded"
	case domain.StatusRequested:
		attemptStatus = "retryable"
	case domain.StatusNeedsInput:
		attemptStatus = "needs_input"
	case domain.StatusManualRequired:
		attemptStatus = "manual_required"
	case domain.StatusFailed, domain.StatusCancelled:
		attemptStatus = "failed"
	default:
		return errors.New("invalid application attempt result")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin application result update: %w", err)
	}
	defer tx.Rollback(ctx)
	var previousStatus domain.Status
	var startedAt time.Time
	if err := tx.QueryRow(ctx, `SELECT a.status,at.started_at FROM job_applications a JOIN job_application_attempts at ON at.application_id=a.id AND at.attempt_number=$2 WHERE a.id=$1 FOR UPDATE OF a,at`, string(id), attempt).Scan(&previousStatus, &startedAt); err != nil {
		return fmt.Errorf("load application before finishing attempt: %w", err)
	}
	_, err = tx.Exec(ctx, `UPDATE job_application_attempts SET status=$3,finished_at=now(),error_code=NULLIF($4,''),result=$5::jsonb WHERE application_id=$1 AND attempt_number=$2 AND status='running'`, string(id), attempt, attemptStatus, errorCode, resultJSON)
	if err != nil {
		return fmt.Errorf("finish application attempt: %w", err)
	}
	_, err = tx.Exec(ctx, `UPDATE job_applications SET status=$2,submitted_at=CASE WHEN $2='submitted' THEN COALESCE($3,now()) ELSE NULL END,
		last_error_code=NULLIF($4,''),last_error_message=NULLIF($5,''),result=$6::jsonb,updated_at=now(),locked_by=NULL,locked_until=NULL WHERE id=$1`,
		string(id), string(status), submittedAt(result), errorCode, safeErrorMessage(errorCode), resultJSON)
	if err != nil {
		return fmt.Errorf("save application result: %w", err)
	}
	if status != previousStatus && status != domain.StatusRequested {
		if err := insertStatusEvent(ctx, tx, string(id)); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit application result: %w", err)
	}
	if status != previousStatus {
		recordApplicationOutcome(status)
		observability.DefaultMetrics.ObserveDuration("hireradar_application_duration", nil, time.Since(startedAt))
		s.recordApplicationConversionByID(ctx, id, string(status))
	}
	return nil
}

func (s *Store) SetStatus(ctx context.Context, id domain.ID, status domain.Status, errorCode, message string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin application status update: %w", err)
	}
	defer tx.Rollback(ctx)
	var previous domain.Status
	if err := tx.QueryRow(ctx, `SELECT status FROM job_applications WHERE id=$1 FOR UPDATE`, string(id)).Scan(&previous); err != nil {
		return fmt.Errorf("load application before status update: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE job_applications SET status=$2,last_error_code=NULLIF($3,''),last_error_message=NULLIF($4,''),updated_at=now(),locked_by=NULL,locked_until=NULL WHERE id=$1`, string(id), string(status), errorCode, message); err != nil {
		return fmt.Errorf("update application status: %w", err)
	}
	if previous != status {
		if err := insertStatusEvent(ctx, tx, string(id)); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit application status update: %w", err)
	}
	if previous != status {
		recordApplicationOutcome(status)
		s.recordApplicationConversionByID(ctx, id, string(status))
	}
	return nil
}

func (s *Store) MarkUncertain(ctx context.Context, id domain.ID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin uncertain application update: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE job_application_attempts SET status='uncertain',finished_at=now(),error_code='outcome_uncertain' WHERE application_id=$1 AND status='running'`, string(id)); err != nil {
		return fmt.Errorf("mark application attempt uncertain: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE job_applications SET status='manual_required',last_error_code='outcome_uncertain',last_error_message='submission outcome could not be confirmed',updated_at=now(),locked_by=NULL,locked_until=NULL WHERE id=$1 AND status='submitting'`, string(id)); err != nil {
		return fmt.Errorf("mark application outcome uncertain: %w", err)
	}
	if err := insertStatusEvent(ctx, tx, string(id)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit uncertain application update: %w", err)
	}
	recordApplicationOutcome(domain.StatusManualRequired)
	s.recordApplicationConversionByID(ctx, id, string(domain.StatusManualRequired))
	return nil
}

func (s *Store) recordApplicationConversionByID(ctx context.Context, id domain.ID, stage string) {
	var score, confidence sql.NullInt64
	err := s.pool.QueryRow(ctx, `SELECT match_score,match_confidence FROM job_applications WHERE id=$1`, string(id)).Scan(&score, &confidence)
	if err != nil || !score.Valid || !confidence.Valid {
		return
	}
	recordApplicationConversion(stage, int(score.Int64), int(confidence.Int64))
}

func recordApplicationOutcome(status domain.Status) {
	name := ""
	switch status {
	case domain.StatusNeedsInput:
		name = "hireradar_applications_needs_input_total"
	case domain.StatusSubmitted:
		name = "hireradar_applications_submitted_total"
	case domain.StatusManualRequired:
		name = "hireradar_applications_manual_total"
	case domain.StatusFailed:
		name = "hireradar_applications_failed_total"
	}
	if name != "" {
		observability.DefaultMetrics.Add(name, nil, 1)
	}
}

func recordApplicationConversion(stage string, score, confidence int) {
	observability.DefaultMetrics.Add("hireradar_product_application_conversion_total", map[string]string{
		"stage": stage, "score_bucket": observability.Bucket100(score), "confidence_bucket": observability.Bucket100(confidence),
	}, 1)
}

func insertStatusEvent(ctx context.Context, tx pgx.Tx, applicationID string) error {
	command, err := tx.Exec(ctx, `INSERT INTO outbox_events(id,event_type,aggregate_type,aggregate_id,payload)
		SELECT $1,'application.status.'||status,'job_application',id::text,jsonb_build_object('application_id',id::text)
		FROM job_applications WHERE id=$2::uuid`, uuid.NewString(), applicationID)
	if err != nil {
		return fmt.Errorf("publish application status update: %w", err)
	}
	if command.RowsAffected() != 1 {
		return errors.New("application status update target not found")
	}
	return nil
}

func submittedAt(result *domain.SubmissionResult) any {
	if result == nil || result.SubmittedAt.IsZero() {
		return nil
	}
	return result.SubmittedAt
}

func safeErrorMessage(code string) string {
	if code == "" {
		return ""
	}
	return "provider reported " + code
}

var _ application.WorkerStore = (*Store)(nil)
var _ application.ProfileStore = (*ProfileStore)(nil)
