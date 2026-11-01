package loader

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/goatkit/goatflow/internal/platform/plugin"
)

// startedPlugin records the policy it was started under.
type startedPlugin struct {
	reg      plugin.GKRegistration
	policy   plugin.ResourcePolicy
	shutdown bool
}

func (p *startedPlugin) GKRegister() plugin.GKRegistration          { return p.reg }
func (p *startedPlugin) Init(context.Context, plugin.HostAPI) error { return nil }
func (p *startedPlugin) Shutdown(context.Context) error             { p.shutdown = true; return nil }
func (p *startedPlugin) Call(context.Context, string, json.RawMessage) (json.RawMessage, error) {
	return nil, nil
}

// TestStartPluginRelaunchesOnlyWhenLaunchLimitsChange: a plugin is started
// a second time, under the limits its registration resolves to, only when
// those differ from the ones it was started with.
func TestStartPluginRelaunchesOnlyWhenLaunchLimitsChange(t *testing.T) {
	grpcLimits := plugin.ResourcePolicy.Launch
	network := []plugin.Permission{{Type: plugin.PermissionNetwork}}
	tests := []struct {
		name       string
		preName    string                  // name known before start
		stored     *plugin.ResourcePolicy  // admin policy for "p"
		declared   *plugin.ResourceRequest // known before start (plugin.yaml)
		registered *plugin.ResourceRequest // in the plugin's registration
		limits     func(plugin.ResourcePolicy) plugin.LaunchLimits
		wantStarts int
		want       plugin.LaunchLimits
	}{
		{
			name:       "registration declares other memory",
			registered: &plugin.ResourceRequest{MemoryMB: 64},
			limits:     grpcLimits, wantStarts: 2,
			want: plugin.LaunchLimits{MemoryMB: 64, CPUMillis: 1000},
		},
		{
			name:       "registration leaves launch limits alone",
			registered: &plugin.ResourceRequest{Permissions: []plugin.Permission{{Type: "db", Access: "read"}}},
			limits:     grpcLimits, wantStarts: 1,
			want: plugin.LaunchLimits{MemoryMB: 512, CPUMillis: 1000},
		},
		{
			name:       "plugin.yaml declared them before start",
			declared:   &plugin.ResourceRequest{MemoryMB: 64, CPUMillis: 250},
			registered: &plugin.ResourceRequest{MemoryMB: 64, CPUMillis: 250},
			limits:     grpcLimits, wantStarts: 1,
			want: plugin.LaunchLimits{MemoryMB: 64, CPUMillis: 250},
		},
		{
			name:       "network permission changes a gRPC launch",
			registered: &plugin.ResourceRequest{Permissions: network},
			limits:     grpcLimits, wantStarts: 2,
			want: plugin.LaunchLimits{MemoryMB: 512, CPUMillis: 1000, Network: true},
		},
		{
			name:       "network permission is no WASM launch limit",
			registered: &plugin.ResourceRequest{Permissions: network},
			limits:     wasmLaunchLimits, wantStarts: 1,
			want: plugin.LaunchLimits{MemoryMB: 512},
		},
		{
			name:       "stored policy beats declarations",
			stored:     &plugin.ResourcePolicy{MemoryMB: 128},
			registered: &plugin.ResourceRequest{MemoryMB: 64},
			limits:     grpcLimits, wantStarts: 1,
			want: plugin.LaunchLimits{MemoryMB: 128, CPUMillis: 1000},
		},
		{
			name:    "stored policy under the registered name",
			preName: "p-file",
			stored:  &plugin.ResourcePolicy{MemoryMB: 32},
			limits:  wasmLaunchLimits, wantStarts: 2,
			want: plugin.LaunchLimits{MemoryMB: 32},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := plugin.NewManager(nil)
			if tt.stored != nil {
				mgr.SetPolicy("p", *tt.stored)
			}
			l := NewLoader(t.TempDir(), mgr, nil)
			preName := tt.preName
			if preName == "" {
				preName = "p"
			}

			var started []*startedPlugin
			got, err := l.startPlugin(context.Background(), preName, tt.declared, tt.limits,
				func(_ context.Context, policy plugin.ResourcePolicy) (plugin.Plugin, error) {
					sp := &startedPlugin{reg: plugin.GKRegistration{Name: "p", Resources: tt.registered}, policy: policy}
					started = append(started, sp)
					return sp, nil
				})
			if err != nil {
				t.Fatal(err)
			}

			if len(started) != tt.wantStarts {
				t.Fatalf("started %d times, want %d", len(started), tt.wantStarts)
			}
			last := started[len(started)-1]
			if got != last || last.shutdown {
				t.Fatal("the last instance started is not the one returned running")
			}
			for _, sp := range started[:len(started)-1] {
				if !sp.shutdown {
					t.Error("replaced instance left running")
				}
			}
			if limits := tt.limits(last.policy); limits != tt.want {
				t.Errorf("running under %+v, want %+v", limits, tt.want)
			}
		})
	}
}
