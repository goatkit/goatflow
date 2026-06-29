package routing

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RouteProfiler collects and analyzes route performance metrics.
type RouteProfiler struct {
	mu          sync.RWMutex
	profiles    map[string]*RouteProfile
	enabled     bool
	sampleRate  float64 // Percentage of requests to profile (0.0 to 1.0)
	maxProfiles int
	alertRules  []AlertRule
	anomalies   []PerformanceAnomaly
}

// RouteProfile contains performance data for a specific route.
type RouteProfile struct {
	Path            string           `json:"path"`
	Method          string           `json:"method"`
	Handler         string           `json:"handler"`
	Samples         []RequestSample  `json:"samples"`
	Statistics      *RouteStatistics `json:"statistics"`
	Recommendations []string         `json:"recommendations"`
	LastUpdated     time.Time        `json:"last_updated"`
}

// RequestSample represents a single request measurement.
type RequestSample struct {
	Timestamp  time.Time          `json:"timestamp"`
	Duration   time.Duration      `json:"duration"`
	StatusCode int                `json:"status_code"`
	BytesIn    int64              `json:"bytes_in"`
	BytesOut   int64              `json:"bytes_out"`
	Error      string             `json:"error,omitempty"`
	Tags       map[string]string  `json:"tags,omitempty"`
	Breakdown  *DurationBreakdown `json:"breakdown,omitempty"`
}

// DurationBreakdown shows where time was spent.
type DurationBreakdown struct {
	Middleware    time.Duration `json:"middleware"`
	Handler       time.Duration `json:"handler"`
	Database      time.Duration `json:"database"`
	External      time.Duration `json:"external"`
	Serialization time.Duration `json:"serialization"`
}

// RouteStatistics contains aggregated performance metrics.
type RouteStatistics struct {
	Count         int64         `json:"count"`
	TotalDuration time.Duration `json:"total_duration"`
	MinDuration   time.Duration `json:"min_duration"`
	MaxDuration   time.Duration `json:"max_duration"`
	AvgDuration   time.Duration `json:"avg_duration"`
	P50Duration   time.Duration `json:"p50_duration"`
	P95Duration   time.Duration `json:"p95_duration"`
	P99Duration   time.Duration `json:"p99_duration"`
	ErrorRate     float64       `json:"error_rate"`
	Throughput    float64       `json:"throughput"`
	AvgBytesIn    int64         `json:"avg_bytes_in"`
	AvgBytesOut   int64         `json:"avg_bytes_out"`
}

// AlertRule defines conditions for performance alerts.
type AlertRule struct {
	Name        string
	Description string
	Check       func(*RouteProfile) bool
	Message     func(*RouteProfile) string
}

// PerformanceAnomaly represents detected performance issues.
type PerformanceAnomaly struct {
	Route       string    `json:"route"`
	Type        string    `json:"type"`
	Severity    string    `json:"severity"`
	Description string    `json:"description"`
	DetectedAt  time.Time `json:"detected_at"`
	Evidence    string    `json:"evidence"`
}

// NewRouteProfiler creates a new performance profiler.
func NewRouteProfiler() *RouteProfiler {
	profiler := &RouteProfiler{
		profiles:    make(map[string]*RouteProfile),
		enabled:     true,
		sampleRate:  0.1, // Sample 10% of requests by default
		maxProfiles: 1000,
		alertRules:  defaultAlertRules(),
	}

	// Start background analysis
	go profiler.analyzeLoop(context.Background())

	return profiler
}

// ProfileMiddleware creates Gin middleware for profiling.
func (rp *RouteProfiler) ProfileMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !rp.shouldProfile() {
			c.Next()
			return
		}

		// Start timing
		start := time.Now()
		path := c.FullPath()
		method := c.Request.Method

		// Track request size
		bytesIn := c.Request.ContentLength
		if bytesIn < 0 {
			bytesIn = 0
		}

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(start)

		// Get response size
		bytesOut := int64(c.Writer.Size())

		// Create sample
		sample := RequestSample{
			Timestamp:  start,
			Duration:   duration,
			StatusCode: c.Writer.Status(),
			BytesIn:    bytesIn,
			BytesOut:   bytesOut,
		}

		// Check for errors
		if len(c.Errors) > 0 {
			sample.Error = c.Errors.String()
		}

		// Add breakdown if available (set by handlers)
		if breakdown, exists := c.Get("duration_breakdown"); exists {
			if db, ok := breakdown.(*DurationBreakdown); ok {
				sample.Breakdown = db
			}
		}

		// Record sample
		rp.recordSample(path, method, sample)
	}
}

