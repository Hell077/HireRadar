package postgres

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	jobapplicationapp "github.com/Hell077/HireRadar/apps/backend/internal/jobapplication/application"
	resumeapp "github.com/Hell077/HireRadar/apps/backend/internal/resume/application"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ResumeReader struct {
	pool    *pgxpool.Pool
	storage resumeapp.ObjectStorage
}

func NewResumeReader(pool *pgxpool.Pool, storage resumeapp.ObjectStorage) *ResumeReader {
	return &ResumeReader{pool: pool, storage: storage}
}

func (r *ResumeReader) ReadResume(ctx context.Context, userID, resumeID string) (jobapplicationapp.ResumeAttachment, error) {
	var filename, objectKey string
	err := r.pool.QueryRow(ctx, `
		SELECT file_name,object_key FROM resumes
		WHERE id=$1 AND user_id=$2 AND content_type='application/pdf' AND status='processed'`, resumeID, userID).Scan(&filename, &objectKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return jobapplicationapp.ResumeAttachment{}, errors.New("selected resume is unavailable")
	}
	if err != nil {
		return jobapplicationapp.ResumeAttachment{}, fmt.Errorf("load selected application resume: %w", err)
	}
	info, content, err := r.storage.Download(ctx, objectKey)
	if err != nil {
		return jobapplicationapp.ResumeAttachment{}, errors.New("selected resume could not be downloaded")
	}
	if len(content) == 0 || len(content) > 10<<20 || info.ContentType != "application/pdf" {
		return jobapplicationapp.ResumeAttachment{}, errors.New("selected resume is invalid")
	}
	return jobapplicationapp.ResumeAttachment{Filename: filepath.Base(filename), ContentType: info.ContentType, Content: content}, nil
}
