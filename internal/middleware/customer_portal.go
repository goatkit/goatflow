package middleware

import (
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/auth"
	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/sysconfig"
)

// CustomerPortalGate loads portal config, enforces enable/disable, and applies optional login rules.
func CustomerPortalGate(jwtManager *auth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Printf("[CUST-PORTAL] path=%s jwtManager=%v", c.Request.URL.Path, jwtManager != nil)
		db, err := database.GetDB()
		if err != nil || db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "customer portal unavailable"})
			c.Abort()
			return
		}

		cfg, err := sysconfig.LoadCustomerPortalConfig(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load portal configuration"})
			c.Abort()
			return
		}

		c.Set("customer_portal_config", cfg)

		if !cfg.Enabled {
			respondPortalDisabled(c, cfg)
			return
		}

		loginRequired := cfg.LoginRequired
		if strings.EqualFold(strings.TrimSpace(os.Getenv("CUSTOMER_FE_ONLY")), "true") || strings.TrimSpace(os.Getenv("CUSTOMER_FE_ONLY")) == "1" {
			loginRequired = true
		}

		if loginRequired {
			if jwtManager != nil {
				optional := NewAuthMiddleware(jwtManager).OptionalAuth()
				optional(c)
				if c.IsAborted() {
					redirectCustomerLoginIfHTML(c)
					return
				}
			}

			if role, ok := c.Get("user_role"); !ok || role != "Customer" {
				if wantsHTML(c) {
					c.Redirect(http.StatusFound, "/customer/login")
					c.Abort()
					return
				}
				c.JSON(http.StatusForbidden, gin.H{"error": "customer access required"})
				c.Abort()
				return
			}
			c.Next()
			return
		}

		// Login not required: attempt optional auth to enrich context, but allow anonymous users through.
		if jwtManager != nil {
			optional := NewAuthMiddleware(jwtManager).OptionalAuth()
			optional(c)
			if c.IsAborted() {
				redirectCustomerLoginIfHTML(c)
				return
			}

			if role, ok := c.Get("user_role"); ok && role != "Customer" {
				if wantsHTML(c) {
					c.Redirect(http.StatusFound, "/customer/login")
					c.Abort()
					return
				}
				c.JSON(http.StatusForbidden, gin.H{"error": "customer access required"})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

func respondPortalDisabled(c *gin.Context, cfg sysconfig.CustomerPortalConfig) {
	accept := c.GetHeader("Accept")
	if strings.Contains(accept, "text/html") {
		c.String(http.StatusServiceUnavailable, cfg.Title+" is currently disabled")
		c.Abort()
		return
	}
	c.JSON(http.StatusServiceUnavailable, gin.H{"error": "customer portal disabled"})
	c.Abort()
}

func wantsHTML(c *gin.Context) bool {
	accept := c.GetHeader("Accept")
	if accept == "" {
		return true
	}
	return strings.Contains(accept, "text/html") || strings.Contains(accept, "*/*")
}

func redirectCustomerLoginIfHTML(c *gin.Context) {
	if wantsHTML(c) {
		c.Redirect(http.StatusFound, "/customer/login")
	}
}