// RecordDatabaseTime records time spent in database operations.
func (rp *RouteProfiler) RecordDatabaseTime(c *gin.Context, duration time.Duration) {
	if breakdown, exists := c.Get("duration_breakdown"); exists {
		if db, ok := breakdown.(*DurationBreakdown); ok {
			db.Database += duration
		}
	} else {
		c.Set("duration_breakdown", &DurationBreakdown{
			Database: duration,
		})
	}
}

// RecordExternalTime records time spent in external API calls.
func (rp *RouteProfiler) RecordExternalTime(c *gin.Context, duration time.Duration) {
	if breakdown, exists := c.Get("duration_breakdown"); exists {
		if db, ok := breakdown.(*DurationBreakdown); ok {
			db.External += duration
		}
	} else {
		c.Set("duration_breakdown", &DurationBreakdown{
			External: duration,
		})
	}
}

// GetProfile returns performance profile for a specific route.
func (rp *RouteProfiler) GetProfile(path, method string) *RouteProfile {
	rp.mu.RLock()
	defer rp.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", method, path)
	return rp.profiles[key]
}

// GetAllProfiles returns all route profiles.
func (rp *RouteProfiler) GetAllProfiles() map[string]*RouteProfile {
	rp.mu.RLock()
	defer rp.mu.RUnlock()

	profiles := make(map[string]*RouteProfile)
	for k, v := range rp.profiles {
		profiles[k] = v
	}
	return profiles
}

// GetTopSlowest returns the slowest routes by P95 latency.
func (rp *RouteProfiler) GetTopSlowest(count int) []*RouteProfile {
	rp.mu.RLock()
	defer rp.mu.RUnlock()

	profiles := make([]*RouteProfile, 0, len(rp.profiles))
	for _, p := range rp.profiles {
		if p.Statistics != nil && p.Statistics.Count > 0 {
			profiles = append(profiles, p)
		}
	}

	// Sort by P95 duration
	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].Statistics.P95Duration > profiles[j].Statistics.P95Duration
	})

	if len(profiles) > count {
		profiles = profiles[:count]
	}

	return profiles
}

// GetAnomalies returns detected performance anomalies.
func (rp *RouteProfiler) GetAnomalies() []PerformanceAnomaly {
	rp.mu.RLock()
	defer rp.mu.RUnlock()

	anomalies := make([]PerformanceAnomaly, len(rp.anomalies))
	copy(anomalies, rp.anomalies)
	return anomalies
}

// GenerateReport creates a comprehensive performance report.
func (rp *RouteProfiler) GenerateReport() *PerformanceReport {
	rp.mu.RLock()
	defer rp.mu.RUnlock()

	report := &PerformanceReport{
		GeneratedAt: time.Now(),
		Profiles:    make(map[string]*RouteProfile),
		Summary:     &PerformanceSummary{},
		Anomalies:   rp.anomalies,
	}

	// Copy profiles
	for k, v := range rp.profiles {
		report.Profiles[k] = v
	}

	// Calculate summary
	var totalRequests int64
	var totalDuration time.Duration
	var totalErrors int64

	for _, profile := range rp.profiles {
		if profile.Statistics != nil {
			totalRequests += profile.Statistics.Count
			totalDuration += profile.Statistics.TotalDuration
			totalErrors += int64(float64(profile.Statistics.Count) * profile.Statistics.ErrorRate)
		}
	}

	report.Summary.TotalRoutes = len(rp.profiles)
	report.Summary.TotalRequests = totalRequests
	if totalRequests > 0 {
		report.Summary.AvgDuration = totalDuration / time.Duration(totalRequests)
		report.Summary.ErrorRate = float64(totalErrors) / float64(totalRequests)
	}

	// Get top issues
	report.Summary.TopSlowestRoutes = rp.GetTopSlowest(5)
	report.Summary.TopErrorRoutes = rp.getTopErrorRoutes(5)

	return report
}

// Private methods

func (rp *RouteProfiler) shouldProfile() bool {
	if !rp.enabled {
		return false
	}

	// Simple sampling
	return time.Now().UnixNano()%100 < int64(rp.sampleRate*100)
}

