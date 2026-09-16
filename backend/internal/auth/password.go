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

// Argon2id parameters. Encoded in the PHC string alongside each hash so
// they can be tuned later without breaking verification of existing hashes.
const (
	argonMemoryKiB   = 64 * 1024
	argonIterations  = 3
	argonParallelism = 4
	argonSaltLen     = 16
	argonKeyLen      = 32
)

var ErrInvalidCredentials = errors.New("auth: invalid credentials")

// dummyHash is a real Argon2id hash of a fixed placeholder, computed once
// at startup. Login verifies against this when the email doesn't match any
// account, so an unknown-email request costs the same Argon2id computation
// as a known-email-wrong-password request — without this, the two cases
// are distinguishable by response latency, which defeats the point of
// returning the same generic error message for both.
var dummyHash = mustHashPassword("this-is-not-a-real-password-used-only-for-timing-safety")

func mustHashPassword(password string) string {
	hash, err := HashPassword(password)
	if err != nil {
		panic(fmt.Sprintf("auth: failed to compute dummy hash at startup: %v", err))
	}
	return hash
}

// DummyHash returns a valid Argon2id hash that never matches any real
// password, for use as the comparison target when no account exists —
// see the comment on dummyHash for why this matters.
func DummyHash() string {
	return dummyHash
}

// HashPassword returns a PHC-formatted Argon2id hash:
// $argon2id$v=19$m=<mem>,t=<iter>,p=<par>$<salt>$<hash>
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, argonIterations, argonMemoryKiB, argonParallelism, argonKeyLen)

	encoded := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemoryKiB, argonIterations, argonParallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	)
	return encoded, nil
}

// VerifyPassword returns ErrInvalidCredentials if the password does not
// match. Any other error indicates a malformed stored hash.
func VerifyPassword(encodedHash, password string) error {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return fmt.Errorf("unrecognized password hash format")
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return fmt.Errorf("parse version: %w", err)
	}

	var memory uint32
	var iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return fmt.Errorf("parse params: %w", err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return fmt.Errorf("decode salt: %w", err)
	}
	storedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return fmt.Errorf("decode hash: %w", err)
	}

	computed := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(storedHash)))

	if subtle.ConstantTimeCompare(storedHash, computed) != 1 {
		return ErrInvalidCredentials
	}
	return nil
}
