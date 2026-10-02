package search

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigFromEnv(t *testing.T) {
	clear := func(t *testing.T) {
		for _, k := range []string{"SEARCH_BACKEND", "SEARCH_INDEX_PREFIX", "ZINC_ENDPOINT", "ZINC_USER", "ZINC_PASSWORD",
			"ELASTICSEARCH_ENDPOINT", "ELASTICSEARCH_USERNAME", "ELASTICSEARCH_PASSWORD"} {
			t.Setenv(k, "")
		}
	}

	t.Run("database backend by default", func(t *testing.T) {
		clear(t)
		cfg, err := ConfigFromEnv()
		require.NoError(t, err)
		assert.Nil(t, cfg)
		t.Setenv("SEARCH_BACKEND", "database")
		t.Setenv("ZINC_ENDPOINT", "http://zinc:4080") // an endpoint alone selects nothing
		cfg, err = ConfigFromEnv()
		require.NoError(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("zinc", func(t *testing.T) {
		clear(t)
		t.Setenv("SEARCH_BACKEND", "zinc")
		_, err := ConfigFromEnv()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ZINC_ENDPOINT is not set")

		t.Setenv("ZINC_ENDPOINT", "http://zinc:4080/")
		t.Setenv("ZINC_USER", "zu")
		t.Setenv("ZINC_PASSWORD", "zp")
		cfg, err := ConfigFromEnv()
		require.NoError(t, err)
		assert.Equal(t, &ExternalConfig{Flavor: FlavorZinc, Endpoint: "http://zinc:4080", Username: "zu", Password: "zp"}, cfg)
	})

	t.Run("elasticsearch", func(t *testing.T) {
		clear(t)
		t.Setenv("SEARCH_BACKEND", "elasticsearch")
		t.Setenv("ELASTICSEARCH_ENDPOINT", "https://es:9200")
		t.Setenv("ELASTICSEARCH_USERNAME", "eu")
		t.Setenv("ELASTICSEARCH_PASSWORD", "ep")
		t.Setenv("SEARCH_INDEX_PREFIX", "tenant1_")
		cfg, err := ConfigFromEnv()
		require.NoError(t, err)
		assert.Equal(t, &ExternalConfig{Flavor: FlavorElasticsearch, Endpoint: "https://es:9200",
			Username: "eu", Password: "ep", IndexPrefix: "tenant1_"}, cfg)
	})

	t.Run("unknown backend is an error, not a silent fallback", func(t *testing.T) {
		clear(t)
		t.Setenv("SEARCH_BACKEND", "solr")
		_, err := ConfigFromEnv()
		require.Error(t, err)
		assert.Contains(t, err.Error(), `SEARCH_BACKEND="solr" is not supported`)
	})
}

// fakeService is an Elasticsearch-compatible API double that records requests.
type fakeService struct {
	mu       sync.Mutex
	requests []fakeRequest
	meta     string // _source of the meta document; "" = meta index missing
	respond  func(w http.ResponseWriter, r *http.Request, body []byte) bool
}

type fakeRequest struct {
	method, path, auth string
	body               map[string]interface{}
}

func (f *fakeService) server(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		req := fakeRequest{method: r.Method, path: r.URL.Path, auth: r.Header.Get("Authorization")}
		_ = json.Unmarshal(raw, &req.body)
		f.mu.Lock()
		f.requests = append(f.requests, req)
		f.mu.Unlock()
		if f.respond != nil && f.respond(w, r, raw) {
			return
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "_meta") && r.Method == http.MethodHead:
			if f.meta == "" {
				w.WriteHeader(http.StatusNotFound)
			}
		case strings.HasSuffix(r.URL.Path, "_meta/_search"):
			_, _ = io.WriteString(w, `{"hits":{"total":{"value":1},"hits":[{"_id":"state","_source":`+f.meta+`}]}}`)
		case strings.HasSuffix(r.URL.Path, "/_search"):
			_, _ = io.WriteString(w, `{"hits":{"total":{"value":0},"hits":[]}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func (f *fakeService) searches() map[string]map[string]interface{} {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[string]map[string]interface{}{}
	for _, r := range f.requests {
		if strings.HasSuffix(r.path, "/_search") && !strings.Contains(r.path, "_meta") {
			out[r.path] = r.body
		}
	}
	return out
}

const readyMeta = `{"schema_version":1,"ready":"true","last_sync":"2026-01-01T00:00:00Z"}`

func TestExternalSearchRestrictsTicketsAndArticlesToPermittedQueues(t *testing.T) {
	fake := &fakeService{meta: readyMeta}
	srv := fake.server(t)
	b := NewExternalBackend(ExternalConfig{Flavor: FlavorZinc, Endpoint: srv.URL, Username: "u", Password: "p"})

	res, err := b.Search(context.Background(), SearchQuery{Query: "printer toner",
		Types: []string{"ticket", "article", "customer"}, RestrictQueues: true, QueueIDs: []int{3, 5}})
	require.NoError(t, err)
	assert.Zero(t, res.TotalHits)

	searches := fake.searches()
	require.Len(t, searches, 3, "Zinc is queried through its /es API, one index per type")
	queueFilter := []interface{}{map[string]interface{}{"terms": map[string]interface{}{"queue_id": []interface{}{3.0, 5.0}}}}
	for _, path := range []string{"/es/goatflow_ticket/_search", "/es/goatflow_article/_search"} {
		require.Contains(t, searches, path)
		q := searches[path]["query"].(map[string]interface{})["bool"].(map[string]interface{})
		assert.Equal(t, queueFilter, q["filter"], path)
		assert.Len(t, q["must"], 2, "every word must match")
	}
	customer := searches["/es/goatflow_customer/_search"]["query"].(map[string]interface{})["bool"].(map[string]interface{})
	assert.NotContains(t, customer, "filter", "customers are not queue-restricted")

	fake.mu.Lock()
	defer fake.mu.Unlock()
	for _, r := range fake.requests {
		assert.Equal(t, "Basic dTpw", r.auth, "%s %s", r.method, r.path)
	}
}

func TestExternalSearchWithoutReadableQueuesSkipsTicketsAndArticles(t *testing.T) {
	fake := &fakeService{meta: readyMeta}
	srv := fake.server(t)
	b := NewExternalBackend(ExternalConfig{Flavor: FlavorElasticsearch, Endpoint: srv.URL})

	_, err := b.Search(context.Background(), SearchQuery{Query: "printer",
		Types: []string{"ticket", "article", "customer"}, RestrictQueues: true})
	require.NoError(t, err)
	searches := fake.searches()
	assert.Len(t, searches, 1)
	assert.Contains(t, searches, "/goatflow_customer/_search", "Elasticsearch is queried at the endpoint root")
}

func TestExternalSearchRefusesUnbuiltIndex(t *testing.T) {
	for name, meta := range map[string]string{
		"meta index missing":     "",
		"build not finished":     `{"schema_version":1,"ready":"false"}`,
		"other document version": `{"schema_version":99,"ready":"true"}`,
	} {
		t.Run(name, func(t *testing.T) {
			fake := &fakeService{meta: meta}
			srv := fake.server(t)
			b := NewExternalBackend(ExternalConfig{Flavor: FlavorZinc, Endpoint: srv.URL})
			_, err := b.Search(context.Background(), SearchQuery{Query: "printer", Types: []string{"ticket"}})
			assert.ErrorIs(t, err, ErrIndexNotReady)
			assert.ErrorIs(t, b.HealthCheck(context.Background()), ErrIndexNotReady)
			assert.Empty(t, fake.searches(), "no type index is searched")
		})
	}
}

func TestExternalSearchReportsServiceFailures(t *testing.T) {
	t.Run("unreachable", func(t *testing.T) {
		srv := httptest.NewServer(http.NotFoundHandler())
		srv.Close()
		b := NewExternalBackend(ExternalConfig{Flavor: FlavorZinc, Endpoint: srv.URL})
		_, err := b.Search(context.Background(), SearchQuery{Query: "printer", Types: []string{"ticket"}})
		var svcErr *ServiceError
		require.ErrorAs(t, err, &svcErr)
		assert.Equal(t, "service unreachable", svcErr.Reason)
		assert.Equal(t, FlavorZinc, svcErr.Backend)
		require.ErrorAs(t, b.HealthCheck(context.Background()), &svcErr)
	})

	t.Run("wrong credentials", func(t *testing.T) {
		fake := &fakeService{meta: readyMeta, respond: func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
			w.WriteHeader(http.StatusUnauthorized)
			return true
		}}
		srv := fake.server(t)
		b := NewExternalBackend(ExternalConfig{Flavor: FlavorElasticsearch, Endpoint: srv.URL, Username: "x", Password: "y"})
		_, err := b.Search(context.Background(), SearchQuery{Query: "printer", Types: []string{"ticket"}})
		var svcErr *ServiceError
		require.ErrorAs(t, err, &svcErr)
		assert.Equal(t, "authentication failed", svcErr.Reason)
	})

	t.Run("type index removed after the build", func(t *testing.T) {
		fake := &fakeService{meta: readyMeta, respond: func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
			if r.URL.Path == "/goatflow_ticket/_search" {
				w.WriteHeader(http.StatusNotFound)
				_, _ = io.WriteString(w, `{"error":{"type":"index_not_found_exception"}}`)
				return true
			}
			return false
		}}
		srv := fake.server(t)
		b := NewExternalBackend(ExternalConfig{Flavor: FlavorElasticsearch, Endpoint: srv.URL})
		_, err := b.Search(context.Background(), SearchQuery{Query: "printer", Types: []string{"ticket"}})
		var svcErr *ServiceError
		require.ErrorAs(t, err, &svcErr, "an error, never an empty result")
		assert.Equal(t, "request rejected", svcErr.Reason)
	})
}

func TestExternalSearchRejectsInvalidQueries(t *testing.T) {
	fake := &fakeService{meta: readyMeta}
	srv := fake.server(t)
	b := NewExternalBackend(ExternalConfig{Flavor: FlavorZinc, Endpoint: srv.URL})
	ctx := context.Background()

	_, err := b.Search(ctx, SearchQuery{Query: "printer", Types: []string{"ticket"}, Filters: map[string]string{"queue_id": "x"}})
	assert.ErrorIs(t, err, ErrInvalidQuery)
	_, err = b.Search(ctx, SearchQuery{Query: "printer", Types: []string{"ticket"}, Offset: maxResultWindow, Limit: 1})
	assert.ErrorIs(t, err, ErrInvalidQuery)

	_, err = b.Search(ctx, SearchQuery{Query: "printer", Types: []string{"ticket"}, Filters: map[string]string{"queue_id": "4", "state_id": "2"}})
	require.NoError(t, err)
	q := fake.searches()["/es/goatflow_ticket/_search"]["query"].(map[string]interface{})["bool"].(map[string]interface{})
	assert.ElementsMatch(t, []interface{}{
		map[string]interface{}{"term": map[string]interface{}{"queue_id": 4.0}},
		map[string]interface{}{"term": map[string]interface{}{"state_id": 2.0}},
	}, q["filter"])
}

func TestBulkReportsItemFailures(t *testing.T) {
	var body string
	fake := &fakeService{respond: func(w http.ResponseWriter, r *http.Request, raw []byte) bool {
		if r.URL.Path != "/_bulk" {
			return false
		}
		body = string(raw)
		_, _ = io.WriteString(w, `{"errors":true,"items":[
			{"delete":{"_id":"7","status":404}},
			{"index":{"_id":"8","status":400,"error":{"type":"mapper_parsing_exception"}}}]}`)
		return true
	}}
	srv := fake.server(t)
	b := NewExternalBackend(ExternalConfig{Flavor: FlavorElasticsearch, Endpoint: srv.URL})

	err := b.bulk(context.Background(), []bulkOp{
		{docType: docTicket, id: "7"},
		{docType: docTicket, id: "8", doc: map[string]string{"title": "x"}},
	})
	require.ErrorIs(t, err, errBulk)
	assert.Contains(t, err.Error(), "index 8: HTTP 400")
	assert.Equal(t, `{"delete":{"_id":"7","_index":"goatflow_ticket"}}`+"\n"+
		`{"index":{"_id":"8","_index":"goatflow_ticket"}}`+"\n"+`{"title":"x"}`+"\n", body)

	// A delete of a document that is already gone is not a failure.
	fake.respond = func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		_, _ = io.WriteString(w, `{"errors":true,"items":[{"delete":{"_id":"7","status":404}}]}`)
		return true
	}
	assert.NoError(t, b.bulk(context.Background(), []bulkOp{{docType: docTicket, id: "7"}}))
}
