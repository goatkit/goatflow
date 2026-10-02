package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
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
	"github.com/goatkit/goatflow/internal/platform/lookups"
)

// searchFixture: queueA readable by agentA and agentAll, queueB by agentAll only.
//
//	tickets[0] queueA open,   title "printer jam <sfx>", customer user <cu> of company <co>,
//	           article subject "toner", body "PC LOAD LETTER <sfx>", from alice@<sfx>.test
//	tickets[1] queueA closed, title "vpn down <sfx>", created 10 days ago
//	tickets[2] queueB open,   title "printer in queue b <sfx>"
type searchFixture struct {
	db                        *sql.DB
	sfx                       string
	queueA, queueB            int
	prioNormal, prioHigh      int
	agentA, agentAll, agentNo int
	loginA, loginAll, loginNo string
	customerUser, company     string
	firstName, companyName    string
	tickets                   []int64
	now                       time.Time
}

func newSearchFixture(t *testing.T) *searchFixture {
	t.Helper()
	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)

	sfx := fmt.Sprintf("%d", time.Now().UnixNano())
	f := &searchFixture{db: db, sfx: sfx, now: time.Now().UTC().Truncate(time.Second)}
	f.customerUser, f.company = "srchcu"+sfx, "srchco"+sfx
	f.firstName, f.companyName = "Zebulon"+sfx, "Acme"+sfx

	var groupIDs, queueIDs, userIDs []int
	t.Cleanup(func() {
		exec := func(q string, args ...interface{}) {
			if _, err := db.Exec(database.ConvertPlaceholders(q), args...); err != nil {
				t.Errorf("cleanup %q: %v", q, err)
			}
		}
		for _, l := range []string{f.loginA, f.loginAll, f.loginNo} {
			exec("DELETE FROM search_profile WHERE login = ?", ticketSearchProfileBase+"::"+l)
		}
		for _, id := range f.tickets {
			exec("DELETE FROM article_data_mime WHERE article_id IN (SELECT id FROM article WHERE ticket_id = ?)", id)
			exec("DELETE FROM article WHERE ticket_id = ?", id)
			exec("DELETE FROM ticket WHERE id = ?", id)
		}
		exec("DELETE FROM customer_user WHERE login = ?", f.customerUser)
		exec("DELETE FROM customer_company WHERE customer_id = ?", f.company)
		for _, id := range userIDs {
			exec("DELETE FROM group_user WHERE user_id = ?", id)
		}
		for _, id := range queueIDs {
			exec("DELETE FROM queue WHERE id = ?", id)
		}
		for _, id := range groupIDs {
			exec("DELETE FROM `groups` WHERE id = ?", id)
		}
		for _, id := range userIDs {
			exec("DELETE FROM users WHERE id = ?", id)
		}
	})

	mustID := func(id int64, err error) int64 {
		t.Helper()
		require.NoError(t, err)
		return id
	}
	adapter := database.GetAdapter()
	exec := func(q string, args ...interface{}) {
		t.Helper()
		_, err := db.Exec(database.ConvertPlaceholders(q), args...)
		require.NoError(t, err, q)
	}
	newGroup := func(name string) int {
		id := int(mustID(adapter.InsertWithReturning(db, database.ConvertPlaceholders("INSERT INTO `groups` (name, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (?, 'search test', 1, ?, 1, ?, 1) RETURNING id"), name, f.now, f.now)))
		groupIDs = append(groupIDs, id)
		return id
	}
	newQueue := func(name string, groupID int) int {
		id := int(mustID(adapter.InsertWithReturning(db, database.ConvertPlaceholders(`INSERT INTO queue (name, group_id, system_address_id, salutation_id, signature_id,
				follow_up_id, follow_up_lock, comments, valid_id, create_time, create_by, change_time, change_by)
			VALUES (?, ?, 1, 1, 1, 1, 0, 'search test', 1, ?, 1, ?, 1) RETURNING id`), name, groupID, f.now, f.now)))
		queueIDs = append(queueIDs, id)
		return id
	}
	newAgent := func(login string) int {
		id := int(mustID(adapter.InsertWithReturning(db, database.ConvertPlaceholders(`INSERT INTO users (login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
			VALUES (?, 'x', 'Search', 'Agent', 1, ?, 1, ?, 1) RETURNING id`), login, f.now, f.now)))
		userIDs = append(userIDs, id)
		return id
	}
	grant := func(userID, groupID int, key string) {
		exec(`INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
			VALUES (?, ?, ?, ?, 1, ?, 1)`, userID, groupID, key, f.now, f.now)
	}

	groupA, groupB := newGroup("srch_a_"+sfx), newGroup("srch_b_"+sfx)
	f.queueA, f.queueB = newQueue("srch_qa_"+sfx, groupA), newQueue("srch_qb_"+sfx, groupB)
	f.loginA, f.loginAll, f.loginNo = "srch_a_"+sfx, "srch_all_"+sfx, "srch_no_"+sfx
	f.agentA, f.agentAll, f.agentNo = newAgent(f.loginA), newAgent(f.loginAll), newAgent(f.loginNo)
	grant(f.agentA, groupA, "ro")
	grant(f.agentAll, groupA, "rw")
	grant(f.agentAll, groupB, "ro")

	exec(`INSERT INTO customer_company (customer_id, name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, 1, ?, 1, ?, 1)`, f.company, f.companyName, f.now, f.now)
	exec(`INSERT INTO customer_user (login, email, customer_id, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, ?, 'Customer', 1, ?, 1, ?, 1)`, f.customerUser, f.customerUser+"@example.test", f.company, f.firstName, f.now, f.now)

	lookupID := func(table lookups.Table, name string) int {
		t.Helper()
		id, err := lookups.ID(context.Background(), db, table, name)
		require.NoError(t, err, "%s %q", table, name)
		return id
	}
	unlock := lookupID(lookups.LockType, lookups.LockUnlock)
	f.prioNormal = lookupID(lookups.PriorityTable, "3 normal")
	f.prioHigh = lookupID(lookups.PriorityTable, "5 very high")
	newTicket := func(i, queueID int, title, state string, priority int, created time.Time) int64 {
		id := mustID(adapter.InsertWithReturning(db, database.ConvertPlaceholders(`INSERT INTO ticket (tn, title, queue_id, ticket_lock_id, user_id, responsible_user_id,
				ticket_priority_id, ticket_state_id, customer_id, customer_user_id, timeout, until_time,
				escalation_time, escalation_update_time, escalation_response_time, escalation_solution_time,
				archive_flag, create_time, create_by, change_time, change_by)
			VALUES (?, ?, ?, ?, 1, 1, ?, ?, ?, ?, 0, 0, 0, 0, 0, 0, 0, ?, 1, ?, 1) RETURNING id`),
			fmt.Sprintf("SR%s%d", sfx[len(sfx)-9:], i), title, queueID, unlock, priority,
			lookupID(lookups.StateLookup, state), f.company, f.customerUser, created, created))
		f.tickets = append(f.tickets, id)
		return id
	}
	sender, channel := lookupID(lookups.SenderType, lookups.SenderCustomer), lookupID(lookups.Channel, lookups.ChannelEmail)
	newArticle := func(ticketID int64, from, subject, body string) {
		articleID := mustID(adapter.InsertWithReturning(db, database.ConvertPlaceholders(`INSERT INTO article (ticket_id, article_sender_type_id, communication_channel_id, is_visible_for_customer,
				create_time, create_by, change_time, change_by)
			VALUES (?, ?, ?, 1, ?, 1, ?, 1) RETURNING id`), ticketID, sender, channel, f.now, f.now))
		exec(`INSERT INTO article_data_mime (article_id, a_from, a_to, a_subject, a_body, incoming_time, create_time, create_by, change_time, change_by)
			VALUES (?, ?, 'support@example.test', ?, ?, 0, ?, 1, ?, 1)`, articleID, from, subject, body, f.now, f.now)
	}

	t0 := newTicket(0, f.queueA, "printer jam "+sfx, lookups.StateOpen, f.prioNormal, f.now.Add(-48*time.Hour))
	newArticle(t0, "alice@"+sfx+".test", "toner", "The display says PC LOAD LETTER "+sfx)
	t1 := newTicket(1, f.queueA, "vpn down "+sfx, lookups.StateClosedSuccessful, f.prioHigh, f.now.Add(-240*time.Hour))
	newArticle(t1, "bob@example.test", "tunnel", "no route to host")
	newTicket(2, f.queueB, "printer in queue b "+sfx, lookups.StateOpen, f.prioNormal, f.now.Add(-24*time.Hour))
	return f
}

