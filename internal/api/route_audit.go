package api

import "github.com/gin-gonic/gin"

// coreRoutes ("METHOD /path") are endpoints every build must serve. They come
// from the YAML route files (routes/api-v1-global.yaml, routes/api-lookups.yaml),
// so a missing one means a route file failed to load.
var coreRoutes = []string{
	"GET /api/v1/states",
	"GET /api/lookups/statuses",
	"GET /api/lookups/queues",
}

// MissingCoreRoutes returns the core routes that neither the main engine r nor
// the dynamic engine serves. YAML and plugin routes live on the dynamic engine
// (reached through r's NoRoute handler), not in r.Routes().
func MissingCoreRoutes(r *gin.Engine) []string {
	served := make(map[string]bool)
	add := func(routes gin.RoutesInfo) {
		for _, ri := range routes {
			served[ri.Method+" "+ri.Path] = true
		}
	}
	add(r.Routes())
	dynMu.RLock()
	if dynEngine != nil {
		add(dynEngine.Routes())
	}
	dynMu.RUnlock()

	var missing []string
	for _, want := range coreRoutes {
		if !served[want] {
			missing = append(missing, want)
		}
	}
	return missing
}
