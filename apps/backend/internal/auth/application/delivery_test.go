package application

import (
	"strings"
	"testing"
)

func TestMessageForEvent(t *testing.T) {
	message, err := MessageForEvent("auth.verification_requested", []byte(`{"email":"person@example.com","verification_token":"opaque"}`), "http://localhost:3000/")
	if err != nil {
		t.Fatal(err)
	}
	if message.To != "person@example.com" || !strings.Contains(message.Body, "/verify-email?token=opaque") {
		t.Fatalf("message = %+v", message)
	}
	if _, err := MessageForEvent("auth.password_reset_requested", []byte(`{"email":"person@example.com"}`), "http://localhost:3000"); err == nil {
		t.Fatal("missing reset token accepted")
	}
}
