//go:build !linux

package grpc

import (
	"os/exec"
	"runtime"
	"fmt"

	"github.com/goatkit/goatflow/internal/plugin"
)

// applyProcessSandbox applies OS-level restrictions to the plugin command.
// On non-Linux platforms, this is a no-op with a warning.
func applyProcessSandbox(cmd *exec.Cmd, policy plugin.ResourcePolicy, pluginName string) error {
	// Log a warning that process sandboxing is not available
	fmt.Printf("Warning: gRPC plugin process sandboxing not available on %s, plugin %q will run with full system access\n", 
		runtime.GOOS, pluginName)
	
	// For full security on non-Linux platforms, consider using containers
	// or restricting gRPC plugins to trusted code only
	return nil
}