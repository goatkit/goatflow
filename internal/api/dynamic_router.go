package api

import (
	"log"
	"net/http"
	"os"
	"sync/atomic"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/routing"
)

// dynamicEngine holds only YAML routes (selective mode) and can be swapped without impacting static routes.
var dynamicEngine atomic.Value // *gin.Engine

// useDynamicSubEngine returns true when selective mode env var enabled.
func useDynamicSubEngine() bool { return os.Getenv("ROUTES_SELECTIVE") != "" }

// mountDynamicEngine installs a catch-all group under / that forwards to current dynamic engine
// while allowing explicit static routes registered earlier to win (Gin matches specific before wildcard).
func mountDynamicEngine(r *gin.Engine) {
	if !useDynamicSubEngine() {
		return
	}
	if v := dynamicEngine.Load(); v == nil { // initialize once
		eng := gin.New()
		eng.Use(gin.Recovery())
		dynamicEngine.Store(eng)
	}
	// Catch-all forwarder placed last
	r.Any("/*dynamicPath", func(c *gin.Context) {
		// Skip if original route already handled (shouldn't happen because this is final)
		if h := dynamicEngine.Load(); h != nil {
			if eng, ok := h.(*gin.Engine); ok && eng != nil {
				eng.HandleContext(c)
				return
			}
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	})
	log.Println("[routes-selective] dynamic sub-engine mounted")
}

// rebuildDynamicEngine rebuilds the dynamic engine with fresh YAML routes only.
func rebuildDynamicEngine(_ interface{}) {
	if !useDynamicSubEngine() {
		return
	}
	eng := gin.New()
	eng.Use(gin.Recovery())
	if err := routing.LoadYAMLRoutesForTesting(eng); err != nil {
		log.Printf("[routes-selective] failed to load YAML routes: %v", err)
	}
	dynamicEngine.Store(eng)
	log.Println("[routes-selective] dynamic engine rebuilt")
}
