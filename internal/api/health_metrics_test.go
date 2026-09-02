package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// setupHealthTestRouter builds a router wired to the real health/metrics
// handlers (via the registry) without booting the full YAML router.
func setupHealthTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/health", HandleHealthCheck)
	r.GET("/health/detailed", HandleDetailedHealthCheck)
	r.GET("/metrics", HandleMetrics)
	return r
}

func TestHealthCheck_DBUp(t *testing.T) {
	r := setupHealthTestRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body %s)", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if body["status"] != "healthy" {
		t.Fatalf("status = %v, want healthy", body["status"])
	}
	comps, _ := body["components"].(map[string]any)
	if comps["database"] != "ok" {
		t.Fatalf("database = %v, want ok", comps["database"])
	}
}

func TestHealthCheck_ResponseShape_Stable(t *testing.T) {
	r := setupHealthTestRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	r.ServeHTTP(w, req)

	out := w.Body.String()
	for _, field := range []string{`"status"`, `"database"`, `"version"`} {
		if !strings.Contains(out, field) {
			t.Fatalf("response missing %s: %s", field, out)
		}
	}
}

func TestDetailedHealthCheck_IncludesCacheAndUptime(t *testing.T) {
	r := setupHealthTestRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health/detailed", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body %s)", w.Code, w.Body.String())
	}
	out := w.Body.String()
	for _, field := range []string{`"database"`, `"cache"`, `"uptime"`, `"version"`} {
		if !strings.Contains(out, field) {
			t.Fatalf("response missing %s: %s", field, out)
		}
	}
	// No cache client is wired in unit tests → "disabled".
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	comps := body["components"].(map[string]any)
	if comps["cache"] != "disabled" {
		t.Fatalf("cache = %v, want disabled in unit test env", comps["cache"])
	}
}

func TestMetrics_ExposesCoreMetrics(t *testing.T) {
	r := setupHealthTestRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	out := w.Body.String()
	// Core process gauges registered by this package's init().
	for _, m := range []string{"goatflow_up", "goatflow_process_start_time_seconds"} {
		if !strings.Contains(out, m) {
			t.Fatalf("metrics output missing %s", m)
		}
	}
	if !strings.Contains(out, "# TYPE") {
		t.Fatalf("metrics output not in Prometheus exposition format")
	}
}
