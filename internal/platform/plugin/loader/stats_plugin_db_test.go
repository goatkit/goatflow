package loader_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/lookups"
	"github.com/goatkit/goatflow/internal/platform/plugin"
	"github.com/goatkit/goatflow/internal/platform/plugin/loader"
)

// statsEnv is the shipped stats.wasm loaded through the real loader and
// manager on a ProdHostAPI backed by the test database.
type statsEnv struct {
	t   *testing.T
	ctx context.Context
	mgr *plugin.Manager
}

func loadStatsPlugin(t *testing.T) *statsEnv {
	t.Helper()
	if err := database.InitTestDB(); err != nil {
		t.Skipf("test database not available: %v", err)
	}
	db, err := database.GetDB()
	require.NoError(t, err)

	_, filename, _, _ := runtime.Caller(0)
	wasmPath := filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "plugins", "stats", "stats.wasm")

	host := plugin.NewProdHostAPI(plugin.WithDB("default", db))
	mgr := plugin.NewManager(host)
	host.PluginManager = mgr
	ctx := context.Background()
	l := loader.NewLoader(t.TempDir(), mgr, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, l.LoadWASMFromPath(ctx, wasmPath))
	t.Cleanup(func() { _ = mgr.Unregister(ctx, "stats") })
	return &statsEnv{t: t, ctx: ctx, mgr: mgr}
}

func (e *statsEnv) call(fn string, args map[string]any) []byte {
	e.t.Helper()
	raw, err := json.Marshal(args)
	require.NoError(e.t, err)
	out, err := e.mgr.Call(e.ctx, "stats", fn, raw)
	require.NoError(e.t, err)
	return out
}

func (e *statsEnv) widgetHTML(fn string, args map[string]any) string {
	e.t.Helper()
	var w struct {
		HTML string `json:"html"`
	}
	out := e.call(fn, args)
	require.NoError(e.t, json.Unmarshal(out, &w), string(out))
	return w.HTML
}

// overviewCard reads one number from the overview widget by its label.
func overviewCard(t *testing.T, html, label string) int {
	t.Helper()
	re := regexp.MustCompile(`gk-stat-value"[^>]*>(\d+)</div>\s*<div class="gk-stat-label">` + regexp.QuoteMeta(label) + `</div>`)
	m := re.FindStringSubmatch(html)
	require.NotNil(t, m, "no %q card in widget HTML:\n%s", label, html)
	n, err := strconv.Atoi(m[1])
	require.NoError(t, err)
	return n
}

type statsFixture struct {
	agentDirect, agentRole, agentRO, agentNone int
}

