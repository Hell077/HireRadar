package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/resume/domain"
	user "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/google/uuid"
)

var (
	ErrNotFound      = domain.ErrNotFound
	ErrNotReady      = errors.New("resume upload is not complete")
	ErrInvalidObject = errors.New("uploaded resume does not match its metadata")
)

type Store interface {
	Create(context.Context, domain.Resume, string) error
	ByID(context.Context, user.UserID, domain.ID) (domain.Resume, string, error)
	List(context.Context, user.UserID) ([]domain.Resume, error)
	MarkUploaded(context.Context, user.UserID, domain.ID) error
	MarkDeleted(context.Context, user.UserID, domain.ID) error
}

type Upload struct {
	Resume    domain.Resume
	UploadURL string
}
type Download struct {
	Resume      domain.Resume
	DownloadURL string
}

type Service struct {
	store   Store
	storage ObjectStorage
	now     func() time.Time
}

func NewService(store Store, storage ObjectStorage, now func() time.Time) *Service {
	return &Service{store: store, storage: storage, now: now}
}

func (s *Service) UploadURL(ctx context.Context, userID user.UserID, fileName, contentType string, size int64) (Upload, error) {
	name, err := domain.ValidateUpload(fileName, contentType, size)
	if err != nil {
		return Upload{}, err
	}
	id := domain.ID(uuid.NewString())
	key := fmt.Sprintf("users/%s/resumes/%s/original.pdf", userID, id)
	now := s.now().UTC()
	resume := domain.Resume{ID: id, UserID: userID, FileName: name, ContentType: "application/pdf", Size: size, Status: domain.StatusPendingUpload, CreatedAt: now, UpdatedAt: now}
	url, err := s.storage.PresignUpload(ctx, key, resume.ContentType, 5*time.Minute)
	if err != nil {
		return Upload{}, err
	}
	if err := s.store.Create(ctx, resume, key); err != nil {
		return Upload{}, err
	}
	return Upload{Resume: resume, UploadURL: url}, nil
}

func (s *Service) Complete(ctx context.Context, userID user.UserID, id domain.ID) (domain.Resume, error) {
	resume, key, err := s.store.ByID(ctx, userID, id)
	if err != nil {
		return domain.Resume{}, err
	}
	if resume.Status == domain.StatusUploaded || resume.Status == domain.StatusProcessing || resume.Status == domain.StatusProcessed {
		return resume, nil
	}
	if resume.Status != domain.StatusPendingUpload {
		return domain.Resume{}, ErrNotReady
	}
	info, err := s.storage.Stat(ctx, key)
	if err != nil {
		return domain.Resume{}, ErrInvalidObject
	}
	if info.Size != resume.Size || info.Size < 5 || info.Size > domain.MaxFileSize || info.ContentType != "application/pdf" {
		return domain.Resume{}, ErrInvalidObject
	}
	signature, err := s.storage.PDFSignature(ctx, key)
	if err != nil || !strings.HasPrefix(string(signature), "%PDF-") {
		return domain.Resume{}, ErrInvalidObject
	}
	if err := s.store.MarkUploaded(ctx, userID, id); err != nil {
		return domain.Resume{}, err
	}
	resume, key, err = s.store.ByID(ctx, userID, id)
	if err != nil {
		return domain.Resume{}, err
	}
	_ = key
	return resume, nil
}

func (s *Service) Get(ctx context.Context, userID user.UserID, id domain.ID) (Download, error) {
	resume, key, err := s.store.ByID(ctx, userID, id)
	if err != nil {
		return Download{}, err
	}
	result := Download{Resume: resume}
	if resume.Status == domain.StatusUploaded || resume.Status == domain.StatusProcessing || resume.Status == domain.StatusProcessed {
		result.DownloadURL, err = s.storage.PresignDownload(ctx, key, 5*time.Minute)
		if err != nil {
			return Download{}, err
		}
	}
	return result, nil
}

func (s *Service) List(ctx context.Context, userID user.UserID) ([]domain.Resume, error) {
	return s.store.List(ctx, userID)
}

func (s *Service) Delete(ctx context.Context, userID user.UserID, id domain.ID) error {
	resume, key, err := s.store.ByID(ctx, userID, id)
	if err != nil {
		return err
	}
	if resume.Status == domain.StatusDeleted {
		return nil
	}
	if err := s.storage.Delete(ctx, key); err != nil {
		return err
	}
	return s.store.MarkDeleted(ctx, userID, id)
}
