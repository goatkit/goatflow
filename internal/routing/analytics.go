// Package routing provides HTTP routing and analytics endpoints.
package routing

import (
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RouteMetrics tracks performance and usage metrics for routes.
type RouteMetrics struct {
	mu             sync.RWMutex
	routeStats     map[string]*RouteStats
	totalRequests  int64
	totalErrors    int64
	uptimeStart    time.Time
	recentRequests []RequestLog
	maxRecentLogs  int
}

// RouteStats contains statistics for a specific route.
type RouteStats struct {
	Path            string        `json:"path"`
	Method          string        `json:"method"`
	Handler         string        `json:"handler"`
	RequestCount    int64         `json:"request_count"`
	ErrorCount      int64         `json:"error_count"`
	TotalDuration   time.Duration `json:"-"`
	AverageDuration time.Duration `json:"average_duration"`
	MinDuration     time.Duration `json:"min_duration"`
	MaxDuration     time.Duration `json:"max_duration"`
	LastAccessed    time.Time     `json:"last_accessed"`
	StatusCodes     map[int]int64 `json:"status_codes"`
}

// RequestLog represents a single request log entry.
type RequestLog struct {
	Timestamp  time.Time     `json:"timestamp"`
	Method     string        `json:"method"`
	Path       string        `json:"path"`
	StatusCode int           `json:"status_code"`
	Duration   time.Duration `json:"duration"`
	UserAgent  string        `json:"user_agent"`
	IP         string        `json:"ip"`
}

// NewRouteMetrics creates a new route metrics tracker.
func NewRouteMetrics() *RouteMetrics {
	return &RouteMetrics{
		routeStats:     make(map[string]*RouteStats),
		uptimeStart:    time.Now(),
		recentRequests: make([]RequestLog, 0),
		maxRecentLogs:  1000, // Keep last 1000 requests in memory
	}
}

// MiddlewareWithMetrics returns a Gin middleware that tracks route metrics.
func (rm *RouteMetrics) MiddlewareWithMetrics(routePath, handlerName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		// Process request
		c.Next()

		// Record metrics
		duration := time.Since(startTime)
		statusCode := c.Writer.Status()

		rm.RecordRequest(RouteRequest{
			Method:     c.Request.Method,
			Path:       routePath,
			Handler:    handlerName,
			StatusCode: statusCode,
			Duration:   duration,
			UserAgent:  c.Request.UserAgent(),
			ClientIP:   c.ClientIP(),
		})
	}
}

// RouteRequest represents a request to be recorded.
type RouteRequest struct {
	Method     string
	Path       string
	Handler    string
	StatusCode int
	Duration   time.Duration
	UserAgent  string
	ClientIP   string
}

// RecordRequest records metrics for a route request.
func (rm *RouteMetrics) RecordRequest(req RouteRequest) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	key := fmt.Sprintf("%s %s", req.Method, req.Path)

	// Initialize route stats if not exists
	if rm.routeStats[key] == nil {
		rm.routeStats[key] = &RouteStats{
			Path:        req.Path,
			Method:      req.Method,
			Handler:     req.Handler,
			StatusCodes: make(map[int]int64),
			MinDuration: req.Duration,
			MaxDuration: req.Duration,
		}
	}

	stats := rm.routeStats[key]
	stats.RequestCount++
	stats.LastAccessed = time.Now()
	stats.TotalDuration += req.Duration
	stats.AverageDuration = stats.TotalDuration / time.Duration(stats.RequestCount)

	if req.Duration < stats.MinDuration {
		stats.MinDuration = req.Duration
	}
	if req.Duration > stats.MaxDuration {
		stats.MaxDuration = req.Duration
	}

	// Track status codes
	stats.StatusCodes[req.StatusCode]++

	// Track errors (4xx and 5xx)
	if req.StatusCode >= 400 {
		stats.ErrorCount++
		rm.totalErrors++
	}

	rm.totalRequests++

	// Add to recent requests log
	rm.recentRequests = append(rm.recentRequests, RequestLog{
		Timestamp:  time.Now(),
		Method:     req.Method,
		Path:       req.Path,
		StatusCode: req.StatusCode,
		Duration:   req.Duration,
		UserAgent:  req.UserAgent,
		IP:         req.ClientIP,
	})

	// Keep only recent requests
	if len(rm.recentRequests) > rm.maxRecentLogs {
		rm.recentRequests = rm.recentRequests[len(rm.recentRequests)-rm.maxRecentLogs:]
	}
}

