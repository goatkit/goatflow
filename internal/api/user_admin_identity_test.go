package api

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/platform/database"
)

func identityTestContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/dashboard", nil)
	return c
}

// An agent is an administrator only through admin-group membership. A login
// that merely contains "admin" (or is the string "root@localhost") must not
// make the template user an admin.
func TestTemplateUserAdminComesOnlyFromAdminGroup(t *testing.T) {
	db := isolatedDB(t)
	agentID, login := createIsolatedAgent(t, "helpdesk_admin")

	for _, l := range []string{login, "root@localhost"} {
		c := identityTestContext()
		c.Set("user", &models.User{ID: uint(agentID), Login: l, ValidID: 1})
		m := getUserMapForTemplate(c)
		assert.Equal(t, false, m["IsAdmin"], "login %q", l)
		assert.Equal(t, false, m["IsInAdminGroup"], "login %q", l)
		assert.Equal(t, "Agent", m["Role"], "login %q", l)
	}

	var adminGroupID int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		"SELECT id FROM `groups` WHERE name = ?"), "admin").Scan(&adminGroupID))
	_, err := db.Exec(database.ConvertPlaceholders(`
		INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
		VALUES (?, ?, 'rw', CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)`), agentID, adminGroupID)
	require.NoError(t, err)

	c := identityTestContext()
	c.Set("user", &models.User{ID: uint(agentID), Login: login, ValidID: 1})
	m := getUserMapForTemplate(c)
	assert.Equal(t, true, m["IsAdmin"])
	assert.Equal(t, true, m["IsInAdminGroup"])
	assert.Equal(t, "Admin", m["Role"])
}

// getUserFromContext must return the authenticated user (the auth middleware
// stores user_id as uint) and never fall back to the admin account.
func TestGetUserFromContextNeverInventsAdmin(t *testing.T) {
	c := identityTestContext()
	assert.Nil(t, getUserFromContext(c), "no identity on the request")

	c = identityTestContext()
	c.Set("user_id", uint(4242))
	c.Set("user_email", "agent@example.test")
	c.Set("user_role", "Agent")
	u := getUserFromContext(c)
	require.NotNil(t, u)
	assert.Equal(t, uint(4242), u.ID)
	assert.Equal(t, "agent@example.test", u.Login)
	assert.Equal(t, "Agent", u.Role)

	c = identityTestContext()
	c.Set("user_id", uint(4242))
	u = getUserFromContext(c)
	require.NotNil(t, u)
	assert.Equal(t, "", u.Role, "missing role must not default to Admin")
}
