package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/search"
)

// useExternalSearch makes the handlers use backend (and cfgErr) for one test.
func useExternalSearch(t *testing.T, backend *search.ExternalBackend, cfgErr error) {
	t.Helper()
	prevBackend, prevErr := externalSearch, externalSearchErr
	externalSearch, externalSearchErr = backend, cfgErr
	t.Cleanup(func() { externalSearch, externalSearchErr = prevBackend, prevErr })
}

func searchAPIRequest(t *testing.T, payload map[string]any) *http.Request {
	t.Helper()
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/search", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	return req
}

var (
	adminCtx = map[string]any{"user_id": 1, "is_queue_admin": true, "isInAdminGroup": true}
	agentCtx = map[string]any{"user_id": 2, "user_role": "Agent", "is_queue_admin": false, "accessible_queue_ids": []uint{1}}
)

func TestHandleReindexAPI_RequiresAdmin(t *testing.T) {
	useExternalSearch(t, nil, nil)
	reindex := func(ctx map[string]any) (int, map[string]any) {
		r := readRouter(http.MethodPost, "/api/v1/search/reindex", HandleReindexAPI, ctx)
		return serveJSON(t, r, httptest.NewRequest(http.MethodPost, "/api/v1/search/reindex", nil))
	}

	code, body := reindex(agentCtx)
	assert.Equal(t, http.StatusForbidden, code)
	assert.Equal(t, map[string]any{"success": false, "error": "Admin access required"}, body)

	code, body = reindex(map[string]any{"user_id": 1, "isInAdminGroup": true})
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "database", body["backend"])

	code, _ = reindex(map[string]any{"user_id": 3, "user_role": "Admin"})
	assert.Equal(t, http.StatusOK, code)
}

// esDouble answers like an Elasticsearch whose index has been built and holds
// no matching documents, recording search bodies by path.
type esDouble struct {
	mu       sync.Mutex
	searches map[string]map[string]any
	ready    bool
	gate     chan struct{} // when set, write requests wait for it to close
}

