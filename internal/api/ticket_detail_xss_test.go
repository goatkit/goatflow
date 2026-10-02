package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// The first article of a ticket is rendered with |safe whenever it contains a
// tag. Its body arrives from customers (portal form, inbound email, API), so
// script-bearing markup that the IsHTML heuristic does not recognise must
// still be sanitised before it reaches an agent's browser.
func TestTicketDescriptionIsSanitised(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := getTestDB(t)

	sfx := fmt.Sprintf("%d", time.Now().UnixNano())
	const payload = `<svg onload="alert(document.cookie)">x</svg><img src=x onerror=alert(1)>`
	ticketID, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO ticket (tn, title, queue_id, ticket_lock_id, type_id, user_id, responsible_user_id,
			ticket_priority_id, ticket_state_id, customer_id, customer_user_id, timeout, until_time,
			escalation_time, escalation_update_time, escalation_response_time, escalation_solution_time,
			archive_flag, create_time, create_by, change_time, change_by)
		VALUES (?, 'xss ticket', 1, 1, 1, 1, 1, 3, 1, 'xss-co', 'xss@example.com', 0, 0, 0, 0, 0, 0, 0,
			NOW(), 1, NOW(), 1) RETURNING id`), "XSS"+sfx)
	require.NoError(t, err)
	articleID, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO article (ticket_id, article_sender_type_id, communication_channel_id, is_visible_for_customer,
			search_index_needs_rebuild, create_time, create_by, change_time, change_by)
		VALUES (?, 3, 3, 1, 1, NOW(), 1, NOW(), 1) RETURNING id`), ticketID)
	require.NoError(t, err)
	_, err = db.Exec(database.ConvertPlaceholders(`
		INSERT INTO article_data_mime (article_id, a_from, a_subject, a_body, a_content_type, incoming_time,
			create_time, create_by, change_time, change_by)
		VALUES (?, 'xss@example.com', 'xss', ?, 'text/plain', 0, NOW(), 1, NOW(), 1)`), articleID, payload)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders("DELETE FROM article_data_mime WHERE article_id = ?"), articleID)
		_, _ = db.Exec(database.ConvertPlaceholders("DELETE FROM ticket_history WHERE ticket_id = ?"), ticketID)
		_, _ = db.Exec(database.ConvertPlaceholders("DELETE FROM article WHERE id = ?"), articleID)
		_, _ = db.Exec(database.ConvertPlaceholders("DELETE FROM ticket WHERE id = ?"), ticketID)
	})

	router := NewSimpleRouterWithDB(db)
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/ticket/%d", ticketID), nil)
	req.Header.Set("Accept", "text/html")
	AddTestAuthCookie(req, GetTestAuthToken(t))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	html := w.Body.String()
	for _, raw := range []string{`onload="alert(document.cookie)"`, "onerror=alert(1)"} {
		require.False(t, strings.Contains(html, raw), "ticket page echoes %q from the first article unsanitised", raw)
	}
}
