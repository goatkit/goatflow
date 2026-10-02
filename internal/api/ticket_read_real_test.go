package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
)

func readTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)
	return db
}

// readRouter registers h at path with the given context values set first,
// standing in for the auth and queue_ro middleware.
func readRouter(method, path string, h gin.HandlerFunc, ctx map[string]any) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		for k, v := range ctx {
			c.Set(k, v)
		}
		c.Next()
	})
	r.Handle(method, path, h)
	return r
}

func serveJSON(t *testing.T, r *gin.Engine, req *http.Request) (int, map[string]any) {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), "body: %s", w.Body.String())
	return w.Code, body
}

func TestHandleGetTicketAPI_RealPath(t *testing.T) {
	db := readTestDB(t)
	title := fmt.Sprintf("ReadFake get %d", time.Now().UnixNano())
	id := createStateTicket(t, db, "open", "", title, 0)
	path := "/api/v1/tickets/" + strconv.FormatInt(id, 10)

	t.Run("unauthenticated request is rejected even with X-Test-Mode", func(t *testing.T) {
		r := readRouter(http.MethodGet, "/api/v1/tickets/:id", HandleGetTicketAPI, nil)
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Test-Mode", "true")
		req.Header.Set("X-Test-NotFound", "true")
		code, body := serveJSON(t, r, req)
		assert.Equal(t, http.StatusUnauthorized, code)
		assert.Equal(t, false, body["success"])
	})

	auth := map[string]any{"user_id": 1, "user_role": "Agent"}

	t.Run("existing ticket returns stored row", func(t *testing.T) {
		r := readRouter(http.MethodGet, "/api/v1/tickets/:id", HandleGetTicketAPI, auth)
		code, body := serveJSON(t, r, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusOK, code, "%v", body)
		data := body["data"].(map[string]any)
		assert.Equal(t, float64(id), data["id"])
		assert.Equal(t, title, data["title"])
		assert.Equal(t, "open", data["state"])
		assert.Equal(t, float64(0), data["article_count"])
	})

	t.Run("large unknown id is a real 404", func(t *testing.T) {
		r := readRouter(http.MethodGet, "/api/v1/tickets/:id", HandleGetTicketAPI, auth)
		code, body := serveJSON(t, r, httptest.NewRequest(http.MethodGet, "/api/v1/tickets/2000000000", nil))
		assert.Equal(t, http.StatusNotFound, code)
		assert.Equal(t, "Ticket not found", body["error"])
	})

	t.Run("non-numeric id is 400, no fake new-ticket form", func(t *testing.T) {
		r := readRouter(http.MethodGet, "/api/v1/tickets/:id", HandleGetTicketAPI, auth)
		code, _ := serveJSON(t, r, httptest.NewRequest(http.MethodGet, "/api/v1/tickets/new", nil))
		assert.Equal(t, http.StatusBadRequest, code)
	})
}

