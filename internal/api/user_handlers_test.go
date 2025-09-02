package api

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateUserWithGroups(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	
	// Initialize database connection
    db, err := database.GetDB()
    if err != nil || db == nil {
        t.Skip("Database not available, skipping integration test")
    }

	// Create a test router
	router := gin.New()
	router.POST("/admin/users", handleCreateUser)

	tests := []struct {
		name           string
		formData       url.Values
		expectedStatus int
		checkFunc      func(t *testing.T)
	}{
		{
			name: "Create user without groups",
			formData: url.Values{
				"login":      {"testuser1"},
				"password":   {"Test123!"},
				"first_name": {"Test"},
				"last_name":  {"User"},
				"title":      {"Mr."},
				"valid_id":   {"1"},
			},
			expectedStatus: http.StatusOK,
			checkFunc: func(t *testing.T) {
				// Check user was created
				userRepo := repository.NewUserRepository(db)
				user, err := userRepo.GetByLogin("testuser1")
				assert.NoError(t, err)
				assert.NotNil(t, user)
				assert.Equal(t, "Test", user.FirstName)
				assert.Equal(t, "User", user.LastName)
			},
		},
		{
			name: "Create user with single group",
			formData: url.Values{
				"login":      {"testuser2"},
				"password":   {"Test123!"},
				"first_name": {"Test2"},
				"last_name":  {"User2"},
				"title":      {"Ms."},
				"valid_id":   {"1"},
				"groups":     {"1"}, // Assuming group ID 1 exists (admin group)
			},
			expectedStatus: http.StatusOK,
			checkFunc: func(t *testing.T) {
				// Check user was created
				userRepo := repository.NewUserRepository(db)
				user, err := userRepo.GetByLogin("testuser2")
				assert.NoError(t, err)
				assert.NotNil(t, user)
				
				// Check group assignment
				groupRepo := repository.NewGroupRepository(db)
				groups, err := groupRepo.GetUserGroups(user.ID)
				assert.NoError(t, err)
				assert.NotEmpty(t, groups)
			},
		},
		{
			name: "Create user with multiple groups",
			formData: url.Values{
				"login":      {"testuser3"},
				"password":   {"Test123!"},
				"first_name": {"Test3"},
				"last_name":  {"User3"},
				"title":      {"Dr."},
				"valid_id":   {"1"},
				"groups":     {"1", "2"}, // Multiple groups
			},
			expectedStatus: http.StatusOK,
			checkFunc: func(t *testing.T) {
				// Check user was created
				userRepo := repository.NewUserRepository(db)
				user, err := userRepo.GetByLogin("testuser3")
				assert.NoError(t, err)
				assert.NotNil(t, user)
				
				// Check group assignments
				groupRepo := repository.NewGroupRepository(db)
				groups, err := groupRepo.GetUserGroups(user.ID)
				assert.NoError(t, err)
				assert.GreaterOrEqual(t, len(groups), 2)
			},
		},
		{
			name: "Create user with missing required fields",
			formData: url.Values{
				"login":    {""},
				"password": {"Test123!"},
				"title":    {"Mr."},
			},
			expectedStatus: http.StatusBadRequest,
			checkFunc: func(t *testing.T) {
				// User should not be created
				userRepo := repository.NewUserRepository(db)
				user, _ := userRepo.GetByLogin("")
				assert.Nil(t, user)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request
			req, err := http.NewRequest("POST", "/admin/users", strings.NewReader(tt.formData.Encode()))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			// Create response recorder
			w := httptest.NewRecorder()

			// Perform request
			router.ServeHTTP(w, req)

			// Check status code
			assert.Equal(t, tt.expectedStatus, w.Code)

			// Run custom checks
			if tt.checkFunc != nil {
				tt.checkFunc(t)
			}

			// Cleanup - remove test users
			if strings.HasPrefix(tt.formData.Get("login"), "testuser") {
				db.Exec("DELETE FROM user_groups WHERE user_id IN (SELECT id FROM users WHERE login LIKE 'testuser%')")
				db.Exec("DELETE FROM users WHERE login LIKE 'testuser%'")
			}
		})
	}
}

func TestUpdateUserGroups(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	
    db, err := database.GetDB()
    if err != nil || db == nil {
        t.Skip("Database not available, skipping integration test")
    }

	// Create a test user first
	userRepo := repository.NewUserRepository(db)
	testUser := createTestUser(t, userRepo)
	defer cleanupTestUser(t, db, testUser.ID)

	// Test adding groups to existing user
	groupRepo := repository.NewGroupRepository(db)
	
	// Add user to group 1
	err = groupRepo.AddUserToGroup(testUser.ID, 1)
	assert.NoError(t, err)
	
	// Check user is in group
	groups, err := groupRepo.GetUserGroups(testUser.ID)
	assert.NoError(t, err)
	assert.NotEmpty(t, groups)
	
	// Try adding to same group again (should not error)
	err = groupRepo.AddUserToGroup(testUser.ID, 1)
	assert.NoError(t, err)
	
	// Remove user from group
	err = groupRepo.RemoveUserFromGroup(testUser.ID, 1)
	assert.NoError(t, err)
	
	// Check user is not in group
	groups, err = groupRepo.GetUserGroups(testUser.ID)
	assert.NoError(t, err)
	assert.Empty(t, groups)
}

func createTestUser(t *testing.T, repo *repository.UserRepository) *models.User {
	user := &models.User{
		Login:      "testuser_" + time.Now().Format("20060102150405"),
		Password:   "$2a$10$test",
		FirstName:  "Test",
		LastName:   "User",
		ValidID:    1,
		CreateBy:   1,
		ChangeBy:   1,
		CreateTime: time.Now(),
		ChangeTime: time.Now(),
	}
	err := repo.Create(user)
	require.NoError(t, err)
	return user
}

func cleanupTestUser(t *testing.T, db *sql.DB, userID uint) {
	db.Exec("DELETE FROM user_groups WHERE user_id = $1", userID)
	db.Exec("DELETE FROM users WHERE id = $1", userID)
}