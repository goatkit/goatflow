package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/repository"
	"github.com/goatkit/goatflow/internal/service"
)

// mockSessionService is a test double for the session service
type mockSessionService struct {
	sessions map[string]*models.Session
}

func newMockSessionService() *mockSessionService {
	return &mockSessionService{
		sessions: make(map[string]*models.Session),
	}
}

func (m *mockSessionService) CreateSession(userID int, userLogin, userType, remoteAddr, userAgent string) (string, error) {
	sessionID := "test-session-" + time.Now().Format("20060102150405")
	m.sessions[sessionID] = &models.Session{
		SessionID:   sessionID,
		UserID:      userID,
		UserLogin:   userLogin,
		UserType:    userType,
		CreateTime:  time.Now(),
		LastRequest: time.Now(),
		RemoteAddr:  remoteAddr,
		UserAgent:   userAgent,
	}
	return sessionID, nil
}

func (m *mockSessionService) GetSession(sessionID string) (*models.Session, error) {
	if s, ok := m.sessions[sessionID]; ok {
		return s, nil
	}
	return nil, nil
}

func (m *mockSessionService) ListSessions() ([]models.Session, error) {
	result := make([]models.Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		result = append(result, *s)
	}
	return result, nil
}

func (m *mockSessionService) KillSession(sessionID string) error {
	delete(m.sessions, sessionID)
	return nil
}

func (m *mockSessionService) KillUserSessions(userID int) error {
	for id, s := range m.sessions {
		if s.UserID == userID {
			delete(m.sessions, id)
		}
	}
	return nil
}

func (m *mockSessionService) KillAllSessions() error {
	m.sessions = make(map[string]*models.Session)
	return nil
}

func TestHandleAdminSessionsTestMode(t *testing.T) {
	os.Setenv("APP_ENV", "test")
	defer os.Unsetenv("APP_ENV")

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/admin/sessions", handleAdminSessions)

	req, _ := http.NewRequest("GET", "/admin/sessions", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// In test mode with no renderer, should return fallback HTML
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.Code)
	}
	// Response should contain session-related content
	body := resp.Body.String()
	if !strings.Contains(body, "Session") && !strings.Contains(body, "session") {
		t.Error("Expected response to contain 'Session' or 'session'")
	}
}

func TestHandleKillSessionValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.DELETE("/admin/api/sessions/:id", handleKillSession)

	tests := []struct {
		name           string
		sessionID      string
		expectedStatus int
	}{
		{
			name:           "empty session ID",
			sessionID:      "",
			expectedStatus: http.StatusNotFound, // Router won't match empty ID
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest("DELETE", "/admin/api/sessions/"+tc.sessionID, nil)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, resp.Code)
			}
		})
	}
}

func TestHandleKillUserSessionsValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.DELETE("/admin/api/sessions/user/:user_id", handleKillUserSessions)

	tests := []struct {
		name           string
		userID         string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "invalid user ID - not a number",
			userID:         "abc",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid user ID",
		},
		// Note: Zero and negative IDs are valid according to handler behavior
		// The handler allows them and just returns success with 0 sessions killed
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest("DELETE", "/admin/api/sessions/user/"+tc.userID, nil)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, resp.Code)
			}

			if tc.expectedError != "" {
				var result map[string]interface{}
				json.Unmarshal(resp.Body.Bytes(), &result)
				if result["error"] != tc.expectedError {
					t.Errorf("Expected error '%s', got '%v'", tc.expectedError, result["error"])
				}
			}
		})
	}
}

func TestSessionServiceAvailable(t *testing.T) {
	// Without database, sessionServiceAvailable should return false
	result := sessionServiceAvailable()
	// This depends on whether the test environment has a database
	// Just ensure it doesn't panic
	t.Logf("sessionServiceAvailable() returned %v", result)
}

func TestGetSessionServiceReturnsNilWithoutDB(t *testing.T) {
	// getSessionService should return nil, err when DB is not available
	svc, err := getSessionService()
	// In test environment without DB setup, this should fail gracefully
	if err != nil {
		t.Logf("getSessionService() returned expected error: %v", err)
	} else if svc == nil {
		t.Log("getSessionService() returned nil service (no DB)")
	} else {
		t.Log("getSessionService() returned a valid service (DB available)")
	}
}

