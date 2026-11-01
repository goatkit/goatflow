package plugin_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/goatkit/goatflow/internal/platform/plugin"
)

// storedPolicyHost serves admin-stored policies (sysconfig name -> JSON) and
// records statements run against it.
type storedPolicyHost struct {
	mockHostAPI
	policies map[string]string
	execs    []string
}

func (h *storedPolicyHost) DBQuery(_ context.Context, _ string, args ...any) ([]map[string]any, error) {
	if len(args) == 1 {
		if key, ok := args[0].(string); ok {
			if v, ok := h.policies[key]; ok {
				return []map[string]any{{"effective_value": v}}, nil
			}
		}
	}
	return nil, nil
}

func (h *storedPolicyHost) DBExec(_ context.Context, query string, args ...any) (int64, error) {
	h.execs = append(h.execs, strings.Join(strings.Fields(query), " ")+" "+jsonArgs(args))
	return 1, nil
}

func jsonArgs(args []any) string {
	b, _ := json.Marshal(args)
	return string(b)
}

func permissionTypes(perms []plugin.Permission) string {
	types := make([]string, 0, len(perms))
	for _, p := range perms {
		types = append(types, p.Type+":"+p.Access)
	}
	return strings.Join(types, ",")
}

// TestResolvePolicyPrecedence: a stored policy beats the declarations,
// which beat the defaults; declared limits are copied, a declared hard
// delete is not, and nothing is cached.
func TestResolvePolicyPrecedence(t *testing.T) {
	host := &storedPolicyHost{policies: map[string]string{
		"Plugin::stored::Policy": `{"plugin_name":"stored","memory_mb":128,"permissions":[{"type":"entity","access":"hard_delete"}]}`,
	}}
	mgr := plugin.NewManager(host)
	declared := &plugin.ResourceRequest{
		MemoryMB: 64, CPUMillis: 250,
		CallTimeout: "5s", InitTimeout: "2s", ShutdownTimeout: "1s",
		Permissions: []plugin.Permission{
			{Type: "db", Access: "read"},
			{Type: "entity", Access: "hard_delete"},
			{Type: "network"},
		},
	}

	stored := mgr.ResolvePolicy("stored", declared)
	if stored.MemoryMB != 128 || permissionTypes(stored.Permissions) != "entity:hard_delete" {
		t.Errorf("stored policy not used: %+v", stored)
	}

	got := mgr.ResolvePolicy("declared", declared)
	if got.MemoryMB != 64 || got.CPUMillis != 250 || got.CallTimeout != "5s" ||
		got.InitTimeout != "2s" || got.ShutdownTimeout != "1s" {
		t.Errorf("declared limits not copied: %+v", got)
	}
	if perms := permissionTypes(got.Permissions); perms != "db:read,network:" {
		t.Errorf("declared permissions = %s, want db:read,network: (no hard delete)", perms)
	}
	if l := got.Launch(); l.MemoryMB != 64 || l.CPUMillis != 250 || !l.Network {
		t.Errorf("launch limits = %+v", l)
	}

	partial := mgr.ResolvePolicy("partial", &plugin.ResourceRequest{MemoryMB: 64})
	def := plugin.DefaultResourcePolicy("partial")
	if partial.MemoryMB != 64 || partial.CPUMillis != def.CPUMillis || partial.CallTimeout != def.CallTimeout ||
		permissionTypes(partial.Permissions) != permissionTypes(def.Permissions) {
		t.Errorf("undeclared fields should keep defaults: %+v", partial)
	}

	if d := mgr.ResolvePolicy("default", nil); d.MemoryMB != 512 || d.CPUMillis != 1000 || d.Status != "pending_review" {
		t.Errorf("default policy = %+v", d)
	}

	mgr.SetPolicy("declared", plugin.ResourcePolicy{MemoryMB: 32})
	if p := mgr.ResolvePolicy("declared", declared); p.MemoryMB != 32 || len(p.Permissions) != 0 {
		t.Errorf("admin policy set at runtime not used: %+v", p)
	}

	for _, name := range []string{"stored", "partial", "default"} {
		if _, ok := mgr.GetPolicy(name); ok {
			t.Errorf("ResolvePolicy cached a policy for %s", name)
		}
	}
}

