//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/tests/e2e/config"
	"github.com/goatkit/goatflow/tests/e2e/helpers"
)

// TestAPIQueueManagement drives the queue endpoints the admin queue page uses
// (routes/api-v1-global.yaml /api/v1/queues for create/update/delete,
// routes/api-queues.yaml /api/queues/:id for the edit form) over a plain HTTP
// session obtained from the login form endpoint.
func TestAPIQueueManagement(t *testing.T) {
	cfg := config.GetConfig()
	cfg.RequireReachable(t)
	baseURL, err := url.Parse(cfg.BaseURL)
	require.NoError(t, err)

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	client := &http.Client{Jar: jar, Timeout: 30 * time.Second}

	call := func(t *testing.T, method, path string, body any) (int, map[string]any) {
		t.Helper()
		var payload bytes.Buffer
		if body != nil {
			require.NoError(t, json.NewEncoder(&payload).Encode(body))
		}
		req, err := http.NewRequest(method, cfg.BaseURL+path, &payload)
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		var out map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&out), "%s %s should return a JSON object", method, path)
		return resp.StatusCode, out
	}
	data := func(t *testing.T, body map[string]any) map[string]any {
		t.Helper()
		d, ok := body["data"].(map[string]any)
		require.True(t, ok, "response should carry a data object: %v", body)
		return d
	}

	name := fmt.Sprintf("E2EAPIQueue_%d", time.Now().UnixNano())
	renamed := name + "_Updated"
	var queueID int
	// Queues are never hard-deleted (OTRS semantics): invalidate on exit.
	t.Cleanup(func() {
		if queueID != 0 {
			status, body := call(t, http.MethodDelete, fmt.Sprintf("/api/v1/queues/%d", queueID), nil)
			assert.Equal(t, http.StatusOK, status, "invalidate test queue: %v", body)
		}
	})

	t.Run("Login", func(t *testing.T) {
		resp, err := client.PostForm(cfg.BaseURL+"/api/auth/login", url.Values{
			"username": {cfg.AdminEmail},
			"password": {cfg.AdminPassword},
		})
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "/dashboard", resp.Request.URL.Path, "login redirects to the dashboard")

		cookies := map[string]bool{}
		for _, c := range jar.Cookies(baseURL) {
			cookies[c.Name] = c.Value != ""
		}
		require.True(t, cookies["access_token"], "login should set the access_token cookie, got %v", cookies)
	})

	t.Run("List Queues", func(t *testing.T) {
		status, body := call(t, http.MethodGet, "/api/v1/queues", nil)
		require.Equal(t, http.StatusOK, status, "%v", body)
		queues, ok := body["data"].([]any)
		require.True(t, ok, "data array: %v", body)
		names := map[string]bool{}
		for _, q := range queues {
			names[q.(map[string]any)["name"].(string)] = true
		}
		assert.True(t, names[helpers.SeedQueueName], "seeded queue %q should be listed: %v", helpers.SeedQueueName, names)
	})

	t.Run("Create Queue", func(t *testing.T) {
		status, body := call(t, http.MethodPost, "/api/v1/queues", map[string]any{
			"name":     name,
			"group_id": 1,
			"comments": "Created via E2E API test",
		})
		require.Equal(t, http.StatusCreated, status, "%v", body)
		created := data(t, body)
		assert.Equal(t, name, created["name"])
		id, ok := created["id"].(float64)
		require.True(t, ok && id > 0, "created queue id: %v", created)
		queueID = int(id)

		status, body = call(t, http.MethodPost, "/api/v1/queues", map[string]any{"name": name, "group_id": 1})
		assert.Equal(t, http.StatusConflict, status, "duplicate name must be rejected: %v", body)
	})

	t.Run("Check Edit Form Population", func(t *testing.T) {
		require.NotZero(t, queueID, "queue creation failed; this flow is sequential")
		// The admin edit modal (editQueue) fills its form from this endpoint.
		status, body := call(t, http.MethodGet, fmt.Sprintf("/api/queues/%d", queueID), nil)
		require.Equal(t, http.StatusOK, status, "%v", body)
		q := data(t, body)
		assert.Equal(t, name, q["name"])
		assert.Equal(t, "Created via E2E API test", q["comments"])
		assert.EqualValues(t, 1, q["group_id"])
		assert.EqualValues(t, 1, q["valid_id"])
	})

	t.Run("Update Queue", func(t *testing.T) {
		require.NotZero(t, queueID, "queue creation failed; this flow is sequential")
		status, body := call(t, http.MethodPut, fmt.Sprintf("/api/v1/queues/%d", queueID), map[string]any{
			"name":     renamed,
			"comments": "Updated via E2E test",
		})
		require.Equal(t, http.StatusOK, status, "%v", body)
		assert.Equal(t, renamed, data(t, body)["name"])

		status, body = call(t, http.MethodGet, fmt.Sprintf("/api/queues/%d", queueID), nil)
		require.Equal(t, http.StatusOK, status, "%v", body)
		q := data(t, body)
		assert.Equal(t, renamed, q["name"])
		assert.Equal(t, "Updated via E2E test", q["comments"])
		assert.EqualValues(t, 1, q["group_id"], "fields absent from the update keep their value")
	})

	t.Run("Delete Queue", func(t *testing.T) {
		require.NotZero(t, queueID, "queue creation failed; this flow is sequential")
		status, body := call(t, http.MethodDelete, fmt.Sprintf("/api/v1/queues/%d", queueID), nil)
		require.Equal(t, http.StatusOK, status, "%v", body)

		// Soft delete: the queue stays, invalid.
		status, body = call(t, http.MethodGet, fmt.Sprintf("/api/queues/%d", queueID), nil)
		require.Equal(t, http.StatusOK, status, "%v", body)
		assert.EqualValues(t, 2, data(t, body)["valid_id"])
	})
}
