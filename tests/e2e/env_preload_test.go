//go:build e2e

package e2e

// This file ensures the .env loader in config runs before any tests access os.Getenv directly.
// It deliberately triggers config.GetConfig() which performs a one-time .env parse.

import (
	"github.com/goatkit/goatflow/tests/e2e/config"
	_ "github.com/goatkit/goatflow/tests/e2e/config" // import for side-effect: init + GetConfig invocation below
)

func init() {
	// Trigger configuration load (and .env parsing) as early as possible.
	config.GetConfig()
}
