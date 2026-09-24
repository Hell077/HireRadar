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

type fakeEmailStore struct {
	user         *domain.User
	request      PasswordResetRequest
	verified     [sha256.Size]byte
	resetHash    [sha256.Size]byte
	passwordHash string
	createCalled bool
}

func (f *fakeEmailStore) ByEmail(context.Context, domain.Email) (*domain.User, error) {
	return f.user, nil
}
func (f *fakeEmailStore) VerifyEmail(_ context.Context, hash [sha256.Size]byte, _ time.Time) error {
	f.verified = hash
	return nil
}
func (f *fakeEmailStore) CreatePasswordReset(_ context.Context, request PasswordResetRequest) error {
	f.request = request
	f.createCalled = true
	return nil
}
func (f *fakeEmailStore) ConsumePasswordReset(_ context.Context, hash [sha256.Size]byte, passwordHash string, _ time.Time) error {
	f.resetHash = hash
	f.passwordHash = passwordHash
	return nil
}

func TestEmailVerificationRejectsMalformedToken(t *testing.T) {
	store := &fakeEmailStore{}
	service := NewEmailService(store, &fakeHasher{}, time.Now)
	if err := service.Verify(context.Background(), "bad"); !errors.Is(err, ErrInvalidVerificationToken) {
		t.Fatalf("Verify() = %v", err)
	}
	if store.verified != ([sha256.Size]byte{}) {
		t.Fatal("store called for malformed token")
	}
}

func TestPasswordResetRequestHidesUnknownEmail(t *testing.T) {
	store := &fakeEmailStore{}
	service := NewEmailService(store, &fakeHasher{}, time.Now)
	for _, email := range []string{"missing@example.com", "bad email"} {
		if err := service.RequestReset(context.Background(), email); err != nil {
			t.Fatal(err)
		}
	}
	if store.createCalled {
		t.Fatal("created token for unknown email")
	}
}

func TestPasswordResetRequestStoresHashAndEvent(t *testing.T) {
	now := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	store := &fakeEmailStore{user: &domain.User{ID: "user-id", Email: "person@example.com", Status: domain.StatusActive}}
	service := NewEmailService(store, &fakeHasher{}, func() time.Time { return now })
	if err := service.RequestReset(context.Background(), "person@example.com"); err != nil {
		t.Fatal(err)
	}
	if !store.createCalled || store.request.ExpiresAt != now.Add(time.Hour) {
		t.Fatalf("request = %+v", store.request)
	}
	var event struct {
		ResetToken string `json:"reset_token"`
	}
	if err := json.Unmarshal(store.request.OutboxEventBytes, &event); err != nil {
		t.Fatal(err)
	}
	if !validOpaqueToken(event.ResetToken) || store.request.TokenHash != sha256.Sum256([]byte(event.ResetToken)) {
		t.Fatal("event token does not match stored hash")
	}
	if err := service.ResetPassword(context.Background(), event.ResetToken, "a replacement password"); err != nil {
		t.Fatal(err)
	}
	if store.resetHash != store.request.TokenHash || store.passwordHash != "hashed" {
		t.Fatal("reset did not use token hash and hashed password")
	}
}
