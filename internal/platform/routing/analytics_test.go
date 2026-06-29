package routing

import (
	"testing"
	"time"
)

func TestNewRouteMetrics(t *testing.T) {
	rm := NewRouteMetrics()

	if rm == nil {
		t.Fatal("expected non-nil RouteMetrics")
	}
	if rm.routeStats == nil {
		t.Error("expected routeStats map to be initialized")
	}
	if rm.recentRequests == nil {
		t.Error("expected recentRequests slice to be initialized")
	}
	if rm.maxRecentLogs != 1000 {
		t.Errorf("expected maxRecentLogs 1000, got %d", rm.maxRecentLogs)
	}
	if rm.uptimeStart.IsZero() {
		t.Error("expected uptimeStart to be set")
	}
}

func TestRecordRequest(t *testing.T) {
	rm := NewRouteMetrics()

	req := RouteRequest{
		Method:     "GET",
		Path:       "/api/tickets",
		Handler:    "TicketListHandler",
		StatusCode: 200,
		Duration:   100 * time.Millisecond,
		UserAgent:  "TestAgent/1.0",
		ClientIP:   "127.0.0.1",
	}

	rm.RecordRequest(req)

	if rm.totalRequests != 1 {
		t.Errorf("expected totalRequests 1, got %d", rm.totalRequests)
	}
	if rm.totalErrors != 0 {
		t.Errorf("expected totalErrors 0, got %d", rm.totalErrors)
	}

	key := "GET /api/tickets"
	stats, exists := rm.routeStats[key]
	if !exists {
		t.Fatal("expected route stats to exist")
	}
	if stats.RequestCount != 1 {
		t.Errorf("expected RequestCount 1, got %d", stats.RequestCount)
	}
	if stats.Path != "/api/tickets" {
		t.Errorf("expected Path /api/tickets, got %s", stats.Path)
	}
	if stats.Handler != "TicketListHandler" {
		t.Errorf("expected Handler TicketListHandler, got %s", stats.Handler)
	}
}

func TestRecordRequest_ErrorTracking(t *testing.T) {
	rm := NewRouteMetrics()

	errorCodes := []int{400, 401, 403, 404, 500, 502, 503}

	for _, code := range errorCodes {
		rm.RecordRequest(RouteRequest{
			Method:     "GET",
			Path:       "/api/error-test",
			StatusCode: code,
			Duration:   50 * time.Millisecond,
		})
	}

	if rm.totalErrors != int64(len(errorCodes)) {
		t.Errorf("expected %d errors, got %d", len(errorCodes), rm.totalErrors)
	}

	key := "GET /api/error-test"
	stats := rm.routeStats[key]
	if stats.ErrorCount != int64(len(errorCodes)) {
		t.Errorf("expected ErrorCount %d, got %d", len(errorCodes), stats.ErrorCount)
	}
}

func TestRecordRequest_DurationTracking(t *testing.T) {
	rm := NewRouteMetrics()

	durations := []time.Duration{
		50 * time.Millisecond,
		100 * time.Millisecond,
		150 * time.Millisecond,
		200 * time.Millisecond,
	}

	for _, d := range durations {
		rm.RecordRequest(RouteRequest{
			Method:     "GET",
			Path:       "/api/duration-test",
			StatusCode: 200,
			Duration:   d,
		})
	}

	key := "GET /api/duration-test"
	stats := rm.routeStats[key]

	if stats.MinDuration != 50*time.Millisecond {
		t.Errorf("expected MinDuration 50ms, got %v", stats.MinDuration)
	}
	if stats.MaxDuration != 200*time.Millisecond {
		t.Errorf("expected MaxDuration 200ms, got %v", stats.MaxDuration)
	}

	expectedAvg := (50 + 100 + 150 + 200) * time.Millisecond / 4
	if stats.AverageDuration != expectedAvg {
		t.Errorf("expected AverageDuration %v, got %v", expectedAvg, stats.AverageDuration)
	}
}

func TestRecordRequest_StatusCodeTracking(t *testing.T) {
	rm := NewRouteMetrics()

	rm.RecordRequest(RouteRequest{Method: "GET", Path: "/api/test", StatusCode: 200, Duration: time.Millisecond})
	rm.RecordRequest(RouteRequest{Method: "GET", Path: "/api/test", StatusCode: 200, Duration: time.Millisecond})
	rm.RecordRequest(RouteRequest{Method: "GET", Path: "/api/test", StatusCode: 201, Duration: time.Millisecond})
	rm.RecordRequest(RouteRequest{Method: "GET", Path: "/api/test", StatusCode: 404, Duration: time.Millisecond})

	key := "GET /api/test"
	stats := rm.routeStats[key]

	if stats.StatusCodes[200] != 2 {
		t.Errorf("expected 2 x 200 status, got %d", stats.StatusCodes[200])
	}
	if stats.StatusCodes[201] != 1 {
		t.Errorf("expected 1 x 201 status, got %d", stats.StatusCodes[201])
	}
	if stats.StatusCodes[404] != 1 {
		t.Errorf("expected 1 x 404 status, got %d", stats.StatusCodes[404])
	}
}

