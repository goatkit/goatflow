package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/auth"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/shared"
)

type tokenPairBody struct {
	Success bool `json:"success"`
	User    struct {
		ID    uint   `json:"id"`
		Login string `json:"login"`
		Role  string `json:"role"`
	} `json:"user"`
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
}

func decodeTokenPair(t *testing.T, body []byte) tokenPairBody {
	t.Helper()
	var out tokenPairBody
	require.NoError(t, json.Unmarshal(body, &out), string(body))
	require.True(t, out.Success, string(body))
	require.NotEmpty(t, out.AccessToken)
	require.NotEmpty(t, out.RefreshToken)
	return out
}

func TestAuthRefreshAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := database.GetDB()
	require.NoError(t, err)
	router := NewSimpleRouter()
	jwtManager := shared.GetJWTManager()

	sfx := fmt.Sprintf("%d", time.Now().UnixNano())
	now := time.Now().UTC().Truncate(time.Second)
	password := "Refresh-Test-" + sfx
	hash, err := auth.NewPasswordHasher().HashPassword(password)
	require.NoError(t, err)

	agentLogin, customerLogin := "refresh_agent_"+sfx, "refresh_cust_"+sfx
	agentID, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO users (login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, 'Refresh', 'Agent', 1, ?, 1, ?, 1) RETURNING id`), agentLogin, hash, now, now)
	require.NoError(t, err)
	customerID, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO customer_user (login, email, customer_id, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, 'refresh-co', ?, 'Refresh', 'Customer', 1, ?, 1, ?, 1) RETURNING id`),
		customerLogin, customerLogin+"@example.test", hash, now, now)
	require.NoError(t, err)
	var adminGroupID int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders("SELECT id FROM `groups` WHERE name = 'admin'")).Scan(&adminGroupID))
	t.Cleanup(func() {
		for _, q := range []struct {
			sql string
			arg interface{}
		}{
			{"DELETE FROM group_user WHERE user_id = ?", agentID},
			{"DELETE FROM users WHERE id = ?", agentID},
			{"DELETE FROM customer_user WHERE id = ?", customerID},
		} {
			if _, err := db.Exec(database.ConvertPlaceholders(q.sql), q.arg); err != nil {
				t.Errorf("cleanup %q: %v", q.sql, err)
			}
		}
	})

	post := func(path string, body interface{}) (int, []byte) {
		w := searchRequest(t, router, http.MethodPost, path, "", body)
		return w.Code, w.Body.Bytes()
	}
	login := func(user string) tokenPairBody {
		t.Helper()
		code, body := post("/api/v1/auth/login", gin.H{"login": user, "password": password})
		require.Equal(t, http.StatusOK, code, string(body))
		return decodeTokenPair(t, body)
	}
	refresh := func(token string) (int, []byte) {
		return post("/api/v1/auth/refresh", gin.H{"refresh_token": token})
	}

	t.Run("agent refresh reissues tokens from the current database state", func(t *testing.T) {
		pair := login(agentLogin)
		assert.Equal(t, int(jwtManager.RefreshTokenDuration().Seconds()), pair.RefreshExpiresIn)
		assert.Greater(t, jwtManager.RefreshTokenDuration(), jwtManager.TokenDuration())

		// Became admin after logging in: the refreshed token must say so.
		_, err := db.Exec(database.ConvertPlaceholders(`INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
			VALUES (?, ?, 'rw', ?, 1, ?, 1)`), agentID, adminGroupID, now, now)
		require.NoError(t, err)

		code, body := refresh(pair.RefreshToken)
		require.Equal(t, http.StatusOK, code, string(body))
		next := decodeTokenPair(t, body)
		assert.Equal(t, uint(agentID), next.User.ID)
		assert.Equal(t, agentLogin, next.User.Login)
		assert.Equal(t, "Admin", next.User.Role)
		assert.Equal(t, "Bearer", next.TokenType)
		assert.Equal(t, int(jwtManager.TokenDuration().Seconds()), next.ExpiresIn)
		assert.NotEqual(t, pair.RefreshToken, next.RefreshToken, "refresh token is rotated")

		claims, err := jwtManager.ValidateToken(next.AccessToken)
		require.NoError(t, err)
		assert.Equal(t, uint(agentID), claims.UserID)
		assert.Equal(t, "Admin", claims.Role)
		assert.True(t, claims.IsAdmin)

		// The new access token works on a protected endpoint; the rotated refresh token refreshes again.
		w := searchRequest(t, router, http.MethodGet, "/api/v1/search/saved", next.AccessToken, nil)
		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
		code, body = refresh(next.RefreshToken)
		assert.Equal(t, http.StatusOK, code, string(body))
	})

	t.Run("token types cannot be swapped", func(t *testing.T) {
		pair := login(agentLogin)
		code, _ := refresh(pair.AccessToken)
		assert.Equal(t, http.StatusUnauthorized, code, "access token used as refresh token")

		w := searchRequest(t, router, http.MethodGet, "/api/v1/search/saved", pair.RefreshToken, nil)
		assert.Equal(t, http.StatusUnauthorized, w.Code, "refresh token used as access token: %s", w.Body.String())
	})

	t.Run("customer refresh stays a customer", func(t *testing.T) {
		pair := login(customerLogin)
		code, body := refresh(pair.RefreshToken)
		require.Equal(t, http.StatusOK, code, string(body))
		next := decodeTokenPair(t, body)
		assert.Equal(t, uint(customerID), next.User.ID)
		assert.Equal(t, "Customer", next.User.Role)
		claims, err := jwtManager.ValidateToken(next.AccessToken)
		require.NoError(t, err)
		assert.Equal(t, "Customer", claims.Role)
		assert.False(t, claims.IsAdmin)

		// An agent refresh token naming the customer's id, login and live session does not reach the customer.
		forged, err := jwtManager.GenerateRefreshToken(auth.AccountKindAgent, uint(customerID), customerLogin, claims.SessionID)
		require.NoError(t, err)
		code, _ = refresh(forged)
		assert.Equal(t, http.StatusUnauthorized, code)
	})

	t.Run("disabled accounts are rejected", func(t *testing.T) {
		agentPair, customerPair := login(agentLogin), login(customerLogin)
		_, err := db.Exec(database.ConvertPlaceholders("UPDATE users SET valid_id = 2 WHERE id = ?"), agentID)
		require.NoError(t, err)
		_, err = db.Exec(database.ConvertPlaceholders("UPDATE customer_user SET valid_id = 2 WHERE id = ?"), customerID)
		require.NoError(t, err)

		code, _ := refresh(agentPair.RefreshToken)
		assert.Equal(t, http.StatusUnauthorized, code)
		code, _ = refresh(customerPair.RefreshToken)
		assert.Equal(t, http.StatusUnauthorized, code)
	})

	t.Run("malformed requests", func(t *testing.T) {
		code, _ := refresh("not-a-jwt")
		assert.Equal(t, http.StatusUnauthorized, code)
		code, _ = post("/api/v1/auth/refresh", gin.H{})
		assert.Equal(t, http.StatusBadRequest, code)
	})
}
