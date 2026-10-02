package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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
)

// listingScopeFixture extends the RBAC fixtures (AgentAlpha reads the Alpha and
// Both queues, not Beta) with: AgentAlpha holding only move_into on the Beta
// group, a newest ticket in Alpha and Beta, an article on the Beta ticket, and
// two customers with one ticket each.
type listingScopeFixture struct {
	*RBACTestFixtures
	router        *gin.Engine
	suffix        string
	alphaNewTN    string
	betaNewTN     string
	betaNewTitle  string
	betaWord      string // only occurs in the Beta article body
	customerLogin string
	customerTN    string
	foreignTN     string
	customerID    int64
}

func newListingScopeFixture(t *testing.T) *listingScopeFixture {
	t.Helper()
	f := &listingScopeFixture{RBACTestFixtures: getRBACFixtures(t)}
	db := f.db
	f.suffix = strconv.FormatInt(time.Now().UnixNano(), 36)

	_, err := db.Exec(database.ConvertPlaceholders(`
		INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
		VALUES (?, ?, 'move_into', ?, 1, ?, 1)`), f.AgentAlpha, f.GroupBeta, time.Now(), time.Now())
	require.NoError(t, err)

	future := time.Now().Add(48 * time.Hour)
	f.alphaNewTN = "LSA" + f.suffix
	alphaID := listingScopeTicket(t, db, f.alphaNewTN, "Scope alpha newest "+f.suffix, f.QueueAlpha, "test@test.com", future)
	listingScopeHistory(t, db, alphaID, f.QueueAlpha, time.Now().Add(time.Hour))
	f.betaNewTN = "LSB" + f.suffix
	f.betaNewTitle = "Scope beta newest " + f.suffix
	betaID := listingScopeTicket(t, db, f.betaNewTN, f.betaNewTitle, f.QueueBeta, "test@test.com", future.Add(time.Hour))
	f.betaWord = "betasecret" + f.suffix
	listingScopeArticle(t, db, betaID, "Beta note", "internal "+f.betaWord)
	listingScopeHistory(t, db, betaID, f.QueueBeta, time.Now().Add(2*time.Hour))

	f.customerLogin = "lscust-" + f.suffix
	f.customerID = custAttInsertCustomer(t, db, f.customerLogin)
	other := "lsother-" + f.suffix
	custAttInsertCustomer(t, db, other)
	f.customerTN = "LSC" + f.suffix
	listingScopeTicket(t, db, f.customerTN, "Customer own "+f.suffix, f.QueueAlpha, f.customerLogin, time.Now())
	f.foreignTN = "LSF" + f.suffix
	listingScopeTicket(t, db, f.foreignTN, "Customer foreign "+f.suffix, f.QueueAlpha, other, time.Now())

	setupTemplateRenderer(t)
	f.router = newCustAttRouter(t)
	return f
}

