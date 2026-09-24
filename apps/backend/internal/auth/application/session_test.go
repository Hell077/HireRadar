package application

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
)

type sessionHasher struct{ comparisons int }

func (*sessionHasher) Hash(string) (string, error) { return "stored-hash", nil }
func (h *sessionHasher) Compare(_ string, password string) error {
	h.comparisons++
	if password != "correct password" {
		return errors.New("mismatch")
	}
	return nil
}

type fakeAccessSigner struct{}

func (fakeAccessSigner) Issue(_ domain.UserID, now time.Time) (string, time.Time, error) {
	return "signed-access", now.Add(15 * time.Minute), nil
}

type fakeSessionStore struct {
	user    *domain.User
	session Session
	revoked bool
}

func (f *fakeSessionStore) ByEmail(context.Context, domain.Email) (*domain.User, error) {
	return f.user, nil
}
func (f *fakeSessionStore) CreateSession(_ context.Context, session Session) error {
	f.session = session
	return nil
}
func (f *fakeSessionStore) RotateSession(_ context.Context, oldHash, newHash [sha256.Size]byte, _ time.Time, expiresAt time.Time) (domain.UserID, error) {
	if f.revoked || oldHash != f.session.RefreshTokenHash {
		return "", ErrInvalidRefreshToken
	}
	f.session.RefreshTokenHash = newHash
	f.session.ExpiresAt = expiresAt
	return f.session.UserID, nil
}
func (f *fakeSessionStore) RevokeSession(_ context.Context, hash [sha256.Size]byte, _ time.Time) error {
	if hash != f.session.RefreshTokenHash || f.revoked {
		return ErrInvalidRefreshToken
	}
	f.revoked = true
	return nil
}

func TestSessionRefreshRotatesTokenAndRejectsReplay(t *testing.T) {
	now := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	store := &fakeSessionStore{user: &domain.User{ID: "user-id", Email: "person@example.com", PasswordHash: "stored-hash", Status: domain.StatusActive}}
	service, err := NewSessionService(store, &sessionHasher{}, fakeAccessSigner{}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.Login(context.Background(), "person@example.com", "correct password", "test-agent", "")
	if err != nil {
		t.Fatal(err)
	}
	if first.AccessToken != "signed-access" || len(first.RefreshToken) != 43 || store.session.RefreshTokenHash != sha256.Sum256([]byte(first.RefreshToken)) {
		t.Fatalf("invalid login tokens or stored hash: %+v", first)
	}
	second, err := service.Refresh(context.Background(), first.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if second.RefreshToken == first.RefreshToken {
		t.Fatal("refresh token was not rotated")
	}
	if _, err := service.Refresh(context.Background(), first.RefreshToken); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("reused refresh token accepted: %v", err)
	}
	if err := service.Logout(context.Background(), second.RefreshToken); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Refresh(context.Background(), second.RefreshToken); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("revoked refresh token accepted: %v", err)
	}
}

func TestLoginUsesDummyHashForUnknownEmail(t *testing.T) {
	hasher := &sessionHasher{}
	service, err := NewSessionService(&fakeSessionStore{}, hasher, fakeAccessSigner{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Login(context.Background(), "missing@example.com", "wrong password", "", ""); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login() = %v", err)
	}
	if hasher.comparisons != 1 {
		t.Fatalf("dummy hash comparisons = %d, want 1", hasher.comparisons)
	}
}