func TestRecordRequest_RecentRequestsLimit(t *testing.T) {
	rm := NewRouteMetrics()
	rm.maxRecentLogs = 10

	for i := 0; i < 20; i++ {
		rm.RecordRequest(RouteRequest{
			Method:     "GET",
			Path:       "/api/test",
			StatusCode: 200,
			Duration:   time.Millisecond,
			ClientIP:   "127.0.0.1",
		})
	}

	if len(rm.recentRequests) != 10 {
		t.Errorf("expected max 10 recent requests, got %d", len(rm.recentRequests))
	}
}

func TestGetStats(t *testing.T) {
	rm := NewRouteMetrics()

	rm.RecordRequest(RouteRequest{Method: "GET", Path: "/api/users", StatusCode: 200, Duration: 100 * time.Millisecond})
	rm.RecordRequest(RouteRequest{Method: "POST", Path: "/api/users", StatusCode: 201, Duration: 150 * time.Millisecond})
	rm.RecordRequest(RouteRequest{Method: "GET", Path: "/api/tickets", StatusCode: 200, Duration: 50 * time.Millisecond})
	rm.RecordRequest(RouteRequest{Method: "GET", Path: "/api/tickets", StatusCode: 500, Duration: 200 * time.Millisecond})

	stats := rm.GetStats()

	if stats.TotalRequests != 4 {
		t.Errorf("expected TotalRequests 4, got %d", stats.TotalRequests)
	}
	if stats.TotalErrors != 1 {
		t.Errorf("expected TotalErrors 1, got %d", stats.TotalErrors)
	}
	if len(stats.Routes) != 3 {
		t.Errorf("expected 3 unique routes, got %d", len(stats.Routes))
	}

	// Verify routes sorted by request count (GET /api/tickets should be first)
	if stats.Routes[0].Path != "/api/tickets" {
		t.Errorf("expected first route /api/tickets, got %s", stats.Routes[0].Path)
	}
}

func TestGetStats_ErrorRate(t *testing.T) {
	rm := NewRouteMetrics()

	// 10 requests, 2 errors = 20% error rate
	for i := 0; i < 8; i++ {
		rm.RecordRequest(RouteRequest{Method: "GET", Path: "/api/test", StatusCode: 200, Duration: time.Millisecond})
	}
	for i := 0; i < 2; i++ {
		rm.RecordRequest(RouteRequest{Method: "GET", Path: "/api/test", StatusCode: 500, Duration: time.Millisecond})
	}

	stats := rm.GetStats()

	if stats.ErrorRate != 20.0 {
		t.Errorf("expected ErrorRate 20.0, got %f", stats.ErrorRate)
	}
}

func TestGetRecentRequests(t *testing.T) {
	rm := NewRouteMetrics()

	for i := 0; i < 100; i++ {
		rm.RecordRequest(RouteRequest{Method: "GET", Path: "/api/test", StatusCode: 200, Duration: time.Millisecond})
	}

	recent := rm.getRecentRequests(50)
	if len(recent) != 50 {
		t.Errorf("expected 50 recent requests, got %d", len(recent))
	}

	recent = rm.getRecentRequests(200)
	if len(recent) != 100 {
		t.Errorf("expected 100 recent requests (all available), got %d", len(recent))
	}
}

func TestInitRouteMetrics(t *testing.T) {
	// Reset global
	globalMetrics = nil

	rm := InitRouteMetrics()

	if rm == nil {
		t.Fatal("expected non-nil RouteMetrics")
	}
	if globalMetrics == nil {
		t.Error("expected globalMetrics to be set")
	}
	if rm != globalMetrics {
		t.Error("expected returned metrics to be globalMetrics")
	}
}

func TestGetGlobalMetrics(t *testing.T) {
	// Reset global
	globalMetrics = nil

	rm := GetGlobalMetrics()
	if rm == nil {
		t.Fatal("expected non-nil RouteMetrics")
	}

	// Should return same instance
	rm2 := GetGlobalMetrics()
	if rm != rm2 {
		t.Error("expected same metrics instance")
	}
}

func TestRouteStats_Fields(t *testing.T) {
	rm := NewRouteMetrics()

	now := time.Now()
	rm.RecordRequest(RouteRequest{
		Method:     "POST",
		Path:       "/api/tickets",
		Handler:    "CreateTicketHandler",
		StatusCode: 201,
		Duration:   75 * time.Millisecond,
		UserAgent:  "Mozilla/5.0",
		ClientIP:   "192.168.1.1",
	})

	stats := rm.routeStats["POST /api/tickets"]

	if stats.Method != "POST" {
		t.Errorf("expected Method POST, got %s", stats.Method)
	}
	if stats.Path != "/api/tickets" {
		t.Errorf("expected Path /api/tickets, got %s", stats.Path)
	}
	if stats.Handler != "CreateTicketHandler" {
		t.Errorf("expected Handler CreateTicketHandler, got %s", stats.Handler)
	}
	if stats.LastAccessed.Before(now) {
		t.Error("expected LastAccessed to be after test start")
	}
}

