package plugin

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/goatkit/goatflow/internal/platform/database"
)

func TestNewProdHostAPI(t *testing.T) {
	h := NewProdHostAPI()
	if h == nil {
		t.Fatal("expected non-nil ProdHostAPI")
	}
	if h.databases == nil {
		t.Error("databases map should be initialized")
	}
	if h.defaultDB != "default" {
		t.Errorf("expected default db name 'default', got %s", h.defaultDB)
	}
	if h.httpClient == nil {
		t.Error("httpClient should be initialized")
	}
	if h.logger == nil {
		t.Error("logger should be initialized")
	}
}

// routingDBs returns the real test database and a second, closed handle of
// the same driver. Queries routed to the closed handle fail with "sql:
// database is closed", so a test can tell from the result which handle a
// database name resolved to.
func routingDBs(t *testing.T) (live, closed *sql.DB) {
	t.Helper()
	live = requireHostTestDB(t)
	driver := "postgres"
	if database.IsMySQL() {
		driver = "mysql"
	}
	closed, err := sql.Open(driver, "")
	if err != nil {
		t.Fatalf("open second %s handle: %v", driver, err)
	}
	if err := closed.Close(); err != nil {
		t.Fatalf("close second handle: %v", err)
	}
	return live, closed
}

// expectRoute runs an unprefixed or "@name:"-prefixed SELECT through DBQuery
// and checks it reached the live database (wantErr == "") or failed with an
// error containing wantErr.
func expectRoute(t *testing.T, h *ProdHostAPI, prefix, wantErr string) {
	t.Helper()
	rows, err := h.DBQuery(context.Background(), prefix+"SELECT 1 AS one")
	if wantErr == "" {
		if err != nil {
			t.Fatalf("DBQuery %q: %v", prefix, err)
		}
		if len(rows) != 1 || fmt.Sprint(rows[0]["one"]) != "1" {
			t.Fatalf("DBQuery %q rows = %v", prefix, rows)
		}
		return
	}
	if err == nil || !strings.Contains(err.Error(), wantErr) {
		t.Fatalf("DBQuery %q error = %v, want %q", prefix, err, wantErr)
	}
}

func TestWithDB(t *testing.T) {
	live, _ := routingDBs(t)
	h := NewProdHostAPI(WithDB("test", live))

	expectRoute(t, h, "@test:", "")
	// On a constructed host the default name stays "default", which is not
	// configured here, so an unprefixed query fails instead of guessing.
	expectRoute(t, h, "", `database "default" not found`)
}

func TestWithDBOnEmptyHost(t *testing.T) {
	live, _ := routingDBs(t)
	// Zero-value host (not via NewProdHostAPI): the first WithDB creates the
	// map and becomes the default.
	h := &ProdHostAPI{}
	WithDB("first", live)(h)

	expectRoute(t, h, "", "")
	expectRoute(t, h, "@first:", "")
}

func TestWithMultipleDBs(t *testing.T) {
	live, closed := routingDBs(t)
	h := NewProdHostAPI(
		WithDB("primary", live),
		WithDB("secondary", closed),
	)

	expectRoute(t, h, "@primary:", "")
	expectRoute(t, h, "@secondary:", "database is closed")
	// Default is still "default" unless WithDefaultDB is used.
	expectRoute(t, h, "", `database "default" not found`)
}

func TestWithDefaultDB(t *testing.T) {
	live, closed := routingDBs(t)
	opts := []ProdHostAPIOption{WithDB("primary", live), WithDB("secondary", closed)}

	toSecondary := NewProdHostAPI(append(opts, WithDefaultDB("secondary"))...)
	expectRoute(t, toSecondary, "", "database is closed")
	if _, err := toSecondary.DBExec(context.Background(), "UPDATE ticket SET title = title WHERE id = 0"); err == nil || !strings.Contains(err.Error(), "database is closed") {
		t.Fatalf("DBExec on default secondary: %v, want database is closed", err)
	}

	toPrimary := NewProdHostAPI(append(opts, WithDefaultDB("primary"))...)
	expectRoute(t, toPrimary, "", "")
	expectRoute(t, toPrimary, "@secondary:", "database is closed")
}

func TestWithCache(t *testing.T) {
	// We can't easily test with a real cache, but we can verify the option works
	h := NewProdHostAPI(WithCache(nil))
	if h.cache != nil {
		t.Error("cache should be nil when passed nil")
	}
}

func TestWithLogger(t *testing.T) {
	logger := slog.Default()
	h := NewProdHostAPI(WithLogger(logger))
	if h.logger != logger {
		t.Error("logger not set correctly")
	}
}

