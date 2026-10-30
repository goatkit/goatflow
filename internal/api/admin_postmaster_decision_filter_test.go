package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/plugin"
	"github.com/goatkit/goatflow/internal/platform/sysconfig"
)

const decisionFilterPagePath = "/admin/postmaster-decision-filter"

// resetDecisionFilterConfig removes any stored decision filter settings now
// and after the test, so each test starts from the disabled default.
func resetDecisionFilterConfig(t *testing.T, db *sql.DB) {
	t.Helper()
	reset := func() {
		_, err := db.Exec(database.ConvertPlaceholders(`DELETE FROM sysconfig_modified WHERE name = ?`), sysconfig.DecisionFilterKey)
		require.NoError(t, err)
	}
	reset()
	t.Cleanup(reset)
}

func postDecisionFilter(t *testing.T, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", 1)
		c.Set("user_role", "Admin")
		c.Next()
	})
	router.POST("/admin/api/postmaster-decision-filter", HandleSaveAdminPostmasterDecisionFilter)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/api/postmaster-decision-filter", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	return w
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}

func validDecisionFilterConfig() sysconfig.DecisionFilterConfig {
	return sysconfig.DecisionFilterConfig{
		Enabled:        true,
		ProviderID:     4,
		Threshold:      0.7,
		TimeoutSeconds: 20,
		Queue: sysconfig.DecisionFilterField{
			Enabled:      true,
			Instructions: "Which queue should handle this email?",
			Options: []sysconfig.DecisionFilterOption{
				{Name: "Billing", Description: "Invoices and refunds"},
				{Name: "Raw"},
			},
		},
		Priority: sysconfig.DecisionFilterField{
			Options: []sysconfig.DecisionFilterOption{{Name: "3 normal"}},
		},
	}
}

func TestAdminPostmasterDecisionFilterSaveRoundTrips(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := getTestDB(t)
	resetDecisionFilterConfig(t, db)

	want := validDecisionFilterConfig()
	w := postDecisionFilter(t, mustJSON(t, want))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	got, err := sysconfig.LoadDecisionFilterConfig(db)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestAdminPostmasterDecisionFilterRejectsInvalidConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := getTestDB(t)
	resetDecisionFilterConfig(t, db)

	stored := validDecisionFilterConfig()
	require.NoError(t, sysconfig.SaveDecisionFilterConfig(db, stored, 1))

	noField := validDecisionFilterConfig()
	noField.Queue.Enabled = false

	oneOption := validDecisionFilterConfig()
	oneOption.Queue.Options = oneOption.Queue.Options[:1]

	tests := []struct {
		name    string
		body    []byte
		wantErr string
	}{
		{"enabled without a field", mustJSON(t, noField), "enable at least one"},
		{"field with one option", mustJSON(t, oneOption), "queue needs between 2"},
		{"malformed body", []byte(`{"enabled":`), "Invalid request body"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := postDecisionFilter(t, tt.body)
			require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			var resp struct {
				Success bool   `json:"success"`
				Error   string `json:"error"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			assert.False(t, resp.Success)
			assert.Contains(t, resp.Error, tt.wantErr)

			got, err := sysconfig.LoadDecisionFilterConfig(db)
			require.NoError(t, err)
			assert.Equal(t, stored, got, "a rejected config must leave the stored one in place")
		})
	}
}

// fakeLLMProviders stands in for goatkit-llm's llm_list_providers.
type fakeLLMProviders struct{ response string }

func (p *fakeLLMProviders) GKRegister() plugin.GKRegistration {
	return plugin.GKRegistration{Name: decisionFilterLLMPlugin, Version: "1.0.0"}
}
func (p *fakeLLMProviders) Init(context.Context, plugin.HostAPI) error { return nil }
func (p *fakeLLMProviders) Shutdown(context.Context) error             { return nil }
func (p *fakeLLMProviders) Call(_ context.Context, fn string, _ json.RawMessage) (json.RawMessage, error) {
	if fn != "llm_list_providers" {
		return json.RawMessage(`{"error":"unknown function","status":404}`), nil
	}
	return json.RawMessage(p.response), nil
}

// optionRow returns the rendered table row of the named option.
func optionRow(t *testing.T, body, name string) string {
	t.Helper()
	start := strings.Index(body, `data-name="`+name+`"`)
	require.GreaterOrEqual(t, start, 0, "no row for %q", name)
	end := strings.Index(body[start:], "</tr>")
	require.GreaterOrEqual(t, end, 0)
	return body[start : start+end]
}

func TestAdminPostmasterDecisionFilterPage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupTemplateRenderer(t)
	db := getTestDB(t)
	resetDecisionFilterConfig(t, db)

	sfx := letterSuffix()
	queueName := "DFQueue" + sfx
	queueID := createTestQueue(t, db, queueName)
	t.Cleanup(func() { cleanupTestQueue(t, db, queueID) })

	gone := "DFGone" + sfx
	require.NoError(t, sysconfig.SaveDecisionFilterConfig(db, sysconfig.DecisionFilterConfig{
		Threshold:      0.5,
		TimeoutSeconds: 30,
		Queue: sysconfig.DecisionFilterField{
			Options: []sysconfig.DecisionFilterOption{{Name: gone, Description: "Retired queue"}},
		},
	}, 1))

	prev := pluginManager
	t.Cleanup(func() { pluginManager = prev })

	t.Run("renders without goatkit-llm", func(t *testing.T) {
		pluginManager = nil
		w := serveAdminGET(t, decisionFilterPagePath, decisionFilterPagePath, HandleAdminPostmasterDecisionFilter)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		body := w.Body.String()

		assert.Contains(t, body, `id="decision-provider-unavailable"`)
		assert.NotContains(t, optionRow(t, body, queueName), "checked", "unsaved queues are offered unselected")

		goneRow := optionRow(t, body, gone)
		assert.Contains(t, goneRow, "checked", "saved option stays selected")
		assert.Contains(t, goneRow, "df-missing", "saved option without a valid queue is marked missing")
		assert.Contains(t, goneRow, "Retired queue")
	})

	t.Run("lists only goatkit-llm decide providers", func(t *testing.T) {
		mgr := plugin.NewManager(nil)
		require.NoError(t, mgr.Register(context.Background(), &fakeLLMProviders{response: `{"providers":[
			{"id":11,"name":"DeciderOne","provider_type":"systemone","purpose":"decide","model":"s1","is_active":1},
			{"id":12,"name":"ChatterOne","provider_type":"ollama","purpose":"chat","model":"m","is_active":true}],"count":2}`}))
		pluginManager = mgr

		w := serveAdminGET(t, decisionFilterPagePath, decisionFilterPagePath, HandleAdminPostmasterDecisionFilter)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		body := w.Body.String()

		assert.NotContains(t, body, `id="decision-provider-unavailable"`)
		assert.Contains(t, body, `value="11"`)
		assert.Contains(t, body, "DeciderOne")
		assert.NotContains(t, body, "ChatterOne")
	})

	t.Run("plugin error falls back to the active provider", func(t *testing.T) {
		mgr := plugin.NewManager(nil)
		require.NoError(t, mgr.Register(context.Background(), &fakeLLMProviders{response: `{"error":"list providers failed","status":500}`}))
		pluginManager = mgr

		w := serveAdminGET(t, decisionFilterPagePath, decisionFilterPagePath, HandleAdminPostmasterDecisionFilter)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), `id="decision-provider-unavailable"`)
	})
}
