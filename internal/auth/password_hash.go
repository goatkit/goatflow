package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// PasswordHashType represents the hashing algorithm to use.
type PasswordHashType string

const (
	HashTypeBcrypt PasswordHashType = "bcrypt"
	HashTypeSHA256 PasswordHashType = "sha256"
	HashTypeSHA512 PasswordHashType = "sha512"
	HashTypeMD5    PasswordHashType = "md5"
	HashTypeAuto   PasswordHashType = "auto" // Auto-detect from hash format
)

// PasswordHasher handles password hashing and verification.
type PasswordHasher struct {
	defaultType PasswordHashType
}

// NewPasswordHasher creates a new password hasher.
func NewPasswordHasher() *PasswordHasher {
	// Get hash type from environment, default to SHA256 for OTRS compatibility
	hashType := os.Getenv("PASSWORD_HASH_TYPE")
	if hashType == "" {
		hashType = "sha256" // Default to OTRS-compatible SHA256
	}

	return &PasswordHasher{
		defaultType: PasswordHashType(strings.ToLower(hashType)),
	}
}

// HashPassword hashes a password using the configured algorithm.
func (h *PasswordHasher) HashPassword(password string) (string, error) {
	switch h.defaultType {
	case HashTypeBcrypt:
		return h.hashBcrypt(password)
	case HashTypeSHA256, "": // SHA256 is default for OTRS compatibility
		return h.hashSHA256(password), nil
	default:
		// Fallback to SHA256 for unknown types
		return h.hashSHA256(password), nil
	}
}

// VerifyPassword checks if a password matches the hash.
func (h *PasswordHasher) VerifyPassword(password, hash string) bool {
	// Use the consolidated password verification logic
	return h.verifyPassword(password, hash)
}

// detectHashType determines the hash algorithm from the hash format
// detectHashType determines the hash algorithm from the hash format.
//
//nolint:unused
func (h *PasswordHasher) detectHashType(hash string) PasswordHashType {
	// Bcrypt hashes start with $2a$, $2b$, or $2y$
	if strings.HasPrefix(hash, "?") {
		return HashTypeBcrypt
	}

	// SHA256 produces 64 character hex strings
	if len(hash) == 64 && isHex(hash) {
		return HashTypeSHA256
	}

	// SHA512 produces 128 character hex strings
	if len(hash) == 128 && isHex(hash) {
		return HashTypeSHA512
	}

	// MD5 produces 32 character hex strings
	if len(hash) == 32 && isHex(hash) {
		return HashTypeMD5
	}

	// Default to SHA256 for OTRS compatibility
	return HashTypeSHA256
}

// hashBcrypt creates a bcrypt hash.
func (h *PasswordHasher) hashBcrypt(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// verifyBcrypt checks a bcrypt hash
// verifyBcrypt checks a bcrypt hash.
//
//nolint:unused
func (h *PasswordHasher) verifyBcrypt(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// hashSHA256 creates a SHA256 hash (OTRS compatible).
func (h *PasswordHasher) hashSHA256(password string) string {
	hasher := sha256.New()
	hasher.Write([]byte(password))
	return hex.EncodeToString(hasher.Sum(nil))
}

// verifySHA256 checks a SHA256 hash (supports both salted and unsalted formats)
// verifySHA256 checks a SHA256 hash (supports both salted and unsalted formats).
//
//nolint:unused
func (h *PasswordHasher) verifySHA256(password, hash string) bool {
	return h.verifyPassword(password, hash)
}

// This is the consolidated password verification logic.
func (h *PasswordHasher) verifyPassword(password, hashedPassword string) bool {
	// Check if it's a bcrypt hash (starts with $2a$, $2b$, or $2y$)
	if strings.HasPrefix(hashedPassword, "$2a$") || strings.HasPrefix(hashedPassword, "$2b$") || strings.HasPrefix(hashedPassword, "$2y$") {
		// Use bcrypt to compare
		err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
		return err == nil
	}

	// Check if it's a salted SHA256 hash (format: sha256$salt$hash)
	parts := strings.Split(hashedPassword, "$")
	if len(parts) == 3 && parts[0] == "sha256" {
		// Extract salt and hash
		salt := parts[1]
		expectedHash := parts[2]

		// Hash the password with the salt
		combined := password + salt
		hash := sha256.Sum256([]byte(combined))
		actualHash := hex.EncodeToString(hash[:])

		return actualHash == expectedHash
	}

	// Otherwise, treat as unsalted SHA256 hash (legacy)
	return h.hashSHA256(password) == hashedPassword
}

// isHex checks if a string contains only hexadecimal characters
// isHex checks if a string contains only hexadecimal characters.
//
//nolint:unused
func isHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// MigratePasswordHash optionally upgrades password hash on successful login.
func (h *PasswordHasher) MigratePasswordHash(password, oldHash string, targetType PasswordHashType) (string, error) {
	// Only migrate if configured to do so
	if os.Getenv("MIGRATE_PASSWORD_HASHES") != "true" {
		return oldHash, nil
	}

	// Verify the password is correct first
	if !h.VerifyPassword(password, oldHash) {
		return "", fmt.Errorf("password verification failed")
	}

	// Generate new hash with target algorithm
	h.defaultType = targetType
	newHash, err := h.HashPassword(password)
	h.defaultType = PasswordHashType(os.Getenv("PASSWORD_HASH_TYPE")) // Reset to default

	return newHash, err
}
