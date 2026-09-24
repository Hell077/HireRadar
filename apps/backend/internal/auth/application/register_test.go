package application

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
)

type fakeHasher struct{ called bool }

func (f *fakeHasher) Hash(string) (string, error) {
	f.called = true
	return "hashed", nil
}
func (*fakeHasher) Compare(string, string) error { return nil }

type fakeRegistrationStore struct {
	called       bool
	registration Registration
	err          error
	exists       bool
}

func (f *fakeRegistrationStore) ExistsByEmail(context.Context, domain.Email) (bool, error) {
	return f.exists, f.err
}

func (f *fakeRegistrationStore) CreateRegistration(_ context.Context, registration Registration) error {
	f.called = true
	f.registration = registration
	return f.err
}

func TestRegisterCreatesUserTokenAndOutboxEvent(t *testing.T) {
	now := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	store := &fakeRegistrationStore{}
	service := NewRegisterService(store, &fakeHasher{}, func() time.Time { return now })
	userID, err := service.Register(context.Background(), " Person@Example.COM ", "a sufficiently long password")
	if err != nil {
		t.Fatal(err)
	}
	registration := store.registration
	if !store.called || registration.User.ID != userID || registration.User.Email != "person@example.com" {
		t.Fatalf("unexpected registration: %+v", registration)
	}
	if registration.TokenExpiresAt != now.Add(24*time.Hour) || registration.OutboxEventType != "auth.verification_requested" {
		t.Fatalf("unexpected token or event: %+v", registration)
	}
	var payload struct {
		VerificationToken string `json:"verification_token"`
	}
	if err := json.Unmarshal(registration.OutboxEventBytes, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.VerificationToken == "" || registration.TokenHash != sha256.Sum256([]byte(payload.VerificationToken)) {
		t.Fatal("verification token hash does not match event token")
	}
}

func TestRegisterRejectsWeakPasswordBeforeHashing(t *testing.T) {
	store := &fakeRegistrationStore{}
	hasher := &fakeHasher{}
	service := NewRegisterService(store, hasher, time.Now)
	_, err := service.Register(context.Background(), "person@example.com", "short")
	if !errors.Is(err, ErrInvalidPassword) || store.called || hasher.called {
		t.Fatalf("Register() = %v, store called = %v, hasher called = %v", err, store.called, hasher.called)
	}
}

func TestRegisterRejectsExistingEmailBeforeHashing(t *testing.T) {
	store := &fakeRegistrationStore{exists: true}
	hasher := &fakeHasher{}
	service := NewRegisterService(store, hasher, time.Now)
	_, err := service.Register(context.Background(), "person@example.com", "a sufficiently long password")
	if !errors.Is(err, ErrEmailTaken) || store.called || hasher.called {
		t.Fatalf("Register() = %v, store called = %v, hasher called = %v", err, store.called, hasher.called)
	}
}
