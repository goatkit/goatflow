package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// companyPagesFixture is a customer company with customer users, tickets and
// services, owned by one test and deleted when it ends.
type companyPagesFixture struct {
	db           *sql.DB
	company      string // customer_id
	otherCompany string
	user1, user2 int    // customer_user ids in company
	login1       string // customer_user logins
	login2       string
	otherUserID  int
	otherLogin   string
	service1     int // valid
	service2     int // valid
	invalidSvc   int // valid_id = 2
	visibleQueue int
	hiddenQueue  int
	visibleGroup int
	oldTicketTN  string // company, visible queue, new, 10 days old
	newTicketTN  string // company, hidden queue, closed, 2 hours old
	otherTN      string // other company
}

func newCompanyPagesFixture(t *testing.T) *companyPagesFixture {
	t.Helper()
	db := getTestDB(t)
	sfx := time.Now().UnixNano() % 1_000_000_000
	f := &companyPagesFixture{
		db:           db,
		company:      fmt.Sprintf("CCP%d", sfx),
		otherCompany: fmt.Sprintf("CCPO%d", sfx),
	}
	exec := func(q string, args ...any) {
		t.Helper()
		_, err := db.Exec(database.ConvertPlaceholders(q), args...)
		require.NoError(t, err, q)
	}
	idOf := func(id int64, err error) int {
		t.Helper()
		require.NoError(t, err)
		return int(id)
	}

	for _, cid := range []string{f.company, f.otherCompany} {
		exec(`INSERT INTO customer_company (customer_id, name, valid_id, create_time, create_by, change_time, change_by)
			VALUES (?, ?, 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)`, cid, "Company "+cid)
	}
	newCustomerUser := func(login, company, first, last string, validID int) int {
		return idOf(database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
			INSERT INTO customer_user (login, email, customer_id, first_name, last_name, phone, valid_id,
				create_time, create_by, change_time, change_by)
			VALUES (?, ?, ?, ?, ?, '+44 1', ?, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1) RETURNING id`),
			login, login+"@example.test", company, first, last, validID))
	}
	f.login1 = fmt.Sprintf("ccp-alice-%d", sfx)
	f.login2 = fmt.Sprintf("ccp-bob-%d", sfx)
	f.otherLogin = fmt.Sprintf("ccp-zed-%d", sfx)
	f.user1 = newCustomerUser(f.login1, f.company, "Alice", "Anders", 1)
	f.user2 = newCustomerUser(f.login2, f.company, "Bob", "Brown", 2)
	f.otherUserID = newCustomerUser(f.otherLogin, f.otherCompany, "Zed", "Zulu", 1)

	newService := func(name string, validID int) int {
		return idOf(database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
			INSERT INTO service (name, valid_id, comments, create_time, create_by, change_time, change_by)
			VALUES (?, ?, 'ccp test service', CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1) RETURNING id`), name, validID))
	}
	f.service1 = newService(fmt.Sprintf("CCP Email %d", sfx), 1)
	f.service2 = newService(fmt.Sprintf("CCP Phone %d", sfx), 1)
	f.invalidSvc = newService(fmt.Sprintf("CCP Retired %d", sfx), 2)

	f.visibleGroup, _ = createIsolatedGroup(t, "ccp")
	f.visibleQueue, _ = createIsolatedQueue(t, "ccp_visible")
	exec(`UPDATE queue SET group_id = ? WHERE id = ?`, f.visibleGroup, f.visibleQueue)
	f.hiddenQueue, _ = createIsolatedQueue(t, "ccp_hidden")

	newTicket := func(tn, title, company, login string, queueID int, state string, created time.Time) {
		exec(`INSERT INTO ticket (tn, title, queue_id, type_id, ticket_state_id, ticket_priority_id, ticket_lock_id,
				user_id, responsible_user_id, customer_id, customer_user_id,
				timeout, until_time, escalation_time, escalation_update_time, escalation_response_time, escalation_solution_time,
				create_time, create_by, change_time, change_by)
			VALUES (?, ?, ?,
				(SELECT MIN(id) FROM ticket_type WHERE valid_id = 1),
				(SELECT id FROM ticket_state WHERE name = ?),
				(SELECT id FROM ticket_priority WHERE name = '3 normal'),
				(SELECT id FROM ticket_lock_type WHERE name = 'unlock'),
				1, 1, ?, ?, 0, 0, 0, 0, 0, 0, ?, 1, ?, 1)`,
			tn, title, queueID, state, company, login, created, created)
	}
	now := time.Now()
	f.oldTicketTN = fmt.Sprintf("CCP%dA", sfx)
	f.newTicketTN = fmt.Sprintf("CCP%dB", sfx)
	f.otherTN = fmt.Sprintf("CCP%dC", sfx)
	newTicket(f.oldTicketTN, "Old visible ticket", f.company, f.login1, f.visibleQueue, "new", now.Add(-10*24*time.Hour))
	newTicket(f.newTicketTN, "Recent hidden ticket", f.company, f.login2, f.hiddenQueue, "closed successful", now.Add(-2*time.Hour))
	newTicket(f.otherTN, "Other company ticket", f.otherCompany, f.otherLogin, f.visibleQueue, "new", now.Add(-time.Hour))

	t.Cleanup(func() {
		logins := []any{f.login1, f.login2, f.otherLogin}
		for _, q := range []string{
			`DELETE FROM service_customer_user WHERE customer_user_login IN (?, ?, ?)`,
			`DELETE FROM ticket_history WHERE ticket_id IN (SELECT id FROM ticket WHERE customer_user_id IN (?, ?, ?))`,
			`DELETE FROM ticket WHERE customer_user_id IN (?, ?, ?)`,
			`DELETE FROM customer_user WHERE login IN (?, ?, ?)`,
		} {
			_, _ = db.Exec(database.ConvertPlaceholders(q), logins...)
		}
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM service WHERE id IN (?, ?, ?)`), f.service1, f.service2, f.invalidSvc)
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM customer_company WHERE customer_id IN (?, ?)`), f.company, f.otherCompany)
	})
	return f
}

