package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/Hell077/HireRadar/apps/backend/internal/resume/domain"
	user "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

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