func searchRequest(t *testing.T, router http.Handler, method, path, token string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

type searchResult struct {
	Success bool `json:"success"`
	Data    struct {
		Tickets []struct {
			ID        int64  `json:"id"`
			Title     string `json:"title"`
			QueueID   int    `json:"queue_id"`
			StateType string `json:"state_type"`
		} `json:"tickets"`
		Pagination struct {
			Page  int `json:"page"`
			Limit int `json:"limit"`
			Total int `json:"total"`
		} `json:"pagination"`
		Name              string                 `json:"name"`
		Parameters        map[string]interface{} `json:"parameters"`
		IgnoredParameters []string               `json:"ignored_parameters"`
	} `json:"data"`
}

func (r searchResult) ids() []int64 {
	out := []int64{}
	for _, tk := range r.Data.Tickets {
		out = append(out, tk.ID)
	}
	return out
}

func searchTickets(t *testing.T, router http.Handler, token string, params url.Values) searchResult {
	t.Helper()
	w := searchRequest(t, router, http.MethodGet, "/api/v1/search/tickets?"+params.Encode(), token, nil)
	require.Equal(t, http.StatusOK, w.Code, "%s: %s", params.Encode(), w.Body.String())
	var res searchResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res), w.Body.String())
	require.True(t, res.Success)
	return res
}

