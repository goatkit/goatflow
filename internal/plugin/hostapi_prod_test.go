package plugin

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "github.com/mattn/go-sqlite3"
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

func TestWithDB(t *testing.T) {
	// Create an in-memory SQLite database for testing
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	h := NewProdHostAPI(WithDB("test", db))
	
	if h.databases["test"] != db {
		t.Error("database not set correctly")
	}
	// Default is "default" unless explicitly changed
	if h.defaultDB != "default" {
		t.Errorf("expected default db 'default', got %s", h.defaultDB)
	}
}

func TestWithDBOnEmptyHost(t *testing.T) {
	// Test WithDB on a zero-value ProdHostAPI to hit defensive branches
	db, _ := sql.Open("sqlite3", ":memory:")
	defer db.Close()

	// Create a zero-value host (not via NewProdHostAPI)
	h := &ProdHostAPI{}
	
	// Apply WithDB option directly
	opt := WithDB("first", db)
	opt(h)

	if h.databases == nil {
		t.Error("databases map should be initialized")
	}
	if h.databases["first"] != db {
		t.Error("database not set correctly")
	}
	if h.defaultDB != "first" {
		t.Errorf("expected 'first' as default, got %s", h.defaultDB)
	}
}

func TestWithMultipleDBs(t *testing.T) {
	db1, _ := sql.Open("sqlite3", ":memory:")
	defer db1.Close()
	db2, _ := sql.Open("sqlite3", ":memory:")
	defer db2.Close()

	h := NewProdHostAPI(
		WithDB("primary", db1),
		WithDB("secondary", db2),
	)

	if h.databases["primary"] != db1 {
		t.Error("primary db not set")
	}
	if h.databases["secondary"] != db2 {
		t.Error("secondary db not set")
	}
	// Default is still "default" unless WithDefaultDB is used
	if h.defaultDB != "default" {
		t.Errorf("expected default 'default', got %s", h.defaultDB)
	}
}

func TestWithDefaultDB(t *testing.T) {
	db1, _ := sql.Open("sqlite3", ":memory:")
	defer db1.Close()
	db2, _ := sql.Open("sqlite3", ":memory:")
	defer db2.Close()

	h := NewProdHostAPI(
		WithDB("primary", db1),
		WithDB("secondary", db2),
		WithDefaultDB("secondary"),
	)

	if h.defaultDB != "secondary" {
		t.Errorf("expected default 'secondary', got %s", h.defaultDB)
	}
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
	db1, _ := sql.Open("sqlite3", ":memory:")
	defer db1.Close()
	db2, _ := sql.Open("sqlite3", ":memory:")
	defer db2.Close()

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

func TestProdHostAPI_DBQuery(t *testing.T) {
	// Use SQLite for testing
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	// Create test table
	_, err = db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	_, err = db.Exec("INSERT INTO test (name) VALUES ('Alice'), ('Bob')")
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	h := NewProdHostAPI(WithDB("default", db))
	ctx := context.Background()

	t.Run("basic query", func(t *testing.T) {
		rows, err := h.DBQuery(ctx, "SELECT id, name FROM test ORDER BY id")
		if err != nil {
			t.Fatalf("DBQuery error: %v", err)
		}
		if len(rows) != 2 {
			t.Errorf("expected 2 rows, got %d", len(rows))
		}
		if rows[0]["name"] != "Alice" {
			t.Errorf("expected Alice, got %v", rows[0]["name"])
		}
	})

	t.Run("query with parameters", func(t *testing.T) {
		rows, err := h.DBQuery(ctx, "SELECT name FROM test WHERE id = ?", 1)
		if err != nil {
			t.Fatalf("DBQuery error: %v", err)
		}
		if len(rows) != 1 {
			t.Errorf("expected 1 row, got %d", len(rows))
		}
	})

	t.Run("query with named db prefix", func(t *testing.T) {
		rows, err := h.DBQuery(ctx, "@default:SELECT COUNT(*) as cnt FROM test")
		if err != nil {
			t.Fatalf("DBQuery error: %v", err)
		}
		if len(rows) != 1 {
			t.Errorf("expected 1 row, got %d", len(rows))
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
	db, _ := sql.Open("sqlite3", ":memory:")
	defer db.Close()
	db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")

	h := NewProdHostAPI(WithDB("default", db))
	ctx := context.Background()

	t.Run("insert", func(t *testing.T) {
		affected, err := h.DBExec(ctx, "INSERT INTO test (name) VALUES (?)", "Charlie")
		if err != nil {
			t.Fatalf("DBExec error: %v", err)
		}
		if affected != 1 {
			t.Errorf("expected 1 affected, got %d", affected)
		}
	})

	t.Run("update", func(t *testing.T) {
		h.DBExec(ctx, "INSERT INTO test (name) VALUES ('Dave')")
		affected, err := h.DBExec(ctx, "UPDATE test SET name = 'Updated' WHERE name = 'Dave'")
		if err != nil {
			t.Fatalf("DBExec error: %v", err)
		}
		if affected != 1 {
			t.Errorf("expected 1 affected, got %d", affected)
		}
	})

	t.Run("delete", func(t *testing.T) {
		h.DBExec(ctx, "INSERT INTO test (name) VALUES ('ToDelete')")
		affected, err := h.DBExec(ctx, "DELETE FROM test WHERE name = 'ToDelete'")
		if err != nil {
			t.Fatalf("DBExec error: %v", err)
		}
		if affected != 1 {
			t.Errorf("expected 1 affected, got %d", affected)
		}
	})

	t.Run("with named db prefix", func(t *testing.T) {
		affected, err := h.DBExec(ctx, "@default:INSERT INTO test (name) VALUES (?)", "Prefixed")
		if err != nil {
			t.Fatalf("DBExec error: %v", err)
		}
		if affected != 1 {
			t.Errorf("expected 1 affected, got %d", affected)
		}
	})

	t.Run("nonexistent db returns error", func(t *testing.T) {
		_, err := h.DBExec(ctx, "@nonexistent:INSERT INTO test (name) VALUES (?)", "Test")
		if err == nil {
			t.Error("expected error for nonexistent db")
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
