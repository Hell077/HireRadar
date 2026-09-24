package domain

import (
	"errors"
	"net/mail"
	"strings"
	"time"
)

var (
	ErrInvalidEmail    = errors.New("invalid email")
	ErrInvalidUser     = errors.New("invalid user")
	ErrUserNotActive   = errors.New("user is not active")
	ErrAlreadyVerified = errors.New("email already verified")
)

type UserID string
type Email string
type Status string

const (
	StatusActive  Status = "active"
	StatusBlocked Status = "blocked"
	StatusDeleted Status = "deleted"
)

type User struct {
	ID            UserID
	Email         Email
	PasswordHash  string
	Status        Status
	EmailVerified bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func NewEmail(raw string) (Email, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if len(value) < 3 || len(value) > 320 {
		return "", ErrInvalidEmail
	}
	address, err := mail.ParseAddress(value)
	if err != nil || address.Address != value || strings.ContainsAny(value, " \t\n\r") {
		return "", ErrInvalidEmail
	}
	return Email(value), nil
}

func NewUser(id UserID, email Email, passwordHash string, now time.Time) (*User, error) {
	if id == "" || email == "" || passwordHash == "" || now.IsZero() {
		return nil, ErrInvalidUser
	}
	normalized, err := NewEmail(string(email))
	if err != nil || normalized != email {
		return nil, ErrInvalidUser
	}
	return &User{
		ID:           id,
		Email:        email,
		PasswordHash: passwordHash,
		Status:       StatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func (u *User) CanLogin() error {
	if u.Status != StatusActive {
		return ErrUserNotActive
	}
	return nil
}

func (u *User) VerifyEmail(now time.Time) error {
	if u.EmailVerified {
		return ErrAlreadyVerified
	}
	if err := u.CanLogin(); err != nil {
		return err
	}
	u.EmailVerified = true
	u.UpdatedAt = now
	return nil
}
