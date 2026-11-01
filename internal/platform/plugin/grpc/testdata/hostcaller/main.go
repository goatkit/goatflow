// Command hostcaller is a test gRPC plugin for runtime_test.go. Each function
// makes one HostAPI call back to the host and reports the host's error, so
// the test can see what the host's sandbox allowed. "dial" and "alloc" probe
// the process's own network and memory limits.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/goatkit/goatflow/pkg/plugin"
	"github.com/goatkit/goatflow/pkg/plugin/grpcutil"
)

type hostCaller struct {
	host plugin.HostAPI
}

// sink keeps "alloc" allocations reachable.
var sink [][]byte

func (p *hostCaller) GKRegister() (*plugin.GKRegistration, error) {
	return &plugin.GKRegistration{Name: "hostcaller", Version: "1.0.0"}, nil
}

func (p *hostCaller) Init(map[string]string) error { return nil }

// InitWithHost sleeps for config "init_delay" (a duration) before returning.
func (p *hostCaller) InitWithHost(config map[string]string, host plugin.HostAPI) error {
	if d, err := time.ParseDuration(config["init_delay"]); err == nil {
		time.Sleep(d)
	}
	p.host = host
	return nil
}

func (p *hostCaller) Shutdown() error { return nil }

func (p *hostCaller) Call(fn string, args json.RawMessage) (json.RawMessage, error) {
	return p.CallWithContext(context.Background(), fn, args)
}

// CallWithContext runs fn. Functions ending in "_no_ctx" use a fresh context
// instead of the call's, so the host sees no call context.
func (p *hostCaller) CallWithContext(ctx context.Context, fn string, args json.RawMessage) (json.RawMessage, error) {
	if p.host == nil {
		return nil, fmt.Errorf("no host")
	}
	switch fn {
	case "caller_view":
		return p.callerView(ctx, args)
	case "dial":
		return dial(args)
	case "alloc":
		return alloc(args)
	}
	var err error
	switch fn {
	case "db_query":
		_, err = p.host.DBQuery(ctx, "SELECT id FROM ticket")
	case "entity_soft_delete":
		err = p.host.EntitySoftDelete(ctx, "ticket", 7, "test")
	case "entity_soft_delete_no_ctx":
		err = p.host.EntitySoftDelete(context.Background(), "ticket", 7, "test")
	case "entity_hard_delete":
		err = p.host.EntityHardDelete(ctx, "ticket", 7, "test")
	default:
		return nil, fmt.Errorf("unknown function: %s", fn)
	}
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]bool{"ok": true})
}

// dial opens a TCP connection to args.addr.
func dial(raw json.RawMessage) (json.RawMessage, error) {
	var args struct {
		Addr string `json:"addr"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	c, err := net.DialTimeout("tcp", args.Addr, 2*time.Second)
	if err != nil {
		return nil, err
	}
	_ = c.Close()
	return json.Marshal(map[string]bool{"ok": true})
}

// alloc allocates and touches args.mb MiB.
func alloc(raw json.RawMessage) (json.RawMessage, error) {
	var args struct {
		MB int `json:"mb"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	for range args.MB {
		b := make([]byte, 1<<20)
		for i := range b {
			b[i] = 1
		}
		sink = append(sink, b)
	}
	return json.Marshal(map[string]bool{"ok": true})
}

// callerView reports what the host shows this call: the organisation, the
// translation of args.key and the rows of args.query (one string argument,
// args.arg).
func (p *hostCaller) callerView(ctx context.Context, raw json.RawMessage) (json.RawMessage, error) {
	var args struct {
		Query string `json:"query"`
		Arg   string `json:"arg"`
		Key   string `json:"key"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	rows, err := p.host.DBQuery(ctx, args.Query, args.Arg)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"org":  p.host.OrgID(ctx),
		"text": p.host.Translate(ctx, args.Key),
		"rows": rows,
	})
}

func main() {
	grpcutil.ServePlugin(&hostCaller{})
}
