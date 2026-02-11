package api

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/routing"
)

// Backwards compatibility stubs for htmx_routes.go and routes_watcher.go
func useDynamicSubEngine() bool          { return false }
func mountDynamicEngine(_ *gin.Engine)   {}
func rebuildDynamicEngine(_ interface{}) { RebuildDynamicEngine() }

var (
	dynEngine   *gin.Engine
	dynMu       sync.RWMutex
	dynRouteDir string
)

// MountDynamicEngine installs a unified dynamic engine that serves both YAML routes
// and plugin routes. The engine is atomically swappable — rebuilt when YAML files
// change or plugins are loaded/reloaded. Static API routes registered directly on
// the main engine take priority (Gin matches those first, NoRoute catches the rest).
func MountDynamicEngine(r *gin.Engine, routesDir string) {
	dynRouteDir = routesDir
	RebuildDynamicEngine()

	r.NoRoute(func(c *gin.Context) {
		dynMu.RLock()
		eng := dynEngine
		dynMu.RUnlock()

		if eng != nil {
			eng.HandleContext(c)
			if c.Writer.Written() {
				return
			}
		}
		sendErrorResponse(c, http.StatusNotFound, "Page not found")
	})
}

// RebuildDynamicEngine rebuilds the unified dynamic engine with current YAML + plugin routes.
// Safe to call from any goroutine.
func RebuildDynamicEngine() {
	eng := gin.New()
	eng.Use(gin.Recovery())

	// 1. YAML routes
	if dynRouteDir != "" {
		if err := routing.LoadYAMLRoutesFromGlobalMap(eng, dynRouteDir); err != nil {
			log.Printf("⚠️  Dynamic engine: failed to load YAML routes: %v", err)
		} else {
			log.Println("✅ YAML routes loaded")
		}
	}

	// 2. Plugin routes
	if pluginManager != nil {
		routes := pluginManager.Routes()
		for _, route := range routes {
			pluginName := route.PluginName
			handlerName := route.RouteSpec.Handler
			middlewares := route.RouteSpec.Middleware

			var mwChain []gin.HandlerFunc
			for _, mw := range middlewares {
				switch mw {
				case "auth":
					mwChain = append(mwChain, SessionOrJWTAuth())
				case "admin":
					mwChain = append(mwChain, SessionOrJWTAuth(), RequireAdmin())
				}
			}

			handler := func(c *gin.Context) {
				args := buildPluginArgs(c)
				ctx := pluginContextWithLanguage(c)
				result, err := pluginManager.Call(ctx, pluginName, handlerName, args)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}

				var response map[string]any
				if err := json.Unmarshal(result, &response); err == nil {
					if html, ok := response["html"].(string); ok {
						// Check if plugin wants raw HTML (no layout wrapping)
						if raw, ok := response["raw"].(bool); ok && raw {
							c.Header("Content-Type", "text/html; charset=utf-8")
							c.String(http.StatusOK, html)
							return
						}
						// Wrap plugin HTML in base layout template
						renderer := getPongo2Renderer()
						if renderer != nil {
							title, _ := response["title"].(string)
							activePage := "plugin"
							if ap, ok := response["active_page"].(string); ok {
								activePage = ap
							}
							renderer.HTML(c, http.StatusOK, "pages/plugin_wrapper.pongo2", pongo2.Context{
								"PluginHTML":   html,
								"PluginTitle":  title,
								"ActivePage":   activePage,
								"User":         getUserMapForTemplate(c),
							})
							return
						}
						// Fallback if no renderer
						c.Header("Content-Type", "text/html; charset=utf-8")
						c.String(http.StatusOK, html)
						return
					}
				}

				c.Data(http.StatusOK, "application/json", result)
			}

			path := route.RouteSpec.Path
			handlers := append(mwChain, handler)
			switch route.RouteSpec.Method {
			case "GET":
				eng.GET(path, handlers...)
			case "POST":
				eng.POST(path, handlers...)
			case "PUT":
				eng.PUT(path, handlers...)
			case "DELETE":
				eng.DELETE(path, handlers...)
			case "PATCH":
				eng.PATCH(path, handlers...)
			default:
				eng.GET(path, handlers...)
			}

			log.Printf("🔌 Registered plugin route: %s %s -> %s.%s",
				route.RouteSpec.Method, path, pluginName, handlerName)
		}

		if len(routes) > 0 {
			log.Printf("🔌 %d plugin route(s) registered", len(routes))
		}
	}

	dynMu.Lock()
	dynEngine = eng
	dynMu.Unlock()

	log.Println("🔄 Dynamic engine rebuilt")
}