func (rp *RouteProfiler) recordSample(path, method string, sample RequestSample) {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	key := fmt.Sprintf("%s:%s", method, path)

	profile, exists := rp.profiles[key]
	if !exists {
		profile = &RouteProfile{
			Path:    path,
			Method:  method,
			Samples: []RequestSample{},
		}
		rp.profiles[key] = profile
	}

	// Add sample (keep last 100 samples)
	profile.Samples = append(profile.Samples, sample)
	if len(profile.Samples) > 100 {
		profile.Samples = profile.Samples[len(profile.Samples)-100:]
	}

	// Update statistics
	profile.Statistics = rp.calculateStatistics(profile.Samples)
	profile.LastUpdated = time.Now()

	// Generate recommendations
	profile.Recommendations = rp.generateRecommendations(profile)
}

func (rp *RouteProfiler) calculateStatistics(samples []RequestSample) *RouteStatistics {
	if len(samples) == 0 {
		return &RouteStatistics{}
	}

	stats := &RouteStatistics{
		Count:       int64(len(samples)),
		MinDuration: time.Hour,
	}

	durations := make([]time.Duration, 0, len(samples))
	var totalBytesIn, totalBytesOut int64
	var errorCount int

	for _, sample := range samples {
		stats.TotalDuration += sample.Duration
		durations = append(durations, sample.Duration)

		if sample.Duration < stats.MinDuration {
			stats.MinDuration = sample.Duration
		}
		if sample.Duration > stats.MaxDuration {
			stats.MaxDuration = sample.Duration
		}

		totalBytesIn += sample.BytesIn
		totalBytesOut += sample.BytesOut

		if sample.StatusCode >= 400 || sample.Error != "" {
			errorCount++
		}
	}

	// Calculate averages
	stats.AvgDuration = stats.TotalDuration / time.Duration(stats.Count)
	stats.AvgBytesIn = totalBytesIn / stats.Count
	stats.AvgBytesOut = totalBytesOut / stats.Count
	stats.ErrorRate = float64(errorCount) / float64(stats.Count)

	// Calculate percentiles
	sort.Slice(durations, func(i, j int) bool {
		return durations[i] < durations[j]
	})

	stats.P50Duration = percentile(durations, 0.50)
	stats.P95Duration = percentile(durations, 0.95)
	stats.P99Duration = percentile(durations, 0.99)

	// Calculate throughput (requests per second)
	if len(samples) > 1 {
		timeSpan := samples[len(samples)-1].Timestamp.Sub(samples[0].Timestamp)
		if timeSpan > 0 {
			stats.Throughput = float64(stats.Count) / timeSpan.Seconds()
		}
	}

	return stats
}

func (rp *RouteProfiler) generateRecommendations(profile *RouteProfile) []string {
	recommendations := []string{}

	if profile.Statistics == nil {
		return recommendations
	}

	stats := profile.Statistics

	// Check for high latency
	if stats.P95Duration > 500*time.Millisecond {
		recommendations = append(recommendations,
			fmt.Sprintf("High P95 latency (%v) - consider optimization", stats.P95Duration))

		// Check breakdown if available
		if len(profile.Samples) > 0 && profile.Samples[len(profile.Samples)-1].Breakdown != nil {
			breakdown := profile.Samples[len(profile.Samples)-1].Breakdown

			if breakdown.Database > stats.AvgDuration*7/10 {
				recommendations = append(recommendations,
					"Database operations dominate request time - optimize queries or add caching")
			}

			if breakdown.External > stats.AvgDuration*5/10 {
				recommendations = append(recommendations,
					"External API calls are slow - consider caching or async processing")
			}
		}
	}

	// Check for high error rate
	if stats.ErrorRate > 0.05 {
		recommendations = append(recommendations,
			fmt.Sprintf("High error rate (%.1f%%) - investigate error patterns", stats.ErrorRate*100))
	}

	// Check for high variance
	if stats.MaxDuration > stats.P95Duration*3 {
		recommendations = append(recommendations,
			"High latency variance - investigate outliers and add timeouts")
	}

	// Check payload sizes
	if stats.AvgBytesOut > 1024*1024 {
		recommendations = append(recommendations,
			"Large response payloads - consider pagination or compression")
	}

	// Check for N+1 patterns
	if profile.Path != "" && stats.Count > 100 {
		// Simple heuristic: if handler time is much less than total time
		if len(profile.Samples) > 0 && profile.Samples[0].Breakdown != nil {
			if profile.Samples[0].Breakdown.Handler < stats.AvgDuration/10 {
				recommendations = append(recommendations,
					"Handler execution is fast but total time is high - check for N+1 queries")
			}
		}
	}

	return recommendations
}