func sortedIDs(ids ...int64) []int64 {
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func TestTicketSearchAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newSearchFixture(t)
	router := NewSimpleRouter()
	tokenA := statsToken(t, f.agentA, f.loginA, "Agent")
	tokenAll := statsToken(t, f.agentAll, f.loginAll, "Agent")
	t0, t1, t2 := f.tickets[0], f.tickets[1], f.tickets[2]

	t.Run("access", func(t *testing.T) {
		path := "/api/v1/search/tickets?q=" + f.sfx
		assert.Equal(t, http.StatusUnauthorized, searchRequest(t, router, http.MethodGet, path, "", nil).Code)
		customer := statsToken(t, f.agentAll, f.customerUser, "Customer")
		assert.Equal(t, http.StatusForbidden, searchRequest(t, router, http.MethodGet, path, customer, nil).Code)
		noQueue := statsToken(t, f.agentNo, f.loginNo, "Agent")
		assert.Equal(t, http.StatusForbidden, searchRequest(t, router, http.MethodGet, path, noQueue, nil).Code)
	})

	t.Run("fulltext terms must all match and results stay in readable queues", func(t *testing.T) {
		q := url.Values{"q": {"printer " + f.sfx}}
		assert.Equal(t, []int64{t0}, searchTickets(t, router, tokenA, q).ids())
		assert.Equal(t, sortedIDs(t0, t2), sortedIDs(searchTickets(t, router, tokenAll, q).ids()...))
	})

	t.Run("fulltext matches article fields, customer user name and company name", func(t *testing.T) {
		for _, q := range []string{"pc load letter " + f.sfx, "alice@" + f.sfx} {
			assert.Equal(t, []int64{t0}, searchTickets(t, router, tokenA, url.Values{"q": {q}}).ids(), q)
		}
		// Both queueA tickets belong to the fixture customer user and company.
		for _, q := range []string{strings.ToLower(f.firstName), f.companyName} {
			assert.Equal(t, sortedIDs(t0, t1), sortedIDs(searchTickets(t, router, tokenA, url.Values{"q": {q}}).ids()...), q)
		}
	})

	t.Run("field filters", func(t *testing.T) {
		assert.Equal(t, []int64{t0}, searchTickets(t, router, tokenA, url.Values{"body": {"LOAD LETTER " + f.sfx}}).ids())
		assert.Equal(t, []int64{t0}, searchTickets(t, router, tokenA, url.Values{"from": {"alice@" + f.sfx}}).ids())
		assert.Equal(t, []int64{t1}, searchTickets(t, router, tokenA, url.Values{
			"title": {"*" + f.sfx}, "state_type": {"closed"}, "queue_id": {fmt.Sprint(f.queueA)},
		}).ids())
		assert.Equal(t, []int64{t1}, searchTickets(t, router, tokenA, url.Values{
			"title": {"*" + f.sfx}, "created_before": {f.now.AddDate(0, 0, -5).Format("2006-01-02")},
		}).ids())
		assert.Equal(t, []int64{t0}, searchTickets(t, router, tokenA, url.Values{
			"customer_user_login": {f.customerUser}, "created_after": {f.now.AddDate(0, 0, -5).Format("2006-01-02")},
		}).ids())
		assert.Equal(t, []int64{t1}, searchTickets(t, router, tokenA, url.Values{"q": {f.sfx}, "priority_id": {fmt.Sprintf("%d,%d", f.prioHigh, f.prioHigh)}}).ids())
		assert.Equal(t, sortedIDs(t0, t1), sortedIDs(searchTickets(t, router, tokenA, url.Values{"q": {f.sfx}, "priority_id": {fmt.Sprint(f.prioNormal), fmt.Sprint(f.prioHigh)}}).ids()...))
		// The '_' in a literal search is not a LIKE wildcard.
		assert.Empty(t, searchTickets(t, router, tokenA, url.Values{"title": {"printer_jam " + f.sfx}}).ids())
		// Asking for a queue the agent cannot read returns nothing, not its tickets.
		assert.Empty(t, searchTickets(t, router, tokenA, url.Values{"queue_id": {fmt.Sprint(f.queueB)}}).ids())
	})

	t.Run("sort and pagination", func(t *testing.T) {
		res := searchTickets(t, router, tokenA, url.Values{"q": {f.sfx}, "sort": {"created"}, "order": {"asc"}})
		assert.Equal(t, []int64{t1, t0}, res.ids())
		assert.Equal(t, 2, res.Data.Pagination.Total)

		res = searchTickets(t, router, tokenA, url.Values{"q": {f.sfx}, "sort": {"created"}, "order": {"asc"}, "limit": {"1"}, "page": {"2"}})
		assert.Equal(t, []int64{t0}, res.ids())
		assert.Equal(t, 2, res.Data.Pagination.Total)
		assert.Equal(t, 2, res.Data.Pagination.Page)
		assert.Equal(t, 1, res.Data.Pagination.Limit)
	})

	t.Run("invalid parameters", func(t *testing.T) {
		for _, q := range []string{"queue_id=abc", "foo=bar", "created_after=yesterday", "limit=101", "sort=owner", "title=a&title=b"} {
			w := searchRequest(t, router, http.MethodGet, "/api/v1/search/tickets?"+q, tokenA, nil)
			assert.Equal(t, http.StatusBadRequest, w.Code, "%s: %s", q, w.Body.String())
		}
	})
}

func TestSavedSearchAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newSearchFixture(t)
	router := NewSimpleRouter()
	tokenA := statsToken(t, f.agentA, f.loginA, "Agent")
	tokenAll := statsToken(t, f.agentAll, f.loginAll, "Agent")
	t0, t1 := f.tickets[0], f.tickets[1]
	name := "open " + f.sfx // spaces must survive the path
	path := "/api/v1/search/saved/" + url.PathEscape(name)
	profileLogin := ticketSearchProfileBase + "::" + f.loginA

	w := searchRequest(t, router, http.MethodPost, "/api/v1/search/saved", tokenA, gin.H{
		"name": name, "parameters": gin.H{"q": f.sfx, "state_type": []string{"open", "new"}},
	})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	// Stored the OTRS way: one row per value, OTRS attribute names.
	rows, err := f.db.Query(database.ConvertPlaceholders(
		"SELECT profile_type, profile_key, profile_value FROM search_profile WHERE login = ? AND profile_name = ? ORDER BY profile_key, profile_value"),
		profileLogin, name)
	require.NoError(t, err)
	var stored []string
	for rows.Next() {
		var typ, key, value string
		require.NoError(t, rows.Scan(&typ, &key, &value))
		stored = append(stored, typ+" "+key+"="+value)
	}
	require.NoError(t, rows.Err())
	require.NoError(t, rows.Close())
	assert.Equal(t, []string{"SCALAR Fulltext=" + f.sfx, "ARRAY StateType=new", "ARRAY StateType=open"}, stored)

	t.Run("duplicate and invalid", func(t *testing.T) {
		w := searchRequest(t, router, http.MethodPost, "/api/v1/search/saved", tokenA, gin.H{"name": name, "parameters": gin.H{"q": "x"}})
		assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
		for _, body := range []gin.H{
			{"name": "", "parameters": gin.H{"q": "x"}},
			{"name": "bad " + f.sfx, "parameters": gin.H{"queue_id": "abc"}},
			{"name": "bad " + f.sfx, "parameters": gin.H{"nonsense": "x"}},
			{"name": "bad " + f.sfx, "parameters": gin.H{}},
			{"name": "bad " + f.sfx, "type": "CustomerSearch", "parameters": gin.H{"q": "x"}},
		} {
			w := searchRequest(t, router, http.MethodPost, "/api/v1/search/saved", tokenA, body)
			assert.Equal(t, http.StatusBadRequest, w.Code, "%v: %s", body, w.Body.String())
		}
	})

	t.Run("list and get are per agent", func(t *testing.T) {
		var list struct {
			Data struct {
				Searches []struct {
					Name string `json:"name"`
					Type string `json:"type"`
				} `json:"searches"`
				Total int `json:"total"`
			} `json:"data"`
		}
		w := searchRequest(t, router, http.MethodGet, "/api/v1/search/saved", tokenA, nil)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
		require.Equal(t, 1, list.Data.Total)
		assert.Equal(t, name, list.Data.Searches[0].Name)
		assert.Equal(t, "TicketSearch", list.Data.Searches[0].Type)

		w = searchRequest(t, router, http.MethodGet, "/api/v1/search/saved", tokenAll, nil)
		require.Equal(t, http.StatusOK, w.Code)
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
		assert.Equal(t, 0, list.Data.Total)
		assert.Equal(t, http.StatusNotFound, searchRequest(t, router, http.MethodGet, path, tokenAll, nil).Code)

		var got searchResult
		w = searchRequest(t, router, http.MethodGet, path, tokenA, nil)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
		assert.Equal(t, f.sfx, got.Data.Parameters["q"])
		assert.ElementsMatch(t, []interface{}{"open", "new"}, got.Data.Parameters["state_type"])
	})

	execute := func(t *testing.T) searchResult {
		t.Helper()
		w := searchRequest(t, router, http.MethodPost, path+"/execute", tokenA, nil)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var res searchResult
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
		return res
	}

	t.Run("execute and update", func(t *testing.T) {
		assert.Equal(t, []int64{t0}, execute(t).ids())

		w := searchRequest(t, router, http.MethodPut, path, tokenA, gin.H{"parameters": gin.H{"q": f.sfx, "state_type": "closed"}})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, []int64{t1}, execute(t).ids())

		w = searchRequest(t, router, http.MethodPut, "/api/v1/search/saved/missing"+f.sfx, tokenA, gin.H{"parameters": gin.H{"q": "x"}})
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("attributes this API does not search on are reported, not applied", func(t *testing.T) {
		_, err := f.db.Exec(database.ConvertPlaceholders(
			"INSERT INTO search_profile (login, profile_name, profile_type, profile_key, profile_value) VALUES (?, ?, 'ARRAY', 'ShownAttributes', 'LabelTitle')"),
			profileLogin, name)
		require.NoError(t, err)
		res := execute(t)
		assert.Equal(t, []int64{t1}, res.ids())
		assert.Equal(t, []string{"ShownAttributes"}, res.Data.IgnoredParameters)
	})

	t.Run("delete", func(t *testing.T) {
		assert.Equal(t, http.StatusNoContent, searchRequest(t, router, http.MethodDelete, path, tokenA, nil).Code)
		assert.Equal(t, http.StatusNotFound, searchRequest(t, router, http.MethodGet, path, tokenA, nil).Code)
		assert.Equal(t, http.StatusNotFound, searchRequest(t, router, http.MethodDelete, path, tokenA, nil).Code)
		var n int
		require.NoError(t, f.db.QueryRow(database.ConvertPlaceholders("SELECT COUNT(*) FROM search_profile WHERE login = ?"), profileLogin).Scan(&n))
		assert.Equal(t, 0, n)
	})
}
