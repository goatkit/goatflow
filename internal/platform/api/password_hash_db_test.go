package api

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/goatkit/goatflow/internal/platform/auth"
	"github.com/goatkit/goatflow/internal/platform/database"
)

func otrsSHA2(password string) string {
	sum := sha256.Sum256([]byte(password))
	return hex.EncodeToString(sum[:])
}

// assertHashType checks a stored pw column holds password in the given format.
func assertHashType(t *testing.T, want auth.PasswordHashType, password, stored string) {
	t.Helper()
	switch want {
	case auth.HashTypeBcrypt:
		assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(stored), []byte(password)), "want bcrypt, got %q", stored)
	case auth.HashTypeSHA256:
		assert.Equal(t, otrsSHA2(password), stored, "want OTRS sha2")
	default:
		t.Fatalf("unknown hash type %q", want)
	}
}

func agentPw(t *testing.T, login string) string {
	t.Helper()
	db, err := database.GetDB()
	require.NoError(t, err)
	var pw string
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders("SELECT pw FROM users WHERE login = ?"), login).Scan(&pw))
	return pw
}

// Both user API write paths hash with PASSWORD_HASH_TYPE; unset means bcrypt.
func TestUserAPIPasswordHashFollowsConfig(t *testing.T) {
	userGroupsTestDB(t)
	for _, tc := range []struct {
		env  string
		want auth.PasswordHashType
	}{
		{"", auth.HashTypeBcrypt},
		{"sha256", auth.HashTypeSHA256},
	} {
		t.Run("PASSWORD_HASH_TYPE="+tc.env, func(t *testing.T) {
			t.Setenv(auth.EnvPasswordHashType, tc.env)
			r := userAPIRouter()
			r.PUT("/api/v1/users/:id", HandleUpdateUserAPI)
			login := fmt.Sprintf("pwapi%d", time.Now().UnixNano())
			t.Cleanup(func() { deleteTestUser(t, login) })

			code, body := doJSON(t, r, http.MethodPost, "/api/v1/users", map[string]any{
				"login": login, "email": login + "@example.test", "password": "Create-Pass-1",
			})
			require.Equal(t, http.StatusCreated, code, body)
			assertHashType(t, tc.want, "Create-Pass-1", agentPw(t, login))

			id := int(body["data"].(map[string]any)["id"].(float64))
			code, body = doJSON(t, r, http.MethodPut, fmt.Sprintf("/api/v1/users/%d", id), map[string]any{
				"password": "Update-Pass-2",
			})
			require.Equal(t, http.StatusOK, code, body)
			assertHashType(t, tc.want, "Update-Pass-2", agentPw(t, login))
		})
	}
}

func TestUserAPIRejectsPasswordBcryptCannotHash(t *testing.T) {
	userGroupsTestDB(t)
	t.Setenv(auth.EnvPasswordHashType, "")
	login := fmt.Sprintf("pwlong%d", time.Now().UnixNano())
	t.Cleanup(func() { deleteTestUser(t, login) })

	code, body := doJSON(t, userAPIRouter(), http.MethodPost, "/api/v1/users", map[string]any{
		"login": login, "email": login + "@example.test", "password": strings.Repeat("x", 73),
	})
	assert.Equal(t, http.StatusBadRequest, code, body)
	assert.Contains(t, body["error"], "72 bytes")
}
