package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/resume/domain"
	"github.com/Hell077/HireRadar/apps/backend/internal/resume/processing"
)

type ProcessingJob struct {
	Resume    domain.Resume
	ObjectKey string
	Attempts  int
}

type ProcessingStore interface {
	Claim(context.Context) (ProcessingJob, bool, error)
	SkillVocabulary(context.Context) ([]processing.SkillTerm, error)
	SaveAnalysis(context.Context, ProcessingJob, domain.ParsedResume) error
	FailProcessing(context.Context, ProcessingJob, error) error
}

type ResumeObjectReader interface {
	Download(context.Context, string) (ObjectInfo, []byte, error)
}

type Worker struct {
	store   ProcessingStore
	objects ResumeObjectReader
}

func NewWorker(store ProcessingStore, objects ResumeObjectReader) *Worker {
	return &Worker{store: store, objects: objects}
}

func (w *Worker) ProcessNext(ctx context.Context) (bool, error) {
	job, found, err := w.store.Claim(ctx)
	if err != nil || !found {
		return found, err
	}
	workCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	info, data, err := w.objects.Download(workCtx, job.ObjectKey)
	if err == nil {
		err = validateStoredPDF(data, info, job.Resume.Size)
	}
	var parsed domain.ParsedResume
	if err == nil {
		var text string
		text, err = processing.ExtractText(workCtx, data)
		if err == nil {
			var vocabulary []processing.SkillTerm
			vocabulary, err = w.store.SkillVocabulary(workCtx)
			if err == nil {
				parsed = processing.Analyze(job.Resume.ID, text, vocabulary)
			}
		}
	}
	if err == nil {
		err = w.store.SaveAnalysis(ctx, job, parsed)
	}
	if err != nil {
		if failErr := w.store.FailProcessing(ctx, job, err); failErr != nil {
			return true, errors.Join(err, failErr)
		}
		return true, fmt.Errorf("process resume %s: %w", job.Resume.ID, err)
	}
	return true, nil
}

func validateStoredPDF(data []byte, info ObjectInfo, expectedSize int64) error {
	if int64(len(data)) != expectedSize || info.Size != expectedSize || info.ContentType != "application/pdf" || len(data) < 5 || int64(len(data)) > domain.MaxFileSize || string(data[:5]) != "%PDF-" {
		return errors.New("stored resume does not match its verified PDF metadata")
	}
	return nil
}

func (w *Worker) Run(ctx context.Context) error {
	for ctx.Err() == nil {
		found, err := w.ProcessNext(ctx)
		if err != nil && ctx.Err() == nil {
			slog.Error("resume processing failed", "error", err)
		}
		if found && err == nil {
			continue
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(2 * time.Second):
		}
	}
	return nil
}
