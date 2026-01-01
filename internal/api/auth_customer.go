package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/auth"
	"github.com/gotrs-io/gotrs-ce/internal/constants"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/middleware"
	"github.com/gotrs-io/gotrs-ce/internal/shared"
	"github.com/gotrs-io/gotrs-ce/internal/sysconfig"
)

// HandleCustomerLogin is the exported handler for customer login POST requests.
var HandleCustomerLogin = func(c *gin.Context) {
	handleCustomerLogin(shared.GetJWTManager())(c)
}

func handleCustomerLogin(jwtManager *auth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var login, password string

		contentType := c.GetHeader("Content-Type")

		if strings.Contains(contentType, "application/json") {
			var payload struct {
				Login    string `json:"login"`
				Password string `json:"password"`
			}
			if err := c.ShouldBindJSON(&payload); err == nil {
				login = payload.Login
				password = payload.Password
			}
		} else {
			// Form data
			login = c.PostForm("login")
			if login == "" {
				login = c.PostForm("username")
			}
			password = c.PostForm("password")
		}

		login = strings.TrimSpace(login)
		password = strings.TrimSpace(password)
		if login == "" || password == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "login and password required"})
			return
		}

		// Server-side rate limiting (fail2ban style)
		clientIP := c.ClientIP()
		if blocked, remaining := auth.DefaultLoginRateLimiter.IsBlocked(clientIP, login); blocked {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"success":         false,
				"error":           fmt.Sprintf("too many failed attempts, try again in %d seconds", int(remaining.Seconds())),
				"retry_after_sec": int(remaining.Seconds()),
			})
			return
		}

		if jwtManager == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "authentication not configured"})
			return
		}

		db, err := database.GetDB()
		if err != nil || db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "database unavailable"})
			return
		}

		provider, err := auth.CreateProvider("database", auth.ProviderDependencies{DB: db})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "auth provider unavailable"})
			return
		}

		authenticator := auth.NewAuthenticator(provider)
		user, err := authenticator.Authenticate(c.Request.Context(), login, password)
		if err != nil || user == nil || strings.ToLower(user.Role) != "customer" {
			auth.DefaultLoginRateLimiter.RecordFailure(clientIP, login)
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "invalid credentials"})
			return
		}

		// Clear rate limit on successful login
		auth.DefaultLoginRateLimiter.RecordSuccess(clientIP, login)

		tenantID := middleware.ResolveTenantFromHost(c.Request.Host)
		token, err := jwtManager.GenerateTokenWithLogin(user.ID, user.Login, user.Email, "Customer", false, tenantID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to generate token"})
			return
		}

		sessionTimeout := constants.DefaultSessionTimeout
		c.SetCookie("access_token", token, sessionTimeout, "/", "", false, true)
		c.SetCookie("auth_token", token, sessionTimeout, "/", "", false, true)

		redirectTo := "/customer/tickets"
		cfg := sysconfig.DefaultCustomerPortalConfig()
		if dbCfg, cfgErr := sysconfig.LoadCustomerPortalConfig(db); cfgErr == nil {
			cfg = dbCfg
		}
		if strings.TrimSpace(cfg.LandingPage) != "" {
			redirectTo = cfg.LandingPage
		}

		c.Header("HX-Redirect", redirectTo)
		c.JSON(http.StatusOK, gin.H{
			"success":      true,
			"access_token": token,
			"token_type":   "Bearer",
			"user": gin.H{
				"id":         user.ID,
				"login":      user.Login,
				"email":      user.Email,
				"first_name": user.FirstName,
				"last_name":  user.LastName,
				"role":       "Customer",
			},
		})
	}
}