func listingScopeTicket(t *testing.T, db *sql.DB, tn, title string, queueID int, customerLogin string, created time.Time) int64 {
	t.Helper()
	id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO ticket (tn, title, queue_id, ticket_lock_id, type_id, user_id, responsible_user_id,
			ticket_priority_id, ticket_state_id, customer_id, customer_user_id, timeout, until_time,
			escalation_time, escalation_update_time, escalation_response_time, escalation_solution_time,
			archive_flag, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, 1, 1, 1, 1, 3, 1, 'test-co', ?, 0, 0, 0, 0, 0, 0, 0, ?, 1, ?, 1) RETURNING id`),
		tn, title, queueID, customerLogin, created, created)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM article_data_mime WHERE article_id IN (SELECT id FROM article WHERE ticket_id = ?)`), id)
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM article WHERE ticket_id = ?`), id)
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM ticket_history WHERE ticket_id = ?`), id)
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM ticket WHERE id = ?`), id)
	})
	return id
}

// listingScopeHistory records a NewTicket history entry at the given time.
func listingScopeHistory(t *testing.T, db *sql.DB, ticketID int64, queueID int, at time.Time) {
	t.Helper()
	_, err := db.Exec(database.ConvertPlaceholders(`
		INSERT INTO ticket_history (name, history_type_id, ticket_id, type_id, queue_id, owner_id,
			priority_id, state_id, create_time, create_by, change_time, change_by)
		VALUES ('created', (SELECT id FROM ticket_history_type WHERE name = 'NewTicket'), ?, 1, ?, 1, 3, 1, ?, 1, ?, 1)`),
		ticketID, queueID, at, at)
	require.NoError(t, err)
}

func listingScopeArticle(t *testing.T, db *sql.DB, ticketID int64, subject, body string) {
	t.Helper()
	now := time.Now()
	articleID, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO article (ticket_id, article_sender_type_id, communication_channel_id, is_visible_for_customer,
			search_index_needs_rebuild, create_time, create_by, change_time, change_by)
		VALUES (?, 1, 3, 0, 1, ?, 1, ?, 1) RETURNING id`), ticketID, now, now)
	require.NoError(t, err)
	_, err = db.Exec(database.ConvertPlaceholders(`
		INSERT INTO article_data_mime (article_id, a_from, a_subject, a_body, a_content_type,
			incoming_time, create_time, create_by, change_time, change_by)
		VALUES (?, 'agent@example.com', ?, ?, 'text/plain', ?, ?, 1, ?, 1)`),
		articleID, subject, body, now.Unix(), now, now)
	require.NoError(t, err)
}

func (f *listingScopeFixture) agentToken(t *testing.T, userID int) string {
	t.Helper()
	return testSessionToken(t, uint(userID), "agent", "agent@example.com", "Agent", false, 0)
}

func (f *listingScopeFixture) do(t *testing.T, token, method, url string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var rd *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, url, rd)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, req)
	return w
}

func (f *listingScopeFixture) betaTNs() []string {
	tns := []string{f.betaNewTN}
	for _, id := range f.TicketsBeta {
		tns = append(tns, fmt.Sprintf("RBAC%d", id))
	}
	return tns
}

