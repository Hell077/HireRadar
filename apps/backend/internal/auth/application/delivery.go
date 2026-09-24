package application

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

type EmailMessage struct {
	To      string
	Subject string
	Body    string
}

func MessageForEvent(eventType string, payload []byte, publicURL string) (EmailMessage, error) {
	var event struct {
		Email             string `json:"email"`
		VerificationToken string `json:"verification_token"`
		ResetToken        string `json:"reset_token"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		return EmailMessage{}, fmt.Errorf("decode email event: %w", err)
	}
	if strings.ContainsAny(event.Email, "\r\n") {
		return EmailMessage{}, errors.New("invalid email recipient")
	}
	base := strings.TrimRight(publicURL, "/")
	switch eventType {
	case "auth.verification_requested":
		if event.VerificationToken == "" {
			return EmailMessage{}, errors.New("verification token missing")
		}
		return EmailMessage{To: event.Email, Subject: "Confirm your HireRadar email", Body: "Confirm your email: " + base + "/verify-email?token=" + url.QueryEscape(event.VerificationToken)}, nil
	case "auth.password_reset_requested":
		if event.ResetToken == "" {
			return EmailMessage{}, errors.New("reset token missing")
		}
		return EmailMessage{To: event.Email, Subject: "Reset your HireRadar password", Body: "Reset your password: " + base + "/reset-password?token=" + url.QueryEscape(event.ResetToken)}, nil
	default:
		return EmailMessage{}, fmt.Errorf("unsupported email event %q", eventType)
	}
}
