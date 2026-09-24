package token

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestSignerIssuesAndVerifiesAccessToken(t *testing.T) {
	signer, err := NewSigner(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	raw, expiresAt, err := signer.Issue("user-id", now)
	if err != nil {
		t.Fatal(err)
	}
	if expiresAt != now.Add(15*time.Minute) {
		t.Fatalf("expiration = %v", expiresAt)
	}
	userID, err := signer.Verify(raw)
	if err != nil || userID != "user-id" {
		t.Fatalf("Verify() = %q, %v", userID, err)
	}
	if _, err := signer.Verify(raw + strings.Repeat("x", 4)); err == nil {
		t.Fatal("tampered token verified")
	}
}

func TestSignerRejectsInvalidSeed(t *testing.T) {
	if _, err := NewSigner("short"); err == nil {
		t.Fatal("invalid seed accepted")
	}
}