func (f *companyPagesFixture) assign(t *testing.T, login string, serviceID int) {
	t.Helper()
	_, err := f.db.Exec(database.ConvertPlaceholders(`
		INSERT INTO service_customer_user (customer_user_login, service_id, create_time, create_by)
		VALUES (?, ?, CURRENT_TIMESTAMP, 1)`), login, serviceID)
	require.NoError(t, err)
}

// assignments returns "login/service" pairs for the fixture's customer users.
func (f *companyPagesFixture) assignments(t *testing.T) []string {
	t.Helper()
	rows, err := f.db.Query(database.ConvertPlaceholders(`
		SELECT customer_user_login, service_id FROM service_customer_user WHERE customer_user_login IN (?, ?, ?)`),
		f.login1, f.login2, f.otherLogin)
	require.NoError(t, err)
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var login string
		var sid int
		require.NoError(t, rows.Scan(&login, &sid))
		out = append(out, fmt.Sprintf("%s/%d", login, sid))
	}
	require.NoError(t, rows.Err())
	sort.Strings(out)
	return out
}

func pair(login string, serviceID int) string { return fmt.Sprintf("%s/%d", login, serviceID) }

func sortedStrings(s ...string) []string { sort.Strings(s); return s }

func companyPageGet(t *testing.T, router *gin.Engine, token, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	AddTestAuthCookie(req, token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// rowFor returns the <tr ...> element whose opening tag contains marker.
func rowFor(t *testing.T, body, marker string) string {
	t.Helper()
	i := strings.Index(body, marker)
	require.GreaterOrEqual(t, i, 0, "row %q not rendered", marker)
	end := strings.Index(body[i:], "</tr>")
	require.Greater(t, end, 0)
	return body[i : i+end]
}

func TestAdminCustomerCompanyUsersPage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newCompanyPagesFixture(t)
	router := NewSimpleRouterWithDB(f.db)
	token := GetTestAuthToken(t)

	w := companyPageGet(t, router, token, "/admin/customer/companies/"+f.company+"/users")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := w.Body.String()
	assert.NotContains(t, body, "Under Construction")
	assert.Contains(t, body, "Company "+f.company)

	alice := rowFor(t, body, `data-login="`+f.login1+`"`)
	assert.Contains(t, alice, "Alice Anders")
	assert.Contains(t, alice, f.login1+"@example.test")
	assert.Contains(t, alice, "/admin/customer-users?search="+url.QueryEscape(f.login1))
	assert.Contains(t, alice, ">1</td>", "alice has one ticket")
	assert.Contains(t, alice, "gk-badge-success")

	bob := rowFor(t, body, `data-login="`+f.login2+`"`)
	assert.Contains(t, bob, "gk-badge-error", "invalid customer user is flagged")
	assert.Less(t, strings.Index(body, f.login1), strings.Index(body, f.login2), "ordered by last name")
	assert.NotContains(t, body, f.otherLogin, "users of other companies are not listed")

	t.Run("unknown company is 404", func(t *testing.T) {
		w := companyPageGet(t, router, token, "/admin/customer/companies/NO-SUCH-"+f.company+"/users")
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestAdminCustomerCompanyTicketsPage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newCompanyPagesFixture(t)
	router := NewSimpleRouterWithDB(f.db)

	t.Run("admin group member sees every queue, newest first", func(t *testing.T) {
		w := companyPageGet(t, router, GetTestAuthToken(t), "/admin/customer/companies/"+f.company+"/tickets")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		body := w.Body.String()
		assert.NotContains(t, body, "Under Construction")

		recent := rowFor(t, body, `data-tn="`+f.newTicketTN+`"`)
		old := rowFor(t, body, `data-tn="`+f.oldTicketTN+`"`)
		assert.Less(t, strings.Index(body, f.newTicketTN), strings.Index(body, f.oldTicketTN))
		assert.Contains(t, recent, `href="/ticket/`+f.newTicketTN+`"`)
		assert.Contains(t, recent, "Recent hidden ticket")
		assert.Contains(t, recent, "closed successful")
		assert.Contains(t, recent, f.login2)
		assert.Contains(t, recent, "hours")
		assert.Contains(t, old, "ccp_visible_")
		assert.Contains(t, old, ">new</span>")
		assert.Contains(t, old, "days")
		assert.NotContains(t, body, f.otherTN, "other company's tickets are not listed")
		assert.Contains(t, body, `id="company-ticket-total" style="color: var(--gk-text-primary);">2<`)
	})

	t.Run("admin outside the admin group sees only readable queues", func(t *testing.T) {
		agentID, login := createIsolatedAgent(t, "ccp_admin")
		_, err := f.db.Exec(database.ConvertPlaceholders(`
			INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
			VALUES (?, ?, 'ro', CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)`), agentID, f.visibleGroup)
		require.NoError(t, err)
		// An admin-scoped credential for an agent who is not in the admin group.
		token := testSessionToken(t, uint(agentID), login, login, "Admin", true, 0)

		w := companyPageGet(t, router, token, "/admin/customer/companies/"+f.company+"/tickets")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		body := w.Body.String()
		assert.Contains(t, body, `data-tn="`+f.oldTicketTN+`"`)
		assert.NotContains(t, body, f.newTicketTN, "ticket in a queue the admin can't read is hidden")
	})

	t.Run("unknown company is 404", func(t *testing.T) {
		w := companyPageGet(t, router, GetTestAuthToken(t), "/admin/customer/companies/NO-SUCH-"+f.company+"/tickets")
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestAdminCustomerCompanyServicesPage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newCompanyPagesFixture(t)
	router := NewSimpleRouterWithDB(f.db)
	token := GetTestAuthToken(t)

	f.assign(t, f.login2, f.service1)
	f.assign(t, f.login1, f.invalidSvc)
	f.assign(t, f.otherLogin, f.service1)

	post := func(t *testing.T, values ...string) *httptest.ResponseRecorder {
		t.Helper()
		form := url.Values{"assign": values}
		req := httptest.NewRequest(http.MethodPost, "/admin/customer/companies/"+f.company+"/services",
			strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	box := func(serviceID, userID int) string { return fmt.Sprintf(`value="%d:%d"`, serviceID, userID) }

	t.Run("page shows valid services with each company user's assignment", func(t *testing.T) {
		w := companyPageGet(t, router, token, "/admin/customer/companies/"+f.company+"/services")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		body := w.Body.String()
		assert.NotContains(t, body, "Under Construction")
		assert.Contains(t, body, `action="/admin/customer/companies/`+f.company+`/services"`)

		s1 := rowFor(t, body, fmt.Sprintf(`data-service-id="%d"`, f.service1))
		assert.Contains(t, s1, "1 / 2")
		assert.Regexp(t, box(f.service1, f.user2)+`[^>]*checked`, s1)
		assert.NotRegexp(t, box(f.service1, f.user1)+`[^>]*checked`, s1)
		assert.NotContains(t, s1, f.otherLogin)
		assert.NotContains(t, body, fmt.Sprintf(`data-service-id="%d"`, f.invalidSvc), "invalid services are not offered")
		assert.NotContains(t, body, `id="services-saved"`)
	})

	t.Run("saving replaces the company users' valid-service assignments", func(t *testing.T) {
		w := post(t, fmt.Sprintf("%d:%d", f.service1, f.user1), fmt.Sprintf("%d:%d", f.service2, f.user1),
			fmt.Sprintf("%d:%d", f.service2, f.user2))
		require.Equal(t, http.StatusSeeOther, w.Code, w.Body.String())
		assert.Equal(t, "/admin/customer/companies/"+f.company+"/services?saved=1", w.Header().Get("Location"))

		assert.Equal(t, sortedStrings(
			pair(f.login1, f.service1), pair(f.login1, f.service2),
			pair(f.login1, f.invalidSvc),   // assignments of invalid services are kept
			pair(f.login2, f.service2),     // service1 unassigned from bob
			pair(f.otherLogin, f.service1), // other company untouched
		), f.assignments(t))

		w = companyPageGet(t, router, token, "/admin/customer/companies/"+f.company+"/services?saved=1")
		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `id="services-saved"`)
	})

	t.Run("an empty submission unassigns every valid service", func(t *testing.T) {
		w := post(t)
		require.Equal(t, http.StatusSeeOther, w.Code, w.Body.String())
		assert.Equal(t, sortedStrings(pair(f.login1, f.invalidSvc), pair(f.otherLogin, f.service1)), f.assignments(t))
	})

	t.Run("rejects pairs outside the company or for invalid services", func(t *testing.T) {
		before := f.assignments(t)
		for name, value := range map[string]string{
			"other company's user": fmt.Sprintf("%d:%d", f.service1, f.otherUserID),
			"invalid service":      fmt.Sprintf("%d:%d", f.invalidSvc, f.user1),
			"malformed":            "x:y",
		} {
			w := post(t, fmt.Sprintf("%d:%d", f.service2, f.user2), value)
			assert.Equal(t, http.StatusBadRequest, w.Code, name)
		}
		assert.Equal(t, before, f.assignments(t), "nothing changes on a rejected submission")
	})

	t.Run("unknown company is 404", func(t *testing.T) {
		w := companyPageGet(t, router, token, "/admin/customer/companies/NO-SUCH-"+f.company+"/services")
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}
