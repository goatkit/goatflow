package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/goatkit/goatflow/internal/platform/auth"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/shared"
	"github.com/goatkit/goatflow/internal/service"
)

const pwHashTestCompany = "PWHASHCO"

func otrsSHA2Hash(password string) string {
	sum := sha256.Sum256([]byte(password))
	return hex.EncodeToString(sum[:])
}

func requireHashType(t *testing.T, want auth.PasswordHashType, password, stored string) {
	t.Helper()
	switch want {
	case auth.HashTypeBcrypt:
		require.NoError(t, bcrypt.CompareHashAndPassword([]byte(stored), []byte(password)), "want bcrypt, got %q", stored)
	case auth.HashTypeSHA256:
		require.Equal(t, otrsSHA2Hash(password), stored, "want OTRS sha2")
	default:
		t.Fatalf("unknown hash type %q", want)
	}
}

func pwOf(t *testing.T, db *sql.DB, table, login string) string {
	t.Helper()
	var pw string
	query := "SELECT pw FROM users WHERE login = ?"
	if table == "customer_user" {
		query = "SELECT pw FROM customer_user WHERE login = ?"
	}
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(query), login).Scan(&pw))
	return pw
}

func idOf(t *testing.T, db *sql.DB, table, login string) int {
	t.Helper()
	var id int
	query := "SELECT id FROM users WHERE login = ?"
	if table == "customer_user" {
		query = "SELECT id FROM customer_user WHERE login = ?"
	}
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(query), login).Scan(&id))
	return id
}

// seedAccount inserts an agent (users) or customer (customer_user) whose pw is
// an OTRS sha2 hash, the format an OTRS import leaves behind.
func seedAccount(t *testing.T, db *sql.DB, table, login, password string) {
	t.Helper()
	if table == "users" {
		_, err := db.Exec(database.ConvertPlaceholders(`
			INSERT INTO users (login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
			VALUES (?, ?, 'Hash', 'Agent', 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)`), login, otrsSHA2Hash(password))
		require.NoError(t, err)
	} else {
		createTestCustomerCompany(t, db, pwHashTestCompany)
		_, err := db.Exec(database.ConvertPlaceholders(`
			INSERT INTO customer_user (login, email, customer_id, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
			VALUES (?, ?, ?, ?, 'Hash', 'Customer', 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)`),
			login, login+"@example.test", pwHashTestCompany, otrsSHA2Hash(password))
		require.NoError(t, err)
	}
	cleanupAccount(t, db, table, login)
}

