package v1

import (
	"testing"

	"github.com/gotrs-io/gotrs-ce/internal/services/adapter"
	"github.com/stretchr/testify/require"
)

func requireDatabase(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	_, err := adapter.InitializeServiceRegistry()
	require.NoError(t, err)

	if err := adapter.AutoConfigureDatabase(); err != nil {
		t.Skipf("skipping integration test: %v", err)
	}
}