// seedStatsFixture creates a visible group/queue and a restricted
// group/queue, four tickets (visible queue: two created today and one
// yesterday in the server's local day; restricted queue: one created today)
// and agents with direct rw, role rw, ro-only and no access. Everything is
// removed when the test ends.
func seedStatsFixture(t *testing.T, now time.Time) statsFixture {
	t.Helper()
	ctx := context.Background()
	db, err := database.GetDB()
	require.NoError(t, err)
	adapter := database.GetAdapter()
	sfx := fmt.Sprintf("%d", time.Now().UnixNano())

	exec := func(q string, args ...any) {
		t.Helper()
		_, err := db.Exec(database.ConvertPlaceholders(q), args...)
		require.NoError(t, err, q)
	}
	mustID := func(id int64, err error) int {
		t.Helper()
		require.NoError(t, err)
		return int(id)
	}
	cleanup := func(id int, queries ...string) {
		t.Cleanup(func() {
			for _, q := range queries {
				if _, err := db.Exec(database.ConvertPlaceholders(q), id); err != nil {
					t.Errorf("cleanup %d: %s: %v", id, q, err)
				}
			}
		})
	}
	ts := now.Format("2006-01-02 15:04:05")

	newGroup := func(name string) int {
		id := mustID(adapter.InsertWithReturning(db, database.ConvertPlaceholders(`INSERT INTO `+"`groups`"+`
			(name, comments, valid_id, create_time, change_time, create_by, change_by)
			VALUES (?, 'stats test', 1, ?, ?, 1, 1) RETURNING id`), name, ts, ts))
		cleanup(id,
			"DELETE FROM group_user WHERE group_id = ?",
			"DELETE FROM group_role WHERE group_id = ?",
			"DELETE FROM `groups` WHERE id = ?")
		return id
	}
	visibleGroup := newGroup("stats_vis_" + sfx)
	hiddenGroup := newGroup("stats_hid_" + sfx)

	var followUpID int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders("SELECT follow_up_id FROM queue ORDER BY id LIMIT 1")).Scan(&followUpID))
	newQueue := func(name string, groupID int) int {
		id := mustID(adapter.InsertWithReturning(db, database.ConvertPlaceholders(`INSERT INTO queue (name, group_id, system_address_id, salutation_id, signature_id,
				follow_up_id, follow_up_lock, valid_id, create_time, change_time, create_by, change_by)
			VALUES (?, ?, 1, 1, 1, ?, 0, 1, ?, ?, 1, 1) RETURNING id`), name, groupID, followUpID, ts, ts))
		cleanup(id, "DELETE FROM ticket WHERE queue_id = ?", "DELETE FROM queue WHERE id = ?")
		return id
	}
	visibleQueue := newQueue("stats_vis_q_"+sfx, visibleGroup)
	hiddenQueue := newQueue("stats_hid_q_"+sfx, hiddenGroup)

	stateNew, err := lookups.ID(ctx, db, lookups.StateLookup, lookups.StateNew)
	require.NoError(t, err)
	lockUnlock, err := lookups.ID(ctx, db, lookups.LockType, lookups.LockUnlock)
	require.NoError(t, err)
	var priorityID int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders("SELECT id FROM ticket_priority WHERE valid_id = 1 ORDER BY id LIMIT 1")).Scan(&priorityID))

	// Wall-clock create times in the server's local zone, as GoatFlow stores them.
	y, m, d := now.Date()
	startToday := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	today := startToday.Add(now.Sub(startToday) / 2)
	yesterday := startToday.Add(-time.Hour)
	newTicket := func(n int, queueID int, created time.Time) {
		c := created.Format("2006-01-02 15:04:05")
		exec(`INSERT INTO ticket (tn, title, queue_id, ticket_lock_id, user_id, responsible_user_id,
				ticket_priority_id, ticket_state_id, timeout, until_time, escalation_time,
				escalation_update_time, escalation_response_time, escalation_solution_time,
				archive_flag, create_time, create_by, change_time, change_by)
			VALUES (?, 'stats test', ?, ?, 1, 1, ?, ?, 0, 0, 0, 0, 0, 0, 0, ?, 1, ?, 1)`,
			fmt.Sprintf("st%s%d", sfx, n), queueID, lockUnlock, priorityID, stateNew, c, c)
	}
	newTicket(1, visibleQueue, today)
	newTicket(2, visibleQueue, today.Add(time.Second))
	newTicket(3, visibleQueue, yesterday)
	newTicket(4, hiddenQueue, today)

	newUser := func(login string) int {
		id := mustID(adapter.InsertWithReturning(db, database.ConvertPlaceholders(`INSERT INTO users (login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
			VALUES (?, 'x', 'Stats', 'Agent', 1, ?, 1, ?, 1) RETURNING id`), login, ts, ts))
		cleanup(id,
			"DELETE FROM group_user WHERE user_id = ?",
			"DELETE FROM role_user WHERE user_id = ?",
			"DELETE FROM users WHERE id = ?")
		return id
	}
	grantUser := func(userID, groupID int, perm string) {
		exec(`INSERT INTO group_user (user_id, group_id, permission_key, create_time, change_time, create_by, change_by)
			VALUES (?, ?, ?, ?, ?, 1, 1)`, userID, groupID, perm, ts, ts)
	}

	var f statsFixture
	f.agentDirect = newUser("stats_direct_" + sfx)
	grantUser(f.agentDirect, visibleGroup, "rw")

	f.agentRole = newUser("stats_role_" + sfx)
	roleID := mustID(adapter.InsertWithReturning(db, database.ConvertPlaceholders(`INSERT INTO roles (name, comments, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, 'stats test', 1, ?, 1, ?, 1) RETURNING id`), "stats_role_"+sfx, ts, ts))
	cleanup(roleID,
		"DELETE FROM role_user WHERE role_id = ?",
		"DELETE FROM group_role WHERE role_id = ?",
		"DELETE FROM roles WHERE id = ?")
	exec(`INSERT INTO group_role (role_id, group_id, permission_key, permission_value, create_time, create_by, change_time, change_by)
		VALUES (?, ?, 'rw', 1, ?, 1, ?, 1)`, roleID, visibleGroup, ts, ts)
	exec(`INSERT INTO role_user (user_id, role_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, 1, ?, 1)`, f.agentRole, roleID, ts, ts)

	f.agentRO = newUser("stats_ro_" + sfx)
	grantUser(f.agentRO, hiddenGroup, "ro")

	f.agentNone = newUser("stats_none_" + sfx)
	return f
}

// useDistantLocalZone makes the server-local day differ from the UTC day for
// the duration of the test, so "today" computed by the database clock (UTC in
// the test containers) and the server's local day cannot agree by accident.
func useDistantLocalZone(t *testing.T) time.Time {
	t.Helper()
	offset := 12 * 3600
	if time.Now().UTC().Hour() < 12 {
		offset = -12 * 3600
	}
	saved := time.Local
	time.Local = time.FixedZone("stats-test", offset)
	t.Cleanup(func() { time.Local = saved })
	return time.Now()
}

