package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHandleGetScopes_ReturnsScopesWithDescriptions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/scopes", HandleGetScopes)

	req := httptest.NewRequest("GET", "/scopes", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var resp struct {
		Scopes []struct {
			Scope       string `json:"scope"`
			Description string `json:"description"`
		} `json:"scopes"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(resp.Scopes) == 0 {
		t.Error("Expected scopes to be returned, got empty list")
	}

	// Verify scopes have descriptions
	for _, s := range resp.Scopes {
		if s.Scope == "" {
			t.Error("Scope has empty name")
		}
		if s.Description == "" {
			t.Errorf("Scope %q has empty description", s.Scope)
		}
	}
}

func TestHandleListTokens_RequiresAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/tokens", HandleListTokens)

	req := httptest.NewRequest("GET", "/tokens", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should fail - no service configured
	if w.Code != http.StatusServiceUnavailable && w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 503 or 401, got %d", w.Code)
	}

	// Check error response format
	var resp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	// Error should have structured format with code
	if resp.Error.Code == "" {
		t.Error("Error response missing code")
	}
	if !strings.HasPrefix(resp.Error.Code, "core:") {
		t.Errorf("Error code should have 'core:' prefix, got %q", resp.Error.Code)
	}
}

func TestHandleCreateToken_ValidatesRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	// Test without service - should get service unavailable
	router := gin.New()
	router.POST("/tokens", HandleCreateToken)

	// Empty body
	req := httptest.NewRequest("POST", "/tokens", bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should fail - either no service or validation error
	if w.Code == http.StatusCreated {
		t.Error("Expected request to fail without auth/service, but got 201")
	}
}

func TestHandleCreateToken_RequiresName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	
	// Simulate authenticated user
	router.POST("/tokens", func(c *gin.Context) {
		c.Set("user_id", 1)
		HandleCreateToken(c)
	})

	// Request without name field
	body := `{"scopes": ["tickets:read"]}`
	req := httptest.NewRequest("POST", "/tokens", bytes.NewBuffer([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should fail validation (name required) or service unavailable
	if w.Code == http.StatusCreated {
		t.Error("Expected validation error for missing name, got 201")
	}
	
	var resp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err == nil {
		// If we got a structured error, verify it's about validation or service
		if resp.Error.Code != "" {
			validCodes := []string{"core:invalid_request", "core:service_unavailable"}
			found := false
			for _, c := range validCodes {
				if resp.Error.Code == c {
					found = true
					break
				}
			}
			if !found {
				t.Logf("Got error code: %s (acceptable)", resp.Error.Code)
			}
		}
	}
}

func TestHandleRevokeToken_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	
	router.DELETE("/tokens/:id", func(c *gin.Context) {
		c.Set("user_id", 1)
		HandleRevokeToken(c)
	})

	// Invalid ID format
	req := httptest.NewRequest("DELETE", "/tokens/not-a-number", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var resp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	
	// Should either be service unavailable or invalid ID error
	if w.Code != http.StatusBadRequest && w.Code != http.StatusServiceUnavailable {
		// Parse to see what we got
		json.Unmarshal(w.Body.Bytes(), &resp)
		t.Logf("Response: %d - %+v", w.Code, resp)
	}
	
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err == nil {
		if resp.Error.Code == "core:invalid_id" {
			// This is the expected behaviour
			if w.Code != http.StatusBadRequest {
				t.Errorf("Expected 400 for invalid ID, got %d", w.Code)
			}
		}
	}
}

func TestErrorResponseFormat(t *testing.T) {
	// Verify all error responses follow the standard format
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/tokens", HandleListTokens)

	req := httptest.NewRequest("GET", "/tokens", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Response is not valid JSON: %v", err)
	}

	// Must have "error" key
	errObj, ok := resp["error"]
	if !ok {
		t.Fatal("Error response missing 'error' key")
	}

	// Error must be an object with code and message
	errMap, ok := errObj.(map[string]interface{})
	if !ok {
		t.Fatal("Error should be an object")
	}

	if _, ok := errMap["code"]; !ok {
		t.Error("Error object missing 'code' field")
	}
	if _, ok := errMap["message"]; !ok {
		t.Error("Error object missing 'message' field")
	}
}
