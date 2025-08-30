package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/flosch/pongo2/v6"
	"github.com/gotrs-io/gotrs-ce/internal/constants"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/service"
)

// HandleAuthLogin handles the login form submission
var HandleAuthLogin = func(c *gin.Context) {
	// Get form data (not JSON since it's coming from an HTML form)
	username := c.PostForm("username")
	password := c.PostForm("password")
	
	if username == "" || password == "" {
		// Return error that HTMX can display
		pongo2Renderer.HTML(c, http.StatusBadRequest, "components/error.pongo2", pongo2.Context{
			"error": "Username and password are required",
		})
		return
	}
	
	// Get auth service
	authService := GetAuthService()
	if authService == nil {
		pongo2Renderer.HTML(c, http.StatusInternalServerError, "components/error.pongo2", pongo2.Context{
			"error": "Authentication service unavailable",
		})
		return
	}
	
	// Authenticate user
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// Use the real auth service for production-grade authentication
	user, accessToken, refreshToken, err := authService.Login(ctx, username, password)
	if err != nil {
		// Check if this is an HTMX request
		if c.GetHeader("HX-Request") == "true" {
			// For HTMX, return error HTML fragment
			c.Data(http.StatusUnauthorized, "text/html; charset=utf-8", []byte(`
				<div class="rounded-md bg-red-50 dark:bg-red-900/20 p-4 mt-4">
					<div class="text-sm text-red-800 dark:text-red-200">
						Invalid username or password
					</div>
				</div>
			`))
		} else {
			// For regular form submission, redirect back to login with error
			c.Redirect(http.StatusSeeOther, "/login?error=Invalid+username+or+password")
		}
		return
	}
	
	// Get user's preferred session timeout
	sessionTimeout := constants.DefaultSessionTimeout // Default 24 hours
	if db, err := database.GetDB(); err == nil && db != nil {
		prefService := service.NewUserPreferencesService(db)
		if userTimeout := prefService.GetSessionTimeout(int(user.ID)); userTimeout > 0 {
			sessionTimeout = userTimeout
		}
	}
	
	// Set cookies for tokens - use access_token name that SessionMiddleware expects
	c.SetCookie(
		"access_token",  // SessionMiddleware looks for this name
		accessToken,
		sessionTimeout,
		"/",
		"",
		false, // Not HTTPS in dev
		true,  // HttpOnly
	)
	
	c.SetCookie(
		"refresh_token", 
		refreshToken,
		constants.RefreshTokenTimeout, // 7 days
		"/",
		"",
		false,
		true,
	)
	
	// Store user in session (use "user_id" to match middleware)
	c.Set("user", user)
	c.Set("user_id", user.ID)
	
	// Check if this is an HTMX request or regular form submission
	if c.GetHeader("HX-Request") == "true" {
		// For HTMX, use HX-Redirect header
		c.Header("HX-Redirect", "/dashboard")
		c.String(http.StatusOK, "Login successful, redirecting...")
	} else {
		// For regular form submission, use standard redirect
		c.Redirect(http.StatusSeeOther, "/dashboard")
	}
}

// HandleAuthLogout handles user logout
var HandleAuthLogout = func(c *gin.Context) {
	// Clear cookies
	c.SetCookie("access_token", "", -1, "/", "", false, true)
	c.SetCookie("refresh_token", "", -1, "/", "", false, true)
	
	// Redirect to login
	c.Redirect(http.StatusSeeOther, "/login")
}

// HandleAuthCheck checks if user is authenticated
var HandleAuthCheck = func(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"authenticated": false,
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"authenticated": true,
		"userID":        userID,
	})
}