package search

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// External backend flavors.
const (
	FlavorZinc          = "zinc"
	FlavorElasticsearch = "elasticsearch"
)

// DefaultIndexPrefix prefixes the names of the indices GoatFlow creates.
const DefaultIndexPrefix = "goatflow_"

// maxResultWindow is the largest from+size Elasticsearch accepts by default.
const maxResultWindow = 10000

// ExternalConfig configures an Elasticsearch-compatible search service.
type ExternalConfig struct {
	Flavor      string // FlavorZinc or FlavorElasticsearch
	Endpoint    string // base URL, e.g. http://zinc:4080
	Username    string // empty: no authentication
	Password    string
	IndexPrefix string // empty: DefaultIndexPrefix
}

// ConfigFromEnv reads the search backend configuration. It returns nil (and
// no error) when the database backend is selected.
//
//	SEARCH_BACKEND          "" or "database", "zinc", "elasticsearch"
//	ZINC_ENDPOINT, ZINC_USER, ZINC_PASSWORD
//	ELASTICSEARCH_ENDPOINT, ELASTICSEARCH_USERNAME, ELASTICSEARCH_PASSWORD
//	SEARCH_INDEX_PREFIX     index name prefix (default goatflow_)
func ConfigFromEnv() (*ExternalConfig, error) {
	cfg := &ExternalConfig{IndexPrefix: os.Getenv("SEARCH_INDEX_PREFIX")}
	var endpointVar string
	switch backend := strings.TrimSpace(os.Getenv("SEARCH_BACKEND")); backend {
	case "", "database":
		return nil, nil
	case FlavorZinc:
		endpointVar = "ZINC_ENDPOINT"
		cfg.Flavor = FlavorZinc
		cfg.Username = os.Getenv("ZINC_USER")
		cfg.Password = os.Getenv("ZINC_PASSWORD")
	case FlavorElasticsearch:
		endpointVar = "ELASTICSEARCH_ENDPOINT"
		cfg.Flavor = FlavorElasticsearch
		cfg.Username = os.Getenv("ELASTICSEARCH_USERNAME")
		cfg.Password = os.Getenv("ELASTICSEARCH_PASSWORD")
	default:
		return nil, fmt.Errorf("SEARCH_BACKEND=%q is not supported (use database, zinc or elasticsearch)", backend)
	}
	cfg.Endpoint = strings.TrimRight(strings.TrimSpace(os.Getenv(endpointVar)), "/")
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("SEARCH_BACKEND=%s but %s is not set", cfg.Flavor, endpointVar)
	}
	return cfg, nil
}

// ExternalBackend searches an Elasticsearch-compatible index (Zinc's /es API
// or Elasticsearch). The index holds one document per ticket, article and
// customer user; Sync and Rebuild keep it in step with the database.
//
// Every ticket and article hit is re-checked against the database before it
// is returned: hits whose row is gone, or whose ticket is now in a queue the
// caller may not read, are dropped. Results are therefore never wider than
// the database backend's, even while the index lags behind.
type ExternalBackend struct {
	cfg    ExternalConfig
	api    string // base URL of the Elasticsearch-compatible API
	client *http.Client

	readyMu    sync.Mutex
	readyUntil time.Time // index known to be ready until then

	syncMu sync.Mutex
	sync   syncState

	rebuildMu sync.Mutex
	rebuild   RebuildStatus
}

// NewExternalBackend returns a backend for cfg.
func NewExternalBackend(cfg ExternalConfig) *ExternalBackend {
	if cfg.IndexPrefix == "" {
		cfg.IndexPrefix = DefaultIndexPrefix
	}
	cfg.Endpoint = strings.TrimRight(cfg.Endpoint, "/")
	api := cfg.Endpoint
	if cfg.Flavor == FlavorZinc {
		api += "/es"
	}
	return &ExternalBackend{
		cfg:    cfg,
		api:    api,
		client: &http.Client{Timeout: 30 * time.Second},
		sync:   newSyncState(),
	}
}

