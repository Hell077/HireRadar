package password

import (
	"errors"
	"testing"
)

func TestArgon2idHashAndCompare(t *testing.T) {
	hasher := Argon2id{}
	first, err := hasher.Hash("a strong password")
	if err != nil {
		t.Fatal(err)
	}
	second, err := hasher.Hash("a strong password")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("salts produced identical hashes")
	}
	if err := hasher.Compare(first, "a strong password"); err != nil {
		t.Fatalf("Compare() correct password: %v", err)
	}
	if err := hasher.Compare(first, "wrong password"); !errors.Is(err, ErrMismatch) {
		t.Fatalf("Compare() wrong password = %v", err)
	}
}

func TestArgon2idRejectsMalformedHash(t *testing.T) {
	if err := (Argon2id{}).Compare("invalid", "password"); !errors.Is(err, ErrInvalidHash) {
		t.Fatalf("Compare() malformed hash = %v", err)
	}
}
