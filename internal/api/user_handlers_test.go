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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
)

type testCase struct {
	name           string
	buildForm      func() (url.Values, string)
	expectedStatus int
	checkFunc      func(t *testing.T, login string)
}

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
	// Use existing admin create handler that accepts form data
	router.POST("/admin/users", HandleAdminUsersCreate)

	tests := []testCase{
		{
			name: "Create user without groups",
			buildForm: func() (url.Values, string) {
				login := generateUniqueLogin("testuser1")
				form := url.Values{
					"login":      {login},
					"password":   {"Test123!"},
					"first_name": {"Test"},
					"last_name":  {"User"},
					"title":      {"Mr."},
					"valid_id":   {"1"},
				}
				return form, login
			},
			expectedStatus: http.StatusOK,
			checkFunc: func(t *testing.T, login string) {
				userRepo := repository.NewUserRepository(db)
				user, err := userRepo.GetByLogin(login)
				require.NoError(t, err)
				require.NotNil(t, user)
				assert.Equal(t, "Test", user.FirstName)
				assert.Equal(t, "User", user.LastName)
			},
		},
		{
			name: "Create user with single group",
			buildForm: func() (url.Values, string) {
				login := generateUniqueLogin("testuser2")
				form := url.Values{
					"login":      {login},
					"password":   {"Test123!"},
					"first_name": {"Test2"},
					"last_name":  {"User2"},
					"title":      {"Ms."},
					"valid_id":   {"1"},
					"groups":     {"1"}, // Assuming group ID 1 exists (admin group)
				}
				return form, login
			},
			expectedStatus: http.StatusOK,
			checkFunc: func(t *testing.T, login string) {
				userRepo := repository.NewUserRepository(db)
				user, err := userRepo.GetByLogin(login)
				require.NoError(t, err)
				require.NotNil(t, user)

				groupRepo := repository.NewGroupRepository(db)
				groups, err := groupRepo.GetUserGroups(user.ID)
				require.NoError(t, err)
				assert.NotEmpty(t, groups)
			},
		},
		{
			name: "Create user with multiple groups",
			buildForm: func() (url.Values, string) {
				login := generateUniqueLogin("testuser3")
				form := url.Values{
					"login":      {login},
					"password":   {"Test123!"},
					"first_name": {"Test3"},
					"last_name":  {"User3"},
					"title":      {"Dr."},
					"valid_id":   {"1"},
					"groups":     {"1", "2"}, // Multiple groups
				}
				return form, login
			},
			expectedStatus: http.StatusOK,
			checkFunc: func(t *testing.T, login string) {
				userRepo := repository.NewUserRepository(db)
				user, err := userRepo.GetByLogin(login)
				require.NoError(t, err)
				require.NotNil(t, user)

				groupRepo := repository.NewGroupRepository(db)
				groups, err := groupRepo.GetUserGroups(user.ID)
				require.NoError(t, err)
				assert.GreaterOrEqual(t, len(groups), 2)
			},
		},
		{
			name: "Create user with missing required fields",
			buildForm: func() (url.Values, string) {
				form := url.Values{
					"login":    {""},
					"password": {"Test123!"},
					"title":    {"Mr."},
				}
				return form, ""
			},
			expectedStatus: http.StatusBadRequest,
			checkFunc: func(t *testing.T, login string) {
				userRepo := repository.NewUserRepository(db)
				user, err := userRepo.GetByLogin(login)
				assert.Error(t, err)
				assert.Nil(t, user)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formData, login := tt.buildForm()

			// Create request
			req, err := http.NewRequest("POST", "/admin/users", strings.NewReader(formData.Encode()))
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
				tt.checkFunc(t, login)
			}

			if strings.HasPrefix(login, "testuser") {
				db.Exec(database.ConvertPlaceholders("DELETE FROM group_user WHERE user_id IN (SELECT id FROM users WHERE login = ?)"), login)
				db.Exec(database.ConvertPlaceholders("DELETE FROM users WHERE login = ?"), login)
			}
		})
	}
}

func generateUniqueLogin(base string) string {
	return base + "_" + time.Now().Format("20060102150405.000000000")
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
	db.Exec(database.ConvertPlaceholders("DELETE FROM group_user WHERE user_id = ?"), userID)
	db.Exec(database.ConvertPlaceholders("DELETE FROM users WHERE id = ?"), userID)
}
