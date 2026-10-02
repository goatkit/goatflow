package loader_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/plugin"
	grpcplugin "github.com/goatkit/goatflow/internal/platform/plugin/grpc"
	"github.com/goatkit/goatflow/internal/platform/plugin/loader"
)

// orgFixture is two organisations, each with one gk_org_plugin_access row
// whose plugin_name starts with marker and ends in its org's letter.
type orgFixture struct {
	marker     string
	orgA, orgB int64
}

func seedTwoOrgs(t *testing.T, db *sql.DB) orgFixture {
	t.Helper()
	f := orgFixture{marker: fmt.Sprintf("orgscope-%d", time.Now().UnixNano())}
	for _, side := range []struct {
		letter string
		id     *int64
	}{{"a", &f.orgA}, {"b", &f.orgB}} {
		slug := f.marker + "-" + side.letter
		id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
			INSERT INTO gk_organisation (name, slug, status, valid_id, create_time, create_by, change_time, change_by)
			VALUES (?, ?, 'active', 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1) RETURNING id`), slug, slug)
		require.NoError(t, err)
		*side.id = id
		_, err = db.Exec(database.ConvertPlaceholders(`
			INSERT INTO gk_org_plugin_access (org_id, plugin_name, group_id, create_time, create_by)
			VALUES (?, ?, 1, CURRENT_TIMESTAMP, 1)`), id, slug)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM gk_org_plugin_access WHERE org_id = ?`), id)
			_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM gk_organisation WHERE id = ?`), id)
		})
	}
	return f
}

// scopedQuery is what the plugin asks for: every fixture row, no org filter.
const scopedQuery = "SELECT plugin_name FROM gk_org_plugin_access WHERE plugin_name LIKE ?"

func (f orgFixture) names(rows []map[string]any) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, fmt.Sprint(r["plugin_name"]))
	}
	sort.Strings(out)
	return out
}

func newCallerContextManager(t *testing.T) (*plugin.Manager, *sql.DB) {
	t.Helper()
	if err := database.InitTestDB(); err != nil {
		t.Skipf("test database not available: %v", err)
	}
	db, err := database.GetDB()
	require.NoError(t, err)
	host := plugin.NewProdHostAPI(plugin.WithDB("default", db))
	mgr := plugin.NewManager(host)
	host.PluginManager = mgr
	return mgr, db
}

func envelope(t *testing.T, env map[string]any) []byte {
	t.Helper()
	raw, err := json.Marshal(env)
	require.NoError(t, err)
	return raw
}

// TestGRPCPluginCallRunsForCallerOrg: a gRPC plugin's DB query made with the
// call's context sees only the caller's organisation's rows, and the host
// reports the caller's org and language; without an org in the envelope the
// query is unscoped.
func TestGRPCPluginCallRunsForCallerOrg(t *testing.T) {
	mgr, db := newCallerContextManager(t)
	f := seedTwoOrgs(t, db)

	_, filename, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(filename), "..", "..", "..", "..")
	bin := filepath.Join(t.TempDir(), "hostcaller")
	build := exec.Command("go", "build", "-o", bin, "./internal/platform/plugin/grpc/testdata/hostcaller")
	build.Dir = repoRoot
	build.Env = os.Environ()
	out, err := build.CombinedOutput()
	require.NoError(t, err, string(out))

	ctx := context.Background()
	p, err := grpcplugin.LoadGRPCPlugin(bin, "hostcaller", plugin.DefaultResourcePolicy("hostcaller"))
	require.NoError(t, err)
	require.NoError(t, mgr.Register(ctx, p))
	t.Cleanup(func() { mgr.ShutdownAll(ctx) })

	view := func(env map[string]any) (int64, string, []string) {
		t.Helper()
		env["query"], env["arg"], env["key"] = scopedQuery, f.marker+"%", "common.save"
		raw, err := mgr.Call(ctx, "hostcaller", "caller_view", envelope(t, env))
		require.NoError(t, err)
		var res struct {
			Org  int64            `json:"org"`
			Text string           `json:"text"`
			Rows []map[string]any `json:"rows"`
		}
		require.NoError(t, json.Unmarshal(raw, &res), string(raw))
		return res.Org, res.Text, f.names(res.Rows)
	}

	org, text, names := view(map[string]any{"_user_id": 1, "_org_id": f.orgA, "_lang": "de"})
	assert.Equal(t, f.orgA, org)
	assert.Equal(t, "Speichern", text)
	assert.Equal(t, []string{f.marker + "-a"}, names, "org A call must not read org B rows")

	org, _, names = view(map[string]any{"_user_id": 1, "_org_id": f.orgB})
	assert.Equal(t, f.orgB, org)
	assert.Equal(t, []string{f.marker + "-b"}, names)

	org, text, names = view(map[string]any{"_user_id": 1})
	assert.Zero(t, org)
	assert.Equal(t, "Save", text)
	assert.Equal(t, []string{f.marker + "-a", f.marker + "-b"}, names, "no org: single-org mode, unscoped")
}

// TestWASMPluginCallRunsForCallerOrg: the same for a WASM plugin, whose
// HostAPI calls always run with the call's context.
func TestWASMPluginCallRunsForCallerOrg(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	wasmPath := filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "plugins", "test-hostapi-wasm", "test-hostapi.wasm")
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		t.Skipf("WASM plugin not built (requires TinyGo): %s", filepath.Base(wasmPath))
	}
	mgr, db := newCallerContextManager(t)
	f := seedTwoOrgs(t, db)

	// The guest declares db access to a scratch table only; grant the org
	// table. test-hostapi is disabled by default (example plugin).
	policy := plugin.DefaultResourcePolicy("test-hostapi")
	policy.Permissions = []plugin.Permission{{Type: "db", Access: "read", Scope: []string{"gk_org_plugin_access"}}}
	mgr.SetPolicy("test-hostapi", policy)
	t.Cleanup(func() {
		for _, key := range []string{"Plugin::test-hostapi::Policy", "Plugin::test-hostapi::Enabled"} {
			_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM sysconfig_modified WHERE name = ?`), key)
		}
	})
	ctx := context.Background()
	l := loader.NewLoader(t.TempDir(), mgr, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, l.LoadWASMFromPath(ctx, wasmPath))
	t.Cleanup(func() { _ = mgr.Unregister(ctx, "test-hostapi") })
	require.NoError(t, mgr.Enable("test-hostapi"))

	host := func(env map[string]any, fn string, args any) []byte {
		t.Helper()
		env["fn"], env["args"] = fn, args
		raw, err := mgr.Call(ctx, "test-hostapi", "host", envelope(t, env))
		require.NoError(t, err)
		return raw
	}
	view := func(env func() map[string]any) (int64, string, []string) {
		t.Helper()
		var org int64
		raw := host(env(), "org_id", map[string]any{})
		require.NoError(t, json.Unmarshal(raw, &org), string(raw))
		var tr struct {
			Value string `json:"value"`
		}
		raw = host(env(), "translate", map[string]any{"key": "common.save"})
		require.NoError(t, json.Unmarshal(raw, &tr), string(raw))
		var rows []map[string]any
		raw = host(env(), "db_query", map[string]any{"query": scopedQuery, "args": []any{f.marker + "%"}})
		require.NoError(t, json.Unmarshal(raw, &rows), string(raw))
		return org, tr.Value, f.names(rows)
	}

	org, text, names := view(func() map[string]any { return map[string]any{"_user_id": 1, "_org_id": f.orgA, "_lang": "de"} })
	assert.Equal(t, f.orgA, org)
	assert.Equal(t, "Speichern", text)
	assert.Equal(t, []string{f.marker + "-a"}, names, "org A call must not read org B rows")

	org, _, names = view(func() map[string]any { return map[string]any{"_org_id": f.orgB} })
	assert.Equal(t, f.orgB, org)
	assert.Equal(t, []string{f.marker + "-b"}, names)

	org, text, names = view(func() map[string]any { return map[string]any{"_user_id": 1} })
	assert.Zero(t, org)
	assert.Equal(t, "Save", text)
	assert.Equal(t, []string{f.marker + "-a", f.marker + "-b"}, names, "no org: single-org mode, unscoped")
}
