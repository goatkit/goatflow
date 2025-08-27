package examples

import (
	"testing"
	"net/http"
	"net/http/httptest"
	"strings"
	"encoding/json"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Example TDD Workflow Demonstration
//
// This file demonstrates the proper TDD workflow using the GOTRS TDD enforcer:
//
// 1. make tdd-init
// 2. make tdd-test-first FEATURE="User Profile Update API"
// 3. Write failing test (this file)
// 4. make tdd-verify --test-failing (should show failing test)
// 5. make tdd-implement
// 6. Write implementation code
// 7. make tdd-verify (all quality gates must pass)
// 8. make tdd-refactor (if needed)
// 9. make tdd-verify --refactor (verify no regressions)

// Step 1: Write a failing test for the new feature
func TestUserProfileUpdateAPI_Integration(t *testing.T) {
	// This test will initially fail because the endpoint doesn't exist
	// This is the RED phase of Red-Green-Refactor
	
	t.Run("should update user profile with valid data", func(t *testing.T) {
		// Arrange
		userID := "12345"
		updateData := map[string]interface{}{
			"first_name": "John",
			"last_name":  "Doe", 
			"email":      "john.doe@example.com",
			"timezone":   "America/New_York",
		}
		
		// This will fail initially because handler doesn't exist
		handler := NewUserProfileHandler() // This function doesn't exist yet
		
		// Create request
		jsonData, err := json.Marshal(updateData)
		require.NoError(t, err)
		
		req := httptest.NewRequest(http.MethodPut, "/api/users/"+userID+"/profile", strings.NewReader(string(jsonData)))
		req.Header.Set("Content-Type", "application/json")
		
		w := httptest.NewRecorder()
		
		// Act
		handler.ServeHTTP(w, req)
		
		// Assert
		assert.Equal(t, http.StatusOK, w.Code, "Expected successful profile update")
		
		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		
		assert.True(t, response["success"].(bool), "Expected success flag to be true")
		assert.Equal(t, "Profile updated successfully", response["message"], "Expected success message")
		assert.NotEmpty(t, response["updated_at"], "Expected updated_at timestamp")
	})
	
	t.Run("should return validation error for invalid email", func(t *testing.T) {
		// Arrange
		userID := "12345"
		updateData := map[string]interface{}{
			"first_name": "John",
			"last_name":  "Doe",
			"email":      "invalid-email", // Invalid email format
			"timezone":   "America/New_York",
		}
		
		handler := NewUserProfileHandler() // This function doesn't exist yet
		
		jsonData, err := json.Marshal(updateData)
		require.NoError(t, err)
		
		req := httptest.NewRequest(http.MethodPut, "/api/users/"+userID+"/profile", strings.NewReader(string(jsonData)))
		req.Header.Set("Content-Type", "application/json")
		
		w := httptest.NewRecorder()
		
		// Act
		handler.ServeHTTP(w, req)
		
		// Assert
		assert.Equal(t, http.StatusBadRequest, w.Code, "Expected validation error")
		
		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		
		assert.False(t, response["success"].(bool), "Expected success flag to be false")
		assert.Contains(t, response["error"].(string), "email", "Expected email validation error")
	})
	
	t.Run("should return error for non-existent user", func(t *testing.T) {
		// Arrange
		userID := "99999" // Non-existent user
		updateData := map[string]interface{}{
			"first_name": "John",
			"last_name":  "Doe",
			"email":      "john.doe@example.com",
			"timezone":   "America/New_York",
		}
		
		handler := NewUserProfileHandler() // This function doesn't exist yet
		
		jsonData, err := json.Marshal(updateData)
		require.NoError(t, err)
		
		req := httptest.NewRequest(http.MethodPut, "/api/users/"+userID+"/profile", strings.NewReader(string(jsonData)))
		req.Header.Set("Content-Type", "application/json")
		
		w := httptest.NewRecorder()
		
		// Act
		handler.ServeHTTP(w, req)
		
		// Assert
		assert.Equal(t, http.StatusNotFound, w.Code, "Expected user not found error")
		
		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		
		assert.False(t, response["success"].(bool), "Expected success flag to be false")
		assert.Contains(t, response["error"].(string), "not found", "Expected user not found error")
	})
	
	t.Run("should require authentication", func(t *testing.T) {
		// Arrange - Request without authentication
		userID := "12345"
		updateData := map[string]interface{}{
			"first_name": "John",
			"last_name":  "Doe",
			"email":      "john.doe@example.com",
		}
		
		handler := NewUserProfileHandler() // This function doesn't exist yet
		
		jsonData, err := json.Marshal(updateData)
		require.NoError(t, err)
		
		req := httptest.NewRequest(http.MethodPut, "/api/users/"+userID+"/profile", strings.NewReader(string(jsonData)))
		req.Header.Set("Content-Type", "application/json")
		// Notably missing: req.Header.Set("Authorization", "Bearer token")
		
		w := httptest.NewRecorder()
		
		// Act
		handler.ServeHTTP(w, req)
		
		// Assert
		assert.Equal(t, http.StatusUnauthorized, w.Code, "Expected authentication required")
	})
	
	t.Run("should validate required fields", func(t *testing.T) {
		// Arrange - Missing required fields
		userID := "12345"
		updateData := map[string]interface{}{
			// Missing first_name and last_name
			"email": "john.doe@example.com",
		}
		
		handler := NewUserProfileHandler() // This function doesn't exist yet
		
		jsonData, err := json.Marshal(updateData)
		require.NoError(t, err)
		
		req := httptest.NewRequest(http.MethodPut, "/api/users/"+userID+"/profile", strings.NewReader(string(jsonData)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer valid-token")
		
		w := httptest.NewRecorder()
		
		// Act
		handler.ServeHTTP(w, req)
		
		// Assert
		assert.Equal(t, http.StatusBadRequest, w.Code, "Expected validation error for missing fields")
		
		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		
		assert.False(t, response["success"].(bool), "Expected success flag to be false")
		assert.Contains(t, strings.ToLower(response["error"].(string)), "required", "Expected required field validation error")
	})
}

// Step 2: Write unit tests for the service layer
func TestUserProfileService_Unit(t *testing.T) {
	// These tests will also fail initially because the service doesn't exist
	
	t.Run("should update profile in database", func(t *testing.T) {
		// Arrange
		service := NewUserProfileService() // This doesn't exist yet
		userID := "12345"
		profile := UserProfile{ // This type doesn't exist yet
			FirstName: "John",
			LastName:  "Doe",
			Email:     "john.doe@example.com",
			Timezone:  "America/New_York",
		}
		
		// Act
		err := service.UpdateProfile(userID, profile)
		
		// Assert
		assert.NoError(t, err, "Expected successful profile update")
	})
	
	t.Run("should validate email format", func(t *testing.T) {
		// Arrange
		service := NewUserProfileService() // This doesn't exist yet
		userID := "12345"
		profile := UserProfile{ // This type doesn't exist yet
			FirstName: "John",
			LastName:  "Doe",
			Email:     "invalid-email-format",
			Timezone:  "America/New_York",
		}
		
		// Act
		err := service.UpdateProfile(userID, profile)
		
		// Assert
		assert.Error(t, err, "Expected validation error for invalid email")
		assert.Contains(t, err.Error(), "email", "Expected email validation error message")
	})
	
	t.Run("should return error for non-existent user", func(t *testing.T) {
		// Arrange
		service := NewUserProfileService() // This doesn't exist yet
		userID := "99999" // Non-existent user
		profile := UserProfile{ // This type doesn't exist yet
			FirstName: "John",
			LastName:  "Doe",
			Email:     "john.doe@example.com",
			Timezone:  "America/New_York",
		}
		
		// Act
		err := service.UpdateProfile(userID, profile)
		
		// Assert
		assert.Error(t, err, "Expected error for non-existent user")
		assert.Contains(t, strings.ToLower(err.Error()), "not found", "Expected user not found error")
	})
}

// Step 3: Write repository tests (if using repository pattern)
func TestUserProfileRepository_Database(t *testing.T) {
	// These will fail initially as repository doesn't exist
	
	t.Run("should save profile to database", func(t *testing.T) {
		// Skip this test if not in integration test environment
		if testing.Short() {
			t.Skip("Skipping database test in short mode")
		}
		
		// Arrange
		repo := NewUserProfileRepository() // This doesn't exist yet
		userID := "test-user-123"
		profile := UserProfile{ // This type doesn't exist yet
			FirstName: "Integration",
			LastName:  "Test",
			Email:     "integration@test.com",
			Timezone:  "UTC",
		}
		
		// Act
		err := repo.UpdateProfile(userID, profile)
		
		// Assert
		assert.NoError(t, err, "Expected successful database update")
		
		// Verify by reading back
		savedProfile, err := repo.GetProfile(userID)
		require.NoError(t, err)
		
		assert.Equal(t, profile.FirstName, savedProfile.FirstName)
		assert.Equal(t, profile.LastName, savedProfile.LastName)
		assert.Equal(t, profile.Email, savedProfile.Email)
		assert.Equal(t, profile.Timezone, savedProfile.Timezone)
		assert.NotZero(t, savedProfile.UpdatedAt, "Expected UpdatedAt to be set")
	})
}

// Placeholder types and functions that will need to be implemented
// These will cause compilation errors initially (RED phase)

// UserProfile represents user profile data
type UserProfile struct {
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Email     string    `json:"email"`
	Timezone  string    `json:"timezone"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NewUserProfileHandler creates a new HTTP handler for user profiles
// This will need to be implemented in the implementation phase
func NewUserProfileHandler() http.Handler {
	// This will cause a compilation error initially
	// Implementation phase will create this function
	panic("not implemented - this is expected during test-first phase")
}

// NewUserProfileService creates a new user profile service
// This will need to be implemented in the implementation phase  
func NewUserProfileService() UserProfileService {
	// This will cause a compilation error initially
	// Implementation phase will create this function
	panic("not implemented - this is expected during test-first phase")
}

// UserProfileService interface for profile operations
type UserProfileService interface {
	UpdateProfile(userID string, profile UserProfile) error
	GetProfile(userID string) (*UserProfile, error)
}

// NewUserProfileRepository creates a new user profile repository
// This will need to be implemented in the implementation phase
func NewUserProfileRepository() UserProfileRepository {
	// This will cause a compilation error initially
	// Implementation phase will create this function
	panic("not implemented - this is expected during test-first phase")
}

// UserProfileRepository interface for database operations
type UserProfileRepository interface {
	UpdateProfile(userID string, profile UserProfile) error
	GetProfile(userID string) (*UserProfile, error)
}

/*
TDD Workflow Steps for this example:

1. Initialize TDD:
   make tdd-init

2. Start test-first phase:
   make tdd-test-first FEATURE="User Profile Update API"

3. Write this test file (RED phase - tests fail)
   - Tests fail because functions/types don't exist
   - This is expected and correct!

4. Verify tests are failing:
   make tdd-verify --test-failing
   - Should show compilation errors or test failures
   - This confirms we're in proper TDD RED phase

5. Implement phase:
   make tdd-implement
   - Create actual implementations:
     * internal/api/user_profile_handlers.go
     * internal/service/user_profile_service.go  
     * internal/repository/user_profile_repository.go
   - Implement just enough to make tests pass

6. Verify implementation (GREEN phase):
   make tdd-verify
   - ALL 7 quality gates must pass
   - Compilation: ✓
   - Service Health: ✓  
   - Templates: ✓
   - Go Tests: ✓ (including these tests)
   - HTTP Endpoints: ✓
   - Browser Console: ✓
   - Log Analysis: ✓
   
7. Refactor phase (REFACTOR phase):
   make tdd-refactor
   - Clean up code
   - Extract common patterns
   - Improve error handling
   
8. Verify no regressions:
   make tdd-verify --refactor
   - All gates must still pass after refactoring
   - Evidence report generated with before/after comparison

Quality Gates prevent claiming success without evidence:
- No "the tests pass" without actually running make tdd-verify
- No "it works" without service health verification
- No "UI is fine" without browser console verification
- No "ready for production" without all 7 gates passing

This prevents the "Claude the intern" pattern of premature success claims.
*/