func TestWithPluginManager(t *testing.T) {
	mgr := NewManager(nil)
	h := NewProdHostAPI(WithPluginManager(mgr))
	if h.PluginManager != mgr {
		t.Error("plugin manager not set correctly")
	}
}

func TestParseDBPrefix(t *testing.T) {
	h := NewProdHostAPI()

	tests := []struct {
		query      string
		wantPrefix string
		wantQuery  string
	}{
		{"SELECT 1", "", "SELECT 1"},
		{"@primary:SELECT 1", "primary", "SELECT 1"},
		{"@secondary:UPDATE x SET y=1", "secondary", "UPDATE x SET y=1"},
		{"@test: SELECT * FROM users", "test", " SELECT * FROM users"},
		{"@:SELECT 1", "", "@:SELECT 1"}, // invalid prefix (no name)
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			prefix, query := h.parseDBPrefix(tt.query)
			if prefix != tt.wantPrefix {
				t.Errorf("prefix: got %q, want %q", prefix, tt.wantPrefix)
			}
			if query != tt.wantQuery {
				t.Errorf("query: got %q, want %q", query, tt.wantQuery)
			}
		})
	}
}

func TestGetDB(t *testing.T) {
	db1, db2 := routingDBs(t)

	h := NewProdHostAPI(
		WithDB("primary", db1),
		WithDB("secondary", db2),
		WithDefaultDB("primary"),
	)

	t.Run("empty name returns default", func(t *testing.T) {
		got, err := h.getDB("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != db1 {
			t.Error("expected primary (default) db")
		}
	})

	t.Run("explicit name returns that db", func(t *testing.T) {
		got, err := h.getDB("secondary")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != db2 {
			t.Error("expected secondary db")
		}
	})

	t.Run("unknown name returns error", func(t *testing.T) {
		_, err := h.getDB("unknown")
		if err == nil {
			t.Error("expected error for unknown db")
		}
	})

	t.Run("no databases configured", func(t *testing.T) {
		emptyH := NewProdHostAPI()
		_, err := emptyH.getDB("")
		if err == nil {
			t.Error("expected error when no databases configured")
		}
	})
}

func TestIndexByte(t *testing.T) {
	tests := []struct {
		s    string
		c    byte
		want int
	}{
		{"hello", 'l', 2},
		{"hello", 'o', 4},
		{"hello", 'x', -1},
		{"", 'x', -1},
		{"x", 'x', 0},
	}

	for _, tt := range tests {
		got := indexByte(tt.s, tt.c)
		if got != tt.want {
			t.Errorf("indexByte(%q, %c) = %d, want %d", tt.s, tt.c, got, tt.want)
		}
	}
}

// hostTestTicketTypes returns a unique ticket_type name prefix and deletes
// every ticket_type row carrying it on cleanup. ticket_type is a plain OTRS
// lookup table (name, valid_id, audit columns) a plugin may read and write.
func hostTestTicketTypes(t *testing.T, db *sql.DB) string {
	t.Helper()
	prefix := fmt.Sprintf("hostapi-%d-", time.Now().UnixNano())
	t.Cleanup(func() {
		if _, err := db.Exec(database.ConvertPlaceholders(`DELETE FROM ticket_type WHERE name LIKE ?`), prefix+"%"); err != nil {
			t.Errorf("cleanup ticket_type %s*: %v", prefix, err)
		}
	})
	return prefix
}

func countTicketTypes(t *testing.T, db *sql.DB, name string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(database.ConvertPlaceholders(`SELECT COUNT(*) FROM ticket_type WHERE name = ?`), name).Scan(&n); err != nil {
		t.Fatalf("count ticket_type %q: %v", name, err)
	}
	return n
}