func TestStatsPlugin_TodayAndQueueAccess(t *testing.T) {
	e := loadStatsPlugin(t)
	now := useDistantLocalZone(t)

	admin := map[string]any{"_user_id": 1, "_is_admin": true}
	before := e.widgetHTML("widget_overview", admin)
	baseTotal := overviewCard(t, before, "Total")
	baseNewToday := overviewCard(t, before, "New Today")

	f := seedStatsFixture(t, now)
	agent := func(id int) map[string]any { return map[string]any{"_user_id": id, "_is_admin": false} }

	t.Run("new today counts the local day only", func(t *testing.T) {
		html := e.widgetHTML("widget_overview", agent(f.agentDirect))
		assert.Equal(t, 2, overviewCard(t, html, "New Today"), html)
		assert.Equal(t, 3, overviewCard(t, html, "Total"), html)
		assert.Equal(t, 3, overviewCard(t, html, "Open"), html)
	})

	t.Run("admin sees the restricted queue", func(t *testing.T) {
		html := e.widgetHTML("widget_overview", admin)
		assert.Equal(t, baseTotal+4, overviewCard(t, html, "Total"), html)
		assert.Equal(t, baseNewToday+3, overviewCard(t, html, "New Today"), html)
	})

	t.Run("role-granted rw counts like direct rw", func(t *testing.T) {
		html := e.widgetHTML("widget_overview", agent(f.agentRole))
		assert.Equal(t, 3, overviewCard(t, html, "Total"), html)
		assert.Equal(t, 2, overviewCard(t, html, "New Today"), html)
	})

	t.Run("ro-only, no access and unidentified callers see nothing", func(t *testing.T) {
		for name, args := range map[string]map[string]any{
			"ro":           agent(f.agentRO),
			"none":         agent(f.agentNone),
			"unidentified": {"_is_admin": false},
		} {
			html := e.widgetHTML("widget_overview", args)
			assert.Equal(t, 0, overviewCard(t, html, "Total"), "%s: %s", name, html)
			assert.Equal(t, 0, overviewCard(t, html, "New Today"), "%s: %s", name, html)
		}
	})

	t.Run("overview route applies queue access", func(t *testing.T) {
		var got struct {
			Total int `json:"total"`
			Open  int `json:"open"`
		}
		args := agent(f.agentDirect)
		args["range"] = "all"
		out := e.call("overview", args)
		require.NoError(t, json.Unmarshal(out, &got), string(out))
		assert.Equal(t, 3, got.Total, string(out))
		assert.Equal(t, 3, got.Open, string(out))

		none := agent(f.agentNone)
		out = e.call("overview", none)
		require.NoError(t, json.Unmarshal(out, &got), string(out))
		assert.Equal(t, 0, got.Total, string(out))
	})

	t.Run("timeline buckets by local day", func(t *testing.T) {
		var got struct {
			Timeline []struct {
				Date  string `json:"date"`
				Count int    `json:"count"`
			} `json:"timeline"`
		}
		args := agent(f.agentDirect)
		args["range"] = "7d"
		out := e.call("timeline", args)
		require.NoError(t, json.Unmarshal(out, &got), string(out))
		require.Len(t, got.Timeline, 7, string(out))
		last, prev := got.Timeline[6], got.Timeline[5]
		assert.Equal(t, now.Format("2006-01-02"), last.Date, string(out))
		assert.Equal(t, 2, last.Count, string(out))
		assert.Equal(t, now.AddDate(0, 0, -1).Format("2006-01-02"), prev.Date, string(out))
		assert.Equal(t, 1, prev.Count, string(out))
	})
}

func TestStatsPlugin_FailedQueryIsUnavailable(t *testing.T) {
	e := loadStatsPlugin(t)
	db, err := database.GetDB()
	require.NoError(t, err)

	original, ok := e.mgr.GetPolicy("stats")
	require.True(t, ok)
	t.Cleanup(func() {
		e.mgr.SetPolicy("stats", original)
		if _, err := db.Exec(database.ConvertPlaceholders(
			"DELETE FROM sysconfig_modified WHERE name = ? AND user_id IS NULL"), "Plugin::stats::Policy"); err != nil {
			t.Errorf("cleanup policy: %v", err)
		}
	})
	// Restrict the plugin's db scope so every ticket query is refused by the
	// sandbox: the real failure path a revoked permission produces.
	broken := original
	broken.Permissions = []plugin.Permission{{Type: "db", Access: "read", Scope: []string{"users"}}}
	e.mgr.SetPolicy("stats", broken)

	admin := map[string]any{"_user_id": 1, "_is_admin": true}
	for _, fn := range []string{"widget_overview", "widget_by_status", "widget_chart", "widget_sla", "widget_time_tracking"} {
		html := e.widgetHTML(fn, admin)
		assert.Contains(t, html, "stats-unavailable", "%s: %s", fn, html)
		assert.NotRegexp(t, `gk-stat-value"[^>]*>\d`, html, fn)
		assert.NotContains(t, html, "No data", fn)
	}

	var got map[string]any
	out := e.call("overview", admin)
	require.NoError(t, json.Unmarshal(out, &got), string(out))
	assert.Equal(t, "query_failed", got["error"], string(out))
	assert.NotContains(t, got, "total", string(out))
}
