package api

import (
	"database/sql"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
)

var writeTestTicketSeq atomic.Int64

// createWriteTestTicket inserts a real ticket row (queue 1, seed state/priority/type) and
// removes it together with every article and history row hanging off it at cleanup.
func createWriteTestTicket(t *testing.T, db *sql.DB, customerUserID *string) int {
	t.Helper()
	tn := fmt.Sprintf("TWF%d%d", time.Now().UnixNano(), writeTestTicketSeq.Add(1))
	id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO ticket (tn, title, queue_id, type_id, ticket_state_id, ticket_priority_id,
		                    ticket_lock_id, user_id, responsible_user_id, customer_user_id,
		                    timeout, until_time, escalation_time, escalation_update_time,
		                    escalation_response_time, escalation_solution_time,
		                    create_time, create_by, change_time, change_by)
		VALUES (?, 'Write path test ticket', 1, 1, 1, 1, 1, 1, 1, ?,
		        0, 0, 0, 0, 0, 0, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)
		RETURNING id
	`), tn, customerUserID)
	require.NoError(t, err)
	ticketID := int(id)
	t.Cleanup(func() { purgeWriteTestTicket(db, ticketID) })
	return ticketID
}

func purgeWriteTestTicket(db *sql.DB, ticketID int) {
	for _, q := range []string{
		`DELETE FROM ticket_history WHERE ticket_id = ?`,
		`DELETE FROM time_accounting WHERE ticket_id = ?`,
		`DELETE FROM article_data_mime WHERE article_id IN (SELECT id FROM article WHERE ticket_id = ?)`,
		`DELETE FROM article WHERE ticket_id = ?`,
		`DELETE FROM ticket WHERE id = ?`,
	} {
		_, _ = db.Exec(database.ConvertPlaceholders(q), ticketID)
	}
}

// serveWriteTest runs one request through a router that sets the given context values
// (the parts the auth middleware would normally set) before the handler.
func serveWriteTest(method, routePath, url, contentType, body string, ctx map[string]any, h gin.HandlerFunc) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Handle(method, routePath, func(c *gin.Context) {
		for k, v := range ctx {
			c.Set(k, v)
		}
		h(c)
	})
	req := httptest.NewRequest(method, url, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func countRows(t *testing.T, db *sql.DB, q string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(q), args...).Scan(&n))
	return n
}
