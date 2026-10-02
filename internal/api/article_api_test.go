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
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/plugin"
)

// createArticleTestTicket inserts a ticket in queue 1 owned by customerLogin
// and makes sure user 1 may write notes on it. Everything the test creates on
// the ticket is removed when the test ends.
func createArticleTestTicket(t *testing.T, db *sql.DB, customerLogin string) int64 {
	t.Helper()
	tn := fmt.Sprintf("ART-%d", time.Now().UnixNano())
	ticketID, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO ticket (tn, title, queue_id, type_id, ticket_state_id, ticket_priority_id,
		                    ticket_lock_id, user_id, responsible_user_id, customer_user_id,
		                    timeout, until_time, escalation_time, escalation_update_time,
		                    escalation_response_time, escalation_solution_time,
		                    create_time, create_by, change_time, change_by)
		VALUES (?, 'Article API test', 1, 1, 1, 3, 1, 1, 1, ?, 0, 0, 0, 0, 0, 0, NOW(), 1, NOW(), 1)
		RETURNING id`), tn, customerLogin)
	require.NoError(t, err)

	var groupID int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		`SELECT group_id FROM queue WHERE id = 1`)).Scan(&groupID))
	var granted int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		`SELECT COUNT(*) FROM group_user WHERE user_id = 1 AND group_id = ? AND permission_key = 'rw'`), groupID).Scan(&granted))
	if granted == 0 {
		_, err = db.Exec(database.ConvertPlaceholders(`
			INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
			VALUES (1, ?, 'rw', NOW(), 1, NOW(), 1)`), groupID)
		require.NoError(t, err)
	}

	t.Cleanup(func() {
		for _, q := range []string{
			`DELETE FROM article_data_mime_attachment WHERE article_id IN (SELECT id FROM article WHERE ticket_id = ?)`,
			`DELETE FROM article_data_mime WHERE article_id IN (SELECT id FROM article WHERE ticket_id = ?)`,
			`DELETE FROM time_accounting WHERE ticket_id = ?`,
			`DELETE FROM ticket_history WHERE ticket_id = ?`,
			`DELETE FROM article WHERE ticket_id = ?`,
			`DELETE FROM ticket WHERE id = ?`,
		} {
			_, _ = db.Exec(database.ConvertPlaceholders(q), ticketID)
		}
		if granted == 0 {
			_, _ = db.Exec(database.ConvertPlaceholders(
				`DELETE FROM group_user WHERE user_id = 1 AND group_id = ? AND permission_key = 'rw'`), groupID)
		}
	})
	return ticketID
}

func articleAPIRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	agent := r.Group("/api/v1", func(c *gin.Context) {
		if c.GetHeader("X-No-Auth") == "" {
			userID := 1
			if u := c.GetHeader("X-User"); u != "" {
				userID, _ = strconv.Atoi(u)
			}
			c.Set("user_id", userID)
			c.Set("is_authenticated", true)
		}
		c.Next()
	})
	agent.GET("/tickets/:id/articles", HandleListArticlesAPI)
	agent.POST("/tickets/:id/articles", HandleCreateArticleAPI)
	agent.GET("/tickets/:id/articles/:article_id", HandleGetArticleAPI)
	agent.PUT("/tickets/:id/articles/:article_id", HandleUpdateArticleAPI)
	agent.DELETE("/tickets/:id/articles/:article_id", HandleDeleteArticleAPI)
	customer := r.Group("/customer-api/v1", func(c *gin.Context) {
		c.Set("user_id", 1)
		c.Set("is_customer", true)
		c.Set("customer_login", c.GetHeader("X-Customer"))
		c.Next()
	})
	customer.POST("/tickets/:id/articles", HandleCreateArticleAPI)
	customer.GET("/tickets/:id/articles", HandleListArticlesAPI)
	customer.GET("/tickets/:id/articles/:article_id", HandleGetArticleAPI)
	customer.PUT("/tickets/:id/articles/:article_id", HandleUpdateArticleAPI)
	customer.DELETE("/tickets/:id/articles/:article_id", HandleDeleteArticleAPI)
	return r
}

func articleDoJSON(t *testing.T, r *gin.Engine, method, path string, body any, headers ...string) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	out := map[string]any{}
	if w.Body.Len() > 0 {
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out), w.Body.String())
	}
	return w.Code, out
}

func createArticleViaAPI(t *testing.T, r *gin.Engine, ticketID int64, payload map[string]any) map[string]any {
	t.Helper()
	code, resp := articleDoJSON(t, r, http.MethodPost, fmt.Sprintf("/api/v1/tickets/%d/articles", ticketID), payload)
	require.Equal(t, http.StatusCreated, code, resp)
	data, ok := resp["data"].(map[string]any)
	require.True(t, ok, resp)
	return data
}

func storedArticle(t *testing.T, db *sql.DB, articleID int64) (channelID, visible int, from, subject string) {
	t.Helper()
	var f, s sql.NullString
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`
		SELECT a.communication_channel_id, a.is_visible_for_customer, m.a_from, m.a_subject
		FROM article a JOIN article_data_mime m ON m.article_id = a.id
		WHERE a.id = ?`), articleID).Scan(&channelID, &visible, &f, &s))
	return channelID, visible, f.String, s.String
}

func articleCountRows(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(query), args...).Scan(&n))
	return n
}

func TestArticleAPI_CreateMapsArticleTypeToChannelAndVisibility(t *testing.T) {
	db, err := database.GetDB()
	require.NoError(t, err)
	r := articleAPIRouter()
	ticketID := createArticleTestTicket(t, db, "art-type-cust")

	cases := []struct {
		name        string
		payload     map[string]any
		wantType    string
		wantChannel int
		wantVisible int
	}{
		{"email external", map[string]any{"article_type": "email-external", "from_email": "agent@example.com"}, "email-external", 1, 1},
		{"email internal", map[string]any{"article_type": "email-internal"}, "email-internal", 1, 0},
		{"phone", map[string]any{"article_type": "phone"}, "phone", 2, 1},
		{"default agent article is an internal note", map[string]any{}, "note-internal", 3, 0},
		{"visibility alone picks external note", map[string]any{"is_visible_for_customer": true}, "note-external", 3, 1},
		{"internal note cannot be made visible", map[string]any{"article_type": "note-internal", "is_visible": true}, "note-internal", 3, 0},
		{"external email can be hidden", map[string]any{"article_type": "email", "is_visible": false}, "email-internal", 1, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := map[string]any{"subject": tc.name, "body": "body of " + tc.name}
			for k, v := range tc.payload {
				payload[k] = v
			}
			data := createArticleViaAPI(t, r, ticketID, payload)
			assert.Equal(t, tc.wantType, data["article_type"])
			assert.Equal(t, float64(tc.wantChannel), data["communication_channel_id"])
			assert.Equal(t, tc.wantVisible == 1, data["is_visible_for_customer"])

			channel, visible, from, subject := storedArticle(t, db, int64(data["id"].(float64)))
			assert.Equal(t, tc.wantChannel, channel)
			assert.Equal(t, tc.wantVisible, visible)
			assert.Equal(t, tc.name, subject)
			if f, ok := tc.payload["from_email"]; ok {
				assert.Equal(t, f, from)
			}
		})
	}
}

func TestArticleAPI_CreateRejectsInvalidTyping(t *testing.T) {
	db, err := database.GetDB()
	require.NoError(t, err)
	r := articleAPIRouter()
	ticketID := createArticleTestTicket(t, db, "art-reject-cust")

	code, resp := articleDoJSON(t, r, http.MethodPost, fmt.Sprintf("/api/v1/tickets/%d/articles", ticketID),
		map[string]any{"body": "x", "article_type": "letter"})
	assert.Equal(t, http.StatusBadRequest, code, resp)
	assert.Equal(t, "Invalid article type", resp["error"])

	code, resp = articleDoJSON(t, r, http.MethodPost, fmt.Sprintf("/api/v1/tickets/%d/articles", ticketID),
		map[string]any{"body": "x", "communication_channel_id": 99})
	assert.Equal(t, http.StatusBadRequest, code, resp)

	// A customer can neither write an internal note nor pose as an agent to do so.
	code, resp = articleDoJSON(t, r, http.MethodPost, fmt.Sprintf("/customer-api/v1/tickets/%d/articles", ticketID),
		map[string]any{"body": "x", "article_type": "note-internal", "sender_type": "agent"}, "X-Customer", "art-reject-cust")
	assert.Equal(t, http.StatusBadRequest, code, resp)

	assert.Equal(t, 0, articleCountRows(t, db, `SELECT COUNT(*) FROM article WHERE ticket_id = ?`, ticketID))

	// Without a type a customer writes a visible email as customer.
	code, resp = articleDoJSON(t, r, http.MethodPost, fmt.Sprintf("/customer-api/v1/tickets/%d/articles", ticketID),
		map[string]any{"body": "customer reply", "sender_type": "agent"}, "X-Customer", "art-reject-cust")
	require.Equal(t, http.StatusCreated, code, resp)
	data := resp["data"].(map[string]any)
	assert.Equal(t, "email-external", data["article_type"])
	assert.Equal(t, float64(3), data["article_sender_type_id"])
}

func TestArticleAPI_LifecycleWithAttachments(t *testing.T) {
	db, err := database.GetDB()
	require.NoError(t, err)
	r := articleAPIRouter()
	ticketID := createArticleTestTicket(t, db, "art-life-cust")
	base := fmt.Sprintf("/api/v1/tickets/%d/articles", ticketID)

	first := createArticleViaAPI(t, r, ticketID, map[string]any{
		"subject": "First", "body": "first body", "article_type": "email-external",
		"from": "agent@example.com", "to": "customer@example.com",
	})
	firstID := int64(first["id"].(float64))
	second := createArticleViaAPI(t, r, ticketID, map[string]any{"subject": "Second", "body": "second body"})
	secondID := int64(second["id"].(float64))

	// Attach two files to the first article through the plugin HostAPI.
	host := newAttachmentHost(db)
	ctx := context.Background()
	pdfID, err := host.CreateArticleAttachment(ctx, firstID, 1, "report.pdf", "application/pdf", []byte("%PDF-1.4 data"))
	require.NoError(t, err)
	_, err = host.CreateArticleAttachment(ctx, firstID, 1, "notes.txt", "text/plain", []byte("plain\\text\x00bytes"))
	require.NoError(t, err)
	listed, err := host.ListArticleAttachments(ctx, firstID)
	require.NoError(t, err)
	require.Len(t, listed, 2)
	var stored []byte
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		`SELECT content FROM article_data_mime_attachment WHERE article_id = ? AND filename = 'notes.txt'`), firstID).Scan(&stored))
	assert.Equal(t, []byte("plain\\text\x00bytes"), stored, "binary content must round-trip unchanged")

	t.Run("get with attachments", func(t *testing.T) {
		code, resp := articleDoJSON(t, r, http.MethodGet, fmt.Sprintf("%s/%d?include_attachments=true", base, firstID), nil)
		require.Equal(t, http.StatusOK, code, resp)
		assert.Equal(t, "First", resp["subject"])
		assert.Equal(t, "first body", resp["body"])
		assert.Equal(t, "agent@example.com", resp["from"])
		assert.Equal(t, "customer@example.com", resp["to"])
		assert.Equal(t, "email-external", resp["article_type"])
		assert.Equal(t, "agent", resp["sender_type"])
		atts := resp["attachments"].([]any)
		require.Len(t, atts, 2)
		pdf := atts[0].(map[string]any)
		assert.Equal(t, float64(pdfID), pdf["id"])
		assert.Equal(t, "report.pdf", pdf["filename"])
		assert.Equal(t, "application/pdf", pdf["content_type"])
		assert.Equal(t, float64(len("%PDF-1.4 data")), pdf["size"])
	})

	t.Run("get is scoped to the ticket", func(t *testing.T) {
		code, _ := articleDoJSON(t, r, http.MethodGet, fmt.Sprintf("/api/v1/tickets/%d/articles/%d", ticketID+100000, firstID), nil)
		assert.Equal(t, http.StatusNotFound, code)
	})

	t.Run("list newest first with attachments", func(t *testing.T) {
		code, resp := articleDoJSON(t, r, http.MethodGet, base+"?include_attachments=true", nil)
		require.Equal(t, http.StatusOK, code, resp)
		assert.Equal(t, float64(2), resp["total"])
		articles := resp["articles"].([]any)
		require.Len(t, articles, 2)
		assert.Equal(t, float64(secondID), articles[0].(map[string]any)["id"])
		assert.Equal(t, "note-internal", articles[0].(map[string]any)["article_type"])
		assert.Empty(t, articles[0].(map[string]any)["attachments"])
		assert.Len(t, articles[1].(map[string]any)["attachments"], 2)
	})

	t.Run("update subject keeps body", func(t *testing.T) {
		code, resp := articleDoJSON(t, r, http.MethodPut, fmt.Sprintf("%s/%d", base, secondID), map[string]any{"subject": "Renamed"})
		require.Equal(t, http.StatusOK, code, resp)
		_, _, _, subject := storedArticle(t, db, secondID)
		assert.Equal(t, "Renamed", subject)
		code, resp = articleDoJSON(t, r, http.MethodGet, fmt.Sprintf("%s/%d", base, secondID), nil)
		require.Equal(t, http.StatusOK, code)
		assert.Equal(t, "second body", resp["body"])
	})

	t.Run("delete removes article, mime data and attachments; keeps accounting", func(t *testing.T) {
		_, err := db.Exec(database.ConvertPlaceholders(`
			INSERT INTO time_accounting (ticket_id, article_id, time_unit, create_time, create_by, change_time, change_by)
			VALUES (?, ?, 5, NOW(), 1, NOW(), 1)`), ticketID, firstID)
		require.NoError(t, err)

		code, resp := articleDoJSON(t, r, http.MethodDelete, fmt.Sprintf("%s/%d", base, firstID), nil)
		require.Equal(t, http.StatusOK, code, resp)
		assert.Equal(t, 0, articleCountRows(t, db, `SELECT COUNT(*) FROM article WHERE id = ?`, firstID))
		assert.Equal(t, 0, articleCountRows(t, db, `SELECT COUNT(*) FROM article_data_mime WHERE article_id = ?`, firstID))
		assert.Equal(t, 0, articleCountRows(t, db, `SELECT COUNT(*) FROM article_data_mime_attachment WHERE article_id = ?`, firstID))
		assert.Equal(t, 1, articleCountRows(t, db, `SELECT COUNT(*) FROM time_accounting WHERE ticket_id = ? AND article_id IS NULL`, ticketID))
		assert.Equal(t, 1, articleCountRows(t, db, `SELECT COUNT(*) FROM article WHERE ticket_id = ?`, ticketID))

		code, _ = articleDoJSON(t, r, http.MethodGet, fmt.Sprintf("%s/%d", base, firstID), nil)
		assert.Equal(t, http.StatusNotFound, code)
		code, _ = articleDoJSON(t, r, http.MethodDelete, fmt.Sprintf("%s/%d", base, firstID), nil)
		assert.Equal(t, http.StatusNotFound, code)
	})

	t.Run("unauthenticated", func(t *testing.T) {
		code, _ := articleDoJSON(t, r, http.MethodGet, base, nil, "X-No-Auth", "1")
		assert.Equal(t, http.StatusUnauthorized, code)
	})
}

func TestArticleAPI_ListUnknownTicket(t *testing.T) {
	r := articleAPIRouter()
	code, resp := articleDoJSON(t, r, http.MethodGet, "/api/v1/tickets/987654321/articles", nil)
	assert.Equal(t, http.StatusNotFound, code)
	assert.Equal(t, "Ticket not found", resp["error"])
}

func newAttachmentHost(db *sql.DB) *plugin.ProdHostAPI {
	return plugin.NewProdHostAPI(plugin.WithDB("default", db), plugin.WithArticleAttachmentStore(PluginArticleAttachmentStore{}))
}

func TestPluginHostAPI_ArticleAttachments(t *testing.T) {
	db, err := database.GetDB()
	require.NoError(t, err)
	r := articleAPIRouter()
	ticketID := createArticleTestTicket(t, db, "art-plugin-cust")
	articleID := int64(createArticleViaAPI(t, r, ticketID, map[string]any{"body": "deliverable"})["id"].(float64))
	host := newAttachmentHost(db)
	ctx := context.Background()

	t.Run("missing article", func(t *testing.T) {
		_, err := host.CreateArticleAttachment(ctx, 987654321, 1, "a.bin", "application/octet-stream", []byte("x"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("size limit", func(t *testing.T) {
		_, err := host.CreateArticleAttachment(ctx, articleID, 1, "big.bin", "application/octet-stream", make([]byte, 11<<20))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds size limit")
		assert.Equal(t, 0, articleCountRows(t, db, `SELECT COUNT(*) FROM article_data_mime_attachment WHERE article_id = ?`, articleID))
	})

	t.Run("create, list, delete", func(t *testing.T) {
		id, err := host.CreateArticleAttachment(ctx, articleID, 1, "invite.ics", "text/calendar", []byte("BEGIN:VCALENDAR"))
		require.NoError(t, err)
		atts, err := host.ListArticleAttachments(ctx, articleID)
		require.NoError(t, err)
		require.Len(t, atts, 1)
		assert.Equal(t, id, atts[0].ID)
		assert.Equal(t, articleID, atts[0].ArticleID)
		assert.Equal(t, "invite.ics", atts[0].Filename)
		assert.Equal(t, "text/calendar", atts[0].ContentType)
		assert.Equal(t, int64(len("BEGIN:VCALENDAR")), atts[0].Size)
		assert.Equal(t, fmt.Sprintf("/api/tickets/%d/articles/%d/attachments/%d", ticketID, articleID, id), atts[0].URL)

		// The attachment belongs to its article: another article id cannot delete it.
		require.Error(t, host.DeleteArticleAttachment(ctx, articleID+1, id))
		require.NoError(t, host.DeleteArticleAttachment(ctx, articleID, id))
		atts, err = host.ListArticleAttachments(ctx, articleID)
		require.NoError(t, err)
		assert.Empty(t, atts)
		err = host.DeleteArticleAttachment(ctx, articleID, id)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// Access rules: read needs ro on the ticket's queue, update/delete need rw; an
// article id is only reachable through its own ticket; customers read only
// their own tickets' customer-visible articles and never modify.
func TestArticleAPI_AccessControl(t *testing.T) {
	db, err := database.GetDB()
	require.NoError(t, err)
	r := articleAPIRouter()

	ticketA := createArticleTestTicket(t, db, "acl-cust-a")
	ticketB := createArticleTestTicket(t, db, "acl-cust-b")
	visibleA := int64(createArticleViaAPI(t, r, ticketA, map[string]any{"subject": "A visible", "body": "a-visible", "article_type": "email-external"})["id"].(float64))
	internalA := int64(createArticleViaAPI(t, r, ticketA, map[string]any{"subject": "A internal", "body": "a-internal"})["id"].(float64))
	articleB := int64(createArticleViaAPI(t, r, ticketB, map[string]any{"subject": "B", "body": "b-body"})["id"].(float64))

	stamp := time.Now().UnixNano()
	noAccess, ok := createTestUserForRole(t, fmt.Sprintf("acl-none-%d", stamp))
	require.True(t, ok)
	readOnly, ok := createTestUserForRole(t, fmt.Sprintf("acl-ro-%d", stamp))
	require.True(t, ok)
	var groupID int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`SELECT group_id FROM queue WHERE id = 1`)).Scan(&groupID))
	_, err = db.Exec(database.ConvertPlaceholders(`
		INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
		VALUES (?, ?, 'ro', NOW(), 1, NOW(), 1)`), readOnly, groupID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM group_user WHERE user_id = ?`), readOnly)
	})

	path := func(ticket, article int64) string {
		if article == 0 {
			return fmt.Sprintf("/api/v1/tickets/%d/articles", ticket)
		}
		return fmt.Sprintf("/api/v1/tickets/%d/articles/%d", ticket, article)
	}
	as := func(user int) []string { return []string{"X-User", strconv.Itoa(user)} }
	subjectOf := func(t *testing.T, id int64) string { _, _, _, s := storedArticle(t, db, id); return s }

	t.Run("agent without queue permission sees nothing", func(t *testing.T) {
		code, _ := articleDoJSON(t, r, http.MethodGet, path(ticketA, 0), nil, as(noAccess)...)
		assert.Equal(t, http.StatusNotFound, code)
		code, _ = articleDoJSON(t, r, http.MethodGet, path(ticketA, visibleA), nil, as(noAccess)...)
		assert.Equal(t, http.StatusNotFound, code)
		code, _ = articleDoJSON(t, r, http.MethodPut, path(ticketA, visibleA), map[string]any{"subject": "hijack"}, as(noAccess)...)
		assert.Equal(t, http.StatusNotFound, code)
		code, _ = articleDoJSON(t, r, http.MethodDelete, path(ticketA, visibleA), nil, as(noAccess)...)
		assert.Equal(t, http.StatusNotFound, code)
		assert.Equal(t, "A visible", subjectOf(t, visibleA))
	})

	t.Run("read-only agent can read but not modify", func(t *testing.T) {
		code, resp := articleDoJSON(t, r, http.MethodGet, path(ticketA, 0), nil, as(readOnly)...)
		require.Equal(t, http.StatusOK, code, resp)
		assert.Equal(t, float64(2), resp["total"])
		code, _ = articleDoJSON(t, r, http.MethodPut, path(ticketA, visibleA), map[string]any{"subject": "ro edit"}, as(readOnly)...)
		assert.Equal(t, http.StatusNotFound, code)
		code, _ = articleDoJSON(t, r, http.MethodDelete, path(ticketA, internalA), nil, as(readOnly)...)
		assert.Equal(t, http.StatusNotFound, code)
		assert.Equal(t, "A visible", subjectOf(t, visibleA))
		assert.Equal(t, 1, articleCountRows(t, db, `SELECT COUNT(*) FROM article WHERE id = ?`, internalA))
	})

	t.Run("article of another ticket is not reachable through this ticket", func(t *testing.T) {
		code, _ := articleDoJSON(t, r, http.MethodGet, path(ticketA, articleB), nil, as(readOnly)...)
		assert.Equal(t, http.StatusNotFound, code)
		code, _ = articleDoJSON(t, r, http.MethodPut, path(ticketA, articleB), map[string]any{"subject": "cross"}, as(1)...)
		assert.Equal(t, http.StatusNotFound, code)
		code, _ = articleDoJSON(t, r, http.MethodDelete, path(ticketA, articleB), nil, as(1)...)
		assert.Equal(t, http.StatusNotFound, code)
		assert.Equal(t, "B", subjectOf(t, articleB))
	})

	t.Run("customer reads only own visible articles and cannot modify", func(t *testing.T) {
		cust := func(ticket, article int64) string {
			return "/customer-api/v1" + path(ticket, article)[len("/api/v1"):]
		}
		code, resp := articleDoJSON(t, r, http.MethodGet, cust(ticketA, 0), nil, "X-Customer", "acl-cust-a")
		require.Equal(t, http.StatusOK, code, resp)
		articles := resp["articles"].([]any)
		require.Len(t, articles, 1)
		assert.Equal(t, float64(visibleA), articles[0].(map[string]any)["id"])

		code, _ = articleDoJSON(t, r, http.MethodGet, cust(ticketA, internalA), nil, "X-Customer", "acl-cust-a")
		assert.Equal(t, http.StatusNotFound, code)
		code, _ = articleDoJSON(t, r, http.MethodGet, cust(ticketA, visibleA), nil, "X-Customer", "acl-cust-b")
		assert.Equal(t, http.StatusNotFound, code)
		code, _ = articleDoJSON(t, r, http.MethodGet, cust(ticketA, 0), nil, "X-Customer", "acl-cust-b")
		assert.Equal(t, http.StatusNotFound, code)
		code, _ = articleDoJSON(t, r, http.MethodPut, cust(ticketA, visibleA), map[string]any{"subject": "cust edit"}, "X-Customer", "acl-cust-a")
		assert.Equal(t, http.StatusForbidden, code)
		code, _ = articleDoJSON(t, r, http.MethodDelete, cust(ticketA, visibleA), nil, "X-Customer", "acl-cust-a")
		assert.Equal(t, http.StatusForbidden, code)
		assert.Equal(t, "A visible", subjectOf(t, visibleA))
	})
}