// TestResetPolicy: resetting drops the stored policy and returns the
// plugin to its declarations.
func TestResetPolicy(t *testing.T) {
	host := &storedPolicyHost{}
	mgr := plugin.NewManager(host)
	p := &initPlugin{name: "reset", resources: &plugin.ResourceRequest{MemoryMB: 64}}
	if err := mgr.Register(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	mgr.SetPolicy("reset", plugin.ResourcePolicy{MemoryMB: 256})
	if !mgr.HasStoredPolicy("reset") {
		t.Fatal("HasStoredPolicy false after SetPolicy")
	}

	host.execs = nil
	got, err := mgr.ResetPolicy("reset")
	if err != nil {
		t.Fatal(err)
	}
	if got.MemoryMB != 64 {
		t.Errorf("reset policy memory = %d, want the declared 64", got.MemoryMB)
	}
	if mgr.HasStoredPolicy("reset") {
		t.Error("HasStoredPolicy true after ResetPolicy")
	}
	if cur, _ := mgr.GetPolicy("reset"); cur.MemoryMB != 64 {
		t.Errorf("current policy memory = %d, want 64", cur.MemoryMB)
	}
	key := strings.Join([]string{"Plugin", "reset", "Policy"}, "::")
	want := `DELETE FROM sysconfig_modified WHERE name = ? ["` + key + `"]`
	if len(host.execs) != 1 || host.execs[0] != want {
		t.Errorf("statements = %q, want %q", host.execs, want)
	}
}

// initPlugin's Init blocks until release is closed, or until its context
// ends when honourCtx is set.
type initPlugin struct {
	name      string
	resources *plugin.ResourceRequest
	honourCtx bool
	release   chan struct{}
}

func (p *initPlugin) GKRegister() plugin.GKRegistration {
	return plugin.GKRegistration{Name: p.name, Version: "1.0.0", Resources: p.resources}
}

func (p *initPlugin) Init(ctx context.Context, _ plugin.HostAPI) error {
	switch {
	case p.release == nil:
		return nil
	case p.honourCtx:
		<-ctx.Done()
		return ctx.Err()
	default:
		<-p.release
		return nil
	}
}

func (p *initPlugin) Call(context.Context, string, json.RawMessage) (json.RawMessage, error) {
	return json.RawMessage(`"` + p.name + `"`), nil
}

func (p *initPlugin) Shutdown(context.Context) error { return nil }

// TestInitTimeout: Init is bounded by the policy's InitTimeout whether or
// not the plugin honours its context; a plugin that runs out of time is not
// registered, and a replacement that does leaves the old plugin serving.
func TestInitTimeout(t *testing.T) {
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	slow := &plugin.ResourceRequest{InitTimeout: "50ms"}

	for _, honour := range []bool{true, false} {
		mgr := plugin.NewManager(&mockHostAPI{})
		start := time.Now()
		err := mgr.Register(context.Background(), &initPlugin{name: "slow", resources: slow, honourCtx: honour, release: release})
		if err == nil || !strings.Contains(err.Error(), "init timed out after 50ms") {
			t.Fatalf("honourCtx=%v: Register error = %v, want init timed out after 50ms", honour, err)
		}
		if elapsed := time.Since(start); elapsed > 5*time.Second {
			t.Fatalf("honourCtx=%v: Register returned after %s", honour, elapsed)
		}
		if mgr.IsRegistered("slow") {
			t.Errorf("honourCtx=%v: plugin registered despite init timeout", honour)
		}
	}

	mgr := plugin.NewManager(&mockHostAPI{})
	ctx := context.Background()
	if err := mgr.Register(ctx, &initPlugin{name: "swap"}); err != nil {
		t.Fatal(err)
	}
	err := mgr.ReplacePlugin(ctx, "swap", &initPlugin{name: "swap", resources: slow, release: release})
	if err == nil || !strings.Contains(err.Error(), "init timed out after 50ms") {
		t.Fatalf("ReplacePlugin error = %v", err)
	}
	if out, err := mgr.Call(ctx, "swap", "x", nil); err != nil || string(out) != `"swap"` {
		t.Errorf("old plugin no longer serving: %s, %v", out, err)
	}
}

// TestLimitDuration: invalid or non-positive durations fall back.
func TestLimitDuration(t *testing.T) {
	for in, want := range map[string]time.Duration{
		"50ms": 50 * time.Millisecond, "": 10 * time.Second, "soon": 10 * time.Second,
		"0s": 10 * time.Second, "-1s": 10 * time.Second,
	} {
		if got := plugin.LimitDuration(in, 10*time.Second); got != want {
			t.Errorf("LimitDuration(%q) = %s, want %s", in, got, want)
		}
	}
}