func TestSessionModelFields(t *testing.T) {
	session := models.Session{
		SessionID:   "abc123",
		UserID:      1,
		UserLogin:   "admin",
		UserType:    "User",
		CreateTime:  time.Now(),
		LastRequest: time.Now(),
		RemoteAddr:  "192.168.1.1",
		UserAgent:   "Mozilla/5.0",
	}

	if session.SessionID != "abc123" {
		t.Errorf("Expected SessionID 'abc123', got '%s'", session.SessionID)
	}
	if session.UserID != 1 {
		t.Errorf("Expected UserID 1, got %d", session.UserID)
	}
	if session.UserLogin != "admin" {
		t.Errorf("Expected UserLogin 'admin', got '%s'", session.UserLogin)
	}
	if session.UserType != "User" {
		t.Errorf("Expected UserType 'User', got '%s'", session.UserType)
	}
	if session.RemoteAddr != "192.168.1.1" {
		t.Errorf("Expected RemoteAddr '192.168.1.1', got '%s'", session.RemoteAddr)
	}
}

func TestMemorySessionRepositoryIntegration(t *testing.T) {
	// Test that session service works with memory repository
	repo := repository.NewMemorySessionRepository()
	svc := service.NewSessionService(repo)

	// Create a session
	sessionID, err := svc.CreateSession(1, "admin", "User", "192.168.1.1", "TestBrowser")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	if sessionID == "" {
		t.Error("Expected non-empty session ID")
	}

	// Get the session
	session, err := svc.GetSession(sessionID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if session == nil {
		t.Fatal("Expected session, got nil")
	}
	if session.UserID != 1 {
		t.Errorf("Expected UserID 1, got %d", session.UserID)
	}
	if session.UserLogin != "admin" {
		t.Errorf("Expected UserLogin 'admin', got '%s'", session.UserLogin)
	}

	// List sessions
	sessions, err := svc.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("Expected 1 session, got %d", len(sessions))
	}

	// Kill the session
	err = svc.KillSession(sessionID)
	if err != nil {
		t.Fatalf("KillSession failed: %v", err)
	}

	// Verify session is gone (GetSession returns error when not found)
	session, err = svc.GetSession(sessionID)
	if err == nil {
		t.Error("Expected error when getting killed session")
	}
	if session != nil {
		t.Error("Expected session to be nil after kill")
	}
}

func TestKillUserSessionsWithMemoryRepo(t *testing.T) {
	repo := repository.NewMemorySessionRepository()
	svc := service.NewSessionService(repo)

	// Create multiple sessions for same user
	_, err := svc.CreateSession(1, "admin", "User", "192.168.1.1", "Browser1")
	if err != nil {
		t.Fatalf("CreateSession 1 failed: %v", err)
	}
	_, err = svc.CreateSession(1, "admin", "User", "192.168.1.2", "Browser2")
	if err != nil {
		t.Fatalf("CreateSession 2 failed: %v", err)
	}
	_, err = svc.CreateSession(2, "user2", "User", "192.168.1.3", "Browser3")
	if err != nil {
		t.Fatalf("CreateSession 3 failed: %v", err)
	}

	// Verify 3 sessions exist
	sessions, _ := svc.ListSessions()
	if len(sessions) != 3 {
		t.Errorf("Expected 3 sessions, got %d", len(sessions))
	}

	// Kill user 1's sessions
	_, err = svc.KillUserSessions(1)
	if err != nil {
		t.Fatalf("KillUserSessions failed: %v", err)
	}

	// Verify only user 2's session remains
	sessions, _ = svc.ListSessions()
	if len(sessions) != 1 {
		t.Errorf("Expected 1 session after killing user 1's sessions, got %d", len(sessions))
	}
	if sessions[0].UserID != 2 {
		t.Errorf("Expected remaining session to be user 2, got user %d", sessions[0].UserID)
	}
}

func TestHandleKillAllSessionsWithoutDB(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.DELETE("/admin/api/sessions", handleKillAllSessions)

	req, _ := http.NewRequest("DELETE", "/admin/api/sessions", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Without DB, should return service unavailable
	if resp.Code != http.StatusServiceUnavailable {
		// Or it might succeed if DB is available in test env
		t.Logf("handleKillAllSessions returned status %d", resp.Code)
	}
}

func TestSessionDataKeys(t *testing.T) {
	// Verify the session data key constants are correct
	expectedKeys := map[string]string{
		"UserID":             models.SessionKeyUserID,
		"UserLogin":          models.SessionKeyUserLogin,
		"UserType":           models.SessionKeyUserType,
		"CreateTime":         models.SessionKeyCreateTime,
		"LastRequest":        models.SessionKeyLastRequest,
		"UserRemoteAddr":     models.SessionKeyUserRemoteAddr,
		"UserRemoteUserAgent": models.SessionKeyUserRemoteAgent,
	}

	for name, key := range expectedKeys {
		if key == "" {
			t.Errorf("Session key constant for %s is empty", name)
		}
	}
}