// GetBackendName returns "zinc" or "elasticsearch".
func (b *ExternalBackend) GetBackendName() string { return b.cfg.Flavor }

// index names
func (b *ExternalBackend) index(docType string) string { return b.cfg.IndexPrefix + docType }

const (
	docTicket   = "ticket"
	docArticle  = "article"
	docCustomer = "customer"
	docMeta     = "meta"
)

// idField is the numeric id field of each document type.
var idField = map[string]string{
	docTicket:   "ticket_id",
	docArticle:  "article_id",
	docCustomer: "customer_user_id",
}

// mappings of the GoatFlow indices. Text fields are searched; keyword fields
// are only returned for display.
var mappings = map[string]map[string]string{
	docTicket: {
		"ticket_id": "long", "queue_id": "long", "state_id": "long",
		"tn": "text", "title": "text", "articles": "text",
		"queue": "keyword", "state": "keyword", "priority": "keyword", "created_at": "keyword",
	},
	docArticle: {
		"article_id": "long", "ticket_id": "long", "queue_id": "long",
		"subject": "text", "body": "text", "sender": "text",
		"ticket_number": "keyword", "ticket_title": "keyword", "created_at": "keyword",
	},
	docCustomer: {
		"customer_user_id": "long",
		"login":            "text", "email": "text", "first_name": "text", "last_name": "text",
		"company": "keyword", "created_at": "keyword",
	},
	docMeta: {
		"schema_version": "long", "ready": "keyword", "last_sync": "keyword",
	},
}

// searchFields are the fields matched per document type, with boosts that
// mirror the database backend's ranking (identifiers and titles first).
var searchFields = map[string][]struct {
	name  string
	boost float64
}{
	docTicket:   {{"tn", 4}, {"title", 3}, {"articles", 1}},
	docArticle:  {{"subject", 3}, {"body", 1}, {"sender", 1}},
	docCustomer: {{"login", 3}, {"email", 3}, {"first_name", 2}, {"last_name", 2}},
}

// do sends a request to the API. A non-nil out receives the decoded JSON body
// of a 2xx response. Transport failures and 401/403/5xx answers are returned
// as *ServiceError; other statuses are returned to the caller with the body.
func (b *ExternalBackend) do(ctx context.Context, method, path, contentType string, body []byte, out interface{}) (int, []byte, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, b.api+path, reader)
	if err != nil {
		return 0, nil, err
	}
	if b.cfg.Username != "" || b.cfg.Password != "" {
		req.SetBasicAuth(b.cfg.Username, b.cfg.Password)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := b.client.Do(req)
	if err != nil {
		return 0, nil, &ServiceError{Backend: b.cfg.Flavor, Reason: "service unreachable", Err: err}
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, &ServiceError{Backend: b.cfg.Flavor, Reason: "service unreachable", Err: err}
	}
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return resp.StatusCode, respBody, &ServiceError{Backend: b.cfg.Flavor, Reason: "authentication failed",
			Err: fmt.Errorf("%s %s: HTTP %d", method, path, resp.StatusCode)}
	case resp.StatusCode >= 500:
		return resp.StatusCode, respBody, &ServiceError{Backend: b.cfg.Flavor, Reason: "service error",
			Err: fmt.Errorf("%s %s: HTTP %d: %s", method, path, resp.StatusCode, truncate(string(respBody), 300))}
	}
	if out != nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return resp.StatusCode, respBody, &ServiceError{Backend: b.cfg.Flavor, Reason: "unexpected response",
				Err: fmt.Errorf("%s %s: %w", method, path, err)}
		}
	}
	return resp.StatusCode, respBody, nil
}

// doJSON sends a JSON request and fails on any non-2xx status.
func (b *ExternalBackend) doJSON(ctx context.Context, method, path string, payload, out interface{}) error {
	var body []byte
	if payload != nil {
		var err error
		if body, err = json.Marshal(payload); err != nil {
			return err
		}
	}
	status, respBody, err := b.do(ctx, method, path, "application/json", body, out)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return b.rejected(method, path, status, respBody)
	}
	return nil
}

