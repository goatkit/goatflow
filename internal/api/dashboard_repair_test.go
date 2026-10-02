package api

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/plugin"
	"github.com/goatkit/goatflow/internal/platform/plugin/core"
	platformservice "github.com/goatkit/goatflow/internal/platform/service"
)

// dashboardBrokenDB returns a closed handle of the current driver: every query
// on it fails.
func dashboardBrokenDB(t *testing.T) *sql.DB {
	t.Helper()
	driver, dsn := "mysql", "u:p@tcp(127.0.0.1:1)/none"
	if database.IsPostgreSQL() {
		driver, dsn = "postgres", "postgres://u:p@127.0.0.1:1/none?sslmode=disable"
	}
	broken, err := sql.Open(driver, dsn)
	require.NoError(t, err)
	require.NoError(t, broken.Close())
	return broken
}

func dashboardAgentGet(t *testing.T, userID int, url string) *httptest.ResponseRecorder {
	t.Helper()
	tok := testSessionToken(t, uint(userID), "agent", "agent@example.com", "Agent", false, 0)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	newCustAttRouter(t).ServeHTTP(w, req)
	return w
}

// The status filter offers exactly the valid ticket_state rows of this
// database; a failed lookup is an error, never a hard-coded state list.
func TestBuildTicketStatusOptions(t *testing.T) {
	db := getTestDB(t)

	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT ts.id, tst.name FROM ticket_state ts
		JOIN ticket_state_type tst ON tst.id = ts.type_id
		WHERE ts.valid_id = 1`))
	require.NoError(t, err)
	want := map[string]bool{}
	wantClosed := false
	for rows.Next() {
		var id int
		var typeName string
		require.NoError(t, rows.Scan(&id, &typeName))
		want[strconv.Itoa(id)] = true
		wantClosed = wantClosed || typeName == "closed"
	}
	require.NoError(t, rows.Err())
	require.NoError(t, rows.Close())

	options, hasClosed, err := buildTicketStatusOptions(db)
	require.NoError(t, err)
	got := map[string]bool{}
	for _, o := range options {
		got[o["Value"].(string)] = true
	}
	assert.Equal(t, want, got)
	assert.Equal(t, wantClosed, hasClosed)

	options, _, err = buildTicketStatusOptions(dashboardBrokenDB(t))
	require.Error(t, err)
	assert.Empty(t, options, "a failed lookup must not invent states")
	_, _, err = buildTicketStatusOptions(nil)
	require.Error(t, err)
}

// Ticket fields are text: the recent-tickets fragment must escape them.
func TestRecentTicketsEscapesTicketFields(t *testing.T) {
	f := getRBACFixtures(t)
	setupTemplateRenderer(t)
	sfx := strconv.FormatInt(time.Now().UnixNano(), 36)
	title := `<img src=x onerror=alert(1)> ` + sfx
	listingScopeTicket(t, f.db, "XSS"+sfx, title, f.QueueAlpha, `<b>cust</b>`, time.Now().Add(30*24*time.Hour))

	w := dashboardAgentGet(t, f.AgentAlpha, "/api/dashboard/recent-tickets")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := w.Body.String()
	assert.Contains(t, body, "&lt;img src=x onerror=alert(1)&gt; "+sfx)
	assert.NotContains(t, body, "<img src=x")
	assert.NotContains(t, body, "<b>cust</b>")
}

// A stored widget layout that cannot be read is a server error, not a silent
// "show every widget".
func TestDashboardFailsOnUnreadableWidgetConfig(t *testing.T) {
	f := getRBACFixtures(t)
	setupTemplateRenderer(t)
	prefs := platformservice.NewUserPreferencesService(f.db)
	require.NoError(t, prefs.SetPreference(f.AgentAlpha, "DashboardWidgets", "{not json"))
	t.Cleanup(func() { _ = prefs.DeletePreference(f.AgentAlpha, "DashboardWidgets") })

	w := dashboardAgentGet(t, f.AgentAlpha, "/dashboard")
	assert.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
}

// When a widget's data cannot be loaded the dashboard keeps the widget and
// shows it as unavailable instead of an invented empty state ("No recent
// tickets") or silently dropping it.
func TestDashboardShowsUnavailableWidgets(t *testing.T) {
	f := getRBACFixtures(t)
	setupTemplateRenderer(t)

	mgr := plugin.NewManager(plugin.NewProdHostAPI(plugin.WithDB("default", dashboardBrokenDB(t))))
	require.NoError(t, mgr.Register(context.Background(), core.NewDashboardPlugin()))
	require.NoError(t, mgr.Enable("dashboard-core"))
	prev := pluginManager
	pluginManager = mgr
	t.Cleanup(func() { pluginManager = prev })

	w := dashboardAgentGet(t, f.AgentAlpha, "/dashboard")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := w.Body.String()
	assert.Equal(t, 2, strings.Count(body, "This widget is currently unavailable."), "both dashboard-core widgets are shown as unavailable")
	assert.Contains(t, body, `gs-id="dashboard-core:recent_tickets"`)
	assert.Contains(t, body, `gs-id="dashboard-core:queue_status"`)
	assert.NotContains(t, body, "No recent tickets")
	assert.NotContains(t, body, "No queues available")
}

// The reminders preference defaults to enabled only when nothing is stored; a
// failed lookup is an error.
func TestRemindersEnabledPreference(t *testing.T) {
	f := getRBACFixtures(t)
	prefs := platformservice.NewUserPreferencesService(f.db)
	t.Cleanup(func() { _ = prefs.DeletePreference(f.AgentAlpha, "RemindersEnabled") })

	require.NoError(t, prefs.DeletePreference(f.AgentAlpha, "RemindersEnabled"))
	enabled, err := prefs.GetRemindersEnabled(f.AgentAlpha)
	require.NoError(t, err)
	assert.True(t, enabled)

	require.NoError(t, prefs.SetRemindersEnabled(f.AgentAlpha, false))
	enabled, err = prefs.GetRemindersEnabled(f.AgentAlpha)
	require.NoError(t, err)
	assert.False(t, enabled)

	_, err = platformservice.NewUserPreferencesService(dashboardBrokenDB(t)).GetRemindersEnabled(f.AgentAlpha)
	require.Error(t, err)
}