func (e *esDouble) serve(t *testing.T) *httptest.Server {
	e.searches = map[string]map[string]any{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		switch {
		case r.Method == http.MethodHead:
			if e.gate != nil {
				<-e.gate
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if !e.ready {
				w.WriteHeader(http.StatusNotFound)
			}
		case strings.HasSuffix(r.URL.Path, "_meta/_search"):
			_, _ = io.WriteString(w, `{"hits":{"total":{"value":1},"hits":[{"_id":"state","_source":{"schema_version":1,"ready":"true","last_sync":"2026-01-01T00:00:00Z"}}]}}`)
		case strings.HasSuffix(r.URL.Path, "/_search"):
			var body map[string]any
			_ = json.Unmarshal(raw, &body)
			e.mu.Lock()
			e.searches[r.URL.Path] = body
			e.mu.Unlock()
			_, _ = io.WriteString(w, `{"hits":{"total":{"value":0},"hits":[]}}`)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestHandleSearchAPI_ExternalBackend(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	readTestDB(t)

	t.Run("agent searches are limited to readable queues", func(t *testing.T) {
		es := &esDouble{ready: true}
		srv := es.serve(t)
		useExternalSearch(t, search.NewExternalBackend(search.ExternalConfig{Flavor: search.FlavorElasticsearch, Endpoint: srv.URL}), nil)

		r := readRouter(http.MethodPost, "/api/v1/search", HandleSearchAPI, agentCtx)
		code, body := serveJSON(t, r, searchAPIRequest(t, map[string]any{"query": "printer", "types": []string{"ticket", "customer"}}))
		require.Equal(t, http.StatusOK, code, "%v", body)
		assert.Equal(t, float64(0), body["total_hits"])

		es.mu.Lock()
		defer es.mu.Unlock()
		ticket := es.searches["/goatflow_ticket/_search"]["query"].(map[string]any)["bool"].(map[string]any)
		assert.Equal(t, []any{map[string]any{"terms": map[string]any{"queue_id": []any{1.0}}}}, ticket["filter"])
		customer := es.searches["/goatflow_customer/_search"]["query"].(map[string]any)["bool"].(map[string]any)
		assert.NotContains(t, customer, "filter")
	})

	t.Run("an unbuilt index is a 503, not an empty result", func(t *testing.T) {
		es := &esDouble{}
		srv := es.serve(t)
		useExternalSearch(t, search.NewExternalBackend(search.ExternalConfig{Flavor: search.FlavorZinc, Endpoint: srv.URL}), nil)

		r := readRouter(http.MethodPost, "/api/v1/search", HandleSearchAPI, adminCtx)
		code, body := serveJSON(t, r, searchAPIRequest(t, map[string]any{"query": "printer"}))
		assert.Equal(t, http.StatusServiceUnavailable, code)
		assert.Equal(t, "zinc", body["backend"])
		assert.Contains(t, body["error"], "search index has not been built yet")

		h := readRouter(http.MethodGet, "/api/v1/search/health", HandleSearchHealthAPI, adminCtx)
		code, body = serveJSON(t, h, httptest.NewRequest(http.MethodGet, "/api/v1/search/health", nil))
		assert.Equal(t, http.StatusServiceUnavailable, code)
		assert.Equal(t, "indexing", body["status"])
		assert.Equal(t, false, body["index"].(map[string]any)["ready"])
	})

	t.Run("an unreachable service is a 503 naming the backend", func(t *testing.T) {
		srv := httptest.NewServer(http.NotFoundHandler())
		srv.Close()
		useExternalSearch(t, search.NewExternalBackend(search.ExternalConfig{Flavor: search.FlavorElasticsearch, Endpoint: srv.URL}), nil)

		r := readRouter(http.MethodPost, "/api/v1/search", HandleSearchAPI, adminCtx)
		code, body := serveJSON(t, r, searchAPIRequest(t, map[string]any{"query": "printer"}))
		assert.Equal(t, http.StatusServiceUnavailable, code)
		assert.Equal(t, map[string]any{"backend": "elasticsearch",
			"error": "Search backend elasticsearch is unavailable: service unreachable"}, body)

		h := readRouter(http.MethodGet, "/api/v1/search/health", HandleSearchHealthAPI, adminCtx)
		code, body = serveJSON(t, h, httptest.NewRequest(http.MethodGet, "/api/v1/search/health", nil))
		assert.Equal(t, http.StatusServiceUnavailable, code)
		assert.Equal(t, "unhealthy", body["status"])
		assert.Equal(t, "Search backend elasticsearch is unavailable: service unreachable", body["error"])
	})

	t.Run("a misconfigured backend fails every endpoint", func(t *testing.T) {
		useExternalSearch(t, nil, errors.New("SEARCH_BACKEND=zinc but ZINC_ENDPOINT is not set"))
		want := "Search backend is misconfigured: SEARCH_BACKEND=zinc but ZINC_ENDPOINT is not set"

		r := readRouter(http.MethodPost, "/api/v1/search", HandleSearchAPI, adminCtx)
		code, body := serveJSON(t, r, searchAPIRequest(t, map[string]any{"query": "printer"}))
		assert.Equal(t, http.StatusServiceUnavailable, code)
		assert.Equal(t, want, body["error"])

		x := readRouter(http.MethodPost, "/api/v1/search/reindex", HandleReindexAPI, adminCtx)
		code, body = serveJSON(t, x, httptest.NewRequest(http.MethodPost, "/api/v1/search/reindex", nil))
		assert.Equal(t, http.StatusServiceUnavailable, code)
		assert.Equal(t, want, body["error"])

		h := readRouter(http.MethodGet, "/api/v1/search/health", HandleSearchHealthAPI, adminCtx)
		code, body = serveJSON(t, h, httptest.NewRequest(http.MethodGet, "/api/v1/search/health", nil))
		assert.Equal(t, http.StatusServiceUnavailable, code)
		assert.Equal(t, want, body["error"])
	})

	t.Run("reindex runs in the background, once at a time, and reports failures", func(t *testing.T) {
		es := &esDouble{gate: make(chan struct{})}
		srv := es.serve(t)
		backend := search.NewExternalBackend(search.ExternalConfig{Flavor: search.FlavorElasticsearch, Endpoint: srv.URL})
		useExternalSearch(t, backend, nil)
		x := readRouter(http.MethodPost, "/api/v1/search/reindex", HandleReindexAPI, adminCtx)

		code, body := serveJSON(t, x, httptest.NewRequest(http.MethodPost, "/api/v1/search/reindex", nil))
		assert.Equal(t, http.StatusAccepted, code)
		assert.Equal(t, "Reindex started", body["message"])
		assert.Equal(t, true, body["reindex"].(map[string]any)["running"])

		code, body = serveJSON(t, x, httptest.NewRequest(http.MethodPost, "/api/v1/search/reindex", nil))
		assert.Equal(t, http.StatusConflict, code)
		assert.Equal(t, "A reindex is already running", body["error"])

		close(es.gate) // the service now fails every request
		require.Eventually(t, func() bool { return !backend.RebuildStatus().Running }, 5*time.Second, 20*time.Millisecond)

		h := readRouter(http.MethodGet, "/api/v1/search/health", HandleSearchHealthAPI, adminCtx)
		code, body = serveJSON(t, h, httptest.NewRequest(http.MethodGet, "/api/v1/search/health", nil))
		assert.Equal(t, http.StatusServiceUnavailable, code)
		assert.Equal(t, "unhealthy", body["status"])
		reindex := body["reindex"].(map[string]any)
		assert.Equal(t, false, reindex["running"])
		assert.Contains(t, reindex["error"], "elasticsearch: service error")
	})
}
