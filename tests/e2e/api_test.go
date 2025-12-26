//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAPIQueueManagement tests queue operations via API
// This gives us visibility into the actual API behavior without needing browser automation
func TestAPIQueueManagement(t *testing.T) {
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://backend:8080"
	}

	// Create HTTP client with cookie jar for session
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: jar,
	}

	// Login first
	t.Run("Login", func(t *testing.T) {
		loginData := url.Values{
			"email":    {os.Getenv("DEMO_ADMIN_EMAIL")},
			"password": {os.Getenv("DEMO_ADMIN_PASSWORD")},
		}

		resp, err := client.PostForm(baseURL+"/api/auth/login", loginData)
		require.NoError(t, err)
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		t.Logf("Login response: %s", body)

		// Check if we got a session cookie
		cookies := jar.Cookies(&url.URL{Scheme: "http", Host: "backend:8080"})
		hasSession := false
		for _, cookie := range cookies {
			if strings.Contains(cookie.Name, "token") || strings.Contains(cookie.Name, "session") {
				hasSession = true
				t.Logf("Got session cookie: %s", cookie.Name)
			}
		}
		assert.True(t, hasSession, "Should have session after login")
	})

	// Test queue list
	t.Run("List Queues", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/queues")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "Should get queues list")

		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)

		// Check if it's HTML (should contain queue-related content)
		assert.Contains(t, bodyStr, "queue", "Should have queue content")
		t.Log("Successfully retrieved queue list page")
	})

	// Test creating a queue via API
	t.Run("Create Queue API", func(t *testing.T) {
		queueData := map[string]interface{}{
			"name":           "APITestQueue",
			"comment":        "Created via E2E API test",
			"system_address": "api-test@example.com",
		}

		jsonData, _ := json.Marshal(queueData)
		req, err := http.NewRequest("POST", baseURL+"/api/queues", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		t.Logf("Create queue response: %d - %s", resp.StatusCode, body)

		// Even if creation fails, we learn about the API
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
			t.Log("✅ Queue creation API works")
		} else {
			t.Logf("Queue creation returned status %d", resp.StatusCode)
		}
	})

	// Test queue edit form population
	t.Run("Check Edit Form Population", func(t *testing.T) {
		// Try to get edit form for queue ID 1 (Postmaster usually exists)
		resp, err := client.Get(baseURL + "/queues/1/edit")
		require.NoError(t, err)
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)

		// Check if form has populated values
		if strings.Contains(bodyStr, "value=\"") {
			// Extract value to see if it's populated
			start := strings.Index(bodyStr, "value=\"") + 7
			if start > 7 && start < len(bodyStr) {
				end := strings.Index(bodyStr[start:], "\"")
				if end > 0 {
					value := bodyStr[start : start+end]
					if value != "" {
						t.Logf("✅ Edit form has populated value: %s", value)
					} else {
						t.Log("❌ Edit form has empty value attribute")
					}
				}
			}
		} else {
			t.Log("❌ Edit form does not contain value attributes")
		}

		// Also check for textarea content
		if strings.Contains(bodyStr, "<textarea") {
			if strings.Contains(bodyStr, "</textarea>") {
				// Check if there's content between textarea tags
				start := strings.Index(bodyStr, "<textarea")
				end := strings.Index(bodyStr, "</textarea>")
				if start > 0 && end > start {
					textareaSection := bodyStr[start:end]
					// Find content after '>'
					contentStart := strings.Index(textareaSection, ">") + 1
					if contentStart > 1 && contentStart < len(textareaSection) {
						content := strings.TrimSpace(textareaSection[contentStart:])
						if content != "" {
							t.Logf("✅ Textarea has content: %s", content[:min(50, len(content))])
						} else {
							t.Log("❌ Textarea is empty")
						}
					}
				}
			}
		}
	})

	// Test queue update
	t.Run("Update Queue", func(t *testing.T) {
		updateData := map[string]interface{}{
			"name":    "Postmaster_Updated",
			"comment": "Updated via E2E test",
		}

		jsonData, _ := json.Marshal(updateData)
		req, err := http.NewRequest("PUT", baseURL+"/api/queues/1", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		t.Logf("Update response: %d - %s", resp.StatusCode, body)

		if resp.StatusCode == http.StatusOK {
			t.Log("✅ Queue update API works")
		}
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
