package client_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/goatkit/goatflow/sdk/go/auth"
	"github.com/goatkit/goatflow/sdk/go/client"
	sdkerrors "github.com/goatkit/goatflow/sdk/go/errors"
	"github.com/goatkit/goatflow/sdk/go/types"
)

// fixture returns sdk/testdata/<name>.json, a response body shared by the Go,
// TypeScript and Python SDK tests. GET bodies were captured from a running
// GoatFlow server; bodies of endpoints that change data are the JSON literal
// the handler writes (internal/api, internal/platform/api).
func fixture(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", name+".json"))
	if err != nil {
		t.Fatalf("fixture %s: %v", name, err)
	}
	return string(raw)
}

type recorded struct {
	method, path, rawQuery, authorization, contentType string
	body                                               map[string]interface{}
}

// serve starts a server that answers every request with status and body and
// records the last request.
func serve(t *testing.T, status int, body string) (*client.Client, *recorded) {
	t.Helper()
	rec := &recorded{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.method, rec.path, rec.rawQuery = r.Method, r.URL.Path, r.URL.RawQuery
		rec.authorization, rec.contentType = r.Header.Get("Authorization"), r.Header.Get("Content-Type")
		raw, _ := io.ReadAll(r.Body) //nolint:errcheck // test server
		rec.body = nil
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &rec.body); err != nil {
				t.Errorf("request body is not JSON: %q", raw)
			}
		}
		if body != "" {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
		}
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body) //nolint:errcheck // test server
	}))
	t.Cleanup(srv.Close)
	return client.NewClientWithAPIKey(srv.URL, "gf_test_token"), rec
}

