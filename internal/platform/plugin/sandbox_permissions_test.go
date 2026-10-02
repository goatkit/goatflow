package plugin

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
)

// gateRecorder records which inner HostAPI methods the sandbox let through.
type gateRecorder struct {
	mockInnerHostAPI
	mu      sync.Mutex
	reached map[string]bool
}

func newGateRecorder() *gateRecorder { return &gateRecorder{reached: map[string]bool{}} }

func (g *gateRecorder) hit(m string) {
	g.mu.Lock()
	g.reached[m] = true
	g.mu.Unlock()
}

func (g *gateRecorder) was(m string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.reached[m]
}

func (g *gateRecorder) EntitySoftDelete(context.Context, string, int64, string) error {
	g.hit("EntitySoftDelete")
	return nil
}
func (g *gateRecorder) EntityRestore(context.Context, string, int64) error {
	g.hit("EntityRestore")
	return nil
}
func (g *gateRecorder) EntityHardDelete(context.Context, string, int64, string) error {
	g.hit("EntityHardDelete")
	return nil
}
func (g *gateRecorder) RecycleBinList(context.Context, string) (json.RawMessage, error) {
	g.hit("RecycleBinList")
	return nil, nil
}
func (g *gateRecorder) CreateArticle(context.Context, int64, int64, string, string, bool) (int64, error) {
	g.hit("CreateArticle")
	return 1, nil
}
func (g *gateRecorder) CreateArticleAttachment(context.Context, int64, int64, string, string, []byte) (int64, error) {
	g.hit("CreateArticleAttachment")
	return 1, nil
}
func (g *gateRecorder) ListArticleAttachments(context.Context, int64) ([]ArticleAttachment, error) {
	g.hit("ListArticleAttachments")
	return nil, nil
}
func (g *gateRecorder) DeleteArticleAttachment(context.Context, int64, int64) error {
	g.hit("DeleteArticleAttachment")
	return nil
}
func (g *gateRecorder) ChangeTicketStatus(context.Context, int64, int64, int64, int64) error {
	g.hit("ChangeTicketStatus")
	return nil
}
func (g *gateRecorder) ListTicketStates(context.Context) ([]TicketStateInfo, error) {
	g.hit("ListTicketStates")
	return nil, nil
}
func (g *gateRecorder) ListTicketViews(context.Context) ([]TicketViewInfo, error) {
	g.hit("ListTicketViews")
	return nil, nil
}
func (g *gateRecorder) StoreFile(context.Context, string, []byte, map[string]string) error {
	g.hit("StoreFile")
	return nil
}
func (g *gateRecorder) GetFile(context.Context, string) ([]byte, map[string]string, error) {
	g.hit("GetFile")
	return nil, nil, nil
}
func (g *gateRecorder) DeleteFile(context.Context, string) error {
	g.hit("DeleteFile")
	return nil
}
func (g *gateRecorder) ListFiles(context.Context, string) ([]FileInfo, error) {
	g.hit("ListFiles")
	return nil, nil
}

