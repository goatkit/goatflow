package plugin

import (
	"context"
	"encoding/json"
	"testing"
)

type menuPlugin struct {
	name  string
	items []MenuItemSpec
}

func (p *menuPlugin) GKRegister() GKRegistration {
	return GKRegistration{Name: p.name, MenuItems: p.items}
}
func (p *menuPlugin) Init(context.Context, HostAPI) error { return nil }
func (p *menuPlugin) Call(context.Context, string, json.RawMessage) (json.RawMessage, error) {
	return nil, nil
}
func (p *menuPlugin) Shutdown(context.Context) error { return nil }

// The nav renders MenuItems in slice order, and plugins live in a map:
// the result must follow Order, then plugin name, then item ID, so the
// menu is the same on every request and after every restart.
func TestMenuItemsFollowOrderThenNameThenID(t *testing.T) {
	ctx := context.Background()
	mgr := NewManager(nil)
	for _, p := range []*menuPlugin{
		{name: "zeta", items: []MenuItemSpec{{ID: "z1", Location: "profile", Order: 10}}},
		{name: "alpha", items: []MenuItemSpec{
			{ID: "a2", Location: "profile", Order: 10},
			{ID: "a1", Location: "profile", Order: 10},
			{ID: "nav", Location: "agent", Order: 1},
		}},
		{name: "mid", items: []MenuItemSpec{{ID: "m1", Location: "profile", Order: 5}}},
	} {
		if err := mgr.Register(ctx, p); err != nil {
			t.Fatalf("register %s: %v", p.name, err)
		}
	}
	want := []string{"mid/m1", "alpha/a1", "alpha/a2", "zeta/z1"}
	for run := 0; run < 20; run++ {
		var got []string
		for _, it := range mgr.MenuItems("profile") {
			got = append(got, it.PluginName+"/"+it.ID)
		}
		if len(got) != len(want) {
			t.Fatalf("profile items = %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("run %d: profile items = %v, want %v", run, got, want)
			}
		}
	}
}
