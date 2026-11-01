//go:build linux

package grpc_test

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/goatkit/goatflow/internal/platform/plugin"
	grpcplugin "github.com/goatkit/goatflow/internal/platform/plugin/grpc"
	"github.com/goatkit/goatflow/internal/platform/plugin/grpc/grpctest"
)

const hostcallerPkg = "./internal/platform/plugin/grpc/testdata/hostcaller"

func approvedPolicy(perms ...plugin.Permission) plugin.ResourcePolicy {
	return plugin.ResourcePolicy{PluginName: "hostcaller", Status: "approved", Permissions: perms}
}

// startHostcaller loads and initialises the hostcaller plugin under policy.
func startHostcaller(t *testing.T, bin string, policy plugin.ResourcePolicy) *grpcplugin.GRPCPlugin {
	t.Helper()
	p, err := grpcplugin.LoadGRPCPlugin(bin, "hostcaller", policy)
	if err != nil {
		t.Fatalf("LoadGRPCPlugin: %v", err)
	}
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })
	if err := p.Init(context.Background(), &mockHostAPI{}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return p
}

func call(t *testing.T, p *grpcplugin.GRPCPlugin, fn string, args map[string]any) error {
	t.Helper()
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err = p.Call(ctx, fn, raw)
	return err
}

func TestLoadGRPCPluginFailsClosedWithoutLauncher(t *testing.T) {
	bin := grpctest.BuildPlugin(t, hostcallerPkg, "hostcaller")

	t.Run("configured launcher missing", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "no-launcher")
		t.Setenv(grpctest.LauncherEnv, missing)
		_, err := grpcplugin.LoadGRPCPlugin(bin, "hostcaller", approvedPolicy())
		if err == nil || !strings.Contains(err.Error(), missing) {
			t.Fatalf("err = %v, want refusal naming %s", err, missing)
		}
	})

	t.Run("launcher not installed", func(t *testing.T) {
		t.Setenv(grpctest.LauncherEnv, "")
		t.Setenv("PATH", t.TempDir())
		_, err := grpcplugin.LoadGRPCPlugin(bin, "hostcaller", approvedPolicy())
		if err == nil || !strings.Contains(err.Error(), "goatflow-plugin-exec") {
			t.Fatalf("err = %v, want refusal naming goatflow-plugin-exec", err)
		}
	})
}

func TestGRPCPluginNetworkFollowsPolicy(t *testing.T) {
	bin := grpctest.BuildPlugin(t, hostcallerPkg, "hostcaller")
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()
	args := map[string]any{"addr": l.Addr().String()}

	denied := startHostcaller(t, bin, approvedPolicy(plugin.Permission{Type: "http", Access: "*"}))
	err = call(t, denied, "dial", args)
	if err == nil || !strings.Contains(err.Error(), "operation not permitted") {
		t.Errorf("dial without network permission: err = %v, want operation not permitted", err)
	}

	allowed := startHostcaller(t, bin, approvedPolicy(plugin.Permission{Type: plugin.PermissionNetwork}))
	if err := call(t, allowed, "dial", args); err != nil {
		t.Errorf("dial with network permission: %v", err)
	}
}

func TestGRPCPluginMemoryFollowsPolicy(t *testing.T) {
	bin := grpctest.BuildPlugin(t, hostcallerPkg, "hostcaller")

	small := approvedPolicy()
	small.MemoryMB = 128
	if err := call(t, startHostcaller(t, bin, small), "alloc", map[string]any{"mb": 256}); err == nil {
		t.Error("allocating 256 MiB under a 128 MiB policy succeeded")
	}

	if err := call(t, startHostcaller(t, bin, approvedPolicy()), "alloc", map[string]any{"mb": 256}); err != nil {
		t.Errorf("allocating 256 MiB under the default policy: %v", err)
	}
}

func TestGRPCPluginInitHonoursContext(t *testing.T) {
	bin := grpctest.BuildPlugin(t, hostcallerPkg, "hostcaller")
	t.Setenv("GOATFLOW_PLUGIN_HOSTCALLER_INIT_DELAY", "10s")
	p, err := grpcplugin.LoadGRPCPlugin(bin, "hostcaller", approvedPolicy())
	if err != nil {
		t.Fatalf("LoadGRPCPlugin: %v", err)
	}
	defer func() { _ = p.Shutdown(context.Background()) }()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	err = p.Init(ctx, &mockHostAPI{})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Init err = %v, want deadline exceeded", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("Init returned after %s, want soon after the 200ms deadline", elapsed)
	}
}

func TestSetDeclaredResourcesOnlyFillsAMissingDeclaration(t *testing.T) {
	p := startHostcaller(t, grpctest.BuildPlugin(t, hostcallerPkg, "hostcaller"), approvedPolicy())
	manifest := &plugin.ResourceRequest{MemoryMB: 256}
	p.SetDeclaredResources(manifest)
	if got := p.GKRegister().Resources; got != manifest {
		t.Fatalf("declared resources = %+v, want the manifest's", got)
	}
	p.SetDeclaredResources(&plugin.ResourceRequest{MemoryMB: 1024})
	if got := p.GKRegister().Resources; got != manifest {
		t.Errorf("second declaration replaced the first: %+v", got)
	}
}