// TestSandbox_GatesPrivilegedMethods: each method that changes or reveals
// tickets, articles, plugin files or deleted entities needs its permission;
// without it the inner host is never reached.
func TestSandbox_GatesPrivilegedMethods(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		method string
		grant  Permission
		call   func(HostAPI) error
	}{
		{"EntitySoftDelete", Permission{Type: "entity", Access: "write"}, func(h HostAPI) error { return h.EntitySoftDelete(ctx, "ticket", 1, "r") }},
		{"EntityRestore", Permission{Type: "entity", Access: "write"}, func(h HostAPI) error { return h.EntityRestore(ctx, "ticket", 1) }},
		{"EntityHardDelete", Permission{Type: "entity", Access: "hard_delete"}, func(h HostAPI) error { return h.EntityHardDelete(ctx, "ticket", 1, "r") }},
		{"RecycleBinList", Permission{Type: "entity", Access: "read"}, func(h HostAPI) error { _, err := h.RecycleBinList(ctx, "ticket"); return err }},
		{"CreateArticle", Permission{Type: "article", Access: "write"}, func(h HostAPI) error { _, err := h.CreateArticle(ctx, 1, 1, "s", "b", false); return err }},
		{"CreateArticleAttachment", Permission{Type: "article", Access: "write"}, func(h HostAPI) error {
			_, err := h.CreateArticleAttachment(ctx, 1, 1, "f", "text/plain", []byte("x"))
			return err
		}},
		{"ListArticleAttachments", Permission{Type: "article", Access: "read"}, func(h HostAPI) error { _, err := h.ListArticleAttachments(ctx, 1); return err }},
		{"DeleteArticleAttachment", Permission{Type: "article", Access: "write"}, func(h HostAPI) error { return h.DeleteArticleAttachment(ctx, 1, 1) }},
		{"ChangeTicketStatus", Permission{Type: "ticket", Access: "write"}, func(h HostAPI) error { return h.ChangeTicketStatus(ctx, 1, 2, 1, 0) }},
		{"ListTicketStates", Permission{Type: "ticket", Access: "read"}, func(h HostAPI) error { _, err := h.ListTicketStates(ctx); return err }},
		{"ListTicketViews", Permission{Type: "ticket", Access: "read"}, func(h HostAPI) error { _, err := h.ListTicketViews(ctx); return err }},
		{"StoreFile", Permission{Type: "file", Access: "write"}, func(h HostAPI) error { return h.StoreFile(ctx, "k", []byte("x"), nil) }},
		{"GetFile", Permission{Type: "file", Access: "read"}, func(h HostAPI) error { _, _, err := h.GetFile(ctx, "k"); return err }},
		{"DeleteFile", Permission{Type: "file", Access: "write"}, func(h HostAPI) error { return h.DeleteFile(ctx, "k") }},
		{"ListFiles", Permission{Type: "file", Access: "read"}, func(h HostAPI) error { _, err := h.ListFiles(ctx, ""); return err }},
	}
	for _, tc := range cases {
		t.Run(tc.method, func(t *testing.T) {
			// Some other permission, but not this one.
			denied := newGateRecorder()
			s := NewSandboxedHostAPI(denied, "p", ResourcePolicy{Status: "approved", Permissions: []Permission{{Type: "db", Access: "readwrite"}}})
			if err := tc.call(s); err == nil || !strings.Contains(err.Error(), "not granted") {
				t.Errorf("without grant: err = %v, want not granted", err)
			}
			if denied.was(tc.method) {
				t.Errorf("without grant the inner %s ran", tc.method)
			}
			if s.Stats().Errors != 1 {
				t.Errorf("denial not counted: errors = %d", s.Stats().Errors)
			}

			allowed := newGateRecorder()
			s = NewSandboxedHostAPI(allowed, "p", ResourcePolicy{Status: "approved", Permissions: []Permission{tc.grant}})
			if err := tc.call(s); err != nil {
				t.Errorf("with grant: %v", err)
			}
			if !allowed.was(tc.method) {
				t.Errorf("with grant the inner %s did not run", tc.method)
			}

			blocked := newGateRecorder()
			s = NewSandboxedHostAPI(blocked, "p", ResourcePolicy{Status: "blocked", Permissions: []Permission{tc.grant}})
			if err := tc.call(s); err == nil || blocked.was(tc.method) {
				t.Errorf("blocked plugin: err = %v, inner ran = %v", err, blocked.was(tc.method))
			}
		})
	}
}

// TestSandbox_HardDeleteNeedsExplicitGrant: entity readwrite covers soft
// delete and restore but never permanent deletion.
func TestSandbox_HardDeleteNeedsExplicitGrant(t *testing.T) {
	inner := newGateRecorder()
	s := NewSandboxedHostAPI(inner, "p", ResourcePolicy{Status: "approved", Permissions: []Permission{{Type: "entity", Access: "readwrite"}}})
	ctx := context.Background()
	if err := s.EntitySoftDelete(ctx, "agent", 3, "r"); err != nil {
		t.Fatalf("soft delete with readwrite: %v", err)
	}
	if err := s.EntityHardDelete(ctx, "agent", 3, "r"); err == nil || inner.was("EntityHardDelete") {
		t.Errorf("hard delete with readwrite: err = %v, inner ran = %v", err, inner.was("EntityHardDelete"))
	}
}

