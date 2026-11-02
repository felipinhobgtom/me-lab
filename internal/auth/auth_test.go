package auth

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestHashAndVerifyPassword(t *testing.T) {
	t.Parallel()

	password := "correct-horse-battery-staple-42!"
	hash1, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if !strings.HasPrefix(hash1, "$argon2id$v=19$m=65536,t=3,p=2$") {
		t.Fatalf("unexpected PHC prefix in hash %q", hash1)
	}

	hash2, err := HashPassword(password)
	if err != nil {
		t.Fatalf("second HashPassword failed: %v", err)
	}
	if hash1 == hash2 {
		t.Fatal("expected distinct random salts to produce distinct hashes")
	}

	ok, err := VerifyPassword(password, hash1)
	if err != nil {
		t.Fatalf("VerifyPassword returned unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected VerifyPassword to succeed for matching password")
	}
}

func TestVerifyPasswordWrongAndMalformed(t *testing.T) {
	t.Parallel()

	hash, err := HashPassword("my-secret-password")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	ok, err := VerifyPassword("wrong-password", hash)
	if err != nil {
		t.Fatalf("VerifyPassword returned unexpected error on wrong password: %v", err)
	}
	if ok {
		t.Fatal("expected VerifyPassword to return false for wrong password")
	}

	if _, err := HashPassword(""); !errors.Is(err, ErrEmptyPassword) {
		t.Fatalf("expected ErrEmptyPassword, got %v", err)
	}

	malformedInputs := []string{
		"",
		"not-a-phc-string",
		"$bcrypt$v=19$m=65536,t=3,p=2$c2FsdA$aGFzaA",
		"$argon2id$v=18$m=65536,t=3,p=2$c2FsdA$aGFzaA",
		"$argon2id$v=19$m=0,t=3,p=2$c2FsdA$aGFzaA",
		"$argon2id$v=19$m=65536,t=3,p=2$!!!$aGFzaA",
	}
	for _, bad := range malformedInputs {
		ok, err := VerifyPassword("my-secret-password", bad)
		if err == nil || ok {
			t.Errorf("expected error and ok=false for malformed hash %q, got ok=%v err=%v", bad, ok, err)
		}
	}
}

func TestJWTIssueAndValidate(t *testing.T) {
	t.Parallel()

	secret := "test-super-secret-hmac-key-32-bytes"
	token, err := IssueToken("user-123", "engineer@melab.dev", secret, time.Hour)
	if err != nil {
		t.Fatalf("IssueToken failed: %v", err)
	}

	claims, err := ValidateToken(token, secret)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}
	if claims.UserID != "user-123" {
		t.Errorf("UserID = %q, want %q", claims.UserID, "user-123")
	}
	if claims.Email != "engineer@melab.dev" {
		t.Errorf("Email = %q, want %q", claims.Email, "engineer@melab.dev")
	}
	if !claims.ExpiresAt.After(claims.IssuedAt) {
		t.Errorf("expected ExpiresAt (%v) after IssuedAt (%v)", claims.ExpiresAt, claims.IssuedAt)
	}
}

func TestJWTTamperedAndUnsupportedAlg(t *testing.T) {
	t.Parallel()

	secret := "test-super-secret-hmac-key-32-bytes"
	token, err := IssueToken("user-123", "engineer@melab.dev", secret, time.Hour)
	if err != nil {
		t.Fatalf("IssueToken failed: %v", err)
	}

	// Wrong verification secret.
	if _, err := ValidateToken(token, "wrong-secret"); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("expected ErrInvalidSignature with wrong secret, got %v", err)
	}

	// Tampered payload with original signature.
	parts := strings.Split(token, ".")
	tamperedPayload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"admin","email":"root@melab.dev","iat":1700000000,"exp":4900000000}`))
	tamperedToken := parts[0] + "." + tamperedPayload + "." + parts[2]
	if _, err := ValidateToken(tamperedToken, secret); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("expected ErrInvalidSignature on tampered payload, got %v", err)
	}

	// Tampered signature bytes.
	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatalf("decode signature: %v", err)
	}
	sigBytes[0] ^= 0xff
	badSigToken := parts[0] + "." + parts[1] + "." + base64.RawURLEncoding.EncodeToString(sigBytes)
	if _, err := ValidateToken(badSigToken, secret); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("expected ErrInvalidSignature on mutated signature, got %v", err)
	}

	// Unsupported algorithm ("none").
	noneHeader := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	noneToken := noneHeader + "." + parts[1] + "." + parts[2]
	if _, err := ValidateToken(noneToken, secret); !errors.Is(err, ErrUnsupportedAlg) {
		t.Fatalf("expected ErrUnsupportedAlg, got %v", err)
	}

	// Malformed tokens.
	for _, malformed := range []string{"", "onlyone", "a.b", "a.b.c.d", "..", "!.@.?"} {
		if _, err := ValidateToken(malformed, secret); !errors.Is(err, ErrMalformedToken) {
			t.Errorf("expected ErrMalformedToken for %q, got %v", malformed, err)
		}
	}
}

func TestJWTExpiredToken(t *testing.T) {
	t.Parallel()

	secret := "test-super-secret-hmac-key-32-bytes"
	expiredToken, err := IssueToken("user-123", "engineer@melab.dev", secret, -time.Minute)
	if err != nil {
		t.Fatalf("IssueToken failed: %v", err)
	}

	if _, err := ValidateToken(expiredToken, secret); !errors.Is(err, ErrExpiredToken) {
		t.Fatalf("expected ErrExpiredToken, got %v", err)
	}
}
