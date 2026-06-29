package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSecurityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SecurityHeaders())
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	tests := []struct {
		header string
		want   string
	}{
		{"Content-Security-Policy", "default-src 'self'"},
		{"Content-Security-Policy", "frame-ancestors 'none'"},
		{"Content-Security-Policy", "base-uri 'self'"},
		{"Content-Security-Policy", "form-action 'self'"},
		// media-src allows blob: URIs so the legend voice-preview button
		// can play synthesised audio via URL.createObjectURL. 'data:' is
		// intentionally NOT permitted — avoids a "CSP weakened" audit flag.
		{"Content-Security-Policy", "media-src 'self' blob:"},
		{"X-Frame-Options", "DENY"},
		{"X-Content-Type-Options", "nosniff"},
		{"X-XSS-Protection", "1; mode=block"},
		{"Referrer-Policy", "strict-origin-when-cross-origin"},
		{"Permissions-Policy", "interest-cohort=()"},
	}

	for _, tt := range tests {
		t.Run(tt.header+"/"+tt.want, func(t *testing.T) {
			got := w.Header().Get(tt.header)
			if got == "" {
				t.Errorf("%s header missing", tt.header)
			} else if !strings.Contains(got, tt.want) {
				t.Errorf("%s = %q, want to contain %q", tt.header, got, tt.want)
			}
		})
	}
}

func TestSecurityHeaders_CSPAllowsAlpineAndHTMX(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SecurityHeaders())
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	csp := w.Header().Get("Content-Security-Policy")

	// Alpine.js requires unsafe-eval (x-data expression evaluation).
	if !strings.Contains(csp, "'unsafe-eval'") {
		t.Error("CSP must include 'unsafe-eval' for Alpine.js")
	}

	// HTMX and Alpine require unsafe-inline (inline scripts and event handlers).
	for _, directive := range strings.Split(csp, ";") {
		directive = strings.TrimSpace(directive)
		if strings.HasPrefix(directive, "script-src") {
			if !strings.Contains(directive, "'unsafe-inline'") {
				t.Error("script-src must include 'unsafe-inline' for HTMX/Alpine.js")
			}
		}
	}
}

func TestSecurityHeaders_PreventsClickjacking(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SecurityHeaders())
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	csp := w.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Error("CSP must block framing with frame-ancestors 'none'")
	}

	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Error("X-Frame-Options must be DENY")
	}
}