// GetStats returns current statistics.
func (rm *RouteMetrics) GetStats() *SystemStats {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	routes := make([]*RouteStats, 0, len(rm.routeStats))
	for _, stats := range rm.routeStats {
		// Create copy to avoid race conditions
		statsCopy := *stats
		routes = append(routes, &statsCopy)
	}

	// Sort by request count (most popular first)
	sort.Slice(routes, func(i, j int) bool {
		return routes[i].RequestCount > routes[j].RequestCount
	})

	return &SystemStats{
		TotalRequests:  rm.totalRequests,
		TotalErrors:    rm.totalErrors,
		ErrorRate:      float64(rm.totalErrors) / float64(rm.totalRequests) * 100,
		Uptime:         time.Since(rm.uptimeStart),
		Routes:         routes,
		RecentRequests: rm.getRecentRequests(50), // Last 50 requests
	}
}

func (rm *RouteMetrics) getRecentRequests(limit int) []RequestLog {
	if len(rm.recentRequests) < limit {
		return rm.recentRequests
	}
	return rm.recentRequests[len(rm.recentRequests)-limit:]
}

// SystemStats represents overall system statistics.
type SystemStats struct {
	TotalRequests  int64         `json:"total_requests"`
	TotalErrors    int64         `json:"total_errors"`
	ErrorRate      float64       `json:"error_rate"`
	Uptime         time.Duration `json:"uptime"`
	Routes         []*RouteStats `json:"routes"`
	RecentRequests []RequestLog  `json:"recent_requests"`
}

// SetupMetricsEndpoints adds analytics endpoints to the router.
func (rm *RouteMetrics) SetupMetricsEndpoints(r *gin.Engine) {
	metrics := r.Group("/metrics")

	// Real-time stats endpoint
	metrics.GET("/stats", func(c *gin.Context) {
		stats := rm.GetStats()
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    stats,
		})
	})

	// Route-specific stats
	metrics.GET("/routes/:method/:path", func(c *gin.Context) {
		method := c.Param("method")
		path := c.Param("path")
		key := fmt.Sprintf("%s /%s", method, path)

		rm.mu.RLock()
		stats, exists := rm.routeStats[key]
		rm.mu.RUnlock()

		if !exists {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "Route not found",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    stats,
		})
	})

	// Analytics dashboard
	metrics.GET("/dashboard", func(c *gin.Context) {
		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, rm.generateDashboardHTML())
	})

	// Health status based on metrics
	metrics.GET("/health", func(c *gin.Context) {
		stats := rm.GetStats()

		status := "healthy"
		if stats.ErrorRate > 5.0 { // More than 5% error rate
			status = "degraded"
		}
		if stats.ErrorRate > 20.0 { // More than 20% error rate
			status = "unhealthy"
		}

		c.JSON(http.StatusOK, gin.H{
			"status":     status,
			"error_rate": stats.ErrorRate,
			"uptime":     stats.Uptime.String(),
			"requests":   stats.TotalRequests,
		})
	})
}

