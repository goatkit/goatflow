package shared

import "github.com/gin-gonic/gin"

// BuildRedirectURL constructs an absolute URL from the gin request context.
// It resolves the scheme from X-Forwarded-Proto (reverse proxy), falls back
// to the TLS state of the connection, then defaults to http.
func BuildRedirectURL(c *gin.Context, path string) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}
	return scheme + "://" + c.Request.Host + path
}