func TestTicketsListUnwrapsEnvelopeAndPagination(t *testing.T) {
	c, rec := serve(t, http.StatusOK, fixture(t, "ticket_list"))

	list, err := c.Tickets.List(context.Background(), &types.TicketListOptions{
		PerPage: 2, Status: "open", QueueID: 37, Include: []string{"article_count", "last_article"},
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if rec.method != http.MethodGet || rec.path != "/api/v1/tickets" {
		t.Errorf("request = %s %s", rec.method, rec.path)
	}
	if want := "include=article_count%2Clast_article&per_page=2&queue_id=37&status=open"; rec.rawQuery != want {
		t.Errorf("query = %q, want %q", rec.rawQuery, want)
	}
	if rec.authorization != "Bearer gf_test_token" {
		t.Errorf("Authorization = %q, want API token as bearer", rec.authorization)
	}
	if len(list.Tickets) != 1 {
		t.Fatalf("got %d tickets, want 1", len(list.Tickets))
	}
	tk := list.Tickets[0]
	if tk.ID != 37558 || tk.TicketNumber != "COACH-9664946" || tk.StateName != "new" || tk.QueueName != "Coaching" {
		t.Errorf("ticket = %+v", tk)
	}
	if tk.ResponsibleUserID == nil || *tk.ResponsibleUserID != 14 {
		t.Errorf("ResponsibleUserID = %v, want 14", tk.ResponsibleUserID)
	}
	if !tk.CreatedAt.Equal(time.Date(2026, 9, 1, 15, 23, 18, 0, time.UTC)) {
		t.Errorf("CreatedAt = %v", tk.CreatedAt)
	}
	want := types.Pagination{Page: 1, PerPage: 2, Total: 37445, TotalPages: 18723, HasNext: true}
	if list.Pagination != want {
		t.Errorf("Pagination = %+v, want %+v", list.Pagination, want)
	}
}

func TestTicketsGetUnwrapsData(t *testing.T) {
	c, rec := serve(t, http.StatusOK, fixture(t, "ticket_get"))

	tk, err := c.Tickets.Get(context.Background(), 37558)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rec.path != "/api/v1/tickets/37558" {
		t.Errorf("path = %s", rec.path)
	}
	if tk.ID != 37558 || tk.Title != "E2E identity check" || tk.State != "new" || tk.Queue != "Coaching" ||
		tk.Priority != "3 normal" || tk.OwnerUserID != 14 || tk.ArticleCount != 1 || tk.CustomerUserID != "emma.robinson.1006" {
		t.Errorf("ticket = %+v", tk)
	}
	if tk.TypeID != nil {
		t.Errorf("TypeID = %v, want nil when the API omits it", *tk.TypeID)
	}
}

func TestErrorEnvelopeBecomesAPIError(t *testing.T) {
	c, _ := serve(t, http.StatusNotFound, fixture(t, "ticket_not_found"))

	tk, err := c.Tickets.Get(context.Background(), 999999999)
	if tk != nil {
		t.Errorf("ticket = %+v, want nil on error", tk)
	}
	apiErr, ok := sdkerrors.AsAPIError(err)
	if !ok {
		t.Fatalf("err = %T %v, want *APIError", err, err)
	}
	if apiErr.StatusCode != http.StatusNotFound || apiErr.Message != "Ticket not found" || apiErr.Code != "" {
		t.Errorf("APIError = %+v", apiErr)
	}
	if !sdkerrors.IsNotFound(err) {
		t.Error("IsNotFound = false")
	}
}

func TestStructuredAuthErrorBecomesAPIError(t *testing.T) {
	c, _ := serve(t, http.StatusUnauthorized, fixture(t, "invalid_token"))

	_, err := c.Users.Me(context.Background())
	apiErr, ok := sdkerrors.AsAPIError(err)
	if !ok {
		t.Fatalf("err = %T %v, want *APIError", err, err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized || apiErr.Code != "core:invalid_token" || apiErr.Message != "Invalid or malformed token" {
		t.Errorf("APIError = %+v", apiErr)
	}
	if !sdkerrors.IsUnauthorized(err) {
		t.Error("IsUnauthorized = false")
	}
}

func TestSuccessFalseWith2xxIsAnError(t *testing.T) {
	c, _ := serve(t, http.StatusOK, `{"success":false,"error":"Database unavailable"}`)

	_, err := c.Queues.Get(context.Background(), 1)
	apiErr, ok := sdkerrors.AsAPIError(err)
	if !ok || apiErr.StatusCode != http.StatusOK || apiErr.Message != "Database unavailable" {
		t.Fatalf("err = %v, want APIError(200, Database unavailable)", err)
	}
}

func TestRedirectIsNotFollowed(t *testing.T) {
	c, _ := serve(t, http.StatusSeeOther, `<a href="/login">See Other</a>.`)

	_, err := c.Tickets.List(context.Background(), nil)
	apiErr, ok := sdkerrors.AsAPIError(err)
	if !ok {
		t.Fatalf("err = %T %v, want *APIError", err, err)
	}
	if apiErr.StatusCode != http.StatusSeeOther || apiErr.Message != "See Other" || apiErr.Body != `<a href="/login">See Other</a>.` {
		t.Errorf("APIError = %+v", apiErr)
	}
}

func TestTicketsCreateSendsBodyAndDecodesCreated(t *testing.T) {
	c, rec := serve(t, http.StatusCreated, fixture(t, "ticket_created"))

	created, err := c.Tickets.Create(context.Background(), &types.TicketCreateRequest{
		Title: "Printer on fire", QueueID: 37, Body: "Smoke everywhere", PriorityID: 3,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if rec.method != http.MethodPost || rec.path != "/api/v1/tickets" || rec.contentType != "application/json" {
		t.Errorf("request = %s %s (%s)", rec.method, rec.path, rec.contentType)
	}
	wantBody := map[string]interface{}{"title": "Printer on fire", "queue_id": float64(37), "body": "Smoke everywhere", "priority_id": float64(3)}
	if !equalJSON(rec.body, wantBody) {
		t.Errorf("request body = %v, want %v", rec.body, wantBody)
	}
	if created.ID != 37559 || created.TN != "2026100110000017" || created.StateID != 1 || created.PriorityID != 3 {
		t.Errorf("created = %+v", created)
	}
}

func TestTicketsDeleteAccepts204(t *testing.T) {
	c, rec := serve(t, http.StatusNoContent, "")

	if err := c.Tickets.Delete(context.Background(), 37558); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if rec.method != http.MethodDelete || rec.path != "/api/v1/tickets/37558" {
		t.Errorf("request = %s %s", rec.method, rec.path)
	}
}

func TestTicketsReopenDecodesTopLevelFields(t *testing.T) {
	c, rec := serve(t, http.StatusOK, fixture(t, "reopen"))

	res, err := c.Tickets.Reopen(context.Background(), 37558, "Customer replied")
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	if rec.path != "/api/v1/tickets/37558/reopen" || rec.body["reason"] != "Customer replied" {
		t.Errorf("request = %s %v", rec.path, rec.body)
	}
	if res.ID != 37558 || res.StateID != 4 || res.State != "open" {
		t.Errorf("result = %+v", res)
	}
}

func TestArticlesListDecodesBareResponse(t *testing.T) {
	c, rec := serve(t, http.StatusOK, fixture(t, "article_list"))

	list, err := c.Articles.List(context.Background(), 37558, true)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if rec.path != "/api/v1/tickets/37558/articles" || rec.rawQuery != "include_attachments=true" {
		t.Errorf("request = %s?%s", rec.path, rec.rawQuery)
	}
	if list.Total != 1 || len(list.Articles) != 1 {
		t.Fatalf("list = %+v", list)
	}
	a := list.Articles[0]
	if a.ID != 91 || a.ArticleType != "note-internal" || a.IsVisibleForCustomer || a.Subject != "Call back" {
		t.Errorf("article = %+v", a)
	}
	if len(a.Attachments) != 1 || a.Attachments[0].Filename != "log.txt" || a.Attachments[0].Size != 120 {
		t.Errorf("attachments = %+v", a.Attachments)
	}
}

func TestUsersMeAndQueuesGet(t *testing.T) {
	c, rec := serve(t, http.StatusOK, fixture(t, "user_me"))
	me, err := c.Users.Me(context.Background())
	if err != nil {
		t.Fatalf("Me: %v", err)
	}
	if rec.path != "/api/v1/users/me" || me.ID != 1 || me.Login != "root@localhost" || !me.Active || len(me.Groups) != 2 || me.Groups[1].Name != "admin" {
		t.Errorf("me = %+v (path %s)", me, rec.path)
	}

	c, rec = serve(t, http.StatusOK, fixture(t, "queue_get"))
	q, err := c.Queues.Get(context.Background(), 37)
	if err != nil {
		t.Fatalf("Queues.Get: %v", err)
	}
	if rec.path != "/api/v1/queues/37" || q.Name != "Coaching" || q.Comments != "Coaching engagement queue (GoatCoach)" ||
		q.SignatureID == nil || *q.SignatureID != 1 || len(q.Groups) != 1 || q.Valid != nil {
		t.Errorf("queue = %+v (path %s)", q, rec.path)
	}
}

func TestStatisticsDashboard(t *testing.T) {
	c, rec := serve(t, http.StatusOK, fixture(t, "dashboard"))

	stats, err := c.Statistics.Dashboard(context.Background())
	if err != nil {
		t.Fatalf("Dashboard: %v", err)
	}
	if rec.path != "/api/v1/statistics/dashboard" {
		t.Errorf("path = %s", rec.path)
	}
	if stats.Overview.TotalTickets != 12 || stats.Overview.OpenTickets != 5 || len(stats.ByQueue) != 1 ||
		stats.ByQueue[0].QueueName != "Coaching" || len(stats.RecentActivity) != 1 || stats.RecentActivity[0].TicketTN != "COACH-9664946" {
		t.Errorf("stats = %+v", stats)
	}
}

func TestSearchQuery(t *testing.T) {
	c, rec := serve(t, http.StatusOK, fixture(t, "search_unavailable"))

	res, err := c.Search.Query(context.Background(), &types.SearchQuery{Query: "printer", Limit: 5})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if rec.method != http.MethodPost || rec.path != "/api/v1/search" || !equalJSON(rec.body, map[string]interface{}{"query": "printer", "limit": float64(5)}) {
		t.Errorf("request = %s %s %v", rec.method, rec.path, rec.body)
	}
	if res.Warning != "search backend unavailable" || res.TotalHits != 0 || len(res.Hits) != 0 {
		t.Errorf("results = %+v", res)
	}
}

func TestLogin(t *testing.T) {
	c, rec := serve(t, http.StatusOK, fixture(t, "login"))
	c.SetAuth(nil)

	resp, err := c.Login(context.Background(), "root@localhost", "secret")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if rec.path != "/api/v1/auth/login" || rec.authorization != "" {
		t.Errorf("request = %s with Authorization %q, want login without credentials", rec.path, rec.authorization)
	}
	if !equalJSON(rec.body, map[string]interface{}{"login": "root@localhost", "password": "secret"}) {
		t.Errorf("request body = %v", rec.body)
	}
	if resp.AccessToken != "eyJhbGciOiJIUzI1NiJ9.e30.sig" || resp.RefreshToken != "eyJhbGciOiJIUzI1NiJ9.e30.ref" ||
		resp.ExpiresIn != 86400 || resp.RefreshExpiresIn != 604800 || resp.User.Role != "Admin" {
		t.Errorf("login = %+v", resp)
	}
	_ = c.Get(context.Background(), "/api/v1/users/me", nil) //nolint:errcheck // only the header matters
	if rec.authorization != "Bearer eyJhbGciOiJIUzI1NiJ9.e30.sig" {
		t.Errorf("Authorization after Login = %q, want the access token", rec.authorization)
	}

	c, _ = serve(t, http.StatusUnauthorized, fixture(t, "login_failed"))
	if _, err := c.Login(context.Background(), "root@localhost", "wrong"); !sdkerrors.IsUnauthorized(err) {
		t.Errorf("err = %v, want 401 APIError", err)
	}
}

func TestRefresh(t *testing.T) {
	c, rec := serve(t, http.StatusOK, fixture(t, "refresh"))

	pair, err := c.Auth.Refresh(context.Background(), "refresh-1")
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if rec.method != http.MethodPost || rec.path != "/api/v1/auth/refresh" || rec.authorization != "" {
		t.Errorf("request = %s %s with Authorization %q", rec.method, rec.path, rec.authorization)
	}
	if !equalJSON(rec.body, map[string]interface{}{"refresh_token": "refresh-1"}) {
		t.Errorf("request body = %v", rec.body)
	}
	if pair.AccessToken != "eyJhbGciOiJIUzI1NiJ9.e30.sig2" || pair.RefreshToken != "eyJhbGciOiJIUzI1NiJ9.e30.ref2" || pair.ExpiresIn != 86400 {
		t.Errorf("pair = %+v", pair)
	}

	c, _ = serve(t, http.StatusUnauthorized, fixture(t, "refresh_rejected"))
	_, err = c.Auth.Refresh(context.Background(), "stale")
	if apiErr, ok := sdkerrors.AsAPIError(err); !ok || apiErr.StatusCode != http.StatusUnauthorized || apiErr.Message != "Invalid or expired refresh token" {
		t.Errorf("err = %v, want 401 Invalid or expired refresh token", err)
	}
}

type route struct {
	status int
	body   string
}

// serveRoutes starts a server answering by path and records every request.
func serveRoutes(t *testing.T, routes map[string]route) (string, *[]recorded) {
	t.Helper()
	var log []recorded
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := recorded{method: r.Method, path: r.URL.Path, authorization: r.Header.Get("Authorization")}
		raw, _ := io.ReadAll(r.Body) //nolint:errcheck // test server
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &rec.body) //nolint:errcheck // asserted by callers
		}
		log = append(log, rec)
		rt, ok := routes[r.URL.Path]
		if !ok {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(rt.status)
		_, _ = io.WriteString(w, rt.body) //nolint:errcheck // test server
	}))
	t.Cleanup(srv.Close)
	return srv.URL, &log
}

func TestExpiredJWTIsRenewedThroughRefreshEndpoint(t *testing.T) {
	url, log := serveRoutes(t, map[string]route{
		"/api/v1/auth/refresh": {http.StatusOK, fixture(t, "refresh")},
		"/api/v1/users/me":     {http.StatusOK, fixture(t, "user_me")},
	})
	c := client.NewClientWithJWT(url, "expired-access", "refresh-1", time.Now().Add(-time.Hour))

	for i := range 2 {
		if _, err := c.Users.Me(context.Background()); err != nil {
			t.Fatalf("Me #%d: %v", i+1, err)
		}
	}

	reqs := *log
	if len(reqs) != 3 {
		t.Fatalf("got %d requests %+v, want refresh + 2x users/me", len(reqs), reqs)
	}
	if reqs[0].path != "/api/v1/auth/refresh" || reqs[0].authorization != "" ||
		!equalJSON(reqs[0].body, map[string]interface{}{"refresh_token": "refresh-1"}) {
		t.Errorf("first request = %+v, want unauthenticated refresh with refresh-1", reqs[0])
	}
	for _, r := range reqs[1:] {
		if r.path != "/api/v1/users/me" || r.authorization != "Bearer eyJhbGciOiJIUzI1NiJ9.e30.sig2" {
			t.Errorf("request = %+v, want users/me with the refreshed token", r)
		}
	}
}

func TestRejectedRefreshFailsTheRequest(t *testing.T) {
	url, log := serveRoutes(t, map[string]route{
		"/api/v1/auth/refresh": {http.StatusUnauthorized, fixture(t, "refresh_rejected")},
	})
	c := client.NewClientWithJWT(url, "expired-access", "refresh-1", time.Now().Add(-time.Hour))

	_, err := c.Users.Me(context.Background())
	if !sdkerrors.IsUnauthorized(err) {
		t.Fatalf("err = %v, want the refresh's 401 APIError", err)
	}
	if len(*log) != 1 {
		t.Errorf("got %d requests, want only the refresh", len(*log))
	}
}

func TestJWTWithoutRefreshTokenFailsWhenExpired(t *testing.T) {
	c, rec := serve(t, http.StatusOK, fixture(t, "user_me"))
	c.SetAuth(auth.NewJWTAuth("old", "", time.Now().Add(-time.Hour), c.Auth.RefreshFunc()))

	if _, err := c.Users.Me(context.Background()); err == nil {
		t.Fatal("Me succeeded with an expired token and no refresh token")
	}
	if rec.method != "" {
		t.Errorf("a request was sent: %s %s", rec.method, rec.path)
	}
}

func TestNetworkError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()

	_, err := client.NewClientWithAPIKey(url, "gf_x").Tickets.Get(context.Background(), 1)
	var netErr *sdkerrors.NetworkError
	if !asNetworkError(err, &netErr) || netErr.Method != http.MethodGet || netErr.URL != url+"/api/v1/tickets/1" {
		t.Fatalf("err = %T %v, want *NetworkError for GET %s/api/v1/tickets/1", err, err, url)
	}
}

func asNetworkError(err error, target **sdkerrors.NetworkError) bool {
	ne, ok := err.(*sdkerrors.NetworkError)
	if ok {
		*target = ne
	}
	return ok
}

func equalJSON(a, b map[string]interface{}) bool {
	x, _ := json.Marshal(a) //nolint:errcheck // maps of JSON values
	y, _ := json.Marshal(b) //nolint:errcheck // maps of JSON values
	return string(x) == string(y)
}
