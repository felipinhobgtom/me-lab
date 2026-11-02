package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonMemory      uint32 = 65536 // 64 MiB
	argonIterations  uint32 = 3
	argonParallelism uint8  = 2
	argonSaltLen     int    = 16
	argonKeyLen      uint32 = 32
)

var (
	// ErrEmptyPassword indicates that an empty password was supplied for hashing.
	ErrEmptyPassword = errors.New("password must not be empty")
	// ErrMalformedHash indicates that an encoded password hash is not valid PHC Argon2id format.
	ErrMalformedHash = errors.New("malformed argon2id password hash")
	// ErrIncompatibleVersion indicates an unsupported Argon2 version in the PHC string.
	ErrIncompatibleVersion = errors.New("incompatible argon2 version")
)

// HashPassword hashes a plaintext password with Argon2id and encodes it in standard PHC format:
// $argon2id$v=19$m=65536,t=3,p=2$<salt_b64>$<hash_b64>.
func HashPassword(password string) (string, error) {
	if password == "" {
		return "", ErrEmptyPassword
	}

	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, argonIterations, argonMemory, argonParallelism, argonKeyLen)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	encoded := fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argonMemory,
		argonIterations,
		argonParallelism,
		b64Salt,
		b64Hash,
	)
	return encoded, nil
}

// VerifyPassword compares a plaintext password against a PHC-encoded Argon2id hash in constant time.
func VerifyPassword(password, encodedHash string) (bool, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return false, ErrMalformedHash
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, fmt.Errorf("%w: %v", ErrMalformedHash, err)
	}
	if version != argon2.Version {
		return false, ErrIncompatibleVersion
	}

	var (
		memory      uint32
		iterations  uint32
		parallelism uint8
	)
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false, fmt.Errorf("%w: %v", ErrMalformedHash, err)
	}
	if memory == 0 || iterations == 0 || parallelism == 0 {
		return false, ErrMalformedHash
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return false, ErrMalformedHash
	}

	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(expectedHash) == 0 {
		return false, ErrMalformedHash
	}

	computedHash := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(expectedHash)))
	match := subtle.ConstantTimeCompare(computedHash, expectedHash) == 1
	return match, nil
}