// TestSandbox_EntityScope limits entity actions to the listed entity types.
func TestSandbox_EntityScope(t *testing.T) {
	inner := newGateRecorder()
	s := NewSandboxedHostAPI(inner, "p", ResourcePolicy{Status: "approved", Permissions: []Permission{
		{Type: "entity", Access: "write", Scope: []string{"ticket"}},
	}})
	ctx := context.Background()
	if err := s.EntitySoftDelete(ctx, "agent", 3, "r"); err == nil || inner.was("EntitySoftDelete") {
		t.Errorf("agent outside scope: err = %v, inner ran = %v", err, inner.was("EntitySoftDelete"))
	}
	if err := s.EntitySoftDelete(ctx, "ticket", 3, "r"); err != nil || !inner.was("EntitySoftDelete") {
		t.Errorf("ticket in scope: err = %v, inner ran = %v", err, inner.was("EntitySoftDelete"))
	}
}

// TestSandbox_DBTableScopes checks table access with several db entries of
// different access levels (the shape first-party plugins declare).
func TestSandbox_DBTableScopes(t *testing.T) {
	policy := ResourcePolicy{Status: "approved", Permissions: []Permission{
		{Type: "db", Access: "readwrite", Scope: []string{"gk_coach_*"}},
		{Type: "db", Access: "read", Scope: []string{"users"}},
		{Type: "db", Access: "readwrite", Scope: []string{"ticket"}},
	}}
	ctx := context.Background()
	query := func(q string) error {
		_, err := NewSandboxedHostAPI(&mockInnerHostAPI{}, "coach", policy).DBQuery(ctx, q)
		return err
	}
	exec := func(q string) error {
		_, err := NewSandboxedHostAPI(&mockInnerHostAPI{}, "coach", policy).DBExec(ctx, q)
		return err
	}
	const upsertBase = "INSERT INTO gk_coach_x (id, n) VALUES (1, 2)"
	allowed := map[string]func(string) error{
		"SELECT s.id FROM gk_coach_session s JOIN users u ON u.id = s.user_id":               query,
		"SELECT EXTRACT(YEAR FROM create_time) FROM ticket":                                  query,
		"WITH recent AS (SELECT id FROM ticket) SELECT * FROM recent":                        query,
		"SELECT id FROM ticket WHERE title = 'select * from secrets'":                        query,
		"SELECT column_name FROM information_schema.columns WHERE table_name = 'gk_coach_x'": query,
		"SELECT a FROM gk_coach_x WHERE b IS DISTINCT FROM c":                                query,
		"INSERT INTO ticket (title) VALUES (?)":                                              exec,
		"UPDATE gk_coach_session SET n = 1 WHERE user_id IN (SELECT id FROM users)":          exec,
		// The sandbox sees plugin SQL before dialect conversion, so it must
		// parse both upsert forms (split so gk-lint doesn't take them for
		// statements this repo runs).
		upsertBase + " ON CONFLICT (id) DO UPDATE SET n = 2":                                     exec,
		upsertBase + " ON DUPLICATE KEY UPDATE n = VALUES(n)":                                    exec,
		"CREATE INDEX IF NOT EXISTS idx_x ON gk_coach_x (n)":                                     exec,
		"CREATE TABLE IF NOT EXISTS gk_coach_y (id INT, user_id INT REFERENCES users(id))":       exec,
		"ALTER TABLE gk_coach_prompt_specs DROP INDEX uk_coach_prompt_specs_name":                exec,
		"DELETE FROM gk_coach_x WHERE id IN (SELECT id FROM gk_coach_y)":                         exec,
		"SELECT t.id FROM ticket t, gk_coach_session s WHERE s.ticket_id = t.id FOR UPDATE":      query,
		"SELECT id FROM generate_series(1, 3) g JOIN ticket t ON t.id = g":                       query,
		"-- users\nSELECT id /* FROM secrets */ FROM ticket":                                     query,
		"SELECT `id` FROM `gk_coach_x`":                                                          query,
		"SELECT id FROM \"ticket\"":                                                              query,
		"UPDATE ticket SET title = ? WHERE id = ?":                                               exec,
		"SELECT COUNT(*) FROM ticket WHERE create_time >= ? AND title LIKE 'from users where %'": query,
		"SELECT 1": query,
		"SELECT TRIM(BOTH ' ' FROM title) FROM ticket":                                            query,
		"SELECT SUBSTRING(title FROM 1 FOR 3) FROM ticket":                                        query,
		"INSERT INTO gk_coach_x (id) SELECT id FROM ticket":                                       exec,
		"SELECT id FROM gk_coach_x UNION SELECT id FROM users":                                    query,
		"SELECT * FROM (SELECT id FROM ticket) sub":                                               query,
		"SELECT id FROM users WHERE id IN (SELECT user_id FROM gk_coach_session) ORDER BY id":     query,
		"SELECT a.id FROM gk_coach_a a LEFT OUTER JOIN gk_coach_b b ON b.a_id = a.id":             query,
		"WITH a AS (SELECT 1), b AS (SELECT id FROM ticket) SELECT * FROM a JOIN b ON true":       query,
		"SELECT id FROM ticket WHERE id = ?; ":                                                    query,
		"SELECT DISTINCT id FROM ticket":                                                          query,
		"DROP TABLE IF EXISTS gk_coach_old":                                                       exec,
		"TRUNCATE TABLE gk_coach_tmp":                                                             exec,
		"SELECT id FROM ticket WHERE note = E'it''s from users'":                                  query,
		"SELECT id FROM ticket WHERE note = 'back\\'slash from users'":                            query,
		"ALTER TABLE gk_coach_x ADD COLUMN y INT":                                                 exec,
		"UPDATE gk_coach_x x JOIN gk_coach_y y ON y.id = x.id SET x.n = y.n":                      exec,
		"DELETE FROM gk_coach_x USING gk_coach_y WHERE gk_coach_x.id = gk_coach_y.id":             exec,
		"SELECT t.title FROM ticket AS t":                                                         query,
		"SELECT id FROM ticket ORDER BY id LIMIT 1 OFFSET 2":                                      query,
		"SELECT id FROM ticket GROUP BY id HAVING COUNT(*) > 1":                                   query,
		"SELECT id FROM gk_coach_x x WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.id = x.uid)": query,
		"SELECT * FROM (ticket) t":                                                                query,
		"SELECT * FROM (gk_coach_a JOIN gk_coach_b ON gk_coach_b.a_id = gk_coach_a.id)":           query,
		"SELECT t.id FROM ticket t JOIN (gk_coach_x) x ON x.id = t.id":                            query,
		"SELECT id FROM ticket ORDER BY id DESC":                                                  query,
		"EXPLAIN SELECT id FROM ticket":                                                           query,
		"DESCRIBE gk_coach_x":                                                                     query,
	}
	for q, run := range allowed {
		if err := run(q); err != nil {
			t.Errorf("allowed %q: %v", q, err)
		}
	}

	denied := map[string]func(string) error{
		"SELECT * FROM gk_coach_x, sessions":                                  query, // comma join
		"SELECT * FROM \"sessions\"":                                          query, // quoted identifier
		"SELECT * FROM `sessions`":                                            query,
		"SELECT * FROM xgk_coach_y":                                           query, // wildcard is anchored
		"SELECT * FROM otrs.sessions":                                         query, // schema-qualified
		"UPDATE users SET pw = ''":                                            exec,  // users is read-only
		"DELETE FROM users WHERE id = 1":                                      query, // write through DBQuery
		"UPDATE users SET pw = ''" + " RETURNING id":                          query,
		"CREATE INDEX idx_login ON users (login)":                             exec,
		"INSERT INTO gk_coach_x SELECT * FROM sessions":                       exec,
		"SELECT * FROM ticket t JOIN sessions s ON s.id = t.id":               query,
		"SELECT * FROM ticket WHERE id IN (SELECT ticket_id FROM sessions)":   query,
		"WITH s AS (SELECT * FROM sessions) SELECT * FROM s":                  query,
		"UPDATE gk_coach_x x JOIN users u ON u.id = x.uid SET u.pw = x.n":     exec, // multi-table UPDATE writes users
		"TRUNCATE users":                                                      exec,
		"DROP TABLE users":                                                    exec,
		"ALTER TABLE users ADD COLUMN x INT":                                  exec,
		"SELECT id FROM gk_coach_x WHERE 1=1 /* */ UNION SELECT pw FROM auth": query,
		// Parenthesised table references (MySQL/MariaDB: FROM (t); both
		// dialects: FROM (a JOIN b ...)) name the table without a FROM or
		// JOIN directly in front of it.
		"SELECT * FROM (sessions)":                                        query,
		"SELECT * FROM (sessions) s":                                      query,
		"SELECT * FROM ((sessions))":                                      query,
		"SELECT * FROM ticket t JOIN (sessions) s ON s.id = t.id":         query,
		"SELECT * FROM (ticket JOIN sessions ON sessions.id = ticket.id)": query,
		"SELECT * FROM gk_coach_x, (sessions)":                            query,
		// Reads without FROM.
		"TABLE sessions":              query, // PostgreSQL, MySQL 8.0.19+
		"HANDLER sessions OPEN":       query, // MySQL/MariaDB
		"HANDLER sessions READ FIRST": query,
		"DESCRIBE sessions":           query,
		"DESC sessions":               query,
		"COPY sessions TO STDOUT":     query, // PostgreSQL
		// SQL the lexer cannot see.
		"CALL dump_sessions()":                    query,
		"PREPARE s FROM 'SELECT * FROM sessions'": query,
		"EXECUTE s": query,
		"LOAD DATA INFILE '/etc/passwd' INTO TABLE gk_coach_x": exec,
		// Vertical tab is whitespace to MySQL/MariaDB.
		"SELECT * FROM\vsessions": query,
	}
	for q, run := range denied {
		if err := run(q); err == nil {
			t.Errorf("denied %q: allowed", q)
		}
	}
}

