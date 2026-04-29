package middleware

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/auth"
	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/sysconfig"
)

// CaptivePluginLandingResolver maps a plugin name to its declared
// landing path. Supplied by the api package at startup to avoid an
// import cycle (middleware → api → middleware). When nil, captive
// redirects degrade to no-op — the customer sees the normal portal
// rather than getting stuck at a blank page.
type CaptivePluginLandingResolver func(pluginName string) string

var captiveLandingResolver CaptivePluginLandingResolver

// SetCaptivePluginLandingResolver installs the landing-path lookup used
// by the captive redirect inside CustomerPortalGate.
func SetCaptivePluginLandingResolver(r CaptivePluginLandingResolver) {
	captiveLandingResolver = r
}

// resolveCustomerPrimaryOrg returns the org that owns the caller's
// customer_company, falling back to gk_user_organisation.is_default.
// Customer_company ↔ gk_organisation is the admin-controlled binding;
// gk_user_organisation alone can be stale, so it's only used when no
// customer_company link exists.
func resolveCustomerPrimaryOrg(db *sql.DB, customerLogin string) int64 {
	var orgID int64
	_ = db.QueryRow(database.ConvertPlaceholders(`
		SELECT o.id FROM customer_user cu
		  JOIN gk_organisation o ON o.customer_company_id = cu.customer_id
		 WHERE cu.login = ?
		 ORDER BY o.id ASC
		 LIMIT 1`), customerLogin).Scan(&orgID)
	if orgID != 0 {
		return orgID
	}
	_ = db.QueryRow(database.ConvertPlaceholders(`
		SELECT org_id FROM gk_user_organisation
		 WHERE customer_login = ?
		 ORDER BY is_default DESC, create_time ASC
		 LIMIT 1`), customerLogin).Scan(&orgID)
	return orgID
}

// CustomerCaptiveRedirect redirects customers whose org is captive to a
// plugin. Must run before CustomerPortalGate: that gate invokes
// OptionalAuth inline which writes the response body via c.Next(),
// after which a later redirect is silently dropped.
//
// Capture only fires when the customer also has plugin access via
// gk_org_plugin_access — captive_plugin alone would lock a user out of
// the portal entirely.
func CustomerCaptiveRedirect(jwtManager *auth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if captiveLandingResolver == nil || jwtManager == nil {
			c.Next()
			return
		}
		token := ExtractToken(c)
		if token == "" {
			c.Next()
			return
		}
		claims, err := jwtManager.ValidateToken(token)
		if err != nil || claims == nil || claims.Role != "Customer" {
			c.Next()
			return
		}
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/customer/login") ||
			strings.HasPrefix(path, "/customer/logout") ||
			strings.HasPrefix(path, "/customer/api") {
			c.Next()
			return
		}
		db, err := database.GetDB()
		if err != nil || db == nil {
			c.Next()
			return
		}
		orgID := resolveCustomerPrimaryOrg(db, claims.Login)
		if orgID == 0 {
			c.Next()
			return
		}
		var cp sql.NullString
		err = db.QueryRow(database.ConvertPlaceholders(
			`SELECT captive_plugin FROM gk_organisation WHERE id = ?`), orgID).Scan(&cp)
		if err != nil || !cp.Valid || cp.String == "" {
			c.Next()
			return
		}
		var ok int
		err = db.QueryRow(database.ConvertPlaceholders(`
			SELECT 1 FROM gk_org_plugin_access opa
			  JOIN group_customer_user gcu ON gcu.group_id = opa.group_id
			 WHERE opa.org_id = ? AND opa.plugin_name = ? AND gcu.user_id = ?
			 LIMIT 1`), orgID, cp.String, claims.Login).Scan(&ok)
		if err != nil || ok != 1 {
			c.Next()
			return
		}
		landing := captiveLandingResolver(cp.String)
		if landing == "" {
			c.Next()
			return
		}
		if path == landing || strings.HasPrefix(path, strings.TrimRight(landing, "/")+"/") {
			c.Next()
			return
		}
		if wantsHTML(c) {
			c.Redirect(http.StatusFound, landing)
			c.Abort()
			return
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "this organisation is captive to plugin " + cp.String})
		c.Abort()
	}
}

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