func (b *ExternalBackend) rejected(method, path string, status int, body []byte) error {
	return &ServiceError{Backend: b.cfg.Flavor, Reason: "request rejected",
		Err: fmt.Errorf("%s %s: HTTP %d: %s", method, path, status, truncate(string(body), 300))}
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

// indexExists reports whether the named index exists.
func (b *ExternalBackend) indexExists(ctx context.Context, name string) (bool, error) {
	status, body, err := b.do(ctx, http.MethodHead, "/"+name, "", nil, nil)
	if err != nil {
		return false, err
	}
	switch status {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	}
	return false, b.rejected(http.MethodHead, "/"+name, status, body)
}

// ensureIndices creates the GoatFlow indices that do not exist yet.
func (b *ExternalBackend) ensureIndices(ctx context.Context) error {
	for _, docType := range []string{docTicket, docArticle, docCustomer, docMeta} {
		name := b.index(docType)
		exists, err := b.indexExists(ctx, name)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		props := map[string]interface{}{}
		for field, typ := range mappings[docType] {
			props[field] = map[string]string{"type": typ}
		}
		if err := b.doJSON(ctx, http.MethodPut, "/"+name,
			map[string]interface{}{"mappings": map[string]interface{}{"properties": props}}, nil); err != nil {
			return err
		}
	}
	return nil
}

// esHit is one hit of a search response.
type esHit struct {
	ID     string                 `json:"_id"`
	Score  *float64               `json:"_score"`
	Source map[string]interface{} `json:"_source"`
}

type esSearchResponse struct {
	Hits struct {
		Total struct {
			Value int `json:"value"`
		} `json:"total"`
		Hits []esHit `json:"hits"`
	} `json:"hits"`
}

// searchIndex runs body against one index.
func (b *ExternalBackend) searchIndex(ctx context.Context, docType string, body map[string]interface{}) (*esSearchResponse, error) {
	var resp esSearchResponse
	if err := b.doJSON(ctx, http.MethodPost, "/"+b.index(docType)+"/_search", body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// metaState is the bookkeeping document stored in the meta index.
type metaState struct {
	SchemaVersion int64  `json:"schema_version"`
	Ready         string `json:"ready"`     // "true" once a full build has completed
	LastSync      string `json:"last_sync"` // RFC 3339, end of the last successful sync
}

// schemaVersion changes whenever the document layout changes; an index built
// with another version is rebuilt.
const schemaVersion = 1

const metaID = "state"

// readMeta returns the stored state, or nil when the index has never been
// built (or was deleted).
func (b *ExternalBackend) readMeta(ctx context.Context) (*metaState, error) {
	exists, err := b.indexExists(ctx, b.index(docMeta))
	if err != nil || !exists {
		return nil, err
	}
	resp, err := b.searchIndex(ctx, docMeta, map[string]interface{}{
		"query": map[string]interface{}{"ids": map[string]interface{}{"values": []string{metaID}}},
		"size":  1,
	})
	if err != nil || len(resp.Hits.Hits) == 0 {
		return nil, err
	}
	raw, err := json.Marshal(resp.Hits.Hits[0].Source)
	if err != nil {
		return nil, err
	}
	var m metaState
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func (m *metaState) usable() bool {
	return m != nil && m.Ready == "true" && m.SchemaVersion == schemaVersion
}

// writeMeta stores the state document. Elasticsearch makes it searchable
// before answering; Zinc ignores the refresh parameter and shows it about a
// second later.
func (b *ExternalBackend) writeMeta(ctx context.Context, m metaState) error {
	return b.bulkTo(ctx, "/_bulk?refresh=wait_for", []bulkOp{{docType: docMeta, id: metaID, doc: m}})
}

// checkReady fails with ErrIndexNotReady until a full build has completed.
// A positive answer is cached briefly.
func (b *ExternalBackend) checkReady(ctx context.Context) error {
	b.readyMu.Lock()
	cached := time.Now().Before(b.readyUntil)
	b.readyMu.Unlock()
	if cached {
		return nil
	}
	meta, err := b.readMeta(ctx)
	if err != nil {
		return err
	}
	if !meta.usable() {
		return ErrIndexNotReady
	}
	b.markReady()
	return nil
}

func (b *ExternalBackend) markReady() {
	b.readyMu.Lock()
	b.readyUntil = time.Now().Add(30 * time.Second)
	b.readyMu.Unlock()
}

// HealthCheck verifies the service is reachable with the configured
// credentials and the index has been built.
func (b *ExternalBackend) HealthCheck(ctx context.Context) error {
	meta, err := b.readMeta(ctx)
	if err != nil {
		return err
	}
	if !meta.usable() {
		return ErrIndexNotReady
	}
	return nil
}

// IndexStatus describes the index for the health endpoint.
type IndexStatus struct {
	Ready     bool           `json:"ready"`
	LastSync  string         `json:"last_sync,omitempty"`
	Documents map[string]int `json:"documents,omitempty"`
}

// Status returns the index state and document counts.
func (b *ExternalBackend) Status(ctx context.Context) (IndexStatus, error) {
	meta, err := b.readMeta(ctx)
	if err != nil {
		return IndexStatus{}, err
	}
	st := IndexStatus{Ready: meta.usable()}
	if meta == nil {
		return st, nil
	}
	st.LastSync = meta.LastSync
	st.Documents = map[string]int{}
	for _, docType := range []string{docTicket, docArticle, docCustomer} {
		n, err := b.countDocs(ctx, docType)
		if err != nil {
			return st, err
		}
		st.Documents[docType] = n
	}
	return st, nil
}

// countDocs returns the number of documents in one index.
func (b *ExternalBackend) countDocs(ctx context.Context, docType string) (int, error) {
	resp, err := b.searchIndex(ctx, docType, map[string]interface{}{
		"query":            map[string]interface{}{"match_all": map[string]interface{}{}},
		"size":             0,
		"track_total_hits": true,
	})
	if err != nil {
		return 0, err
	}
	return resp.Hits.Total.Value, nil
}

// Search searches the requested entity types. As in the database backend,
// hits are concatenated in the order of query.Types, each type ordered by
// relevance, then paginated.
func (b *ExternalBackend) Search(ctx context.Context, query SearchQuery) (*SearchResults, error) {
	start := time.Now()
	results := &SearchResults{Query: query.Query, Hits: []SearchHit{}}
	if query.Limit <= 0 {
		query.Limit = 20
	}
	if query.Offset < 0 {
		query.Offset = 0
	}
	words := strings.Fields(strings.TrimSpace(query.Query))
	if len(words) == 0 {
		return results, nil
	}
	fetch := query.Offset + query.Limit
	if fetch > maxResultWindow {
		return nil, fmt.Errorf("%w: offset+limit must not exceed %d", ErrInvalidQuery, maxResultWindow)
	}
	filters, err := ticketFilters(query.Filters)
	if err != nil {
		return nil, err
	}
	if err := b.checkReady(ctx); err != nil {
		return nil, err
	}

	for _, docType := range query.Types {
		if _, ok := searchFields[docType]; !ok {
			continue
		}
		restricted := query.RestrictQueues && docType != docCustomer
		if restricted && len(query.QueueIDs) == 0 {
			continue
		}
		var filter []interface{}
		if restricted {
			filter = append(filter, map[string]interface{}{"terms": map[string]interface{}{"queue_id": query.QueueIDs}})
		}
		if docType == docTicket {
			filter = append(filter, filters...)
		}
		resp, err := b.searchIndex(ctx, docType, buildQuery(docType, words, filter, fetch))
		if err != nil {
			return nil, err
		}
		hits, dropped, err := b.verifyHits(ctx, docType, resp.Hits.Hits, query)
		if err != nil {
			return nil, err
		}
		for _, h := range hits {
			results.Hits = append(results.Hits, toSearchHit(docType, h, query))
		}
		results.TotalHits += resp.Hits.Total.Value - dropped
	}

	results.Took = time.Since(start).Milliseconds()
	if query.Offset < len(results.Hits) {
		end := min(query.Offset+query.Limit, len(results.Hits))
		results.Hits = results.Hits[query.Offset:end]
	} else {
		results.Hits = []SearchHit{}
	}
	return results, nil
}

// ticketFilters turns the queue_id/state_id filters into term filters.
func ticketFilters(filters map[string]string) ([]interface{}, error) {
	var out []interface{}
	for _, f := range []struct{ key, field string }{{"queue_id", "queue_id"}, {"state_id", "state_id"}} {
		raw, ok := filters[f.key]
		if !ok {
			continue
		}
		id, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("%w: filter %s must be an integer", ErrInvalidQuery, f.key)
		}
		out = append(out, map[string]interface{}{"term": map[string]interface{}{f.field: id}})
	}
	return out, nil
}

// buildQuery requires every word to match (all of its tokens) in at least one
// of the type's fields.
func buildQuery(docType string, words []string, filter []interface{}, size int) map[string]interface{} {
	must := make([]interface{}, 0, len(words))
	for _, w := range words {
		should := make([]interface{}, 0, len(searchFields[docType]))
		for _, f := range searchFields[docType] {
			should = append(should, map[string]interface{}{"match": map[string]interface{}{
				f.name: map[string]interface{}{"query": w, "operator": "and", "boost": f.boost},
			}})
		}
		must = append(must, map[string]interface{}{"bool": map[string]interface{}{
			"should": should, "minimum_should_match": 1,
		}})
	}
	boolQuery := map[string]interface{}{"must": must}
	if len(filter) > 0 {
		boolQuery["filter"] = filter
	}
	return map[string]interface{}{
		"query":            map[string]interface{}{"bool": boolQuery},
		"size":             size,
		"from":             0,
		"track_total_hits": true,
	}
}

func srcString(src map[string]interface{}, key string) string {
	if s, ok := src[key].(string); ok {
		return s
	}
	return ""
}

func toSearchHit(docType string, h esHit, query SearchQuery) SearchHit {
	src := h.Source
	hit := SearchHit{ID: h.ID, Type: docType}
	if h.Score != nil {
		hit.Score = *h.Score
	}
	switch docType {
	case docTicket:
		hit.Title = srcString(src, "title")
		hit.Content = hit.Title
		metadata := map[string]interface{}{
			"ticket_number": srcString(src, "tn"),
			"created_at":    srcString(src, "created_at"),
		}
		for _, k := range []string{"queue", "state", "priority"} {
			if v := srcString(src, k); v != "" {
				metadata[k] = v
			}
		}
		hit.Metadata = metadata
		if query.Highlight {
			hit.Highlights = map[string][]string{"title": {highlightText(hit.Title, query.Query)}}
		}
	case docArticle:
		hit.Title = srcString(src, "subject")
		hit.Content = srcString(src, "body")
		hit.Metadata = map[string]interface{}{
			"created_at":    srcString(src, "created_at"),
			"ticket_number": srcString(src, "ticket_number"),
			"ticket_title":  srcString(src, "ticket_title"),
		}
		if query.Highlight {
			hit.Highlights = map[string][]string{
				"subject": {highlightText(hit.Title, query.Query)},
				"body":    {highlightText(hit.Content, query.Query)},
			}
		}
		if len(hit.Content) > 200 {
			hit.Content = hit.Content[:200] + "..."
		}
	case docCustomer:
		login, email := srcString(src, "login"), srcString(src, "email")
		hit.Title = strings.TrimSpace(srcString(src, "first_name") + " " + srcString(src, "last_name"))
		if hit.Title == "" {
			hit.Title = login
		}
		hit.Content = "Email: " + email
		metadata := map[string]interface{}{
			"login":      login,
			"email":      email,
			"created_at": srcString(src, "created_at"),
		}
		if company := srcString(src, "company"); company != "" {
			metadata["company"] = company
		}
		hit.Metadata = metadata
	}
	return hit
}

// errBulk reports per-item failures of a bulk request.
var errBulk = errors.New("bulk request had failures")