func (rm *RouteMetrics) generateDashboardHTML() string {
	stats := rm.GetStats()

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>GOTRS Route Analytics</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; background: #f8fafc; }
        .header { background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); color: white; padding: 2rem; }
        .container { max-width: 1200px; margin: 0 auto; padding: 2rem; }
        .stats-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(250px, 1fr)); gap: 1.5rem; margin: 2rem 0; }
        .stat-card { background: white; padding: 1.5rem; border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        .stat-value { font-size: 2rem; font-weight: bold; color: #2d3748; }
        .stat-label { color: #718096; margin-top: 0.5rem; }
        .route-table { background: white; border-radius: 8px; overflow: hidden; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        .route-table th { background: #4a5568; color: white; padding: 1rem; text-align: left; }
        .route-table td { padding: 1rem; border-bottom: 1px solid #e2e8f0; }
        .method { padding: 0.25rem 0.75rem; border-radius: 4px; font-weight: bold; color: white; }
        .GET { background: #48bb78; }
        .POST { background: #ed8936; }
        .PUT { background: #4299e1; }
        .DELETE { background: #f56565; }
        .error-rate { color: #f56565; font-weight: bold; }
        .success-rate { color: #48bb78; font-weight: bold; }
    </style>
    <script>
        // Auto-refresh every 30 seconds
        setTimeout(() => window.location.reload(), 30000);
    </script>
</head>
<body>
    <div class="header">
        <h1>🚀 GOTRS Route Analytics</h1>
        <p>Real-time performance monitoring for YAML-defined routes</p>
    </div>
    
    <div class="container">
        <div class="stats-grid">
            <div class="stat-card">
                <div class="stat-value">%d</div>
                <div class="stat-label">Total Requests</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">%d</div>
                <div class="stat-label">Total Errors</div>
            </div>
            <div class="stat-card">
                <div class="stat-value %.1f%%</div>
                <div class="stat-label">Error Rate</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">%s</div>
                <div class="stat-label">Uptime</div>
            </div>
        </div>
        
        <h2>📊 Route Performance</h2>
        <div class="route-table">
            <table style="width: 100%%; border-collapse: collapse;">
                <tr>
                    <th>Route</th>
                    <th>Requests</th>
                    <th>Avg Response</th>
                    <th>Error Rate</th>
                    <th>Last Used</th>
                </tr>
                %s
            </table>
        </div>
    </div>
</body>
</html>`,
		stats.TotalRequests,
		stats.TotalErrors,
		stats.ErrorRate,
		stats.Uptime.Round(time.Second).String(),
		rm.generateRouteRows(stats.Routes))
}

func (rm *RouteMetrics) generateRouteRows(routes []*RouteStats) string {
	if len(routes) == 0 {
		return "<tr><td colspan='5' style='text-align: center; color: #718096;'>No routes tracked yet</td></tr>"
	}

	rows := ""
	for _, route := range routes {
		errorRate := float64(route.ErrorCount) / float64(route.RequestCount) * 100
		errorClass := "success-rate"
		if errorRate > 5 {
			errorClass = "error-rate"
		}

		rows += fmt.Sprintf(`
                <tr>
                    <td>
                        <span class="method %s">%s</span>
                        <code>%s</code>
                    </td>
                    <td>%d</td>
                    <td>%s</td>
                    <td class="%s">%.1f%%</td>
                    <td>%s</td>
                </tr>`,
			route.Method,
			route.Method,
			route.Path,
			route.RequestCount,
			route.AverageDuration.Round(time.Millisecond).String(),
			errorClass,
			errorRate,
			route.LastAccessed.Format("15:04:05"))
	}

	return rows
}

// Global metrics instance.
var globalMetrics *RouteMetrics

// InitRouteMetrics initializes the global metrics tracker.
func InitRouteMetrics() *RouteMetrics {
	globalMetrics = NewRouteMetrics()
	return globalMetrics
}

// GetGlobalMetrics returns the global metrics instance.
func GetGlobalMetrics() *RouteMetrics {
	if globalMetrics == nil {
		globalMetrics = NewRouteMetrics()
	}
	return globalMetrics
}
