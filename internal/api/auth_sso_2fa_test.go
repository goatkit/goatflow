package api

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/service"
)

// OIDC and SAML callbacks call needs2FA before issuing a session. An agent who
// enabled TOTP must be sent to the second factor, exactly like password login.
func TestSSOLoginRequiresSecondFactorWhenTOTPEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := getTestDB(t)

	login := fmt.Sprintf("sso2fa%d", time.Now().UnixNano())
	id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO users (login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, 'x', 'SSO', 'Agent', 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)
		RETURNING id`), login)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM user_preferences WHERE user_id = ?`), id)
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM users WHERE id = ?`), id)
	})

	required, err := needs2FA(uint(id))
	require.NoError(t, err)
	require.False(t, required, "agent without MFA must log straight in")

	require.NoError(t, service.NewUserPreferencesBackend(db, int(id)).Set("UserTOTPEnabled", "1"))
	required, err = needs2FA(uint(id))
	require.NoError(t, err)
	require.True(t, required, "agent with TOTP enabled must be sent to the second factor")
}

// A second-factor lookup that fails must be an error, never "no MFA": the
// login gates refuse the login instead of skipping the second factor.
func TestMFAStatusFailsClosedOnLookupError(t *testing.T) {
	db := getTestDB(t)
	driver := "mysql"
	if database.IsPostgreSQL() {
		driver = "postgres"
	}
	broken, err := sql.Open(driver, "postgres://u:p@127.0.0.1:1/none?sslmode=disable")
	if driver == "mysql" {
		broken, err = sql.Open(driver, "u:p@tcp(127.0.0.1:1)/none")
	}
	require.NoError(t, err)
	require.NoError(t, broken.Close())

	_, err = agentMFAStatus(broken, 1)
	require.Error(t, err)
	_, err = customerMFAStatus(broken, "someone")
	require.Error(t, err)
	_, err = agentMFAStatus(nil, 1)
	require.Error(t, err)

	status, err := agentMFAStatus(db, 1)
	require.NoError(t, err, "a healthy database yields a status")
	_ = status
}
