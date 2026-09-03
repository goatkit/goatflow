package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/version"
)

// healthCheckTimeout bounds the database ping inside /health so a wedged
// connection can't hang the health endpoint (TrueNAS/Docker probes run
// with a 3s timeout).
const healthCheckTimeout = 500 * time.Millisecond

// detailedHealthCheckTimeout bounds the cache (Valkey) check inside
// /health/detailed, which is for operators and not on a probe loop.
const detailedHealthCheckTimeout = 2 * time.Second

// processStart is captured at package init and powers the "uptime" field
// on /health/detailed.
var processStart = time.Now()

// checkDatabase performs a short-timeout ping and returns "ok" or "error".
func checkDatabase() string {
	db, err := database.GetDB()
	if err != nil {
		return "error"
	}
	ctx, cancel := context.WithTimeout(context.Background(), healthCheckTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return "error"
	}
	return "ok"
}

// checkCache probes the shared Valkey cache when one is configured.
// Returns ("disabled", nil) when no cache client was wired at startup.
func checkCache() string {
	if valkeyCache == nil {
		return "disabled"
	}
	ctx, cancel := context.WithTimeout(context.Background(), detailedHealthCheckTimeout)
	defer cancel()
	if _, err := valkeyCache.Info(ctx); err != nil {
		return "error"
	}
	return "ok"
}

// HandleHealthCheck is the liveness/readiness probe used by the Dockerfile
// HEALTHCHECK, the TrueNAS app template and Kubernetes probes. It performs
// a real (short-timeout) database ping so a backend with a dead DB reports
// 503 instead of a misleading "healthy".
func HandleHealthCheck(c *gin.Context) {
	dbStatus := checkDatabase()
	status := "healthy"
	code := http.StatusOK
	if dbStatus == "error" {
		status = "unhealthy"
		code = http.StatusServiceUnavailable
	}
	c.JSON(code, gin.H{
		"status":     status,
		"components": gin.H{"database": dbStatus},
		"version":    version.Short(),
	})
}

// HandleDetailedHealthCheck reports every probeable component plus uptime
// and build information. It is admin-gated at route level
// (routes/basic.yaml): an operator endpoint, not a probe target — /health
// covers Docker/TrueNAS/k8s probes and stays public.
func HandleDetailedHealthCheck(c *gin.Context) {
	dbStatus := checkDatabase()
	cacheStatus := checkCache()

	status := "healthy"
	code := http.StatusOK
	if dbStatus == "error" {
		status = "unhealthy"
		code = http.StatusServiceUnavailable
	}

	c.JSON(code, gin.H{
		"status":     status,
		"components": gin.H{"database": dbStatus, "cache": cacheStatus},
		// version.Short() only: no git commit SHA on any health endpoint.
		// Operators wanting the full build string use the CLI/`goatflow
		// version` or the admin UI, which already require credentials.
		"version": version.Short(),
		"uptime":  time.Since(processStart).String(),
	})
}
