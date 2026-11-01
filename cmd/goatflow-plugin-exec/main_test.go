//go:build linux

package main

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The test binary doubles as the launcher (GOATFLOW_EXEC_TEST_LAUNCHER=1) and
// as the plugin it starts (first argument "probe").
const launcherEnv = "GOATFLOW_EXEC_TEST_LAUNCHER"

func TestMain(m *testing.M) {
	if len(os.Args) > 2 && os.Args[1] == "probe" {
		if err := probe(os.Args[2], os.Args[3:]); err != nil {
			fmt.Println(err)
			os.Exit(3)
		}
		os.Exit(0)
	}
	if os.Getenv(launcherEnv) == "1" {
		main()
		return
	}
	os.Exit(m.Run())
}

// sink keeps probe allocations reachable.
var sink [][]byte

func probe(mode string, args []string) error {
	switch mode {
	case "dial":
		c, err := net.DialTimeout("tcp", args[0], 2*time.Second)
		if err != nil {
			return err
		}
		return c.Close()
	case "listen-unix":
		l, err := net.Listen("unix", args[0])
		if err != nil {
			return err
		}
		return l.Close()
	case "alloc":
		mb, _ := strconv.Atoi(args[0])
		for range mb {
			b := make([]byte, 1<<20)
			for i := range b {
				b[i] = 1
			}
			sink = append(sink, b)
		}
		return nil
	case "runtime":
		fmt.Printf("GOMAXPROCS=%d GOMEMLIMIT=%d\n", runtime.GOMAXPROCS(0), debug.SetMemoryLimit(-1))
		return nil
	case "touch":
		return os.WriteFile(args[0], nil, 0o600)
	}
	return errors.New("unknown probe " + mode)
}

// launch runs the launcher with flags, starting the probe with probeArgs.
func launch(t *testing.T, flags []string, probeArgs ...string) (string, error) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	args := append(append(flags, "--", self, "probe"), probeArgs...)
	cmd := exec.Command(self, args...)
	cmd.Env = append(os.Environ(), launcherEnv+"=1")
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err = cmd.Run()
	return out.String(), err
}

func tcpListener(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()
	return l.Addr().String()
}

var defaultFlags = []string{"--memory-mb", "512", "--cpu-millis", "1000"}

func TestDenyNetworkBlocksTCPButNotUnixSockets(t *testing.T) {
	addr := tcpListener(t)
	deny := append([]string{"--deny-network"}, defaultFlags...)

	out, err := launch(t, deny, "dial", addr)
	if err == nil {
		t.Fatalf("TCP dial succeeded under --deny-network: %s", out)
	}
	if !strings.Contains(out, "operation not permitted") {
		t.Errorf("TCP dial failed for the wrong reason: %s", out)
	}

	sock := filepath.Join(t.TempDir(), "p.sock")
	if out, err := launch(t, deny, "listen-unix", sock); err != nil {
		t.Errorf("unix socket refused under --deny-network: %v: %s", err, out)
	}
}

func TestNetworkAllowedWithoutDenyNetwork(t *testing.T) {
	addr := tcpListener(t)
	if out, err := launch(t, defaultFlags, "dial", addr); err != nil {
		t.Errorf("TCP dial failed without --deny-network: %v: %s", err, out)
	}
}

func TestMemoryLimitStopsOverAllocation(t *testing.T) {
	limit := []string{"--memory-mb", "128", "--cpu-millis", "1000"}
	if out, err := launch(t, limit, "alloc", "16"); err != nil {
		t.Fatalf("allocating 16 MiB under a 128 MiB limit failed: %v: %s", err, out)
	}
	out, err := launch(t, limit, "alloc", "256")
	if err == nil {
		t.Fatalf("allocating 256 MiB under a 128 MiB limit succeeded: %s", out)
	}
	if !strings.Contains(out, "out of memory") {
		t.Errorf("over-allocation failed for the wrong reason: %s", out)
	}
}

func TestGoRuntimeLimitsFollowFlags(t *testing.T) {
	t.Setenv("GOMAXPROCS", "")
	t.Setenv("GOMEMLIMIT", "")
	_ = os.Unsetenv("GOMAXPROCS") // t.Setenv restores them afterwards
	_ = os.Unsetenv("GOMEMLIMIT")
	out, err := launch(t, []string{"--memory-mb", "200", "--cpu-millis", "1500"}, "runtime")
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	want := fmt.Sprintf("GOMAXPROCS=2 GOMEMLIMIT=%d", 180<<20)
	if strings.TrimSpace(out) != want {
		t.Errorf("plugin runtime = %q, want %q", strings.TrimSpace(out), want)
	}
}

func TestRefusesToStartPluginWithoutLimits(t *testing.T) {
	for name, flags := range map[string][]string{
		"no memory": {"--cpu-millis", "1000", "--deny-network"},
		"no cpu":    {"--memory-mb", "512"},
		"bad flag":  {"--memory-mb", "512", "--cpu-millis", "1000", "--allow-everything"},
	} {
		t.Run(name, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "started")
			out, err := launch(t, flags, "touch", marker)
			if err == nil {
				t.Fatalf("launcher succeeded: %s", out)
			}
			if !strings.Contains(out, "goatflow-plugin-exec:") {
				t.Errorf("no launcher error on stderr: %s", out)
			}
			if _, err := os.Stat(marker); err == nil {
				t.Error("plugin ran although its limits were not applied")
			}
		})
	}
}
