package api

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

// hotReloadableEngine holds an *gin.Engine atomically swapped during hot reload.
var hotReloadableEngine atomic.Value // *gin.Engine

// getCurrentEngine returns current engine (falls back to provided default if unset).
func getCurrentEngine(defaultEngine *gin.Engine) *gin.Engine {
	if v := hotReloadableEngine.Load(); v != nil {
		if eng, ok := v.(*gin.Engine); ok && eng != nil {
			return eng
		}
	}
	return defaultEngine
}

// engineHandlerMiddleware proxies requests to current engine (for hot reload).
func engineHandlerMiddleware(base *gin.Engine) gin.HandlerFunc {
	return func(c *gin.Context) {
		eng := getCurrentEngine(base)
		eng.HandleContext(c)
		c.Abort()
	}
}

// startRoutesWatcher watches the routes directory and regenerates the manifest on change.
// It does not re-register handlers (requires restart) but provides fast feedback in dev.
func startRoutesWatcher() {
	if os.Getenv("ROUTES_WATCH") == "" {
		return
	}
	// Use a lightweight polling fallback (avoid adding fsnotify dep if not present)
	go func() {
		prev := map[string]time.Time{}
		dir := "./routes"
		for {
			changed := false
			_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error { //nolint:errcheck // Best effort file watching
				if err != nil || info == nil {
					return nil //nolint:nilerr // continue walking on error
				}
				if info.IsDir() {
					return nil
				}
				if !(filepath.Ext(path) == ".yaml" || filepath.Ext(path) == ".yml") {
					return nil
				}
				mt := info.ModTime()
				if p, ok := prev[path]; !ok || mt.After(p) {
					prev[path] = mt
					changed = true
				}
				return nil
			})
			if changed {
				log.Println("[routes-watcher] change detected")
				if useDynamicSubEngine() {
					log.Println("[routes-watcher] selective mode rebuild dynamic engine")
					rebuildDynamicEngine(nil)
				} else if os.Getenv("ROUTES_HOT_RELOAD") != "" {
					log.Println("[routes-watcher] hot reload rebuilding engine")
					// Build new engine, re-run core setup minimal, then YAML registration.
					newEngine := gin.New()
					newEngine.Use(gin.Recovery())
					// Basic 404 placeholder; full middleware stack not replicated (dev only)
					newEngine.NoRoute(func(c *gin.Context) { c.JSON(http.StatusNotFound, gin.H{"error": "not found"}) })
					registerYAMLRoutes(newEngine, nil)
					hotReloadableEngine.Store(newEngine)
					log.Println("[routes-watcher] engine swapped")
				} else {
					log.Println("[routes-watcher] regenerating manifest only")
					registerYAMLRoutesManifestOnly()
				}
			}
			time.Sleep(2 * time.Second)
		}
	}()
	log.Println("[routes-watcher] enabled (poll mode)")
}

// registerYAMLRoutesManifestOnly rebuilds the manifest without re-registering routes.
// NOTE: This function is a stub for hot-reload manifest regeneration.
// Currently the manifest is only written during initial route registration.
//
//nolint:staticcheck // SA4010: manifest variable intentionally unused in stub
func registerYAMLRoutesManifestOnly() {
	docs, err := loadYAMLRouteGroups("./routes")
	if err != nil {
		return
	}
	type manifestRoute struct {
		Group      string   `json:"group"`
		Method     string   `json:"method"`
		Path       string   `json:"path"`
		Handler    string   `json:"handler,omitempty"`
		RedirectTo string   `json:"redirectTo,omitempty"`
		Status     int      `json:"status,omitempty"`
		Websocket  bool     `json:"websocket,omitempty"`
		Middleware []string `json:"middleware,omitempty"`
	}
	var manifest []manifestRoute
	for _, doc := range docs {
		for _, rt := range doc.Spec.Routes {
			if rt.Path == "" || rt.Method == "" {
				continue
			}
			method := rt.Method
			prefix := doc.Spec.Prefix
			fullPath := filepath.Join(prefix, rt.Path)
			if fullPath == "." {
				fullPath = "/"
			}
			if len(fullPath) == 0 || fullPath[0] != '/' {
				fullPath = "/" + fullPath
			}
			manifest = append(manifest, manifestRoute{
				Group:      doc.Metadata.Name,
				Method:     method,
				Path:       fullPath,
				Handler:    rt.HandlerName,
				RedirectTo: rt.RedirectTo,
				Status:     rt.Status,
				Websocket:  rt.Websocket,
				Middleware: append(doc.Spec.Middleware, rt.Middleware...),
			})
		}
	}
	_ = manifest // Stub: would write manifest here
}
