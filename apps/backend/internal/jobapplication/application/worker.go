package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	jobdomain "github.com/Hell077/HireRadar/apps/backend/internal/job/domain"
	"github.com/Hell077/HireRadar/apps/backend/internal/jobapplication/domain"
	"github.com/Hell077/HireRadar/apps/backend/internal/observability"
	"github.com/Hell077/HireRadar/apps/backend/internal/outbox"
	userdomain "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/google/uuid"
)

type WorkerStore interface {
	Application(context.Context, domain.ID) (domain.Application, error)
	Job(context.Context, string) (jobdomain.Job, error)
	Profile(context.Context, userdomain.UserID) (domain.ApplicationProfile, error)
	Prepared(context.Context, domain.ID) (domain.PreparedForm, error)
	SavePrepared(context.Context, domain.ID, domain.PreparedForm, []domain.Question) error
	BeginAttempt(context.Context, domain.ID, domain.Provider) (int, bool, error)
	FinishAttempt(context.Context, domain.ID, int, domain.Status, string, *domain.SubmissionResult) error
	SetStatus(context.Context, domain.ID, domain.Status, string, string) error
	MarkUncertain(context.Context, domain.ID) error
}

type Worker struct {
	events   outbox.Store
	store    WorkerStore
	registry Registry
	workerID string
	lease    time.Duration
	now      func() time.Time
}

func NewWorker(events outbox.Store, store WorkerStore, registry Registry, workerID string, now func() time.Time) *Worker {
	if now == nil {
		now = time.Now
	}
	if workerID == "" {
		workerID = uuid.NewString()
	}
	return &Worker{events: events, store: store, registry: registry, workerID: workerID, lease: 5 * time.Minute, now: now}
}

func (w *Worker) ProcessNext(ctx context.Context) (bool, error) { return w.process(ctx, "") }

func (w *Worker) ProcessEvent(ctx context.Context, eventID string) (bool, error) {
	return w.process(ctx, eventID)
}

