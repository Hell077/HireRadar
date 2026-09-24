package domain

import (
	"errors"
	"testing"
	"time"
)

func TestNewEmailNormalizesAndRejectsInvalidAddress(t *testing.T) {
	email, err := NewEmail("  Person@Example.COM  ")
	if err != nil || email != "person@example.com" {
		t.Fatalf("NewEmail() = %q, %v", email, err)
	}
	for _, raw := range []string{"", "not-an-email", "Person <person@example.com>", "a b@example.com"} {
		if _, err := NewEmail(raw); !errors.Is(err, ErrInvalidEmail) {
			t.Errorf("NewEmail(%q) error = %v", raw, err)
		}
	}
}

func TestBlockedUserCannotLoginOrVerify(t *testing.T) {
	now := time.Now()
	user, err := NewUser("user-id", "person@example.com", "hash", now)
	if err != nil {
		t.Fatal(err)
	}
	user.Status = StatusBlocked
	if err := user.CanLogin(); !errors.Is(err, ErrUserNotActive) {
		t.Fatalf("CanLogin() = %v", err)
	}
	if err := user.VerifyEmail(now.Add(time.Minute)); !errors.Is(err, ErrUserNotActive) || user.EmailVerified {
		t.Fatalf("VerifyEmail() = %v, verified = %v", err, user.EmailVerified)
	}
}