// TestSandbox_HTTPStarScope: "*" in an http scope allows every host.
func TestSandbox_HTTPStarScope(t *testing.T) {
	inner := &mockInnerHostAPI{}
	s := NewSandboxedHostAPI(inner, "p", ResourcePolicy{Status: "approved", Permissions: []Permission{
		{Type: "http", Access: "readwrite", Scope: []string{"*"}},
	}})
	if _, _, err := s.HTTPRequest(context.Background(), "GET", "https://anything.example/x", nil, nil); err != nil {
		t.Fatalf("http with * scope: %v", err)
	}
	if inner.httpCalls != 1 {
		t.Errorf("inner http calls = %d, want 1", inner.httpCalls)
	}
}

// TestSandbox_CallPluginStampsCaller: plugin-to-plugin calls carry the
// calling plugin, so the host never treats plugin-built args as a
// host-built envelope.
func TestSandbox_CallPluginStampsCaller(t *testing.T) {
	inner := &callerRecorder{}
	s := NewSandboxedHostAPI(inner, "coach", ResourcePolicy{Status: "approved", Permissions: []Permission{{Type: "plugin_call"}}})
	if _, err := s.CallPlugin(context.Background(), "llm", "fn", nil); err != nil {
		t.Fatal(err)
	}
	if inner.caller != "coach" {
		t.Errorf("CallPlugin caller = %q, want coach", inner.caller)
	}
}

