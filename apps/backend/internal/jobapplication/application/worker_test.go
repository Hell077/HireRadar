package application

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	jobdomain "github.com/Hell077/HireRadar/apps/backend/internal/job/domain"
	"github.com/Hell077/HireRadar/apps/backend/internal/jobapplication/domain"
	"github.com/Hell077/HireRadar/apps/backend/internal/outbox"
	userdomain "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
)

type workerEvents struct {
	event     *outbox.Event
	completed bool
	retried   bool
	failed    bool
}

func (e *workerEvents) Claim(context.Context, string, []string, time.Duration, string) (*outbox.Event, error) {
	return e.event, nil
}
func (e *workerEvents) Complete(context.Context, string, string) error {
	e.completed = true
	return nil
}
func (e *workerEvents) Retry(context.Context, string, string, error) error {
	e.retried = true
	return nil
}
func (e *workerEvents) Fail(context.Context, string, string, error) error {
	e.failed = true
	return nil
}

type workerStore struct {
	app          domain.Application
	form         domain.PreparedForm
	questions    []domain.Question
	prepared     int
	attempt      int
	status       domain.Status
	uncertain    bool
	finishStatus domain.Status
}

func (s *workerStore) Application(context.Context, domain.ID) (domain.Application, error) {
	return s.app, nil
}
func (*workerStore) Job(context.Context, string) (jobdomain.Job, error) {
	return jobdomain.Job{Status: jobdomain.Active}, nil
}
func (*workerStore) Profile(context.Context, userdomain.UserID) (domain.ApplicationProfile, error) {
	return domain.ApplicationProfile{}, nil
}
func (s *workerStore) Prepared(context.Context, domain.ID) (domain.PreparedForm, error) {
	return s.form, nil
}
func (s *workerStore) SavePrepared(_ context.Context, _ domain.ID, form domain.PreparedForm, questions []domain.Question) error {
	s.prepared++
	s.form, s.questions = form, questions
	if len(questions) > 0 {
		s.app.Status = domain.StatusNeedsInput
	} else {
		s.app.Status = domain.StatusReady
	}
	return nil
}
func (s *workerStore) BeginAttempt(context.Context, domain.ID, domain.Provider) (int, bool, error) {
	if s.app.Status != domain.StatusReady {
		return s.attempt, false, nil
	}
	s.attempt++
	s.app.Status = domain.StatusSubmitting
	return s.attempt, true, nil
}
func (s *workerStore) FinishAttempt(_ context.Context, _ domain.ID, _ int, status domain.Status, _ string, _ *domain.SubmissionResult) error {
	s.finishStatus, s.app.Status = status, status
	return nil
}
func (s *workerStore) SetStatus(_ context.Context, _ domain.ID, status domain.Status, _, _ string) error {
	s.status, s.app.Status = status, status
	return nil
}
func (s *workerStore) MarkUncertain(context.Context, domain.ID) error {
	s.uncertain, s.app.Status = true, domain.StatusManualRequired
	return nil
}

type workerProvider struct {
	form     domain.PreparedForm
	result   domain.SubmissionResult
	err      error
	prepareN int
	submitN  int
}

func (p *workerProvider) Name() domain.Provider { return "test" }
func (p *workerProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{AutoApply: true}
}
func (*workerProvider) Supports(jobdomain.Job) bool { return true }
func (p *workerProvider) Prepare(context.Context, jobdomain.Job, domain.ApplicationProfile) (domain.PreparedForm, error) {
	p.prepareN++
	return p.form, p.err
}
func (p *workerProvider) Submit(context.Context, domain.Application, domain.PreparedForm) (domain.SubmissionResult, error) {
	p.submitN++
	return p.result, p.err
}

func newWorkerTestEvent(appID string) *outbox.Event {
	payload, _ := json.Marshal(map[string]string{"application_id": appID})
	return &outbox.Event{ID: "event", EventType: "application.requested", AggregateType: "job_application", AggregateID: appID, Payload: payload, Attempts: 1}
}

func TestWorkerPersistsMissingRequiredInputWithoutSubmitting(t *testing.T) {
	appID := "860f9a77-2739-49b7-bd92-8675293c8626"
	events := &workerEvents{event: newWorkerTestEvent(appID)}
	store := &workerStore{app: domain.Application{ID: domain.ID(appID), UserID: "user", JobID: "job", Status: domain.StatusRequested}}
	provider := &workerProvider{form: domain.PreparedForm{Fields: []domain.Field{{Key: "work_authorization", Type: "boolean", Required: true}}}}
	worker := NewWorker(events, store, NewProviderRegistry(provider), "worker", time.Now)
	if processed, err := worker.ProcessNext(context.Background()); err != nil || !processed {
		t.Fatalf("processed=%v err=%v", processed, err)
	}
	if store.app.Status != domain.StatusNeedsInput || len(store.questions) != 1 || provider.submitN != 0 || !events.completed {
		t.Fatalf("application=%+v questions=%+v submits=%d complete=%v", store.app, store.questions, provider.submitN, events.completed)
	}
}

