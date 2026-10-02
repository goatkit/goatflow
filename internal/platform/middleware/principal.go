package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	platformmodels "github.com/goatkit/goatflow/internal/platform/models"
)

// IsCustomerPrincipal reports whether the authenticated caller is a customer:
// a customer JWT (role "Customer") or a customer API token. Customer ids live
// in customer_user, whose id space overlaps users.id, so a customer must never
// be treated as an agent with the same numeric id.
func IsCustomerPrincipal(c *gin.Context) bool {
	if v, ok := c.Get("is_customer"); ok {
		if b, isBool := v.(bool); isBool && b {
			return true
		}
	}
	if role, ok := c.Get("user_role"); ok && role == "Customer" {
		return true
	}
	if v, ok := c.Get("api_token"); ok {
		if tok, isTok := v.(*platformmodels.APIToken); isTok && tok.UserType != platformmodels.APITokenUserAgent {
			return true
		}
	}
	return false
}

// isAuthenticated reports whether an auth middleware identified the caller.
func isAuthenticated(c *gin.Context) bool {
	_, hasID := c.Get("user_id")
	_, hasRole := c.Get("user_role")
	return hasID && hasRole
}

// denyPrincipal answers an authorization refusal: 401 when nobody is
// authenticated, 403 otherwise. Browser page requests are redirected to the
// matching login page instead of getting a JSON body.
func denyPrincipal(c *gin.Context, status int, msg string) {
	if c.Request.Method == http.MethodGet && wantsHTMLPage(c) {
		login := "/login"
		if strings.HasPrefix(c.Request.URL.Path, "/customer") {
			login = "/customer/login"
		}
		c.Redirect(http.StatusSeeOther, login)
		c.Abort()
		return
	}
	c.JSON(status, gin.H{"success": false, "error": msg})
	c.Abort()
}

func wantsHTMLPage(c *gin.Context) bool {
	p := c.Request.URL.Path
	if p == "/api" || strings.HasPrefix(p, "/api/") || strings.Contains(p, "/api/") {
		return false
	}
	if c.GetHeader("HX-Request") != "" {
		return false
	}
	return strings.Contains(strings.ToLower(c.GetHeader("Accept")), "text/html")
}

// RequireAgent admits authenticated agents (agent JWTs and agent API tokens)
// and refuses customers and anonymous callers.
func RequireAgent() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isAuthenticated(c) {
			denyPrincipal(c, http.StatusUnauthorized, "Authentication required")
			return
		}
		if IsCustomerPrincipal(c) {
			denyPrincipal(c, http.StatusForbidden, "Agent access required")
			return
		}
		c.Next()
	}
}

// RequireCustomer admits authenticated customers (customer JWTs and customer
// API tokens) and refuses agents and anonymous callers.
func RequireCustomer() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isAuthenticated(c) {
			denyPrincipal(c, http.StatusUnauthorized, "Authentication required")
			return
		}
		if !IsCustomerPrincipal(c) {
			denyPrincipal(c, http.StatusForbidden, "Customer access required")
			return
		}
		c.Next()
	}
}