func TestRequestLog_Fields(t *testing.T) {
	rm := NewRouteMetrics()

	rm.RecordRequest(RouteRequest{
		Method:     "DELETE",
		Path:       "/api/users/123",
		StatusCode: 204,
		Duration:   30 * time.Millisecond,
		UserAgent:  "curl/7.64.1",
		ClientIP:   "10.0.0.1",
	})

	if len(rm.recentRequests) != 1 {
		t.Fatal("expected 1 recent request")
	}

	log := rm.recentRequests[0]

	if log.Method != "DELETE" {
		t.Errorf("expected Method DELETE, got %s", log.Method)
	}
	if log.Path != "/api/users/123" {
		t.Errorf("expected Path /api/users/123, got %s", log.Path)
	}
	if log.StatusCode != 204 {
		t.Errorf("expected StatusCode 204, got %d", log.StatusCode)
	}
	if log.Duration != 30*time.Millisecond {
		t.Errorf("expected Duration 30ms, got %v", log.Duration)
	}
	if log.UserAgent != "curl/7.64.1" {
		t.Errorf("expected UserAgent curl/7.64.1, got %s", log.UserAgent)
	}
	if log.IP != "10.0.0.1" {
		t.Errorf("expected IP 10.0.0.1, got %s", log.IP)
	}
}

func TestGenerateRouteRows_Empty(t *testing.T) {
	rm := NewRouteMetrics()

	rows := rm.generateRouteRows([]*RouteStats{})

	if rows == "" {
		t.Error("expected non-empty HTML for empty routes")
	}
	if !contains(rows, "No routes tracked yet") {
		t.Error("expected 'No routes tracked yet' message")
	}
}

func TestGenerateRouteRows_WithRoutes(t *testing.T) {
	rm := NewRouteMetrics()

	routes := []*RouteStats{
		{
			Path:            "/api/tickets",
			Method:          "GET",
			RequestCount:    100,
			ErrorCount:      2,
			AverageDuration: 50 * time.Millisecond,
			LastAccessed:    time.Now(),
		},
		{
			Path:            "/api/users",
			Method:          "POST",
			RequestCount:    50,
			ErrorCount:      10, // >5% error rate
			AverageDuration: 100 * time.Millisecond,
			LastAccessed:    time.Now(),
		},
	}

	rows := rm.generateRouteRows(routes)

	if !contains(rows, "/api/tickets") {
		t.Error("expected /api/tickets in rows")
	}
	if !contains(rows, "/api/users") {
		t.Error("expected /api/users in rows")
	}
	if !contains(rows, "GET") {
		t.Error("expected GET method in rows")
	}
	if !contains(rows, "POST") {
		t.Error("expected POST method in rows")
	}
	if !contains(rows, "success-rate") {
		t.Error("expected success-rate class for low error route")
	}
	if !contains(rows, "error-rate") {
		t.Error("expected error-rate class for high error route")
	}
}

func TestGenerateDashboardHTML(t *testing.T) {
	rm := NewRouteMetrics()

	rm.RecordRequest(RouteRequest{Method: "GET", Path: "/api/test", StatusCode: 200, Duration: time.Millisecond})

	html := rm.generateDashboardHTML()

	if !contains(html, "GoatFlow Route Analytics") {
		t.Error("expected title in dashboard HTML")
	}
	if !contains(html, "Total Requests") {
		t.Error("expected 'Total Requests' in dashboard HTML")
	}
	if !contains(html, "Error Rate") {
		t.Error("expected 'Error Rate' in dashboard HTML")
	}
	if !contains(html, "Route Performance") {
		t.Error("expected 'Route Performance' in dashboard HTML")
	}
}

func TestConcurrentRequests(t *testing.T) {
	rm := NewRouteMetrics()

	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				rm.RecordRequest(RouteRequest{
					Method:     "GET",
					Path:       "/api/concurrent",
					StatusCode: 200,
					Duration:   time.Millisecond,
				})
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	if rm.totalRequests != 1000 {
		t.Errorf("expected 1000 total requests, got %d", rm.totalRequests)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func BenchmarkRecordRequest(b *testing.B) {
	rm := NewRouteMetrics()
	req := RouteRequest{
		Method:     "GET",
		Path:       "/api/benchmark",
		StatusCode: 200,
		Duration:   50 * time.Millisecond,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rm.RecordRequest(req)
	}
}

func BenchmarkGetStats(b *testing.B) {
	rm := NewRouteMetrics()

	for i := 0; i < 100; i++ {
		rm.RecordRequest(RouteRequest{
			Method:     "GET",
			Path:       "/api/route" + string(rune(i%10)),
			StatusCode: 200,
			Duration:   time.Millisecond * time.Duration(i),
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rm.GetStats()
	}
}
