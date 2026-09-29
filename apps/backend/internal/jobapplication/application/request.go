package application

import (
	"context"
	"errors"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/jobapplication/domain"
	userdomain "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/google/uuid"
)

var ErrApplicationUnavailable = errors.New("job is not available for application")
var ErrTelegramAccountNotFound = errors.New("Telegram account is not connected")
var ErrInvalidApplicationAnswer = errors.New("invalid application answer")
var ErrInvalidApplicationOperation = errors.New("application cannot perform this operation")

type RequestStore interface {
	Request(context.Context, domain.Application) (domain.Application, error)
}

type TelegramUserStore interface {
	UserForTelegram(context.Context, int64) (userdomain.UserID, error)
}

type ApplicationQueryStore interface {
	Get(context.Context, userdomain.UserID, domain.ID) (domain.Application, error)
	List(context.Context, userdomain.UserID, int) ([]domain.Application, error)
}

type ApplicationQuestionStore interface {
	Questions(context.Context, userdomain.UserID, domain.ID) ([]domain.ApplicationQuestion, error)
	AnswerQuestion(context.Context, userdomain.UserID, string, string) error
}

type ApplicationControlStore interface {
	Cancel(context.Context, userdomain.UserID, domain.ID) error
	Retry(context.Context, userdomain.UserID, domain.ID) error
}

type RequestService struct {
	store RequestStore
	newID func() string
	now   func() time.Time
}

func NewRequestService(store RequestStore, newID func() string, now func() time.Time) *RequestService {
	if newID == nil {
		newID = uuid.NewString
	}
	if now == nil {
		now = time.Now
	}
	return &RequestService{store: store, newID: newID, now: now}
}

func (s *RequestService) Request(ctx context.Context, userID userdomain.UserID, jobID string) (domain.Application, error) {
	if userID == "" || jobID == "" {
		return domain.Application{}, ErrApplicationUnavailable
	}
	app, err := domain.NewApplication(domain.ID(s.newID()), string(userID), jobID, "unknown", s.now())
	if err != nil {
		return domain.Application{}, err
	}
	return s.store.Request(ctx, app)
}

func (s *RequestService) RequestFromTelegram(ctx context.Context, telegramUserID int64, jobID string) error {
	store, ok := s.store.(TelegramUserStore)
	if !ok || telegramUserID <= 0 {
		return ErrTelegramAccountNotFound
	}
	userID, err := store.UserForTelegram(ctx, telegramUserID)
	if err != nil {
		return err
	}
	if userID == "" {
		return ErrTelegramAccountNotFound
	}
	_, err = s.Request(ctx, userID, jobID)
	return err
}

func (s *RequestService) Get(ctx context.Context, userID userdomain.UserID, id domain.ID) (domain.Application, error) {
	store, ok := s.store.(ApplicationQueryStore)
	if !ok {
		return domain.Application{}, ErrApplicationUnavailable
	}
	return store.Get(ctx, userID, id)
}

func (s *RequestService) List(ctx context.Context, userID userdomain.UserID, limit int) ([]domain.Application, error) {
	store, ok := s.store.(ApplicationQueryStore)
	if !ok {
		return nil, ErrApplicationUnavailable
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}
	return store.List(ctx, userID, limit)
}

func (s *RequestService) Questions(ctx context.Context, userID userdomain.UserID, id domain.ID) ([]domain.ApplicationQuestion, error) {
	store, ok := s.store.(ApplicationQuestionStore)
	if !ok {
		return nil, ErrApplicationUnavailable
	}
	return store.Questions(ctx, userID, id)
}

func (s *RequestService) AnswerQuestion(ctx context.Context, userID userdomain.UserID, questionID, answer string) error {
	store, ok := s.store.(ApplicationQuestionStore)
	if !ok {
		return ErrApplicationUnavailable
	}
	if userID == "" || questionID == "" || len(answer) == 0 || len(answer) > 2000 {
		return ErrInvalidApplicationAnswer
	}
	return store.AnswerQuestion(ctx, userID, questionID, answer)
}

func (s *RequestService) AnswerFromTelegram(ctx context.Context, telegramUserID int64, questionID, answer string) error {
	store, ok := s.store.(TelegramUserStore)
	if !ok || telegramUserID <= 0 {
		return ErrTelegramAccountNotFound
	}
	userID, err := store.UserForTelegram(ctx, telegramUserID)
	if err != nil {
		return err
	}
	return s.AnswerQuestion(ctx, userID, questionID, answer)
}

func (s *RequestService) Cancel(ctx context.Context, userID userdomain.UserID, id domain.ID) error {
	store, ok := s.store.(ApplicationControlStore)
	if !ok {
		return ErrApplicationUnavailable
	}
	return store.Cancel(ctx, userID, id)
}

func (s *RequestService) Retry(ctx context.Context, userID userdomain.UserID, id domain.ID) error {
	store, ok := s.store.(ApplicationControlStore)
	if !ok {
		return ErrApplicationUnavailable
	}
	return store.Retry(ctx, userID, id)
}
