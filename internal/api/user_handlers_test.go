package api

import (
	"database/sql"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/repository"
)

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