type callerRecorder struct {
	mockInnerHostAPI
	caller string
}

func (c *callerRecorder) CallPlugin(ctx context.Context, _, _ string, _ json.RawMessage) (json.RawMessage, error) {
	c.caller, _ = ctx.Value(PluginCallerKey).(string)
	return nil, nil
}

// TestPolicyFromDeclarations: without a stored policy a plugin gets what it
// declares, never hard delete, and the defaults when it declares nothing.
func TestPolicyFromDeclarations(t *testing.T) {
	declared := &ResourceRequest{Permissions: []Permission{
		{Type: "db", Access: "readwrite", Scope: []string{"gk_x_*"}},
		{Type: "http", Scope: []string{"api.example.com"}},
		{Type: "entity", Access: "hard_delete"},
		{Type: "entity", Access: "write"},
	}}
	p := policyFromDeclarations("x", declared)
	want := []Permission{declared.Permissions[0], declared.Permissions[1], declared.Permissions[3]}
	if len(p.Permissions) != len(want) {
		t.Fatalf("permissions = %+v, want %+v", p.Permissions, want)
	}
	for i := range want {
		if p.Permissions[i].Type != want[i].Type || p.Permissions[i].Access != want[i].Access {
			t.Errorf("permission %d = %+v, want %+v", i, p.Permissions[i], want[i])
		}
	}
	if p.MaxDBQueriesPerMin != DefaultResourcePolicy("x").MaxDBQueriesPerMin {
		t.Errorf("default limits not kept: %+v", p)
	}
	if got := policyFromDeclarations("x", nil).Permissions; len(got) != len(DefaultResourcePolicy("x").Permissions) {
		t.Errorf("no declarations: permissions = %+v, want defaults", got)
	}
}

