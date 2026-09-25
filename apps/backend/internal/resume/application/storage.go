package application

import (
	"context"
	"time"
)

type ObjectInfo struct {
	Size        int64
	ContentType string
}

type ObjectStorage interface {
	PresignUpload(context.Context, string, string, time.Duration) (string, error)
	PresignDownload(context.Context, string, time.Duration) (string, error)
	Stat(context.Context, string) (ObjectInfo, error)
	PDFSignature(context.Context, string) ([]byte, error)
	Delete(context.Context, string) error
}