func TestProdHostAPI_DBQuery(t *testing.T) {
	db := requireHostTestDB(t)
	prefix := hostTestTicketTypes(t, db)
	var aliceID int64
	for _, n := range []string{"Alice", "Bob"} {
		now := time.Now()
		id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(
			`INSERT INTO ticket_type (name, valid_id, create_time, create_by, change_time, change_by)
			 VALUES (?, 1, ?, 1, ?, 1) RETURNING id`), prefix+n, now, now)
		if err != nil {
			t.Fatalf("insert ticket_type %s: %v", n, err)
		}
		if n == "Alice" {
			aliceID = id
		}
	}

	h := NewProdHostAPI(WithDB("default", db))
	ctx := context.Background()

	t.Run("basic query", func(t *testing.T) {
		rows, err := h.DBQuery(ctx, "SELECT id, name FROM ticket_type WHERE name LIKE ? ORDER BY id", prefix+"%")
		if err != nil {
			t.Fatalf("DBQuery error: %v", err)
		}
		if len(rows) != 2 {
			t.Fatalf("expected 2 rows, got %d", len(rows))
		}
		if rows[0]["name"] != prefix+"Alice" || rows[1]["name"] != prefix+"Bob" {
			t.Errorf("expected Alice then Bob as strings, got %#v, %#v", rows[0]["name"], rows[1]["name"])
		}
	})

	t.Run("query with parameters", func(t *testing.T) {
		rows, err := h.DBQuery(ctx, "SELECT name FROM ticket_type WHERE id = ?", aliceID)
		if err != nil {
			t.Fatalf("DBQuery error: %v", err)
		}
		if len(rows) != 1 || rows[0]["name"] != prefix+"Alice" {
			t.Errorf("expected only Alice, got %v", rows)
		}
	})

	t.Run("query with named db prefix", func(t *testing.T) {
		rows, err := h.DBQuery(ctx, "@default:SELECT COUNT(*) AS cnt FROM ticket_type WHERE name LIKE ?", prefix+"%")
		if err != nil {
			t.Fatalf("DBQuery error: %v", err)
		}
		if len(rows) != 1 || fmt.Sprint(rows[0]["cnt"]) != "2" {
			t.Errorf("expected cnt=2, got %v", rows)
		}
	})

	t.Run("query nonexistent db returns error", func(t *testing.T) {
		_, err := h.DBQuery(ctx, "@nonexistent:SELECT 1")
		if err == nil {
			t.Error("expected error for nonexistent db")
		}
	})
}

func TestProdHostAPI_DBExec(t *testing.T) {
	db := requireHostTestDB(t)
	prefix := hostTestTicketTypes(t, db)
	h := NewProdHostAPI(WithDB("default", db))
	ctx := context.Background()
	const insert = `INSERT INTO ticket_type (name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)`

	t.Run("insert", func(t *testing.T) {
		affected, err := h.DBExec(ctx, insert, prefix+"Charlie")
		if err != nil {
			t.Fatalf("DBExec error: %v", err)
		}
		if affected != 1 || countTicketTypes(t, db, prefix+"Charlie") != 1 {
			t.Errorf("expected Charlie inserted (1 affected), got %d affected", affected)
		}
	})

	t.Run("update", func(t *testing.T) {
		if _, err := h.DBExec(ctx, insert, prefix+"Dave"); err != nil {
			t.Fatalf("seed Dave: %v", err)
		}
		affected, err := h.DBExec(ctx, "UPDATE ticket_type SET name = ? WHERE name = ?", prefix+"Updated", prefix+"Dave")
		if err != nil {
			t.Fatalf("DBExec error: %v", err)
		}
		if affected != 1 || countTicketTypes(t, db, prefix+"Updated") != 1 || countTicketTypes(t, db, prefix+"Dave") != 0 {
			t.Errorf("expected Dave renamed (1 affected), got %d affected", affected)
		}
	})

	t.Run("delete", func(t *testing.T) {
		if _, err := h.DBExec(ctx, insert, prefix+"ToDelete"); err != nil {
			t.Fatalf("seed ToDelete: %v", err)
		}
		affected, err := h.DBExec(ctx, "DELETE FROM ticket_type WHERE name = ?", prefix+"ToDelete")
		if err != nil {
			t.Fatalf("DBExec error: %v", err)
		}
		if affected != 1 || countTicketTypes(t, db, prefix+"ToDelete") != 0 {
			t.Errorf("expected ToDelete removed (1 affected), got %d affected", affected)
		}
	})

	t.Run("with named db prefix", func(t *testing.T) {
		affected, err := h.DBExec(ctx, "@default:"+insert, prefix+"Prefixed")
		if err != nil {
			t.Fatalf("DBExec error: %v", err)
		}
		if affected != 1 || countTicketTypes(t, db, prefix+"Prefixed") != 1 {
			t.Errorf("expected Prefixed inserted (1 affected), got %d affected", affected)
		}
	})

	t.Run("nonexistent db returns error", func(t *testing.T) {
		_, err := h.DBExec(ctx, "@nonexistent:"+insert, prefix+"Test")
		if err == nil {
			t.Error("expected error for nonexistent db")
		}
		if countTicketTypes(t, db, prefix+"Test") != 0 {
			t.Error("row written although the named db does not exist")
		}
	})
}