func TestTicketListingScope_AgentOnlySeesReadableQueues(t *testing.T) {
	f := newListingScopeFixture(t)
	alpha := f.agentToken(t, f.AgentAlpha)

	t.Run("select-all ticket ids ignore a queue the agent cannot read", func(t *testing.T) {
		w := f.do(t, alpha, http.MethodGet, fmt.Sprintf("/agent/api/tickets/ids?status=all&queue=%d", f.QueueBeta), nil)
		require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())

		w = f.do(t, alpha, http.MethodGet, "/agent/api/tickets/ids?status=all&search=RBAC95", nil)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var resp struct {
			TicketIDs  []int `json:"ticket_ids"`
			TotalCount int   `json:"total_count"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.ElementsMatch(t, append(append([]int{}, f.TicketsAlpha...), f.TicketsBoth...), resp.TicketIDs)
		assert.Equal(t, len(f.TicketsAlpha)+len(f.TicketsBoth), resp.TotalCount)
	})

	t.Run("ticket list page hides queues held with move_into only", func(t *testing.T) {
		w := f.do(t, alpha, http.MethodGet, "/agent/tickets?status=all&per_page=100&search="+f.suffix, nil)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		body := w.Body.String()
		assert.Contains(t, body, f.alphaNewTN)
		assert.NotContains(t, body, f.betaNewTN)
		assert.NotContains(t, body, "RBACTest-Beta-Queue")

		w = f.do(t, alpha, http.MethodGet, fmt.Sprintf("/agent/tickets?status=all&queue=%d", f.QueueBeta), nil)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("queues page lists only readable queues", func(t *testing.T) {
		w := f.do(t, alpha, http.MethodGet, "/agent/queues", nil)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		body := w.Body.String()
		assert.Contains(t, body, "RBACTest-Alpha-Queue")
		assert.NotContains(t, body, "RBACTest-Beta-Queue")
	})

	t.Run("dashboard recent tickets widget", func(t *testing.T) {
		w := f.do(t, alpha, http.MethodGet, "/api/dashboard/recent-tickets", nil)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		body := w.Body.String()
		assert.Contains(t, body, f.alphaNewTN)
		assert.NotContains(t, body, f.betaNewTN)
		assert.NotContains(t, body, f.betaNewTitle)
	})

	t.Run("dashboard activity stream", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		req := httptest.NewRequest(http.MethodGet, "/api/dashboard/activity-stream", nil).WithContext(ctx)
		req.Header.Set("Authorization", "Bearer "+alpha)
		w := httptest.NewRecorder()
		f.router.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), "created ticket "+f.alphaNewTN)
		assert.NotContains(t, w.Body.String(), f.betaNewTN)
	})

	t.Run("global search over tickets and articles", func(t *testing.T) {
		w := f.do(t, alpha, http.MethodPost, "/api/v1/search", gin.H{"query": "Scope newest " + f.suffix, "types": []string{"ticket"}})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), f.alphaNewTN)
		assert.NotContains(t, w.Body.String(), f.betaNewTN)
		var res struct {
			TotalHits int `json:"total_hits"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
		assert.Equal(t, 1, res.TotalHits)

		w = f.do(t, alpha, http.MethodPost, "/api/v1/search", gin.H{"query": f.betaWord, "types": []string{"ticket", "article"}})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
		assert.Equal(t, 0, res.TotalHits, w.Body.String())
		assert.NotContains(t, w.Body.String(), f.betaNewTN)

		both := f.agentToken(t, f.AgentBoth)
		w = f.do(t, both, http.MethodPost, "/api/v1/search", gin.H{"query": f.betaWord, "types": []string{"ticket", "article"}})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
		assert.Equal(t, 2, res.TotalHits, "reader of Beta finds the ticket and its article: %s", w.Body.String())
	})

	t.Run("queue list stats only for readable queues", func(t *testing.T) {
		w := f.do(t, alpha, http.MethodGet, "/api/v1/queues?include_stats=true", nil)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var resp struct {
			Data []map[string]interface{} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		seen := map[int]map[string]interface{}{}
		for _, q := range resp.Data {
			seen[int(q["id"].(float64))] = q
		}
		require.Contains(t, seen, f.QueueAlpha)
		assert.EqualValues(t, len(f.TicketsAlpha)+3, seen[f.QueueAlpha]["ticket_count"], "fixture tickets, newest ticket, two customer tickets")
		if beta, ok := seen[f.QueueBeta]; ok {
			assert.NotContains(t, beta, "ticket_count", "move_into on Beta must not reveal its ticket counts")
		}
	})
}

func TestTicketListingScope_CustomerTicketList(t *testing.T) {
	f := newListingScopeFixture(t)

	list := func(t *testing.T, token string) string {
		t.Helper()
		w := f.do(t, token, http.MethodGet, "/api/v1/tickets?per_page=100&search="+f.suffix, nil)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		return w.Body.String()
	}

	t.Run("JWT whose email differs from the login", func(t *testing.T) {
		tok := testSessionToken(t, uint(f.customerID), f.customerLogin, "other-address@example.com", "Customer", false, 0)
		body := list(t, tok)
		assert.Contains(t, body, f.customerTN)
		assert.NotContains(t, body, f.foreignTN)
		assert.NotContains(t, body, f.alphaNewTN)
	})

	t.Run("JWT without an email", func(t *testing.T) {
		tok := testSessionToken(t, uint(f.customerID), f.customerLogin, "", "Customer", false, 0)
		body := list(t, tok)
		assert.Contains(t, body, f.customerTN)
		assert.NotContains(t, body, f.foreignTN)
		assert.NotContains(t, body, f.alphaNewTN)
	})

	t.Run("JWT with no resolvable customer login is refused", func(t *testing.T) {
		tok := testSessionToken(t, 999999999, "", "", "Customer", false, 0)
		w := f.do(t, tok, http.MethodGet, "/api/v1/tickets?per_page=100&search="+f.suffix, nil)
		assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
		assert.False(t, strings.Contains(w.Body.String(), f.foreignTN))
	})
}
