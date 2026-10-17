package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/middleware"
	platformservice "github.com/goatkit/goatflow/internal/platform/service"
)

// TestPluginEnvelopeCarriesUserTimezone: a plugin call carries the caller's
// stored time zone as _user_timezone, a client-supplied value under that
// name never reaches the plugin, and an agent without a preference sends no
// key at all (plugins then fall back to app.timezone).
func TestPluginEnvelopeCarriesUserTimezone(t *testing.T) {
	env := newCallerContextEnv(t)
	prefs := platformservice.NewUserPreferencesService(env.db)
	t.Cleanup(func() { _ = prefs.SetTimezone(env.adminID, "") })

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.NewI18nMiddleware().Handle())
	RegisterPluginAPIRoutes(router.Group("/api/v1"))
	callPath := "/api/v1/plugins/" + env.probe.name + "/call/probe"
	forged := `{"_user_timezone":"Mars/Olympus_Mons","name":"kept"}`

	decodeView(t, env.do(t, router, http.MethodPost, callPath, forged, env.orgA))
	args := env.probe.lastArgs()
	assert.NotContains(t, args, "_user_timezone", "no preference: the forged value is dropped and nothing replaces it")
	assert.Equal(t, "kept", args["name"])

	require.NoError(t, prefs.SetTimezone(env.adminID, "Pacific/Auckland"))
	decodeView(t, env.do(t, router, http.MethodPost, callPath, forged, env.orgA))
	args = env.probe.lastArgs()
	assert.Equal(t, "Pacific/Auckland", args["_user_timezone"], "the stored preference wins over the body")

	// A stored value the host cannot load (zone renamed, tzdata trimmed)
	// is treated as unset rather than handed to plugins.
	require.NoError(t, prefs.SetPreference(env.adminID, platformservice.TimezonePreferenceKey, "Nowhere/Invalid"))
	decodeView(t, env.do(t, router, http.MethodPost, callPath, forged, env.orgA))
	assert.NotContains(t, env.probe.lastArgs(), "_user_timezone")
}

// The timezone preference round-trips through its handlers: an IANA name is
// stored, "Local" and unknown names are refused with a 400 naming the rule,
// and an empty value clears the preference.
func TestTimezonePreferenceHandlers(t *testing.T) {
	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Skip("database not available")
	}
	const userID = 1
	prefs := platformservice.NewUserPreferencesService(db)
	t.Cleanup(func() { _ = prefs.SetTimezone(userID, "") })

	router := setupPreferencesTestRouter()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Next()
	})
	router.GET("/api/preferences/timezone", HandleGetTimezone)
	router.POST("/api/preferences/timezone", HandleSetTimezone)

	post := func(body string) (int, map[string]any) {
		req := httptest.NewRequest(http.MethodPost, "/api/preferences/timezone", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		var out map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out
	}
	get := func() map[string]any {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/preferences/timezone", nil))
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var out map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
		return out
	}

	code, _ := post(`{"value":" Europe/London "}`)
	require.Equal(t, http.StatusOK, code)
	got := get()
	assert.Equal(t, "Europe/London", got["value"], "stored trimmed")
	assert.NotEmpty(t, got["default"], "the instance default travels with the value")

	for _, bad := range []string{"Local", "GMT+1 somewhere", "Nowhere/Invalid"} {
		code, out := post(`{"value":"` + bad + `"}`)
		assert.Equal(t, http.StatusBadRequest, code, bad)
		assert.Contains(t, out["error"], "IANA", bad)
	}
	assert.Equal(t, "Europe/London", get()["value"], "a refused value leaves the stored one alone")

	code, _ = post(`{"value":""}`)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "", get()["value"], "empty clears the preference")
}
