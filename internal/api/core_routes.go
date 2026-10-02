package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	platformapi "github.com/goatkit/goatflow/internal/platform/api"
)

// RegisterCoreRoutes registers the routes that live on the main engine
// instead of in routes/*.yaml: the i18n API, the plugin management API and
// the plugin SSE stream. cmd/goats and the route authorization test both use
// it, so the test covers exactly what production serves.
func RegisterCoreRoutes(r *gin.Engine, sse http.Handler) {
	v1 := r.Group("/api/v1")
	platformapi.NewI18nHandlers().RegisterRoutes(v1)
	RegisterPluginAPIRoutes(v1)

	// Plugin events can carry ticket data: authenticated callers only.
	v1.GET("/sse", SessionOrJWTAuth(), gin.WrapH(sse))
}
