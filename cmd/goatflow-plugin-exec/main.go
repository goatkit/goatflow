// Command goatflow-plugin-exec starts a GoatFlow gRPC plugin under resource
// and network limits and in a Landlock sandbox: it applies them to itself
// and then execs the plugin, which inherits them.
//
//	goatflow-plugin-exec --memory-mb 512 --cpu-millis 1000 --write-dir /app/tmp/x [--deny-network] -- /path/to/plugin [args...]
//
// It never execs the plugin when a limit cannot be applied. Where the kernel
// has no Landlock it warns and starts the plugin without the sandbox.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type limits struct {
	memoryMB    int
	cpuMillis   int
	denyNetwork bool
	writeDir    string
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "goatflow-plugin-exec: %v\n", err)
		os.Exit(1)
	}
}

// parse reads the flags and returns the limits and the plugin's argv.
func parse(args []string) (limits, []string, error) {
	var l limits
	fs := flag.NewFlagSet("goatflow-plugin-exec", flag.ContinueOnError)
	fs.IntVar(&l.memoryMB, "memory-mb", 0, "memory limit in MiB (required)")
	fs.IntVar(&l.cpuMillis, "cpu-millis", 0, "CPU limit in millicores (required)")
	fs.BoolVar(&l.denyNetwork, "deny-network", false, "refuse every socket except AF_UNIX")
	fs.StringVar(&l.writeDir, "write-dir", "", "the only directory the plugin may write (required, absolute)")
	if err := fs.Parse(args); err != nil {
		return l, nil, err
	}
	if l.memoryMB <= 0 {
		return l, nil, errors.New("--memory-mb must be a positive number of MiB")
	}
	if l.cpuMillis <= 0 {
		return l, nil, errors.New("--cpu-millis must be a positive number of millicores")
	}
	if !filepath.IsAbs(l.writeDir) {
		return l, nil, errors.New("--write-dir must be an absolute path")
	}
	argv := fs.Args()
	if len(argv) == 0 || argv[0] == "" {
		return l, nil, errors.New("no plugin binary given (usage: goatflow-plugin-exec [flags] -- <plugin> [args...])")
	}
	return l, argv, nil
}

// childEnv adds the Go runtime limits matching l to env unless env sets them.
func childEnv(env []string, l limits) []string {
	out := make([]string, len(env), len(env)+2)
	copy(out, env)
	set := func(key, value string) {
		for _, kv := range env {
			if strings.HasPrefix(kv, key+"=") {
				return
			}
		}
		out = append(out, key+"="+value)
	}
	set("GOMEMLIMIT", strconv.Itoa(l.memoryMB*9/10)+"MiB")
	set("GOMAXPROCS", strconv.Itoa(max(1, (l.cpuMillis+999)/1000)))
	return out
}