func TestWorkerKeepsProviderQuestionWithoutDuplicatingMissingField(t *testing.T) {
	form := domain.PreparedForm{
		Fields:    []domain.Field{{Key: "work_authorization", Type: "boolean", Required: true}},
		Questions: []domain.Question{{ExternalKey: "work_authorization", Text: "Are you authorized to work?", Type: "boolean", Required: true, Options: []domain.Option{{Value: "true", Label: "Yes"}, {Value: "false", Label: "No"}}}},
	}
	questions := missingRequiredQuestions(form)
	if len(questions) != 1 || questions[0].Text != "Are you authorized to work?" || len(questions[0].Options) != 2 {
		t.Fatalf("expected one provider question with options, got %+v", questions)
	}
}

func TestWorkerSubmitsPreparedApplicationOnce(t *testing.T) {
	appID := "860f9a77-2739-49b7-bd92-8675293c8627"
	events := &workerEvents{event: newWorkerTestEvent(appID)}
	store := &workerStore{app: domain.Application{ID: domain.ID(appID), UserID: "user", JobID: "job", Status: domain.StatusRequested}}
	provider := &workerProvider{result: domain.SubmissionResult{Status: domain.StatusSubmitted}}
	worker := NewWorker(events, store, NewProviderRegistry(provider), "worker", func() time.Time { return time.Unix(100, 0).UTC() })
	if processed, err := worker.ProcessNext(context.Background()); err != nil || !processed {
		t.Fatalf("processed=%v err=%v", processed, err)
	}
	if store.finishStatus != domain.StatusSubmitted || provider.prepareN != 1 || provider.submitN != 1 || !events.completed {
		t.Fatalf("status=%q prepared=%d submitted=%d complete=%v", store.finishStatus, provider.prepareN, provider.submitN, events.completed)
	}
}

func TestWorkerMarksStaleSubmittingAttemptUncertain(t *testing.T) {
	appID := "860f9a77-2739-49b7-bd92-8675293c8628"
	events := &workerEvents{event: newWorkerTestEvent(appID)}
	store := &workerStore{app: domain.Application{ID: domain.ID(appID), Status: domain.StatusSubmitting}}
	provider := &workerProvider{}
	worker := NewWorker(events, store, NewProviderRegistry(provider), "worker", time.Now)
	if processed, err := worker.ProcessNext(context.Background()); err != nil || !processed {
		t.Fatalf("processed=%v err=%v", processed, err)
	}
	if !store.uncertain || provider.submitN != 0 || store.app.Status != domain.StatusManualRequired || !events.completed {
		t.Fatalf("uncertain=%v submits=%d application=%+v", store.uncertain, provider.submitN, store.app)
	}
}

func TestWorkerDoesNotExposeProviderErrorsToOutboxRetry(t *testing.T) {
	appID := "860f9a77-2739-49b7-bd92-8675293c8629"
	events := &workerEvents{event: newWorkerTestEvent(appID)}
	store := &workerStore{app: domain.Application{ID: domain.ID(appID), UserID: "user", JobID: "job", Status: domain.StatusRequested}}
	provider := &workerProvider{err: errors.New("secret https://provider.test/?token=do-not-log")}
	worker := NewWorker(events, store, NewProviderRegistry(provider), "worker", time.Now)
	if processed, err := worker.ProcessNext(context.Background()); !processed || err == nil {
		t.Fatalf("processed=%v err=%v", processed, err)
	}
	if !events.retried {
		t.Fatal("temporary provider error was not scheduled for retry")
	}
}

func TestWorkerClassifiesPreparationFailures(t *testing.T) {
	for _, test := range []struct {
		category domain.ErrorCategory
		status   domain.Status
		retry    bool
	}{
		{domain.ErrorTemporary, domain.StatusRequested, true},
		{domain.ErrorRateLimited, domain.StatusRequested, true},
		{domain.ErrorProvider, domain.StatusRequested, true},
		{domain.ErrorUnknown, domain.StatusRequested, true},
		{domain.ErrorValidation, domain.StatusFailed, false},
		{domain.ErrorMissingInput, domain.StatusNeedsInput, false},
		{domain.ErrorManualRequired, domain.StatusManualRequired, false},
		{domain.ErrorUnsupported, domain.StatusManualRequired, false},
		{domain.ErrorJobClosed, domain.StatusCancelled, false},
		{domain.ErrorAuthentication, domain.StatusManualRequired, false},
	} {
		t.Run(string(test.category), func(t *testing.T) {
			appID := "860f9a77-2739-49b7-bd92-8675293c8630"
			events := &workerEvents{event: newWorkerTestEvent(appID)}
			store := &workerStore{app: domain.Application{ID: domain.ID(appID), UserID: "user", JobID: "job", Status: domain.StatusRequested}}
			provider := &workerProvider{err: &domain.ProviderError{Category: test.category, Code: "fixture_error"}}
			worker := NewWorker(events, store, NewProviderRegistry(provider), "worker", time.Now)
			if processed, err := worker.ProcessNext(context.Background()); !processed || (err != nil) != test.retry {
				t.Fatalf("processed=%v err=%v retry=%v", processed, err, test.retry)
			}
			if store.app.Status != test.status || events.retried != test.retry || events.completed == test.retry {
				t.Fatalf("status=%q retry=%v complete=%v", store.app.Status, events.retried, events.completed)
			}
		})
	}
}
