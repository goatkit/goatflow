package api

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// The startup route audit must pass on a correct build: the core endpoints are
// YAML routes served by the dynamic engine, not main-engine routes.
func TestMissingCoreRoutesProductionRouter(t *testing.T) {
	r := newProductionRouter(t)
	assert.Empty(t, MissingCoreRoutes(r))
}

// With no route files loaded the audit must report every core endpoint.
func TestMissingCoreRoutesWithoutYAMLRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	prevDir := dynRouteDir
	dynMu.RLock()
	prevEng := dynEngine
	dynMu.RUnlock()
	t.Cleanup(func() {
		dynRouteDir = prevDir
		dynMu.Lock()
		dynEngine = prevEng
		dynMu.Unlock()
	})

	r := gin.New()
	MountDynamicEngine(r, t.TempDir())
	assert.Equal(t, []string{
		"GET /api/v1/states",
		"GET /api/lookups/statuses",
		"GET /api/lookups/queues",
	}, MissingCoreRoutes(r))
}
