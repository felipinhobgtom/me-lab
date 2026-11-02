package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	// ErrInvalidSecret indicates an empty signing/verification secret.
	ErrInvalidSecret = errors.New("jwt secret must not be empty")
	// ErrInvalidUser indicates missing required user identity when issuing a token.
	ErrInvalidUser = errors.New("jwt user id must not be empty")
	// ErrMalformedToken indicates the JWT string or its JSON segments cannot be parsed.
	ErrMalformedToken = errors.New("malformed jwt token")
	// ErrUnsupportedAlg indicates the JWT header specifies an algorithm other than HS256.
	ErrUnsupportedAlg = errors.New("unsupported jwt signing algorithm")
	// ErrInvalidSignature indicates the HMAC-SHA256 signature verification failed.
	ErrInvalidSignature = errors.New("invalid jwt signature")
	// ErrExpiredToken indicates the token's expiration timestamp has passed.
	ErrExpiredToken = errors.New("jwt token expired")
)

// Claims represents validated identity claims extracted from a JWT.
type Claims struct {
	UserID    string    `json:"user_id"`
	Email     string    `json:"email"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

type jwtPayload struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Iat   int64  `json:"iat"`
	Exp   int64  `json:"exp"`
}

// IssueToken creates and signs an HS256 JWT containing userID and email for the given ttl.
func IssueToken(userID, email, secret string, ttl time.Duration) (string, error) {
	if secret == "" {
		return "", ErrInvalidSecret
	}
	if userID == "" {
		return "", ErrInvalidUser
	}

	now := time.Now().UTC()
	exp := now.Add(ttl)

	headerJSON, err := json.Marshal(jwtHeader{
		Alg: "HS256",
		Typ: "JWT",
	})
	if err != nil {
		return "", fmt.Errorf("marshal jwt header: %w", err)
	}

	payloadJSON, err := json.Marshal(jwtPayload{
		Sub:   userID,
		Email: email,
		Iat:   now.Unix(),
		Exp:   exp.Unix(),
	})
	if err != nil {
		return "", fmt.Errorf("marshal jwt payload: %w", err)
	}

	encodedHeader := base64.RawURLEncoding.EncodeToString(headerJSON)
	encodedPayload := base64.RawURLEncoding.EncodeToString(payloadJSON)
	signingInput := encodedHeader + "." + encodedPayload

	sig := signHS256(signingInput, secret)
	encodedSig := base64.RawURLEncoding.EncodeToString(sig)

	return signingInput + "." + encodedSig, nil
}

// ValidateToken verifies an HS256 JWT's structure, algorithm, HMAC-SHA256 signature, and expiration.
func ValidateToken(tokenStr, secret string) (*Claims, error) {
	if secret == "" {
		return nil, ErrInvalidSecret
	}

	segments := strings.Split(tokenStr, ".")
	if len(segments) != 3 || segments[0] == "" || segments[1] == "" || segments[2] == "" {
		return nil, ErrMalformedToken
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(segments[0])
	if err != nil {
		return nil, ErrMalformedToken
	}

	var header jwtHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, ErrMalformedToken
	}
	if header.Alg != "HS256" {
		return nil, ErrUnsupportedAlg
	}
	if header.Typ != "" && header.Typ != "JWT" {
		return nil, ErrMalformedToken
	}

	sigBytes, err := base64.RawURLEncoding.DecodeString(segments[2])
	if err != nil {
		return nil, ErrMalformedToken
	}

	signingInput := segments[0] + "." + segments[1]
	expectedSig := signHS256(signingInput, secret)
	if !hmac.Equal(sigBytes, expectedSig) {
		return nil, ErrInvalidSignature
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(segments[1])
	if err != nil {
		return nil, ErrMalformedToken
	}

	var payload jwtPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, ErrMalformedToken
	}
	if payload.Sub == "" || payload.Exp == 0 {
		return nil, ErrMalformedToken
	}

	if time.Now().UTC().Unix() >= payload.Exp {
		return nil, ErrExpiredToken
	}

	return &Claims{
		UserID:    payload.Sub,
		Email:     payload.Email,
		IssuedAt:  time.Unix(payload.Iat, 0).UTC(),
		ExpiresAt: time.Unix(payload.Exp, 0).UTC(),
	}, nil
}

func signHS256(signingInput, secret string) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(signingInput))
	return mac.Sum(nil)
}