func TestHandleListTicketsAPI_RequiresAuth(t *testing.T) {
	r := readRouter(http.MethodGet, "/api/v1/tickets", HandleListTicketsAPI, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets", nil)
	req.Header.Set("X-Test-Mode", "true")
	code, body := serveJSON(t, r, req)
	assert.Equal(t, http.StatusUnauthorized, code)
	assert.Equal(t, false, body["success"])
}

func TestHandleListTicketsAPI_ReturnsStoredTicket(t *testing.T) {
	db := readTestDB(t)
	title := fmt.Sprintf("ReadFake list %d", time.Now().UnixNano())
	id := createStateTicket(t, db, "open", "", title, 0)

	r := readRouter(http.MethodGet, "/api/tickets", HandleListTicketsAPI,
		map[string]any{"user_id": 1, "is_queue_admin": true})
	code, body := serveJSON(t, r, httptest.NewRequest(http.MethodGet, "/api/tickets?search="+url.QueryEscape(title), nil))
	require.Equal(t, http.StatusOK, code, "%v", body)
	rows := body["data"].([]any)
	require.Len(t, rows, 1)
	row := rows[0].(map[string]any)
	assert.Equal(t, float64(id), row["id"])
	assert.Equal(t, title, row["title"])
}

func TestHandleSearchTickets_RealPath(t *testing.T) {
	db := readTestDB(t)
	word := fmt.Sprintf("rfsearch%d", time.Now().UnixNano())
	id := createStateTicket(t, db, "open", "", "Printer "+word+" jammed", 0)
	var tn string
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`SELECT tn FROM ticket WHERE id = ?`), id).Scan(&tn))

	search := func(ctx map[string]any, q string) (int, map[string]any) {
		r := readRouter(http.MethodGet, "/api/tickets/search", handleSearchTickets, ctx)
		return serveJSON(t, r, httptest.NewRequest(http.MethodGet, "/api/tickets/search?q="+url.QueryEscape(q), nil))
	}

	t.Run("finds ticket in an accessible queue", func(t *testing.T) {
		code, body := search(map[string]any{"user_id": 1, "accessible_queue_ids": []uint{1}}, word)
		require.Equal(t, http.StatusOK, code, "%v", body)
		results := body["results"].([]any)
		require.Len(t, results, 1)
		assert.Equal(t, tn, results[0].(map[string]any)["id"])
		assert.Equal(t, float64(1), body["total"])
	})

	t.Run("hides tickets in queues the agent cannot read", func(t *testing.T) {
		code, body := search(map[string]any{"user_id": 2, "accessible_queue_ids": []uint{999999}}, word)
		require.Equal(t, http.StatusOK, code, "%v", body)
		assert.Empty(t, body["results"])
	})

	t.Run("queue admin sees all queues", func(t *testing.T) {
		code, body := search(map[string]any{"user_id": 1, "is_queue_admin": true}, word)
		require.Equal(t, http.StatusOK, code, "%v", body)
		assert.Len(t, body["results"], 1)
	})

	t.Run("empty query is 400, not a fake marker", func(t *testing.T) {
		code, body := search(map[string]any{"user_id": 1, "is_queue_admin": true}, "  ")
		assert.Equal(t, http.StatusBadRequest, code)
		assert.Equal(t, "Search query is required", body["error"])
	})
}

func TestHandleFilterTickets_RestrictsToAccessibleQueues(t *testing.T) {
	db := readTestDB(t)
	id := createStateTicket(t, db, "open", "", fmt.Sprintf("ReadFake filter %d", time.Now().UnixNano()), 0)
	var tn string
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`SELECT tn FROM ticket WHERE id = ?`), id).Scan(&tn))

	filter := func(ctx map[string]any) []any {
		r := readRouter(http.MethodGet, "/api/tickets/filter", handleFilterTickets, ctx)
		code, body := serveJSON(t, r, httptest.NewRequest(http.MethodGet, "/api/tickets/filter?queue=1&agent=1", nil))
		require.Equal(t, http.StatusOK, code, "%v", body)
		return body["tickets"].([]any)
	}
	contains := func(list []any) bool {
		for _, it := range list {
			if it.(map[string]any)["id"] == tn {
				return true
			}
		}
		return false
	}

	assert.False(t, contains(filter(map[string]any{"user_id": 2, "accessible_queue_ids": []uint{999999}})))
	assert.True(t, contains(filter(map[string]any{"user_id": 1, "accessible_queue_ids": []uint{1}})))
}

func TestHandleSearchAPI_FindsStoredTicket(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	db := readTestDB(t)
	word := fmt.Sprintf("rfapisearch%d", time.Now().UnixNano())
	id := createStateTicket(t, db, "open", "", "Router "+word+" offline", 0)

	r := readRouter(http.MethodPost, "/api/v1/search", HandleSearchAPI, map[string]any{"user_id": 1})
	payload, err := json.Marshal(map[string]any{"query": word, "types": []string{"ticket"}, "limit": 10})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/search", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	code, body := serveJSON(t, r, req)
	require.Equal(t, http.StatusOK, code, "%v", body)
	hits := body["hits"].([]any)
	require.NotEmpty(t, hits, "database backend must find the stored ticket")
	assert.Equal(t, strconv.FormatInt(id, 10), hits[0].(map[string]any)["id"])
}
