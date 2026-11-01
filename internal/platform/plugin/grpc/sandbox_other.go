//go:build !linux

package grpc

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/goatkit/goatflow/internal/platform/plugin"
)

// pluginCgroup has no counterpart off Linux.
type pluginCgroup struct{}

func (*pluginCgroup) remove() {}

// pluginCommand returns the command that starts the plugin at execPath. Off
// Linux there is no launcher: the plugin runs without limits.
func pluginCommand(execPath, pluginName string, _ plugin.ResourcePolicy) (*exec.Cmd, *pluginCgroup, error) {
	fmt.Printf("Warning: gRPC plugin process sandboxing not available on %s, plugin %q will run with full system access\n",
		runtime.GOOS, pluginName)
	return exec.Command(execPath), nil, nil // #nosec G204 -- plugin binary is chosen by the host
}
