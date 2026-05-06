package api

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/pluginui"
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

	// 2. Plugin UI routes
	if pluginManager != nil {
		if db, err := database.GetDB(); err == nil && db != nil {
			repo := pluginui.NewRepositoryWithDB(db)
			if err := pluginui.RegisterUIRoutes(eng, repo, pluginManager, getPongo2Renderer(), slog.Default()); err != nil {
				log.Printf("⚠️  Dynamic engine: failed to load plugin UI routes: %v", err)
			}
		}
	}

	// 3. Plugin routes
	if pluginManager != nil {
		routes := pluginManager.Routes()
		for _, route := range routes {
			pluginName := route.PluginName
			handlerName := route.RouteSpec.Handler
			middlewares := route.RouteSpec.Middleware

			var mwChain []gin.HandlerFunc
			for _, mw := range middlewares {
				switch {
				case mw == "auth":
					mwChain = append(mwChain, SessionOrJWTAuth())
				case mw == "admin":
					mwChain = append(mwChain, SessionOrJWTAuth(), RequireAdmin())
				case strings.HasPrefix(mw, "group:"):
					groupName := strings.TrimPrefix(mw, "group:")
					mwChain = append(mwChain, SessionOrJWTAuth(), RequireGroup(groupName))
				case strings.HasPrefix(mw, "plugin:"):
					// "plugin:<pluginName>:<agentGroup>" — agent gate name
					// is required so the middleware knows which group a
					// support agent must belong to (customers go through
					// the per-org gk_org_plugin_access table instead).
					rest := strings.TrimPrefix(mw, "plugin:")
					parts := strings.SplitN(rest, ":", 2)
					if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
						log.Printf("dynamic_router: malformed plugin middleware spec %q — expected plugin:<pluginName>:<agentGroup>", mw)
						continue
					}
					mwChain = append(mwChain, SessionOrJWTAuth(), RequirePluginAccess(parts[0], parts[1]))
				case mw == "webhook":
					mwChain = append(mwChain, WebhookRateLimit(pluginName), WebhookAuth(pluginName))
				}
			}

			handler := func(c *gin.Context) {
				args := buildPluginArgs(c, pluginName)
				ctx := pluginContextWithLanguage(c)
				result, err := pluginManager.Call(ctx, pluginName, handlerName, args)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}

				var response map[string]any
				if err := json.Unmarshal(result, &response); err == nil {
					// Redirect response — plugin asks the platform to send the
					// browser elsewhere. Used by customer pages that resolve no
					// org and bail to /customer/dashboard.
					if redirect, ok := response["redirect"].(string); ok && redirect != "" {
						code := http.StatusSeeOther
						if s, ok := response["status"].(float64); ok {
							code = int(s)
						}
						c.Redirect(code, redirect)
						return
					}
					// Binary response — decode base64 data and stream with content type.
					if isBinary, ok := response["_binary"].(bool); ok && isBinary {
						if contentType, ok := response["content_type"].(string); ok {
							if dataB64, ok := response["data"].(string); ok {
								if decoded, err := base64.StdEncoding.DecodeString(dataB64); err == nil {
									c.Data(http.StatusOK, contentType, decoded)
									return
								}
							}
						}
					}
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
								"PluginHTML":  html,
								"PluginTitle": title,
								"ActivePage":  activePage,
								"User":        getUserMapForTemplate(c),
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
