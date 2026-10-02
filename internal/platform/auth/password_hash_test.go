package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// otrsSHA2 is how OTRS/Znuny stores a sha2 password: unsalted hex SHA-256.
func otrsSHA2(password string) string {
	sum := sha256.Sum256([]byte(password))
	return hex.EncodeToString(sum[:])
}

func TestPasswordHasherHashTypeSelection(t *testing.T) {
	cases := []struct {
		env  string
		want PasswordHashType
	}{
		{"", HashTypeBcrypt},
		{"bcrypt", HashTypeBcrypt},
		{"BCRYPT", HashTypeBcrypt},
		{"sha256", HashTypeSHA256},
		{" SHA256 ", HashTypeSHA256},
		{"argon2", HashTypeBcrypt}, // unknown never falls back to a weaker algorithm
	}
	for _, tc := range cases {
		t.Run("env="+tc.env, func(t *testing.T) {
			t.Setenv(EnvPasswordHashType, tc.env)
			h := NewPasswordHasher()
			assert.Equal(t, tc.want, h.HashType())

			hash, err := h.HashPassword("S3cret!pw")
			require.NoError(t, err)
			if tc.want == HashTypeBcrypt {
				require.NoError(t, bcrypt.CompareHashAndPassword([]byte(hash), []byte("S3cret!pw")), "want bcrypt, got %q", hash)
			} else {
				assert.Equal(t, otrsSHA2("S3cret!pw"), hash, "sha256 must be OTRS-readable unsalted hex")
			}
			assert.True(t, h.VerifyPassword("S3cret!pw", hash))
			assert.False(t, h.VerifyPassword("wrong", hash))
		})
	}
}

func TestPasswordHasherVerifiesAllStoredFormats(t *testing.T) {
	const pw = "Tr0ub4dor&3"
	bc, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	require.NoError(t, err)
	saltedSum := sha256.Sum256([]byte(pw + "abc123"))
	stored := map[string]string{
		"bcrypt":        string(bc),
		"otrs sha2":     otrsSHA2(pw),
		"salted sha256": "sha256$abc123$" + hex.EncodeToString(saltedSum[:]),
	}
	for _, cfg := range []string{"bcrypt", "sha256"} {
		t.Setenv(EnvPasswordHashType, cfg)
		h := NewPasswordHasher()
		for name, hash := range stored {
			assert.True(t, h.VerifyPassword(pw, hash), "config %s must verify %s", cfg, name)
			assert.False(t, h.VerifyPassword(pw+"x", hash), "config %s accepted wrong password for %s", cfg, name)
		}
		assert.False(t, h.VerifyPassword("", ""), "empty stored hash must never verify")
		assert.False(t, h.VerifyPassword(pw, pw), "plaintext stored value must not verify")
	}
}

func TestPasswordHasherBcryptRejectsOverlongPassword(t *testing.T) {
	t.Setenv(EnvPasswordHashType, "")
	_, err := NewPasswordHasher().HashPassword(strings.Repeat("a", 73))
	assert.ErrorIs(t, err, ErrPasswordTooLong)
}

func TestMigratePasswordHash(t *testing.T) {
	const pw = "Upgrade-me-1"
	bc, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	require.NoError(t, err)

	cases := []struct {
		name       string
		hashType   string
		migrate    string
		stored     string
		wantChange bool
		wantType   PasswordHashType
	}{
		{"disabled leaves sha2", "bcrypt", "false", otrsSHA2(pw), false, ""},
		{"unset leaves sha2", "", "", otrsSHA2(pw), false, ""},
		{"sha2 upgraded to bcrypt", "", "true", otrsSHA2(pw), true, HashTypeBcrypt},
		{"salted sha256 upgraded to bcrypt", "bcrypt", "true", "sha256$s$" + otrsSHA2(pw+"s"), true, HashTypeBcrypt},
		{"bcrypt already current", "bcrypt", "true", string(bc), false, ""},
		{"bcrypt moved to sha256 for OTRS side by side", "sha256", "true", string(bc), true, HashTypeSHA256},
		{"sha2 already current under sha256", "sha256", "true", otrsSHA2(pw), false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(EnvPasswordHashType, tc.hashType)
			t.Setenv(EnvMigratePasswordHashes, tc.migrate)
			h := NewPasswordHasher()
			require.True(t, h.VerifyPassword(pw, tc.stored))

			newHash, changed, err := h.MigratePasswordHash(pw, tc.stored)
			require.NoError(t, err)
			assert.Equal(t, tc.wantChange, changed)
			if !tc.wantChange {
				assert.Empty(t, newHash)
				return
			}
			assert.Equal(t, tc.wantType, hashTypeOf(newHash))
			assert.True(t, h.VerifyPassword(pw, newHash))
		})
	}
}
