package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// An agent's internal note must never reach the customer portal, while a
// customer-visible note on the same ticket must. Regression: the article
// repository turned is_visible_for_customer = 0 into 1 on insert, and the note
// handler wrote a communication channel that does not exist.
func TestInternalNoteStaysHiddenFromCustomer(t *testing.T) {
	db, err := database.GetDB()
	require.NoError(t, err)
	setupTemplateRenderer(t)

	login := fmt.Sprintf("vis-cust-%d", time.Now().UnixNano())
	ticketID := createArticleTestTicket(t, db, login)
	secret := fmt.Sprintf("SECRET-INTERNAL-%d", time.Now().UnixNano())
	public := fmt.Sprintf("PUBLIC-REPLY-%d", time.Now().UnixNano())

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/tickets/:id/notes", func(c *gin.Context) { c.Set("user_id", 1); c.Next() }, handleAddTicketNote)
	r.GET("/customer/tickets/:id", func(c *gin.Context) {
		c.Set("username", login)
		c.Set("user_role", "Customer")
		c.Next()
	}, handleCustomerTicketView(db))

	addNote := func(content string, internal bool) {
		t.Helper()
		body, _ := json.Marshal(map[string]any{"content": content, "internal": internal, "time_units": 1})
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/tickets/%d/notes", ticketID), bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Less(t, w.Code, 300, w.Body.String())
	}
	addNote(secret, true)
	addNote(public, false)

	assert.Equal(t, 0, articleCountRows(t, db, `
		SELECT COUNT(*) FROM article a JOIN article_data_mime m ON m.article_id = a.id
		WHERE a.ticket_id = ? AND m.a_body = ? AND a.is_visible_for_customer = 1`, ticketID, secret),
		"internal note stored as customer-visible")
	assert.Equal(t, 2, articleCountRows(t, db, `
		SELECT COUNT(*) FROM article WHERE ticket_id = ? AND communication_channel_id = 3`, ticketID))

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/customer/tickets/%d", ticketID), nil)
	req.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	page := w.Body.String()
	assert.True(t, strings.Contains(page, public), "customer-visible note missing from portal")
	assert.False(t, strings.Contains(page, secret), "internal note leaked to the customer portal")
}
