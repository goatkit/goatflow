//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/plugin"
	"github.com/gotrs-io/gotrs-ce/internal/plugin/example"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPluginWithRealDatabase(t *testing.T) {
	db, err := database.GetDB()
	require.NoError(t, err, "Failed to get database connection")
	require.NotNil(t, db, "Database should not be nil")

	ctx := context.Background()

	// Create a real HostAPI with actual database
	hostAPI := plugin.NewProdHostAPI(plugin.WithDB("default", db))
	require.NotNil(t, hostAPI, "HostAPI should not be nil")

	// Create plugin manager with real host
	mgr := plugin.NewManager(hostAPI)
	require.NotNil(t, mgr, "Manager should not be nil")

	// Register the hello plugin
	helloPlugin := example.NewHelloPlugin()
	err = mgr.Register(ctx, helloPlugin)
	require.NoError(t, err, "Failed to register hello plugin")

	t.Run("List plugins", func(t *testing.T) {
		plugins := mgr.List()
		assert.Len(t, plugins, 1, "Should have 1 plugin")
		assert.Equal(t, "hello", plugins[0].Name)
	})

	t.Run("Call plugin function", func(t *testing.T) {
		args, _ := json.Marshal(map[string]string{"name": "IntegrationTest"})
		result, err := mgr.Call(ctx, "hello", "hello", args)
		require.NoError(t, err, "Call should succeed")

		var response map[string]any
		err = json.Unmarshal(result, &response)
		require.NoError(t, err, "Should parse response")

		msg, ok := response["message"].(string)
		assert.True(t, ok, "Should have message")
		assert.Contains(t, msg, "IntegrationTest", "Should include name")
	})

	t.Run("Get plugin routes", func(t *testing.T) {
		routes := mgr.Routes()
		assert.NotEmpty(t, routes, "Should have routes")

		// Find the hello route
		found := false
		for _, r := range routes {
			if r.RouteSpec.Path == "/api/plugins/hello" {
				found = true
				assert.Equal(t, "GET", r.RouteSpec.Method)
				break
			}
		}
		assert.True(t, found, "Should find hello route")
	})

	t.Run("Disable and enable plugin", func(t *testing.T) {
		// Disable
		err := mgr.Disable("hello")
		assert.NoError(t, err, "Disable should succeed")

		// Call should fail when disabled
		_, err = mgr.Call(ctx, "hello", "hello", nil)
		assert.Error(t, err, "Call should fail when disabled")

		// Enable
		err = mgr.Enable("hello")
		assert.NoError(t, err, "Enable should succeed")

		// Call should work again
		_, err = mgr.Call(ctx, "hello", "hello", nil)
		assert.NoError(t, err, "Call should work after enable")
	})

	t.Run("Shutdown all plugins", func(t *testing.T) {
		err := mgr.ShutdownAll(ctx)
		assert.NoError(t, err, "ShutdownAll should succeed")
	})
}

func TestProdHostAPIWithRealDatabase(t *testing.T) {
	db, err := database.GetDB()
	require.NoError(t, err, "Failed to get database connection")

	ctx := context.Background()

	hostAPI := plugin.NewProdHostAPI(plugin.WithDB("default", db))
	require.NotNil(t, hostAPI, "HostAPI should not be nil")

	t.Run("DBQuery returns results", func(t *testing.T) {
		// Query a table that should exist
		results, err := hostAPI.DBQuery(ctx, "SELECT id, name FROM valid LIMIT 5")
		require.NoError(t, err, "Query should succeed")
		assert.NotEmpty(t, results, "Should have results")

		// Check structure
		for _, row := range results {
			_, hasID := row["id"]
			_, hasName := row["name"]
			assert.True(t, hasID, "Row should have id")
			assert.True(t, hasName, "Row should have name")
		}
	})

	t.Run("DBQuery with parameters", func(t *testing.T) {
		results, err := hostAPI.DBQuery(ctx, "SELECT id, name FROM valid WHERE id = ?", 1)
		require.NoError(t, err, "Query with params should succeed")
		assert.Len(t, results, 1, "Should have 1 result")
	})

	t.Run("DBQuery handles errors", func(t *testing.T) {
		_, err := hostAPI.DBQuery(ctx, "SELECT * FROM nonexistent_table_xyz")
		assert.Error(t, err, "Query on nonexistent table should fail")
	})

	t.Run("Log writes to buffer", func(t *testing.T) {
		hostAPI.Log(ctx, "info", "Integration test log", map[string]any{
			"test": true,
			"component": "plugins_test",
		})

		// Check log buffer
		logs := plugin.GetLogBuffer().GetRecent(10)
		found := false
		for _, entry := range logs {
			if entry.Message == "Integration test log" {
				found = true
				break
			}
		}
		assert.True(t, found, "Log should be in buffer")
	})

	t.Run("ConfigGet returns values", func(t *testing.T) {
		// This may return empty if config not set, but shouldn't error
		_, err := hostAPI.ConfigGet(ctx, "app.name")
		assert.NoError(t, err, "ConfigGet should not error")
	})
}

func TestPluginManagerConcurrency(t *testing.T) {
	db, err := database.GetDB()
	require.NoError(t, err)

	ctx := context.Background()
	hostAPI := plugin.NewProdHostAPI(plugin.WithDB("default", db))
	mgr := plugin.NewManager(hostAPI)

	helloPlugin := example.NewHelloPlugin()
	err = mgr.Register(ctx, helloPlugin)
	require.NoError(t, err)

	// Run concurrent calls
	const numGoroutines = 10
	const callsPerGoroutine = 5

	errCh := make(chan error, numGoroutines*callsPerGoroutine)
	doneCh := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < callsPerGoroutine; j++ {
				args, _ := json.Marshal(map[string]string{"name": "Concurrent"})
				_, err := mgr.Call(ctx, "hello", "hello", args)
				if err != nil {
					errCh <- err
				}
			}
			doneCh <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-doneCh
	}
	close(errCh)

	// Check for errors
	var errors []error
	for err := range errCh {
		errors = append(errors, err)
	}
	assert.Empty(t, errors, "No errors should occur during concurrent calls")

	mgr.ShutdownAll(ctx)
}