func (w *Worker) process(ctx context.Context, onlyEvent string) (found bool, retErr error) {
	started := time.Now()
	defer func() {
		if found {
			observability.DefaultMetrics.Add("hireradar_application_events_total", nil, 1)
			observability.DefaultMetrics.ObserveDuration("hireradar_application_processing_duration", nil, time.Since(started))
		}
	}()
	w.refreshQueueMetrics(ctx)
	event, err := w.events.Claim(ctx, w.workerID, []string{"application.requested", "application.input_provided", "application.retry_requested"}, w.lease, onlyEvent)
	if err != nil || event == nil {
		return event != nil, err
	}
	var payload struct {
		ApplicationID string `json:"application_id"`
	}
	if event.AggregateType != "job_application" || json.Unmarshal(event.Payload, &payload) != nil || uuid.Validate(payload.ApplicationID) != nil || payload.ApplicationID != event.AggregateID {
		cause := errors.New("application event has invalid payload")
		return true, w.events.Fail(ctx, event.ID, w.workerID, cause)
	}
	app, err := w.store.Application(ctx, domain.ID(payload.ApplicationID))
	if err != nil {
		return true, w.retry(ctx, event, err)
	}
	if terminal(app.Status) || app.Status == domain.StatusNeedsInput {
		return true, w.events.Complete(ctx, event.ID, w.workerID)
	}
	if app.Status == domain.StatusSubmitting {
		if err := w.store.MarkUncertain(ctx, app.ID); err != nil {
			return true, w.retry(ctx, event, err)
		}
		return true, w.events.Complete(ctx, event.ID, w.workerID)
	}
	job, err := w.store.Job(ctx, app.JobID)
	if err != nil {
		return true, w.retry(ctx, event, err)
	}
	if job.Status != jobdomain.Active {
		if err := w.store.SetStatus(ctx, app.ID, domain.StatusCancelled, "job_closed", "job is no longer active"); err != nil {
			return true, w.retry(ctx, event, err)
		}
		return true, w.events.Complete(ctx, event.ID, w.workerID)
	}
	provider, ok := w.registry.Resolve(job)
	if !ok {
		if err := w.store.SetStatus(ctx, app.ID, domain.StatusManualRequired, "unsupported", "provider does not support automatic application"); err != nil {
			return true, w.retry(ctx, event, err)
		}
		return true, w.events.Complete(ctx, event.ID, w.workerID)
	}
	profile, err := w.store.Profile(ctx, userdomain.UserID(app.UserID))
	if err != nil {
		return true, w.retry(ctx, event, err)
	}
	var form domain.PreparedForm
	if app.Status == domain.StatusRequested || app.Status == domain.StatusPreparing {
		form, err = provider.Prepare(ctx, job, profile)
		if err != nil {
			return true, w.handleProviderError(ctx, event, app, err)
		}
		questions := missingRequiredQuestions(form)
		if err := w.store.SavePrepared(ctx, app.ID, form, questions); err != nil {
			return true, w.retry(ctx, event, err)
		}
		form, err = w.store.Prepared(ctx, app.ID)
		if err != nil {
			return true, w.retry(ctx, event, err)
		}
		if len(questions) > 0 {
			if len(missingRequiredQuestions(form)) > 0 {
				return true, w.events.Complete(ctx, event.ID, w.workerID)
			}
		}
		app.Status = domain.StatusReady
	} else if app.Status == domain.StatusReady {
		form, err = w.store.Prepared(ctx, app.ID)
		if err != nil {
			return true, w.retry(ctx, event, err)
		}
	} else {
		return true, w.events.Complete(ctx, event.ID, w.workerID)
	}
	if questions := missingRequiredQuestions(form); len(questions) > 0 {
		if err := w.store.SavePrepared(ctx, app.ID, form, questions); err != nil {
			return true, w.retry(ctx, event, err)
		}
		return true, w.events.Complete(ctx, event.ID, w.workerID)
	}
	attempt, begun, err := w.store.BeginAttempt(ctx, app.ID, provider.Name())
	if err != nil {
		return true, w.retry(ctx, event, err)
	}
	if !begun {
		return true, w.events.Complete(ctx, event.ID, w.workerID)
	}
	result, err := provider.Submit(ctx, app, form)
	if err != nil {
		var providerErr *domain.ProviderError
		if !errors.As(err, &providerErr) || providerErr.Category == domain.ErrorUnknown {
			if persistErr := w.store.FinishAttempt(ctx, app.ID, attempt, domain.StatusManualRequired, "outcome_uncertain", nil); persistErr != nil {
				return true, w.retry(ctx, event, persistErr)
			}
			return true, w.events.Complete(ctx, event.ID, w.workerID)
		}
		return true, w.handleSubmissionError(ctx, event, app, attempt, providerErr)
	}
	if result.Status != domain.StatusSubmitted {
		if err := w.store.FinishAttempt(ctx, app.ID, attempt, domain.StatusManualRequired, "provider_unconfirmed", &result); err != nil {
			return true, w.retry(ctx, event, err)
		}
		return true, w.events.Complete(ctx, event.ID, w.workerID)
	}
	if result.SubmittedAt.IsZero() {
		result.SubmittedAt = w.now()
	}
	if err := w.store.FinishAttempt(ctx, app.ID, attempt, domain.StatusSubmitted, "", &result); err != nil {
		return true, w.retry(ctx, event, err)
	}
	return true, w.events.Complete(ctx, event.ID, w.workerID)
}

func (w *Worker) refreshQueueMetrics(ctx context.Context) {
	metrics := observability.DefaultMetrics
	if !metrics.ShouldRefresh("application_queue", time.Now(), 30*time.Second) {
		return
	}
	stats, ok := w.events.(interface {
		QueueStats(context.Context, []string) (int64, float64, error)
	})
	if !ok {
		return
	}
	depth, oldestAge, err := stats.QueueStats(ctx, []string{"application.requested", "application.input_provided", "application.retry_requested"})
	if err == nil {
		metrics.SetGauge("hireradar_application_queue_depth", nil, float64(depth))
		metrics.SetGauge("hireradar_application_queue_oldest_pending_age_seconds", nil, oldestAge)
	}
}

func (w *Worker) handleProviderError(ctx context.Context, event *outbox.Event, app domain.Application, err error) error {
	var providerErr *domain.ProviderError
	if !errors.As(err, &providerErr) {
		providerErr = &domain.ProviderError{Category: domain.ErrorUnknown}
	}
	switch providerErr.Category {
	case domain.ErrorTemporary, domain.ErrorRateLimited, domain.ErrorProvider, domain.ErrorUnknown:
		if event.Attempts >= 12 {
			if err := w.store.SetStatus(ctx, app.ID, domain.StatusFailed, safeCode(providerErr.Code), "provider retries exhausted"); err != nil {
				return w.retry(ctx, event, err)
			}
			return w.events.Complete(ctx, event.ID, w.workerID)
		}
		if app.Status == domain.StatusPreparing {
			// Preserve the in-progress state; the same event resumes preparation.
		}
		return w.retry(ctx, event, errors.New("application provider temporary failure"))
	case domain.ErrorManualRequired, domain.ErrorAuthentication, domain.ErrorUnsupported:
		if err := w.store.SetStatus(ctx, app.ID, domain.StatusManualRequired, safeCode(providerErr.Code), "manual action is required"); err != nil {
			return w.retry(ctx, event, err)
		}
	case domain.ErrorJobClosed:
		if err := w.store.SetStatus(ctx, app.ID, domain.StatusCancelled, safeCode(providerErr.Code), "job is no longer accepting applications"); err != nil {
			return w.retry(ctx, event, err)
		}
	case domain.ErrorMissingInput:
		if err := w.store.SetStatus(ctx, app.ID, domain.StatusNeedsInput, safeCode(providerErr.Code), "additional information is required"); err != nil {
			return w.retry(ctx, event, err)
		}
	default:
		if err := w.store.SetStatus(ctx, app.ID, domain.StatusFailed, safeCode(providerErr.Code), "application could not be prepared"); err != nil {
			return w.retry(ctx, event, err)
		}
	}
	return w.events.Complete(ctx, event.ID, w.workerID)
}

