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
		{"X-Frame-Options", "DENY"},
		{"X-Content-Type-Options", "nosniff"},
		{"X-XSS-Protection", "1; mode=block"},
		{"Referrer-Policy", "strict-origin-when-cross-origin"},
		{"Permissions-Policy", "interest-cohort=()"},
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			got := w.Header().Get(tt.header)
			if got == "" {
				t.Errorf("%s header missing", tt.header)
			} else if !strings.Contains(got, tt.want) {
				t.Errorf("%s = %q, want to contain %q", tt.header, got, tt.want)
			}
		})
	}
}

func TestSecurityHeaders_CSPBlocksInlineScripts(t *testing.T) {
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

	// script-src must NOT include 'unsafe-inline'
	// Parse the script-src directive specifically (not the whole CSP string,
	// since style-src legitimately uses 'unsafe-inline' for Tailwind).
	for _, directive := range strings.Split(csp, ";") {
		directive = strings.TrimSpace(directive)
		if strings.HasPrefix(directive, "script-src") && strings.Contains(directive, "'unsafe-inline'") {
			t.Error("CSP script-src must not allow 'unsafe-inline' — this defeats XSS protection")
		}
	}

	// script-src should be 'self' only
	if !strings.Contains(csp, "script-src 'self'") {
		t.Errorf("CSP should contain script-src 'self', got: %s", csp)
	}

	// frame-ancestors should be 'none' (anti-clickjacking)
	if !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Errorf("CSP should contain frame-ancestors 'none', got: %s", csp)
	}
}
