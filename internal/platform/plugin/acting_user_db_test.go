package plugin

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// agentDeleter is an in-process plugin that soft-deletes the agent named in
// its call args through the HostAPI it was given.
type agentDeleter struct {
	host HostAPI
}

func (p *agentDeleter) GKRegister() GKRegistration {
	return GKRegistration{Name: "acting-user-test", Version: "1.0.0", Resources: &ResourceRequest{
		Permissions: []Permission{{Type: "entity", Access: "write", Scope: []string{"agent"}}},
	}}
}

func (p *agentDeleter) Init(_ context.Context, host HostAPI) error {
	p.host = host
	return nil
}

func (p *agentDeleter) Shutdown(context.Context) error { return nil }

func (p *agentDeleter) Call(ctx context.Context, fn string, args json.RawMessage) (json.RawMessage, error) {
	var req struct {
		Target int64 `json:"target"`
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, err
	}
	switch fn {
	case "delete":
		return nil, p.host.EntitySoftDelete(ctx, "agent", req.Target, "acting user test")
	case "restore":
		return nil, p.host.EntityRestore(ctx, "agent", req.Target)
	}
	return nil, fmt.Errorf("unknown function %s", fn)
}

func insertTestAgent(t *testing.T, db *sql.DB, login string) int64 {
	t.Helper()
	now := time.Now()
	id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(
		`INSERT INTO users (login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
		 VALUES (?, 'x', 'Acting', 'Test', 1, ?, 1, ?, 1) RETURNING id`), login, now, now)
	if err != nil {
		t.Fatalf("insert agent %s: %v", login, err)
	}
	t.Cleanup(func() {
		for _, q := range []string{
			"DELETE FROM gk_recycle_bin WHERE entity_type = 'agent' AND entity_id = ?",
			"DELETE FROM gk_deletion_log WHERE entity_type = 'agent' AND entity_id = ?",
			"DELETE FROM gk_recycle_bin WHERE deleted_by = ?",
			"DELETE FROM gk_deletion_log WHERE deleted_by = ?",
			"DELETE FROM users WHERE id = ?",
		} {
			if _, err := db.Exec(database.ConvertPlaceholders(q), id); err != nil {
				t.Errorf("cleanup %q: %v", q, err)
			}
		}
	})
	return id
}

// TestEntityActionsRecordActingUser: a plugin's soft delete and restore are
// recorded as the agent the plugin call runs for (the envelope's _user_id),
// and as the system user when the call has no agent (here: a customer).
func TestEntityActionsRecordActingUser(t *testing.T) {
	if err := database.InitTestDB(); err != nil {
		t.Skipf("test database not available: %v", err)
	}
	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Skipf("test database not available: %v", err)
	}
	ctx := context.Background()
	suffix := time.Now().UnixNano()
	actor := insertTestAgent(t, db, fmt.Sprintf("acting-actor-%d", suffix))
	target := insertTestAgent(t, db, fmt.Sprintf("acting-target-%d", suffix))
	target2 := insertTestAgent(t, db, fmt.Sprintf("acting-target2-%d", suffix))

	host := NewProdHostAPI(WithDB("default", db))
	mgr := NewManager(host)
	host.PluginManager = mgr
	if err := mgr.Register(ctx, &agentDeleter{}); err != nil {
		t.Fatalf("register: %v", err)
	}
	t.Cleanup(func() { _ = mgr.Unregister(ctx, "acting-user-test") })

	call := func(fn string, env map[string]any) {
		t.Helper()
		args, _ := json.Marshal(env)
		if _, err := mgr.Call(ctx, "acting-user-test", fn, args); err != nil {
			t.Fatalf("%s: %v", fn, err)
		}
	}
	recordedBy := func(query string, args ...any) int64 {
		t.Helper()
		var by int64
		if err := db.QueryRow(database.ConvertPlaceholders(query), args...).Scan(&by); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		return by
	}

	call("delete", map[string]any{"target": target, "_user_id": actor, "_user_role": "Agent"})
	if by := recordedBy("SELECT deleted_by FROM gk_recycle_bin WHERE entity_type = 'agent' AND entity_id = ?", target); by != actor {
		t.Errorf("recycle bin deleted_by = %d, want acting agent %d", by, actor)
	}
	if by := recordedBy("SELECT change_by FROM users WHERE id = ?", target); by != actor {
		t.Errorf("agent change_by = %d, want acting agent %d", by, actor)
	}

	call("restore", map[string]any{"target": target, "_user_id": actor})
	if by := recordedBy("SELECT deleted_by FROM gk_deletion_log WHERE entity_type = 'agent' AND entity_id = ? AND action = 'restore'", target); by != actor {
		t.Errorf("restore logged as %d, want acting agent %d", by, actor)
	}

	call("delete", map[string]any{"target": target2, "_user_id": actor, "_customer_login": "someone@example.com"})
	if by := recordedBy("SELECT deleted_by FROM gk_recycle_bin WHERE entity_type = 'agent' AND entity_id = ?", target2); by != SystemUserID {
		t.Errorf("customer-initiated delete recorded as %d, want system user %d", by, SystemUserID)
	}
}