// TestDeclaredPolicyFollowsReload: a new plugin version's declarations take
// effect on reload when no admin policy is stored; an admin policy wins.
func TestDeclaredPolicyFollowsReload(t *testing.T) {
	m := &Manager{policies: map[string]*ResourcePolicy{}}
	v1 := &ResourceRequest{Permissions: []Permission{{Type: "db", Access: "read"}}}
	v2 := &ResourceRequest{Permissions: []Permission{{Type: "db", Access: "read"}, {Type: "article", Access: "write"}}}

	if p := m.getOrCreatePolicy("x", v1); len(p.Permissions) != 1 {
		t.Fatalf("v1 permissions = %+v", p.Permissions)
	}
	if p := m.getOrCreatePolicy("x", v2); len(p.Permissions) != 2 {
		t.Errorf("after reload with v2: permissions = %+v, want v2's two", p.Permissions)
	}

	m.policies["x"] = &ResourcePolicy{PluginName: "x", Status: "restricted"}
	delete(m.declaredPolicy, "x") // what SetPolicy does, minus persistence
	if p := m.getOrCreatePolicy("x", v2); p.Status != "restricted" || len(p.Permissions) != 0 {
		t.Errorf("admin policy replaced by declarations: %+v", p)
	}
}

// ctxPlugin is an in-process plugin that reports the acting user its calls
// run for.
type ctxPlugin struct {
	mu    sync.Mutex
	actor int64
	has   bool
}

func (p *ctxPlugin) GKRegister() GKRegistration          { return GKRegistration{Name: "ctx"} }
func (p *ctxPlugin) Init(context.Context, HostAPI) error { return nil }
func (p *ctxPlugin) Shutdown(context.Context) error      { return nil }
func (p *ctxPlugin) Call(ctx context.Context, _ string, _ json.RawMessage) (json.RawMessage, error) {
	p.mu.Lock()
	p.actor, p.has = ActingUserID(ctx)
	p.mu.Unlock()
	return nil, nil
}

// TestManagerCall_ActingUserFromEnvelope: the host-built envelope's agent is
// the acting user; customers and plugin-to-plugin args are not.
func TestManagerCall_ActingUserFromEnvelope(t *testing.T) {
	p := &ctxPlugin{}
	m := &Manager{plugins: map[string]*registeredPlugin{"ctx": {plugin: p, enabled: true}}}
	ctx := context.Background()
	cases := []struct {
		name  string
		call  func() error
		actor int64
		has   bool
	}{
		{"agent number", func() error { _, err := m.Call(ctx, "ctx", "f", []byte(`{"_user_id":42}`)); return err }, 42, true},
		{"agent string", func() error { _, err := m.Call(ctx, "ctx", "f", []byte(`{"_user_id":"7"}`)); return err }, 7, true},
		{"customer", func() error {
			_, err := m.Call(ctx, "ctx", "f", []byte(`{"_user_id":5,"_customer_login":"c@x"}`))
			return err
		}, 0, false},
		{"customer role", func() error {
			_, err := m.Call(ctx, "ctx", "f", []byte(`{"_user_id":5,"_user_role":"Customer"}`))
			return err
		}, 0, false},
		{"no user", func() error { _, err := m.Call(ctx, "ctx", "f", nil); return err }, 0, false},
		{"plugin to plugin", func() error {
			_, err := m.CallFrom(ctx, "other", "ctx", "f", []byte(`{"_user_id":42}`))
			return err
		}, 0, false},
		{"plugin to plugin keeps caller's actor", func() error {
			_, err := m.CallFrom(WithActingUser(ctx, 9), "other", "ctx", "f", []byte(`{"_user_id":42}`))
			return err
		}, 9, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); err != nil {
				t.Fatal(err)
			}
			p.mu.Lock()
			defer p.mu.Unlock()
			if p.actor != tc.actor || p.has != tc.has {
				t.Errorf("acting user = %d (%v), want %d (%v)", p.actor, p.has, tc.actor, tc.has)
			}
		})
	}
}