func (rp *RouteProfiler) analyzeLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rp.detectAnomalies()
		}
	}
}

func (rp *RouteProfiler) detectAnomalies() {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	newAnomalies := []PerformanceAnomaly{}

	for route, profile := range rp.profiles {
		// Run alert rules
		for _, rule := range rp.alertRules {
			if rule.Check(profile) {
				anomaly := PerformanceAnomaly{
					Route:       route,
					Type:        rule.Name,
					Severity:    "warning",
					Description: rule.Message(profile),
					DetectedAt:  time.Now(),
				}
				newAnomalies = append(newAnomalies, anomaly)
			}
		}
	}

	// Keep last 100 anomalies
	rp.anomalies = append(newAnomalies, rp.anomalies...)
	if len(rp.anomalies) > 100 {
		rp.anomalies = rp.anomalies[:100]
	}
}

func (rp *RouteProfiler) getTopErrorRoutes(count int) []*RouteProfile {
	profiles := make([]*RouteProfile, 0, len(rp.profiles))
	for _, p := range rp.profiles {
		if p.Statistics != nil && p.Statistics.ErrorRate > 0 {
			profiles = append(profiles, p)
		}
	}

	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].Statistics.ErrorRate > profiles[j].Statistics.ErrorRate
	})

	if len(profiles) > count {
		profiles = profiles[:count]
	}

	return profiles
}

// Helper functions

func percentile(durations []time.Duration, p float64) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	index := int(math.Ceil(p * float64(len(durations))))
	if index > 0 {
		index--
	}
	if index >= len(durations) {
		index = len(durations) - 1
	}

	return durations[index]
}

func defaultAlertRules() []AlertRule {
	return []AlertRule{
		{
			Name:        "high_latency",
			Description: "Route has high P95 latency",
			Check: func(p *RouteProfile) bool {
				return p.Statistics != nil && p.Statistics.P95Duration > time.Second
			},
			Message: func(p *RouteProfile) string {
				return fmt.Sprintf("P95 latency is %v (threshold: 1s)", p.Statistics.P95Duration)
			},
		},
		{
			Name:        "high_error_rate",
			Description: "Route has high error rate",
			Check: func(p *RouteProfile) bool {
				return p.Statistics != nil && p.Statistics.ErrorRate > 0.1
			},
			Message: func(p *RouteProfile) string {
				return fmt.Sprintf("Error rate is %.1f%% (threshold: 10%%)", p.Statistics.ErrorRate*100)
			},
		},
		{
			Name:        "latency_spike",
			Description: "Route experienced latency spike",
			Check: func(p *RouteProfile) bool {
				if p.Statistics == nil || len(p.Samples) < 10 {
					return false
				}
				recent := p.Samples[len(p.Samples)-1].Duration
				return recent > p.Statistics.P95Duration*2
			},
			Message: func(p *RouteProfile) string {
				recent := p.Samples[len(p.Samples)-1].Duration
				return fmt.Sprintf("Recent latency %v is 2x higher than P95 %v", recent, p.Statistics.P95Duration)
			},
		},
	}
}

// PerformanceReport represents a complete performance analysis.
type PerformanceReport struct {
	GeneratedAt time.Time                `json:"generated_at"`
	Profiles    map[string]*RouteProfile `json:"profiles"`
	Summary     *PerformanceSummary      `json:"summary"`
	Anomalies   []PerformanceAnomaly     `json:"anomalies"`
}

// PerformanceSummary contains high-level performance metrics.
type PerformanceSummary struct {
	TotalRoutes      int             `json:"total_routes"`
	TotalRequests    int64           `json:"total_requests"`
	AvgDuration      time.Duration   `json:"avg_duration"`
	ErrorRate        float64         `json:"error_rate"`
	TopSlowestRoutes []*RouteProfile `json:"top_slowest_routes"`
	TopErrorRoutes   []*RouteProfile `json:"top_error_routes"`
}
