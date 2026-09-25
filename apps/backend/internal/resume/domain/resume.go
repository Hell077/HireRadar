package domain

import (
	"errors"
	"path/filepath"
	"strings"
	"time"

	user "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
)

const MaxFileSize int64 = 10 << 20

var ErrInvalidResume = errors.New("invalid resume upload")

type ID string
type Status string

const (
	StatusPendingUpload Status = "pending_upload"
	StatusUploaded Status = "uploaded"
	StatusProcessing Status = "processing"
	StatusProcessed Status = "processed"
	StatusFailed Status = "failed"
	StatusDeleted Status = "deleted"
)

type Resume struct {
	ID ID `json:"id"`
	UserID user.UserID `json:"-"`
	FileName string `json:"file_name"`
	ContentType string `json:"content_type"`
	Size int64 `json:"size"`
	Status Status `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func ValidateUpload(fileName, contentType string, size int64) (string, error) {
	fileName = strings.TrimSpace(filepath.Base(strings.ReplaceAll(fileName, "\\", "/")))
	if fileName == "" || len(fileName) > 255 || strings.ContainsAny(fileName, "\r\n") ||
		!strings.EqualFold(filepath.Ext(fileName), ".pdf") || contentType != "application/pdf" || size < 1 || size > MaxFileSize {
		return "", ErrInvalidResume
	}
	return fileName, nil
}
