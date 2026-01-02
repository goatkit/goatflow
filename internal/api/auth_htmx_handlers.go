package api

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/auth"
	"github.com/gotrs-io/gotrs-ce/internal/constants"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/service"
	"github.com/gotrs-io/gotrs-ce/internal/shared"
)

// handleLoginPage shows the login page.
func handleLoginPage(c *gin.Context) {
	if cookie, err := c.Cookie("access_token"); err == nil && cookie != "" {
		c.Redirect(http.StatusFound, "/dashboard")
		return
	}

	errorMsg := c.Query("error")

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/login.pongo2", pongo2.Context{
		"error": errorMsg,
	})
}

func handleCustomerLoginPage(c *gin.Context) {
	if cookie, err := c.Cookie("access_token"); err == nil && cookie != "" {
		jwtManager := shared.GetJWTManager()
		if claims, err := jwtManager.ValidateToken(cookie); err == nil && claims.Role == "Customer" {
			c.Redirect(http.StatusFound, "/customer/tickets")
			return
		}
		c.SetCookie("access_token", "", -1, "/", "", false, true)
		c.SetCookie("auth_token", "", -1, "/", "", false, true)
	}

	errorMsg := c.Query("error")

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/customer/login.pongo2", pongo2.Context{
		"error": errorMsg,
	})
}

// handleLogin processes login requests.
func handleLogin(jwtManager *auth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		username := c.PostForm("username")
		password := c.PostForm("password")

		clientIP := c.ClientIP()
		if blocked, remaining := auth.DefaultLoginRateLimiter.IsBlocked(clientIP, username); blocked {
			if c.GetHeader("HX-Request") == "true" {
				c.JSON(http.StatusTooManyRequests, gin.H{
					"success":         false,
					"error":           fmt.Sprintf("too many failed attempts, try again in %d seconds", int(remaining.Seconds())),
					"retry_after_sec": int(remaining.Seconds()),
				})
			} else {
				getPongo2Renderer().HTML(c, http.StatusTooManyRequests, "pages/login.pongo2", pongo2.Context{
					"Error": fmt.Sprintf("Too many failed attempts. Please try again in %d seconds.", int(remaining.Seconds())),
				})
			}
			return
		}

		validLogin := false
		userID := uint(1)

		db, err := database.GetDB()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invalid credentials",
			})
			return
		}

		var dbUserID int
		var dbPassword string
		var validID int

		query := database.ConvertPlaceholders(`
			SELECT id, pw, valid_id
			FROM users
			WHERE login = $1
			AND valid_id = 1`)
		err = db.QueryRow(query, username).Scan(&dbUserID, &dbPassword, &validID)
		if err != nil {
			// User not found or other database error
		} else if validID == 1 {
			if verifyPassword(password, dbPassword) {
				validLogin = true
				userID = uint(dbUserID)
			}
		} else {
			query2 := database.ConvertPlaceholders(`
				SELECT id, pw, valid_id
				FROM users
				WHERE login = $1
				AND pw = $2
				AND valid_id = 1`)
			err = db.QueryRow(query2, username, password).Scan(&dbUserID, &dbPassword, &validID)

			if err == nil && validID == 1 {
				validLogin = true
				userID = uint(dbUserID)

				salt := generateSalt()
				combined := password + salt
				hash := sha256.Sum256([]byte(combined))
				hashedPassword := fmt.Sprintf("sha256$%s$%s", salt, hex.EncodeToString(hash[:]))

				updateQuery := database.ConvertPlaceholders(`
					UPDATE users
					SET pw = $1,
					    change_time = CURRENT_TIMESTAMP
					WHERE id = $2`)
				_, _ = db.Exec(updateQuery, hashedPassword, dbUserID) //nolint:errcheck // Best-effort password rehash
			}
		}

		if !validLogin {
			auth.DefaultLoginRateLimiter.RecordFailure(clientIP, username)
			isHXRequest := c.GetHeader("HX-Request") == "true"
			isJSONRequest := strings.Contains(c.GetHeader("Accept"), "application/json")
			renderMissing := getPongo2Renderer() == nil || getPongo2Renderer().TemplateSet() == nil
			if isHXRequest || isJSONRequest || renderMissing {
				c.JSON(http.StatusUnauthorized, gin.H{
					"success": false,
					"error":   "Invalid credentials",
				})
				return
			}
			getPongo2Renderer().HTML(c, http.StatusUnauthorized, "pages/login.pongo2", pongo2.Context{
				"Error": "Invalid username or password",
			})
			return
		}

		auth.DefaultLoginRateLimiter.RecordSuccess(clientIP, username)

		var token string
		if jwtManager != nil {
			tokenStr, err := jwtManager.GenerateToken(userID, username, "user", 1)
			if err != nil {
				sendErrorResponse(c, http.StatusInternalServerError, "Failed to generate token")
				return
			}
			token = tokenStr
		} else {
			token = fmt.Sprintf("demo_session_%d_%d", userID, time.Now().Unix())
		}

		sessionTimeout := constants.DefaultSessionTimeout
		if db != nil {
			prefService := service.NewUserPreferencesService(db)
			if userTimeout := prefService.GetSessionTimeout(int(userID)); userTimeout > 0 {
				sessionTimeout = userTimeout
			}
		}

		c.SetCookie("access_token", token, sessionTimeout, "/", "", false, true)
		c.SetCookie("auth_token", token, sessionTimeout, "/", "", false, true)

		if c.GetHeader("HX-Request") == "true" {
			c.Header("HX-Redirect", "/dashboard")
			c.JSON(http.StatusOK, gin.H{
				"success":  true,
				"redirect": "/dashboard",
			})
			return
		}

		c.Redirect(http.StatusFound, "/dashboard")
	}
}

