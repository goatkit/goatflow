package routing

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/audit"
	"github.com/goatkit/goatflow/internal/platform/database"
)

// auditSink is a database/sql driver that keeps the gk_audit_event inserts.
type auditSink struct {
	mu     sync.Mutex
	events []sinkEvent
}

type sinkEvent struct {
	category, action, outcome string
	actorID                   any
	details                   map[string]any
	rawDetails                string
}

func (s *auditSink) Connect(context.Context) (driver.Conn, error) { return sinkConn{s}, nil }
func (s *auditSink) Driver() driver.Driver                        { return nil }

func (s *auditSink) recorded() []sinkEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]sinkEvent(nil), s.events...)
}

type sinkConn struct{ s *auditSink }

func (sinkConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not supported") }
func (sinkConn) Close() error                        { return nil }
func (sinkConn) Begin() (driver.Tx, error)           { return nil, errors.New("not supported") }

func (c sinkConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if !strings.Contains(query, "gk_audit_event") || len(args) != 13 {
		return nil, errors.New("unexpected statement")
	}
	e := sinkEvent{actorID: args[5].Value}
	e.category, _ = args[1].Value.(string)
	e.action, _ = args[2].Value.(string)
	e.outcome, _ = args[3].Value.(string)
	if raw, ok := args[12].Value.(string); ok {
		e.rawDetails = raw
		if err := json.Unmarshal([]byte(raw), &e.details); err != nil {
			return nil, err
		}
	}
	c.s.mu.Lock()
	c.s.events = append(c.s.events, e)
	c.s.mu.Unlock()
	return driver.RowsAffected(1), nil
}

func withAuditSink(t *testing.T) *auditSink {
	t.Helper()
	sink := &auditSink{}
	db := sql.OpenDB(sink)
	database.SetDB(db)
	t.Cleanup(func() {
		database.ResetDB()
		_ = db.Close()
	})
	return sink
}

func adminAuditRouter(t *testing.T, ctx map[string]any, handler gin.HandlerFunc) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	registry := NewHandlerRegistry()
	RegisterExistingHandlers(registry)
	adminMw, err := registry.GetMiddleware("admin")
	require.NoError(t, err)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		for k, v := range ctx {
			c.Set(k, v)
		}
		c.Next()
	})
	router.Use(adminMw)
	router.Any("/admin/users/:id", handler)
	return router
}

var adminCaller = map[string]any{"user_id": uint(7), "user_role": "Admin"}

func TestAdminMiddlewareRecordsWriteOutcomeFromStatus(t *testing.T) {
	cases := []struct {
		status  int
		outcome string
	}{
		{http.StatusOK, audit.OutcomeSuccess},
		{http.StatusSeeOther, audit.OutcomeSuccess},
		{http.StatusUnauthorized, audit.OutcomeDenied},
		{http.StatusForbidden, audit.OutcomeDenied},
		{http.StatusUnprocessableEntity, audit.OutcomeFailure},
		{http.StatusInternalServerError, audit.OutcomeFailure},
	}
	for _, tc := range cases {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			sink := withAuditSink(t)
			router := adminAuditRouter(t, adminCaller, func(c *gin.Context) { c.Status(tc.status) })

			req := httptest.NewRequest(http.MethodPut, "/admin/users/42?token=s3cret", strings.NewReader("password=hunter2"))
			router.ServeHTTP(httptest.NewRecorder(), req)

			events := sink.recorded()
			require.Len(t, events, 1)
			e := events[0]
			assert.Equal(t, audit.CategoryAdmin, e.category)
			assert.Equal(t, "admin.request", e.action)
			assert.Equal(t, tc.outcome, e.outcome)
			assert.EqualValues(t, 7, e.actorID)
			assert.Equal(t, http.MethodPut, e.details["method"])
			assert.Equal(t, "/admin/users/:id", e.details["route"])
			assert.EqualValues(t, tc.status, e.details["status"])
			assert.Equal(t, map[string]any{"id": "42"}, e.details["params"])
			assert.NotContains(t, e.rawDetails, "s3cret")
			assert.NotContains(t, e.rawDetails, "hunter2")
		})
	}
}

func TestAdminMiddlewareSkipsReadsAndHandlerRecordedWrites(t *testing.T) {
	t.Run("read", func(t *testing.T) {
		sink := withAuditSink(t)
		router := adminAuditRouter(t, adminCaller, func(c *gin.Context) { c.Status(http.StatusOK) })
		router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/admin/users/42", nil))
		assert.Empty(t, sink.recorded())
	})

	t.Run("handler recorded its own event", func(t *testing.T) {
		sink := withAuditSink(t)
		router := adminAuditRouter(t, adminCaller, func(c *gin.Context) {
			audit.RecordRequest(c, audit.Event{
				Category: audit.CategoryAdmin, Action: "admin.user.update", Outcome: audit.OutcomeSuccess,
				TargetType: "user", TargetID: "42",
			})
			c.Status(http.StatusOK)
		})
		router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPut, "/admin/users/42", nil))

		events := sink.recorded()
		require.Len(t, events, 1)
		assert.Equal(t, "admin.user.update", events[0].action)
	})
}

func TestAdminMiddlewareRecordsDeniedNonAdmin(t *testing.T) {
	t.Run("authenticated agent", func(t *testing.T) {
		sink := withAuditSink(t)
		called := false
		router := adminAuditRouter(t, map[string]any{"user_id": uint(9), "user_role": "Agent"}, func(c *gin.Context) { called = true })

		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/admin/users/42", nil))

		assert.Equal(t, http.StatusForbidden, resp.Code)
		assert.False(t, called)
		events := sink.recorded()
		require.Len(t, events, 1)
		e := events[0]
		assert.Equal(t, "admin.request", e.action)
		assert.Equal(t, audit.OutcomeDenied, e.outcome)
		assert.EqualValues(t, 9, e.actorID)
		assert.Equal(t, http.MethodGet, e.details["method"])
		assert.Equal(t, "/admin/users/:id", e.details["route"])
		assert.Equal(t, "/admin/users/42", e.details["path"])
	})

	t.Run("no caller", func(t *testing.T) {
		sink := withAuditSink(t)
		router := adminAuditRouter(t, map[string]any{}, func(c *gin.Context) {})
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/admin/users/42", nil))
		assert.Equal(t, http.StatusForbidden, resp.Code)
		assert.Empty(t, sink.recorded())
	})
}
