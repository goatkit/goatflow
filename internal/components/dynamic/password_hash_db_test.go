package dynamic

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/goatkit/goatflow/internal/platform/auth"
	"github.com/goatkit/goatflow/internal/platform/database"
)

// The users module's create and update store pw with the hasher selected by
// PASSWORD_HASH_TYPE (default bcrypt), and refuse what bcrypt cannot hash.
func TestDynamicUsersModulePasswordHashFollowsConfig(t *testing.T) {
	r := newModulesTestRouter(t)
	db, err := database.GetDB()
	require.NoError(t, err)

	storedPw := func(login string) string {
		var pw string
		require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`SELECT pw FROM users WHERE login = ?`), login).Scan(&pw))
		return pw
	}
	requireType := func(want auth.PasswordHashType, password, stored string) {
		t.Helper()
		if want == auth.HashTypeSHA256 {
			sum := sha256.Sum256([]byte(password))
			require.Equal(t, hex.EncodeToString(sum[:]), stored)
			return
		}
		require.NoError(t, bcrypt.CompareHashAndPassword([]byte(stored), []byte(password)), "want bcrypt, got %q", stored)
	}

	for _, tc := range []struct {
		env  string
		want auth.PasswordHashType
	}{
		{"", auth.HashTypeBcrypt},
		{"sha256", auth.HashTypeSHA256},
	} {
		t.Run("PASSWORD_HASH_TYPE="+tc.env, func(t *testing.T) {
			t.Setenv(auth.EnvPasswordHashType, tc.env)
			login := fmt.Sprintf("dynpw%d", time.Now().UnixNano())
			t.Cleanup(func() {
				_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM users WHERE login = ?`), login) //nolint:errcheck // cleanup
			})

			code, body := serveDynamic(t, r, http.MethodPost, "/dynamic/users", url.Values{
				"login": {login}, "pw": {"Create-Pass-1"}, "first_name": {"Dyn"}, "last_name": {"Hash"},
			})
			require.Equal(t, http.StatusCreated, code, body)
			requireType(tc.want, "Create-Pass-1", storedPw(login))

			userID := idByColumn(t, "users", "login", login)
			code, body = serveDynamic(t, r, http.MethodPut, fmt.Sprintf("/dynamic/users/%d", userID), url.Values{
				"login": {login}, "pw": {"Update-Pass-2"}, "first_name": {"Dyn"}, "last_name": {"Hash"},
			})
			require.Equal(t, http.StatusOK, code, body)
			requireType(tc.want, "Update-Pass-2", storedPw(login))
		})
	}

	t.Run("bcrypt rejects passwords over 72 bytes", func(t *testing.T) {
		t.Setenv(auth.EnvPasswordHashType, "")
		login := fmt.Sprintf("dynlong%d", time.Now().UnixNano())
		code, body := serveDynamic(t, r, http.MethodPost, "/dynamic/users", url.Values{
			"login": {login}, "pw": {strings.Repeat("p", 73)}, "first_name": {"Dyn"}, "last_name": {"Long"},
		})
		assert.Equal(t, http.StatusBadRequest, code, body)
		var n int
		require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`SELECT COUNT(*) FROM users WHERE login = ?`), login).Scan(&n))
		assert.Zero(t, n)
	})
}
