package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

// PasswordHashType is the algorithm used to create new password hashes.
type PasswordHashType string

const (
	// HashTypeBcrypt is the default: salted, adaptive bcrypt.
	HashTypeBcrypt PasswordHashType = "bcrypt"
	// HashTypeSHA256 is OTRS's unsalted sha2 format (64 hex chars). Only for
	// running side by side with an OTRS that must read the same user tables.
	HashTypeSHA256 PasswordHashType = "sha256"
)

// Environment variables that configure password hashing.
const (
	// EnvPasswordHashType selects the algorithm for new hashes (bcrypt|sha256).
	EnvPasswordHashType = "PASSWORD_HASH_TYPE"
	// EnvMigratePasswordHashes, when true, rehashes a password with the
	// configured algorithm on successful login if the stored hash uses another.
	EnvMigratePasswordHashes = "MIGRATE_PASSWORD_HASHES"
)

// ErrPasswordTooLong is returned by HashPassword when bcrypt is configured and
// the password exceeds bcrypt's 72-byte input limit.
var ErrPasswordTooLong = errors.New("password must be at most 72 bytes")

var unknownHashTypeWarned sync.Map

// PasswordHasher is the single place that creates and verifies password hashes
// for agents (users.pw) and customers (customer_user.pw). Its configuration is
// read once at construction and never mutated, so it is safe for concurrent use.
type PasswordHasher struct {
	hashType PasswordHashType
	migrate  bool
}

// NewPasswordHasher returns a hasher configured from PASSWORD_HASH_TYPE
// (default bcrypt) and MIGRATE_PASSWORD_HASHES (default false).
func NewPasswordHasher() *PasswordHasher {
	migrate, _ := strconv.ParseBool(strings.TrimSpace(os.Getenv(EnvMigratePasswordHashes))) //nolint:errcheck // unset/invalid means disabled
	return &PasswordHasher{
		hashType: ConfiguredPasswordHashType(),
		migrate:  migrate,
	}
}

// ConfiguredPasswordHashType returns the algorithm selected by
// PASSWORD_HASH_TYPE. Unset means bcrypt; an unknown value logs a warning and
// falls back to bcrypt rather than to a weaker algorithm.
func ConfiguredPasswordHashType() PasswordHashType {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv(EnvPasswordHashType)))
	switch PasswordHashType(raw) {
	case "", HashTypeBcrypt:
		return HashTypeBcrypt
	case HashTypeSHA256:
		return HashTypeSHA256
	default:
		if _, seen := unknownHashTypeWarned.LoadOrStore(raw, true); !seen {
			log.Printf("auth: unknown %s=%q, using bcrypt (supported: bcrypt, sha256)", EnvPasswordHashType, raw)
		}
		return HashTypeBcrypt
	}
}

// HashType returns the algorithm this hasher uses for new hashes.
func (h *PasswordHasher) HashType() PasswordHashType {
	return h.hashType
}

// HashPassword hashes a password using the configured algorithm.
func (h *PasswordHasher) HashPassword(password string) (string, error) {
	if h.hashType == HashTypeSHA256 {
		return hashSHA256(password), nil
	}
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if errors.Is(err, bcrypt.ErrPasswordTooLong) {
		return "", ErrPasswordTooLong
	}
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// VerifyPassword checks a password against a stored hash in any supported
// format, regardless of the configured algorithm: bcrypt ($2a$/$2b$/$2y$),
// salted "sha256$salt$hash", and the OTRS/Znuny formats (see verifyOTRSHash).
func (h *PasswordHasher) VerifyPassword(password, storedHash string) bool {
	if isBcryptHash(storedHash) {
		return bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password)) == nil
	}
	if parts := strings.Split(storedHash, "$"); len(parts) == 3 && parts[0] == "sha256" {
		sum := sha256.Sum256([]byte(password + parts[1]))
		return constantTimeEqual(hex.EncodeToString(sum[:]), parts[2])
	}
	return verifyOTRSHash(password, storedHash)
}

// MigratePasswordHash returns a new hash of password in the configured format
// when MIGRATE_PASSWORD_HASHES is enabled and storedHash is in another format.
// The caller must already have verified password against storedHash. When no
// migration is due it returns ("", false, nil).
func (h *PasswordHasher) MigratePasswordHash(password, storedHash string) (string, bool, error) {
	if !h.migrate || hashTypeOf(storedHash) == h.hashType {
		return "", false, nil
	}
	newHash, err := h.HashPassword(password)
	if err != nil {
		return "", false, err
	}
	return newHash, true, nil
}

// hashTypeOf reports which configurable algorithm produced storedHash, or ""
// for formats GoatFlow no longer creates (e.g. salted "sha256$salt$hash").
func hashTypeOf(storedHash string) PasswordHashType {
	switch {
	case isBcryptHash(storedHash):
		return HashTypeBcrypt
	case len(storedHash) == 64 && isLowerHex(storedHash):
		return HashTypeSHA256
	default:
		return ""
	}
}

func isBcryptHash(s string) bool {
	return strings.HasPrefix(s, "$2a$") || strings.HasPrefix(s, "$2b$") || strings.HasPrefix(s, "$2y$")
}

func hashSHA256(password string) string {
	sum := sha256.Sum256([]byte(password))
	return hex.EncodeToString(sum[:])
}

func isLowerHex(s string) bool {
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func constantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
