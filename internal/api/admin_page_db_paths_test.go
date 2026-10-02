package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/repository"
)

func serveAdminGET(t *testing.T, path, pattern string, h gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", 1)
		c.Set("user_role", "Admin")
		c.Next()
	})
	router.GET(pattern, h)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w
}

// The postmaster filter list and forms render rows and dropdowns read from the database.
func TestAdminPostmasterFilterPagesUseDatabase(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupTemplateRenderer(t)
	db := getTestDB(t)

	sfx := letterSuffix()
	queueName := "PMFQueue" + sfx
	queueID := createTestQueue(t, db, queueName)
	t.Cleanup(func() { cleanupTestQueue(t, db, queueID) })

	filterName := "PMFFilter" + sfx
	repo := repository.NewPostmasterFilterRepository(db)
	require.NoError(t, repo.Create(context.Background(), &repository.PostmasterFilter{
		Name:    filterName,
		Matches: []repository.FilterMatch{{Key: "Subject", Value: "invoice"}},
		Sets:    []repository.FilterSet{{Key: "X-GoatFlow-Queue", Value: queueName}},
	}))
	t.Cleanup(func() { _ = repo.Delete(context.Background(), filterName) })

	w := serveAdminGET(t, "/admin/postmaster-filters", "/admin/postmaster-filters", HandleAdminPostmasterFilters)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), filterName)

	w = serveAdminGET(t, "/admin/postmaster-filters/new", "/admin/postmaster-filters/new", HandleAdminPostmasterFilterNew)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), fmt.Sprintf(`{"id":%d,"name":"%s"}`, queueID, queueName))

	w = serveAdminGET(t, "/admin/postmaster-filters/"+filterName, "/admin/postmaster-filters/:name", HandleAdminPostmasterFilterEdit)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), filterName)
	assert.Contains(t, w.Body.String(), fmt.Sprintf(`{"id":%d,"name":"%s"}`, queueID, queueName))
}

// Dashboard ticket activity counts tickets on both drivers (the windowed counts
// used MySQL-only DATE_SUB, which failed on PostgreSQL and was swallowed as 0).
func TestAdminDashboardTicketActivityCountsTickets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := getTestDB(t)
	if valkeyCache != nil {
		t.Fatal("valkeyCache must be nil so activity is computed from the database")
	}

	before, err := getTicketActivityFromCache(nil, db)
	require.NoError(t, err)

	createStateTicket(t, db, "new", "dash-activity@example.com", "Dashboard activity", 0)
	createStateTicket(t, db, "closed successful", "dash-activity@example.com", "Dashboard activity closed", 0)

	after, err := getTicketActivityFromCache(nil, db)
	require.NoError(t, err)
	for _, key := range []string{"created_day", "created_week", "created_month"} {
		assert.Equal(t, before[key]+2, after[key], key)
	}
	for _, key := range []string{"closed_day", "closed_week", "closed_month"} {
		assert.Equal(t, before[key]+1, after[key], key)
	}
	assert.Equal(t, before["open"]+1, after["open"], "open")
}

// The admin dashboard renders counts read from the database.
func TestAdminDashboardRendersDatabaseCounts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupTemplateRenderer(t)
	db := getTestDB(t)

	var users int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders("SELECT COUNT(*) FROM users WHERE valid_id = 1")).Scan(&users))

	w := serveAdminGET(t, "/admin", "/admin", handleAdminDashboard)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), fmt.Sprintf(`<p class="gk-stat-value mt-2">%d</p>`, users))
}

// Setup task forms list options read from the database.
func TestAdminSetupTaskFormsUseDatabase(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupTemplateRenderer(t)
	db := getTestDB(t)

	sfx := letterSuffix()
	queueName := "SetupFormQueue" + sfx
	queueID := createTestQueue(t, db, queueName)
	t.Cleanup(func() { cleanupTestQueue(t, db, queueID) })

	companyID := "setupform" + sfx
	companyName := "SetupFormCo" + sfx
	_, err := db.Exec(database.ConvertPlaceholders(`INSERT INTO customer_company
		(customer_id, name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, 1, NOW(), 1, NOW(), 1)`), companyID, companyName)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders("DELETE FROM customer_company WHERE customer_id = ?"), companyID)
	})

	slaName := "SetupFormSLA" + sfx
	slaID, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`INSERT INTO sla
		(name, first_response_time, update_time, solution_time, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, 0, 0, 0, 1, NOW(), 1, NOW(), 1) RETURNING id`), slaName)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = db.Exec(database.ConvertPlaceholders("DELETE FROM sla WHERE id = ?"), slaID) })

	const pattern = "/admin/setup/task/:plugin/:task_id"
	w := serveAdminGET(t, "/admin/setup/task/setup-assistant/create_customer", pattern, handleAdminSetupTask)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := w.Body.String()
	assert.Contains(t, body, fmt.Sprintf(`<option value="%d">%s</option>`, queueID, queueName))
	assert.Contains(t, body, fmt.Sprintf(`<option value="%d">%s</option>`, slaID, slaName))
	assert.Contains(t, body, fmt.Sprintf(`{"customer_id": "%s", "name": "%s"}`, companyID, companyName))

	var agentID int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		"SELECT MIN(id) FROM users WHERE valid_id = 1")).Scan(&agentID))
	w = serveAdminGET(t, "/admin/setup/task/setup-assistant/assign_agent_group", pattern, handleAdminSetupTask)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), fmt.Sprintf(`{"id": %d, "login": "`, agentID))
}

// letterSuffix is a unique letters-only suffix: names rendered through escapejs
// (which escapes digits) stay searchable verbatim.
func letterSuffix() string {
	digits := fmt.Sprint(time.Now().UnixNano())
	out := make([]byte, len(digits))
	for i := range digits {
		out[i] = 'a' + (digits[i] - '0')
	}
	return string(out)
}
