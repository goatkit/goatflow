package api

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/routing"
	"github.com/goatkit/goatflow/internal/platform/shared"
)

// NewSimpleRouterWithDB is NewSimpleRouter for tests that hold a DB handle;
// handlers resolve the connection through database.GetDB(), so db only
// documents that the caller set the test database up first.
func NewSimpleRouterWithDB(_ *sql.DB) *gin.Engine {
	return NewSimpleRouter()
}

// NewSimpleRouter builds a test engine with the template renderer (when the
// templates directory exists) and every YAML route group, wired exactly as
// the server wires them. It panics if the YAML routes cannot be loaded so a
// broken route file fails the test instead of silently shrinking the router.
func NewSimpleRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	templateDir := os.Getenv("TEMPLATES_DIR")
	if templateDir == "" {
		for _, c := range []string{"./templates", "../templates", "../../templates"} {
			if fi, err := os.Stat(c); err == nil && fi.IsDir() {
				templateDir = c
				break
			}
		}
	}
	if templateDir != "" {
		if fi, err := os.Stat(templateDir); err == nil && fi.IsDir() {
			renderer, err := shared.NewTemplateRenderer(templateDir)
			if err != nil {
				panic(fmt.Sprintf("NewSimpleRouter: template renderer: %v", err))
			}
			shared.SetGlobalRenderer(renderer)
		}
	}

	routesPath := ""
	for _, c := range []string{"routes", "../routes", "../../routes"} {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			routesPath = c
			break
		}
	}
	if routesPath == "" {
		panic("NewSimpleRouter: routes directory not found")
	}
	abs, _ := filepath.Abs(routesPath) //nolint:errcheck // only used for the log line
	log.Printf("NewSimpleRouter: loading routes from %s", abs)

	resolver := NewRoutingHandlerResolver()
	routing.SetGlobalRegistry(resolver.Registry())
	if err := routing.LoadYAMLRoutes(r, routesPath, resolver); err != nil {
		panic(fmt.Sprintf("NewSimpleRouter: load routes: %v", err))
	}
	return r
}