func cleanupAccount(t *testing.T, db *sql.DB, table, login string) {
	t.Cleanup(func() {
		if table == "customer_user" {
			_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM customer_user WHERE login = ?`), login) //nolint:errcheck // cleanup
			return
		}
		for _, q := range []string{
			`DELETE FROM group_user WHERE user_id IN (SELECT id FROM users WHERE login = ?)`,
			`DELETE FROM user_preferences WHERE user_id IN (SELECT id FROM users WHERE login = ?)`,
			`DELETE FROM users WHERE login = ?`,
		} {
			_, _ = db.Exec(database.ConvertPlaceholders(q), login) //nolint:errcheck // cleanup
		}
	})
}

func sendJSON(t *testing.T, r *gin.Engine, method, path string, body any) (int, string) {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(method, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code, w.Body.String()
}

// Every HTTP path that stores a new password hashes it with the hasher
// selected by PASSWORD_HASH_TYPE (default bcrypt), for agents and customers.
func TestPasswordCreationPathsUseConfiguredHash(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := getTestDB(t)
	const oldPw, newPw = "0ld-Pass-Word-A", "N3w-Pass-Word-B"

	type creationPath struct {
		name  string
		table string
		// run stores newPw for login via the handler and returns the HTTP status/body.
		run func(t *testing.T, login string) (int, string)
	}
	paths := []creationPath{
		{"admin agent create", "users", func(t *testing.T, login string) (int, string) {
			cleanupAccount(t, db, "users", login)
			r := seedAdminRouter()
			r.POST("/admin/users", HandleAdminUserCreate)
			return sendJSON(t, r, http.MethodPost, "/admin/users", gin.H{
				"login": login, "first_name": "Hash", "last_name": "Agent", "password": newPw, "valid_id": 1,
			})
		}},
		{"admin agent update", "users", func(t *testing.T, login string) (int, string) {
			seedAccount(t, db, "users", login, oldPw)
			r := seedAdminRouter()
			r.PUT("/admin/users/:id", HandleAdminUserUpdate)
			return sendJSON(t, r, http.MethodPut, fmt.Sprintf("/admin/users/%d", idOf(t, db, "users", login)), gin.H{
				"login": login, "first_name": "Hash", "last_name": "Agent", "password": newPw, "valid_id": 1,
			})
		}},
		{"admin agent reset password", "users", func(t *testing.T, login string) (int, string) {
			seedAccount(t, db, "users", login, oldPw)
			r := seedAdminRouter()
			r.POST("/admin/users/:id/reset-password", HandleAdminUserResetPassword)
			return sendJSON(t, r, http.MethodPost, fmt.Sprintf("/admin/users/%d/reset-password", idOf(t, db, "users", login)), gin.H{
				"password": newPw,
			})
		}},
		{"agent self-service change", "users", func(t *testing.T, login string) (int, string) {
			seedAccount(t, db, "users", login, oldPw)
			id := idOf(t, db, "users", login)
			r := seedAdminRouter()
			r.Use(func(c *gin.Context) { c.Set("user_id", uint(id)); c.Next() })
			r.POST("/agent/password/change", HandleAgentChangePassword)
			return sendJSON(t, r, http.MethodPost, "/agent/password/change", gin.H{
				"current_password": oldPw, "new_password": newPw, "confirm_password": newPw,
			})
		}},
		{"admin customer user create", "customer_user", func(t *testing.T, login string) (int, string) {
			createTestCustomerCompany(t, db, pwHashTestCompany)
			cleanupAccount(t, db, "customer_user", login)
			r := seedAdminRouter()
			r.POST("/admin/customer-users", HandleAdminCustomerUsersCreate)
			return sendJSON(t, r, http.MethodPost, "/admin/customer-users", gin.H{
				"login": login, "email": login + "@example.test", "customer_id": pwHashTestCompany,
				"first_name": "Hash", "last_name": "Customer", "password": newPw, "valid_id": 1,
			})
		}},
		{"admin customer user update", "customer_user", func(t *testing.T, login string) (int, string) {
			seedAccount(t, db, "customer_user", login, oldPw)
			r := seedAdminRouter()
			r.PUT("/admin/customer-users/:id", HandleAdminCustomerUsersUpdate)
			return sendJSON(t, r, http.MethodPut, fmt.Sprintf("/admin/customer-users/%d", idOf(t, db, "customer_user", login)), gin.H{
				"login": login, "email": login + "@example.test", "customer_id": pwHashTestCompany,
				"first_name": "Hash", "last_name": "Customer", "password": newPw, "valid_id": 1,
			})
		}},
		{"customer self-service change", "customer_user", func(t *testing.T, login string) (int, string) {
			seedAccount(t, db, "customer_user", login, oldPw)
			r := seedAdminRouter()
			r.Use(func(c *gin.Context) { c.Set("username", login); c.Set("user_role", "Customer"); c.Next() })
			r.POST("/customer/password/change", handleCustomerChangePassword(db))
			return sendJSON(t, r, http.MethodPost, "/customer/password/change", gin.H{
				"current_password": oldPw, "new_password": newPw, "confirm_password": newPw,
			})
		}},
	}

	for _, cfg := range []struct {
		env  string
		want auth.PasswordHashType
	}{
		{"", auth.HashTypeBcrypt},
		{"sha256", auth.HashTypeSHA256},
	} {
		for _, p := range paths {
			t.Run(fmt.Sprintf("%s/PASSWORD_HASH_TYPE=%s", p.name, cfg.env), func(t *testing.T) {
				t.Setenv(auth.EnvPasswordHashType, cfg.env)
				login := fmt.Sprintf("pwh%d", time.Now().UnixNano())
				code, body := p.run(t, login)
				require.Contains(t, []int{http.StatusOK, http.StatusCreated}, code, body)
				requireHashType(t, cfg.want, newPw, pwOf(t, db, p.table, login))
			})
		}
	}
}

// Setup assistant onboarding hashes the generated portal password with the
// configured algorithm.
func TestSetupAssistantOnboardPasswordHashFollowsConfig(t *testing.T) {
	db := getTestDB(t)
	svc := service.NewSetupAssistantService(db, nil)
	for _, cfg := range []struct {
		env  string
		want auth.PasswordHashType
	}{
		{"", auth.HashTypeBcrypt},
		{"sha256", auth.HashTypeSHA256},
	} {
		t.Run("PASSWORD_HASH_TYPE="+cfg.env, func(t *testing.T) {
			t.Setenv(auth.EnvPasswordHashType, cfg.env)
			sfx := fmt.Sprint(time.Now().UnixNano())
			cid, login := "PWHONB"+sfx, "pwhonb"+sfx
			cleanupAccount(t, db, "customer_user", login)
			t.Cleanup(func() {
				_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM customer_company WHERE customer_id = ?`), cid) //nolint:errcheck // cleanup
			})

			res := svc.OnboardCustomer(context.Background(), service.OnboardCustomerRequest{
				CustomerID: cid, Name: "Hash Onboard " + sfx,
				Users: []service.CustomerUserInput{{Login: login, Email: login + "@example.test", FirstName: "On", LastName: "Board"}},
			})
			require.True(t, res.Success, res.Error)
			require.Len(t, res.UsersCreated, 1)
			requireHashType(t, cfg.want, res.UsersCreated[0].Password, pwOf(t, db, "customer_user", login))
		})
	}
}