func (w *Worker) handleSubmissionError(ctx context.Context, event *outbox.Event, app domain.Application, attempt int, providerErr *domain.ProviderError) error {
	switch providerErr.Category {
	case domain.ErrorTemporary, domain.ErrorRateLimited, domain.ErrorProvider:
		if event.Attempts >= 12 {
			if err := w.store.FinishAttempt(ctx, app.ID, attempt, domain.StatusFailed, safeCode(providerErr.Code), nil); err != nil {
				return w.retry(ctx, event, err)
			}
			return w.events.Complete(ctx, event.ID, w.workerID)
		}
		if err := w.store.FinishAttempt(ctx, app.ID, attempt, domain.StatusRequested, safeCode(providerErr.Code), nil); err != nil {
			return w.retry(ctx, event, err)
		}
		return w.retry(ctx, event, errors.New("application submission can be retried"))
	case domain.ErrorMissingInput:
		if err := w.store.FinishAttempt(ctx, app.ID, attempt, domain.StatusNeedsInput, safeCode(providerErr.Code), nil); err != nil {
			return w.retry(ctx, event, err)
		}
	case domain.ErrorManualRequired, domain.ErrorAuthentication, domain.ErrorUnsupported:
		if err := w.store.FinishAttempt(ctx, app.ID, attempt, domain.StatusManualRequired, safeCode(providerErr.Code), nil); err != nil {
			return w.retry(ctx, event, err)
		}
	case domain.ErrorJobClosed:
		if err := w.store.FinishAttempt(ctx, app.ID, attempt, domain.StatusCancelled, safeCode(providerErr.Code), nil); err != nil {
			return w.retry(ctx, event, err)
		}
	default:
		if err := w.store.FinishAttempt(ctx, app.ID, attempt, domain.StatusFailed, safeCode(providerErr.Code), nil); err != nil {
			return w.retry(ctx, event, err)
		}
	}
	return w.events.Complete(ctx, event.ID, w.workerID)
}

func (w *Worker) retry(ctx context.Context, event *outbox.Event, cause error) error {
	if err := w.events.Retry(ctx, event.ID, w.workerID, cause); err != nil {
		return fmt.Errorf("application work failed; schedule retry: %w", err)
	}
	observability.DefaultMetrics.Add("hireradar_applications_retry_total", nil, 1)
	return cause
}

func safeCode(code string) string {
	code = strings.TrimSpace(code)
	if code == "" || len(code) > 64 {
		return "provider_error"
	}
	for _, r := range code {
		if !(r == '_' || r == '-' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return "provider_error"
		}
	}
	return code
}

func terminal(status domain.Status) bool {
	return status == domain.StatusSubmitted || status == domain.StatusManualRequired || status == domain.StatusFailed || status == domain.StatusCancelled
}

func missingRequiredQuestions(form domain.PreparedForm) []domain.Question {
	questions := make([]domain.Question, 0)
	questionIndexes := make(map[string]int)
	known := make(map[string]bool, len(form.Fields))
	for _, field := range form.Fields {
		known[field.Key] = !missingValue(field.Value)
		if field.Required && !known[field.Key] {
			questionIndexes[field.Key] = len(questions)
			questions = append(questions, domain.Question{ExternalKey: field.Key, Text: field.Key, Type: field.Type, Required: true})
		}
	}
	for _, question := range form.Questions {
		if !question.Required || (question.ExternalKey != "" && known[question.ExternalKey]) {
			continue
		}
		if index, ok := questionIndexes[question.ExternalKey]; ok {
			questions[index] = question
			continue
		}
		if question.ExternalKey != "" {
			questionIndexes[question.ExternalKey] = len(questions)
		}
		questions = append(questions, question)
	}
	return questions
}

func missingValue(value any) bool {
	if value == nil {
		return true
	}
	text, ok := value.(string)
	return ok && strings.TrimSpace(text) == ""
}