// handleHTMXLogin handles HTMX login requests.
func handleHTMXLogin(c *gin.Context) {
	demoEmail := os.Getenv("DEMO_LOGIN_EMAIL")
	demoPassword := os.Getenv("DEMO_LOGIN_PASSWORD")

	var payload struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	_ = c.ShouldBindJSON(&payload) //nolint:errcheck // Defaults to empty

	if strings.TrimSpace(payload.Email) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "email required"})
		return
	}

	if demoEmail != "" || demoPassword != "" {
		if payload.Email != demoEmail || payload.Password != demoPassword {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Invalid credentials"})
			return
		}

		token, err := getJWTManager().GenerateToken(1, demoEmail, "Agent", 0)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to generate token"})
			return
		}

		c.Header("HX-Redirect", "/dashboard")
		c.JSON(http.StatusOK, gin.H{
			"success":      true,
			"access_token": token,
			"token_type":   "Bearer",
			"user": gin.H{
				"login":      demoEmail,
				"email":      demoEmail,
				"first_name": "Test",
				"last_name":  "User",
				"role":       "Agent",
			},
		})
		return
	}

	testEmail := os.Getenv("TEST_AUTH_EMAIL")
	testPass := os.Getenv("TEST_AUTH_PASSWORD")
	if testEmail != "" && testPass != "" && payload.Email == testEmail && payload.Password == testPass {
		token := "test-token"
		c.Header("HX-Redirect", "/dashboard")
		c.JSON(http.StatusOK, gin.H{
			"success":      true,
			"access_token": token,
			"token_type":   "Bearer",
			"user": gin.H{
				"login":      payload.Email,
				"email":      payload.Email,
				"first_name": "Admin",
				"last_name":  "User",
				"role":       "Agent",
			},
		})
		return
	}

	c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Invalid credentials"})
}

// handleDemoCustomerLogin creates a demo customer token for testing.
func handleDemoCustomerLogin(c *gin.Context) {
	token := fmt.Sprintf("demo_customer_%s_%d", "john.customer", time.Now().Unix())
	c.SetCookie("access_token", token, 86400, "/", "", false, true)
	c.Redirect(http.StatusFound, "/customer/")
}

// handleLogout handles logout requests.
func handleLogout(c *gin.Context) {
	c.SetCookie("access_token", "", -1, "/", "", false, true)
	c.SetCookie("auth_token", "", -1, "/", "", false, true)
	c.SetCookie("token", "", -1, "/", "", false, true)
	c.SetCookie("access_token", "", -1, "/customer", "", false, true)
	c.SetCookie("auth_token", "", -1, "/customer", "", false, true)
	c.SetCookie("token", "", -1, "/customer", "", false, true)
	c.Redirect(http.StatusFound, loginRedirectPath(c))
}

func loginRedirectPath(c *gin.Context) string {
	path := c.Request.URL.Path
	if strings.Contains(path, "/customer") {
		return "/customer/login"
	}

	if ref := c.Request.Referer(); strings.Contains(ref, "/customer/") || strings.HasSuffix(ref, "/customer") {
		return "/customer/login"
	}

	if full := c.FullPath(); full != "" && strings.HasPrefix(full, "/customer") {
		return "/customer/login"
	}

	if role, ok := c.Get("user_role"); ok {
		if strings.EqualFold(fmt.Sprintf("%v", role), "customer") {
			return "/customer/login"
		}
	}

	if isCustomer, ok := c.Get("is_customer"); ok {
		if val, ok := isCustomer.(bool); ok && val {
			return "/customer/login"
		}
	}

	if strings.HasPrefix(c.Request.URL.Path, "/customer") {
		return "/customer/login"
	}

	switch strings.ToLower(strings.TrimSpace(os.Getenv("CUSTOMER_FE_ONLY"))) {
	case "1", "true":
		return "/customer/login"
	}

	return "/login"
}
