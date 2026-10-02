package api

import (
	"context"
	"database/sql"
	"encoding/csv"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/lookups"
	"github.com/goatkit/goatflow/internal/platform/middleware"
	"github.com/goatkit/goatflow/internal/platform/routing"
	"github.com/goatkit/goatflow/internal/platform/shared"
)

// portalPagesRouter is the production stack (security headers + YAML routes)
// with the real template renderer installed.
func portalPagesRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	renderer, err := shared.NewTemplateRenderer("../../templates")
	require.NoError(t, err)
	previous := shared.GetGlobalRenderer()
	shared.SetGlobalRenderer(renderer)
	t.Cleanup(func() { shared.SetGlobalRenderer(previous) })

	r := gin.New()
	r.Use(middleware.SecurityHeaders())
	require.NoError(t, routing.LoadYAMLRoutesForTesting(r))
	return r
}

func portalValidID(t *testing.T, db *sql.DB, name string) int {
	t.Helper()
	id, err := lookups.ID(context.Background(), db, lookups.ValidLookup, name)
	require.NoError(t, err)
	return id
}

func portalInsertCompany(t *testing.T, db *sql.DB, customerID, name, url string, validID int) {
	t.Helper()
	_, err := db.Exec(database.ConvertPlaceholders(`
		INSERT INTO customer_company (customer_id, name, street, zip, city, country, url, comments, valid_id,
			create_time, create_by, change_time, change_by)
		VALUES (?, ?, 'Main Street 1', '12345', 'Springfield', 'Utopia', ?, 'agents only note', ?, NOW(), 1, NOW(), 1)`),
		customerID, name, url, validID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM customer_company WHERE customer_id = ?`), customerID)
	})
}

func portalInsertCustomerUser(t *testing.T, db *sql.DB, login, customerID, first, last string, validID int) int64 {
	t.Helper()
	id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO customer_user (login, email, customer_id, first_name, last_name, valid_id,
			create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, ?, ?, ?, NOW(), 1, NOW(), 1) RETURNING id`), login, login, customerID, first, last, validID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM customer_user WHERE id = ?`), id)
	})
	return id
}

func portalGet(t *testing.T, r http.Handler, url, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Accept", "text/html")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func portalCustomerToken(t *testing.T, id int64, login string) string {
	t.Helper()
	return testSessionToken(t, uint(id), login, login, "Customer", false, 0)
}

func TestCustomerCompanyPagesShowOnlyOwnCompany(t *testing.T) {
	db := getTestDB(t)
	r := portalPagesRouter(t)
	valid := portalValidID(t, db, "valid")
	invalid := portalValidID(t, db, "invalid")

	sfx := strconv.FormatInt(time.Now().UnixNano(), 36)
	coA, coB := "pcA-"+sfx, "pcB-"+sfx
	portalInsertCompany(t, db, coA, "Alpha Widgets "+sfx, "https://alpha.example.com", valid)
	portalInsertCompany(t, db, coB, "Beta Gadgets "+sfx, "javascript:alert(1)", valid)

	alice := "alice-" + sfx + "@alpha.example.com"
	aliceID := portalInsertCustomerUser(t, db, alice, coA, "Alice", "Anders", valid)
	bob := "bob-" + sfx + "@alpha.example.com"
	portalInsertCustomerUser(t, db, bob, coA, "Bob", "Brown", valid)
	gone := "gone-" + sfx + "@alpha.example.com"
	portalInsertCustomerUser(t, db, gone, coA, "Gone", "Away", invalid)
	carol := "carol-" + sfx + "@beta.example.com"
	carolID := portalInsertCustomerUser(t, db, carol, coB, "Carol", "Clark", valid)

	aliceToken := portalCustomerToken(t, aliceID, alice)
	carolToken := portalCustomerToken(t, carolID, carol)

	t.Run("company info shows own company only", func(t *testing.T) {
		w := portalGet(t, r, "/customer/company", aliceToken)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		body := w.Body.String()
		assert.Contains(t, body, "Alpha Widgets "+sfx)
		assert.Contains(t, body, `id="company-customer-id">`+coA+`</span>`)
		assert.Contains(t, body, `href="https://alpha.example.com"`)
		assert.Contains(t, body, "Main Street 1")
		assert.Contains(t, body, "View users (2)", "only the two valid users count")
		assert.NotContains(t, body, "Beta Gadgets")
		assert.NotContains(t, body, "agents only note", "company comments are agent-internal")
		assert.NotContains(t, strings.ToLower(body), "under construction")
		assert.Contains(t, body, `href="/customer/company"`, "nav links to the company page")
	})

	t.Run("company users lists valid colleagues only", func(t *testing.T) {
		w := portalGet(t, r, "/customer/company/users", aliceToken)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		body := w.Body.String()
		assert.Contains(t, body, alice)
		assert.Contains(t, body, bob)
		assert.Contains(t, body, "Bob Brown")
		assert.NotContains(t, body, gone, "invalid users are hidden")
		assert.NotContains(t, body, carol, "other companies' users are hidden")
	})

	t.Run("other company sees its own data", func(t *testing.T) {
		w := portalGet(t, r, "/customer/company/users", carolToken)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		body := w.Body.String()
		assert.Contains(t, body, carol)
		assert.NotContains(t, body, alice)
		assert.NotContains(t, body, bob)

		w = portalGet(t, r, "/customer/company", carolToken)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), "Beta Gadgets "+sfx)
		assert.NotContains(t, w.Body.String(), "javascript:alert", "non-http URLs are not rendered")
		assert.NotContains(t, w.Body.String(), "Alpha Widgets")
	})

	t.Run("invalid company is not shown", func(t *testing.T) {
		_, err := db.Exec(database.ConvertPlaceholders(`UPDATE customer_company SET valid_id = ? WHERE customer_id = ?`), invalid, coB)
		require.NoError(t, err)
		for _, url := range []string{"/customer/company", "/customer/company/users"} {
			w := portalGet(t, r, url, carolToken)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			assert.Contains(t, w.Body.String(), `id="company-none"`)
			assert.NotContains(t, w.Body.String(), "Beta Gadgets")
			assert.NotContains(t, w.Body.String(), carol+"</a>")
		}
	})

	t.Run("requires a customer login", func(t *testing.T) {
		w := portalGet(t, r, "/customer/company", "")
		assert.Equal(t, http.StatusFound, w.Code)
		assert.Equal(t, "/customer/login", w.Header().Get("Location"))
	})
}

func TestCustomerDashboardKnowledgeBaseCardFollowsPlugin(t *testing.T) {
	db := getTestDB(t)
	r := portalPagesRouter(t)
	valid := portalValidID(t, db, "valid")
	sfx := strconv.FormatInt(time.Now().UnixNano(), 36)
	login := "kbcard-" + sfx + "@example.com"
	id := portalInsertCustomerUser(t, db, login, "kbcard-"+sfx, "Kay", "Bee", valid)
	token := portalCustomerToken(t, id, login)

	w := portalGet(t, r, "/customer", token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.NotContains(t, w.Body.String(), `href="/customer/kb"`, "no KB link without the goat-kb plugin")

	shared.SetPluginMenuProvider(func(location string) []map[string]any {
		if location != "customer" {
			return nil
		}
		return []map[string]any{{"Label": "Knowledge Base", "Path": "/customer/kb", "Icon": "fa-book"}}
	})
	t.Cleanup(func() { shared.SetPluginMenuProvider(nil) })

	w = portalGet(t, r, "/customer", token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `<a href="/customer/kb" class="block group">`, "dashboard card shown when the plugin registers its menu item")
}

func TestAdminReportsPageAndExport(t *testing.T) {
	db := getTestDB(t)
	r := portalPagesRouter(t)
	token := GetTestAuthToken(t)

	get := func(url string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, url, nil)
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	w := get("/admin/reports")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := w.Body.String()
	assert.Contains(t, body, `id="reports-page"`)
	assert.Contains(t, body, "Reports &amp; Analytics")
	assert.Contains(t, body, `/api/v1/statistics/export?type=tickets&format=csv&period=7d`)
	assert.NotContains(t, strings.ToLower(body), "under construction")

	// The page's export link works with the same browser session and is
	// scoped to readable queues: a fresh ticket in a seeded queue is listed.
	tn := "RPT" + strconv.FormatInt(time.Now().UnixNano(), 10)
	ticketID := custAttInsertTicket(t, db, tn, "reports-"+tn+"@example.com")
	require.Positive(t, ticketID)

	w = get("/api/v1/statistics/export?type=tickets&format=csv&period=24h")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "text/csv", w.Header().Get("Content-Type"))
	records, err := csv.NewReader(strings.NewReader(w.Body.String())).ReadAll()
	require.NoError(t, err)
	require.NotEmpty(t, records)
	assert.Equal(t, []string{"Ticket Number", "Title", "Queue", "State", "Priority", "Customer", "Created"}, records[0])
	found := false
	for _, rec := range records[1:] {
		if rec[0] == tn {
			found = true
			assert.Equal(t, "reports-"+tn+"@example.com", rec[5])
		}
	}
	assert.True(t, found, "export lists the new ticket %s", tn)
}

func TestRemovedAdminPlaceholderRoutes(t *testing.T) {
	r := portalPagesRouter(t)
	token := GetTestAuthToken(t)
	for _, url := range []string{"/admin/modules", "/admin/groups/1/edit"} {
		req := httptest.NewRequest(http.MethodGet, url, nil)
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code, url)
	}
}
