package token

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/golang-jwt/jwt/v5"
)

const accessTokenLifetime = 15 * time.Minute

type Signer struct {
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
}

func NewSigner(encodedSeed string) (*Signer, error) {
	seed, err := base64.StdEncoding.DecodeString(encodedSeed)
	if err != nil || len(seed) != ed25519.SeedSize {
		return nil, errors.New("JWT_PRIVATE_KEY must be a base64-encoded 32-byte Ed25519 seed")
	}
	privateKey := ed25519.NewKeyFromSeed(seed)
	return &Signer{privateKey: privateKey, publicKey: privateKey.Public().(ed25519.PublicKey)}, nil
}

func (s *Signer) Issue(userID domain.UserID, now time.Time) (string, time.Time, error) {
	if userID == "" {
		return "", time.Time{}, errors.New("user ID is required")
	}
	expiresAt := now.Add(accessTokenLifetime)
	claims := jwt.RegisteredClaims{
		Issuer:    "hireradar",
		Subject:   string(userID),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims).SignedString(s.privateKey)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}
	return signed, expiresAt, nil
}

func (s *Signer) Verify(raw string) (domain.UserID, error) {
	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(*jwt.Token) (any, error) {
		return s.publicKey, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}), jwt.WithIssuer("hireradar"), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if err != nil || !token.Valid || claims.Subject == "" {
		return "", errors.New("invalid access token")
	}
	return domain.UserID(claims.Subject), nil
}