func TestProdHostAPI_Log(t *testing.T) {
	h := NewProdHostAPI()
	ctx := context.Background()

	// Should not panic and should add to log buffer
	GetLogBuffer().Clear()

	t.Run("info level", func(t *testing.T) {
		h.Log(ctx, "info", "info message", map[string]any{"key": "value"})
	})

	t.Run("debug level", func(t *testing.T) {
		h.Log(ctx, "debug", "debug message", nil)
	})

	t.Run("warn level", func(t *testing.T) {
		h.Log(ctx, "warn", "warn message", nil)
	})

	t.Run("error level", func(t *testing.T) {
		h.Log(ctx, "error", "error message", nil)
	})

	t.Run("unknown level defaults to info", func(t *testing.T) {
		h.Log(ctx, "unknown", "unknown level", nil)
	})

	t.Run("with plugin in fields", func(t *testing.T) {
		GetLogBuffer().Clear()
		h.Log(ctx, "info", "plugin log", map[string]any{"plugin": "test-plugin"})

		logs := GetLogBuffer().GetByPlugin("test-plugin")
		if len(logs) == 0 {
			t.Error("expected log with plugin name")
		}
	})

	t.Run("with plugin in context", func(t *testing.T) {
		GetLogBuffer().Clear()
		ctxWithPlugin := context.WithValue(ctx, PluginCallerKey, "context-plugin")
		h.Log(ctxWithPlugin, "info", "context plugin log", nil)

		logs := GetLogBuffer().GetByPlugin("context-plugin")
		if len(logs) == 0 {
			t.Error("expected log with plugin from context")
		}
	})
}

func TestProdHostAPI_HTTPRequest(t *testing.T) {
	h := NewProdHostAPI()
	ctx := context.Background()

	t.Run("GET request", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" {
				t.Errorf("expected GET, got %s", r.Method)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		}))
		defer ts.Close()

		status, body, err := h.HTTPRequest(ctx, "GET", ts.URL, nil, nil)
		if err != nil {
			t.Fatalf("HTTPRequest error: %v", err)
		}
		if status != 200 {
			t.Errorf("expected 200, got %d", status)
		}
		if len(body) == 0 {
			t.Error("expected response body")
		}
	})

	t.Run("POST request with body", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"created":true}`))
		}))
		defer ts.Close()

		reqBody := []byte(`{"name":"test"}`)
		status, body, err := h.HTTPRequest(ctx, "POST", ts.URL, nil, reqBody)
		if err != nil {
			t.Fatalf("HTTPRequest error: %v", err)
		}
		if status != 201 {
			t.Errorf("expected 201, got %d", status)
		}
		if len(body) == 0 {
			t.Error("expected response body")
		}
	})

	t.Run("with custom headers", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Custom") != "test-value" {
				t.Errorf("expected custom header, got %s", r.Header.Get("X-Custom"))
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()

		headers := map[string]string{"X-Custom": "test-value"}
		status, _, err := h.HTTPRequest(ctx, "GET", ts.URL, headers, nil)
		if err != nil {
			t.Fatalf("HTTPRequest error: %v", err)
		}
		if status != 200 {
			t.Errorf("expected 200, got %d", status)
		}
	})

	t.Run("connection refused returns error", func(t *testing.T) {
		_, _, err := h.HTTPRequest(ctx, "GET", "http://127.0.0.1:59999", nil, nil)
		if err == nil {
			t.Error("expected error for connection refused")
		}
	})
}

func TestProdHostAPI_CallPlugin(t *testing.T) {
	ctx := context.Background()

	t.Run("without plugin manager", func(t *testing.T) {
		h := NewProdHostAPI()
		_, err := h.CallPlugin(ctx, "other", "func", nil)
		if err == nil {
			t.Error("expected error without plugin manager")
		}
		if err.Error() != "plugin manager not available" {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("with plugin manager", func(t *testing.T) {
		mgr := NewManager(nil)
		h := NewProdHostAPI(WithPluginManager(mgr))

		// Call nonexistent plugin
		_, err := h.CallPlugin(ctx, "nonexistent", "func", nil)
		if err == nil {
			t.Error("expected error for nonexistent plugin")
		}
	})

	t.Run("with caller context", func(t *testing.T) {
		mgr := NewManager(nil)
		h := NewProdHostAPI(WithPluginManager(mgr))

		// Add caller to context
		ctx := context.WithValue(context.Background(), PluginCallerKey, "caller-plugin")

		// Call nonexistent plugin - should use CallFrom
		_, err := h.CallPlugin(ctx, "other", "func", nil)
		if err == nil {
			t.Error("expected error for nonexistent plugin")
		}
	})
}

func TestProdHostAPI_Translate(t *testing.T) {
	h := NewProdHostAPI()
	ctx := context.Background()

	// Without i18n instance, should return key
	result := h.Translate(ctx, "some.key")
	if result != "some.key" {
		t.Errorf("expected 'some.key', got %s", result)
	}

	// With language in context
	ctx = context.WithValue(ctx, PluginLanguageKey, "de")
	result = h.Translate(ctx, "another.key")
	if result != "another.key" {
		t.Errorf("expected 'another.key', got %s", result)
	}
}
