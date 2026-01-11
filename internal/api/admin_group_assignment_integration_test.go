package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
)

// 2. But UI/API might not be displaying or updating them correctly.
func TestRealGroupAssignmentIssue(t *testing.T) {
	config := GetTestConfig()

	if db, err := database.GetDB(); err == nil && db != nil {
		ensureTestUserWithGroups(t, db, config)
	}

	aggregateQuery := func() string {
		if database.IsMySQL() {
			return `
				SELECT GROUP_CONCAT(g.name SEPARATOR ', ') as groups
				FROM users u
				LEFT JOIN group_user gu ON u.id = gu.user_id
				LEFT JOIN groups g ON gu.group_id = g.id
				WHERE u.login = ?
				GROUP BY u.id, u.login, u.first_name, u.last_name`
		}
		return `
			SELECT string_agg(g.name, ', ') as groups
			FROM users u
			LEFT JOIN group_user gu ON u.id = gu.user_id
			LEFT JOIN groups g ON gu.group_id = g.id
			WHERE u.login = ?
			GROUP BY u.id, u.login, u.first_name, u.last_name`
	}

	// Manual database check first - this should pass based on our earlier query
	t.Run("Database verification: Test user should have groups", func(t *testing.T) {
		db, err := database.GetDB()
		if err != nil || db == nil {
			t.Skip("Database not available")
		}

		ensureTestUserWithGroups(t, db, config)

		// Query to verify user groups
		var groups string
		err = db.QueryRow(database.ConvertPlaceholders(aggregateQuery()), config.UserLogin).Scan(&groups)

		if err != nil {
			t.Logf("Could not find test user or query failed: %v", err)
			t.Skip("Test user not found - create him first through UI")
		}

		assert.NotEmpty(t, groups, "Test user should have groups in database")
		// Check for expected groups from config
		for _, expectedGroup := range config.UserGroups {
			assert.Contains(t, groups, expectedGroup, "Test user should have %s group", expectedGroup)
		}
		t.Logf("SUCCESS: Database shows test user has groups: %s", groups)
	})

	t.Run("API GET verification: Does HandleAdminUserGet return the groups?", func(t *testing.T) {
		db, err := database.GetDB()
		if err != nil || db == nil {
			t.Skip("Database not available")
		}

		ensureTestUserWithGroups(t, db, config)

		// Get test user's ID
		var testUserID int
		testUsername := os.Getenv("TEST_USERNAME")
		if testUsername == "" {
			testUsername = "testuser"
		}
		err = db.QueryRow(database.ConvertPlaceholders("SELECT id FROM users WHERE login = ?"), testUsername).Scan(&testUserID)
		if err != nil {
			t.Skip("Test user not found")
		}

		// Test the HandleAdminUserGet function directly
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.GET("/admin/users/:id", HandleAdminUserGet)

		// Make request
		req, _ := http.NewRequest("GET", "/admin/users/"+strconv.Itoa(testUserID), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Check response
		assert.Equal(t, http.StatusOK, w.Code, "API should return 200")

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Check if response contains groups
		if data, ok := response["data"].(map[string]interface{}); ok {
			if groups, hasGroups := data["groups"]; hasGroups {
				t.Logf("SUCCESS: API returns groups: %v", groups)
			} else {
				t.Error("FAILED: API does not include groups in response")
			}
		} else {
			t.Error("FAILED: API response malformed")
		}
	})

	t.Run("Form submission test: Can we update Test user's groups via HandleAdminUserUpdate?", func(t *testing.T) {
		db, err := database.GetDB()
		if err != nil || db == nil {
			t.Skip("Database not available")
		}

		ensureTestUserWithGroups(t, db, config)

		// Get test user's ID and current data
		var testUserID int
		var firstName, lastName, login string
		testUsernameLocal := os.Getenv("TEST_USERNAME")
		if testUsernameLocal == "" {
			testUsernameLocal = "testuser"
		}
		err = db.QueryRow(database.ConvertPlaceholders("SELECT id, login, first_name, last_name FROM users WHERE login = ?"), testUsernameLocal).Scan(&testUserID, &login, &firstName, &lastName)
		if err != nil {
			t.Skip("Test user not found")
		}

		// Test the HandleAdminUserUpdate function
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.PUT("/admin/users/:id", HandleAdminUserUpdate)

		// Create form data that adds test groups
		formData := url.Values{}
		formData.Set("login", login)
		formData.Set("first_name", firstName)
		formData.Set("last_name", lastName)
		formData.Set("valid_id", "1")
		// Add groups from config
		for _, group := range config.UserGroups {
			formData.Add("groups", group)
		}

		// Make request
		req, _ := http.NewRequest("PUT", "/admin/users/"+strconv.Itoa(testUserID),
			strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Check response
		t.Logf("Update response status: %d", w.Code)
		t.Logf("Update response body: %s", w.Body.String())

		// Now check if the database was actually updated
		var newGroups string
		err = db.QueryRow(database.ConvertPlaceholders(aggregateQuery()), config.UserLogin).Scan(&newGroups)

		if err == nil {
			t.Logf("Groups after update: %s", newGroups)
		} else {
			t.Logf("Could not query groups after update: %v", err)
		}
	})
}

func ensureTestUserWithGroups(t *testing.T, db *sql.DB, config TestConfig) {
	t.Helper()

	userRepo := repository.NewUserRepository(db)
	user, err := userRepo.GetByLogin(config.UserLogin)
	if err != nil {
		user, err = findUserAnyStatus(db, config.UserLogin)
		if err != nil {
			if err != sql.ErrNoRows {
				require.NoError(t, err)
			}

			user = &models.User{
				Login:      config.UserLogin,
				Password:   "$2a$10$test",
				FirstName:  config.UserFirstName,
				LastName:   config.UserLastName,
				ValidID:    1,
				CreateBy:   1,
				ChangeBy:   1,
				CreateTime: time.Now(),
				ChangeTime: time.Now(),
			}

			err = userRepo.Create(user)
			if err != nil {
				if isDuplicateKeyError(err) {
					user, err = findUserAnyStatus(db, config.UserLogin)
					require.NoError(t, err)
				} else {
					require.NoError(t, err)
				}
			}
		}
	}

	if user.ValidID != 1 {
		require.NoError(t, userRepo.SetValidID(user.ID, 1, 1, time.Now()))
		user.ValidID = 1
	}

	groupRepo := repository.NewGroupRepository(db)
	for _, groupName := range config.UserGroups {
		groupID := ensureGroupExists(t, db, groupRepo, groupName)
		require.NoError(t, groupRepo.AddUserToGroup(user.ID, groupID))
	}
}

func findUserAnyStatus(db *sql.DB, login string) (*models.User, error) {
	query := database.ConvertPlaceholders("SELECT id, valid_id FROM users WHERE login = ? LIMIT 1")
	var id int64
	var validID int
	err := db.QueryRow(query, login).Scan(&id, &validID)
	if err != nil {
		return nil, err
	}

	return &models.User{
		ID:      uint(id),
		Login:   login,
		ValidID: validID,
	}, nil
}

func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate entry") ||
		strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "duplicate key value") ||
		strings.Contains(msg, "unique violation")
}

func ensureGroupExists(t *testing.T, db *sql.DB, groupRepo *repository.GroupSQLRepository, name string) uint {
	t.Helper()

	query := database.ConvertPlaceholders("SELECT id FROM groups WHERE name = ?")
	var existingID uint64
	err := db.QueryRow(query, name).Scan(&existingID)
	if err == nil {
		return uint(existingID)
	}

	if err != sql.ErrNoRows {
		require.NoError(t, err)
	}

	group := &models.Group{
		Name:     name,
		ValidID:  1,
		CreateBy: 1,
		ChangeBy: 1,
	}

	require.NoError(t, groupRepo.Create(group))
	switch id := group.ID.(type) {
	case int64:
		return uint(id)
	case int:
		return uint(id)
	case uint:
		return id
	case uint64:
		return uint(id)
	default:
		require.Failf(t, "invalid group id", "unexpected group id type %T", group.ID)
		return 0
	}
}