// Imported OTRS sha2 passwords log in on every live login endpoint. With
// MIGRATE_PASSWORD_HASHES=true the stored hash becomes bcrypt on that login and
// keeps working; with it off the stored hash is left alone.
func TestLoginPasswordHashMigration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := getTestDB(t)
	const password = "Otrs-Imported-1"

	endpoints := []struct {
		name  string
		table string
		path  string
		h     gin.HandlerFunc
	}{
		{"agent web login", "users", "/api/auth/login", HandleAuthLogin},
		{"agent API login", "users", "/api/v1/auth/login", HandleAPIv1AuthLogin},
		{"customer portal login", "customer_user", "/api/auth/customer/login", handleCustomerLogin(shared.GetJWTManager())},
		{"customer API login", "customer_user", "/api/v1/auth/login", HandleAPIv1AuthLogin},
	}
	for _, ep := range endpoints {
		for _, migrate := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/MIGRATE_PASSWORD_HASHES=%v", ep.name, migrate), func(t *testing.T) {
				t.Setenv(auth.EnvPasswordHashType, "")
				t.Setenv(auth.EnvMigratePasswordHashes, fmt.Sprint(migrate))
				login := fmt.Sprintf("pwm%d", time.Now().UnixNano())
				seedAccount(t, db, ep.table, login, password)
				r := gin.New()
				r.POST(ep.path, ep.h)

				code, body := sendJSON(t, r, http.MethodPost, ep.path, gin.H{"login": login, "password": password})
				require.Equal(t, http.StatusOK, code, body)

				stored := pwOf(t, db, ep.table, login)
				if migrate {
					requireHashType(t, auth.HashTypeBcrypt, password, stored)
				} else {
					assert.Equal(t, otrsSHA2Hash(password), stored, "hash must be unchanged when migration is off")
				}

				code, body = sendJSON(t, r, http.MethodPost, ep.path, gin.H{"login": login, "password": password})
				require.Equal(t, http.StatusOK, code, "second login: %s", body)
				code, _ = sendJSON(t, r, http.MethodPost, ep.path, gin.H{"login": login, "password": password + "x"})
				assert.Equal(t, http.StatusUnauthorized, code)
				assert.Equal(t, stored, pwOf(t, db, ep.table, login), "a failed login must not touch the hash")
			})
		}
	}
}
