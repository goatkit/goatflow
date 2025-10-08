package routing

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/api"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/shared"
)

// RegisterExistingHandlers registers existing handlers with the registry
func RegisterExistingHandlers(registry *HandlerRegistry) {
	// Register middleware only - all route handlers are now in YAML
	middlewares := map[string]gin.HandlerFunc{
		"auth": func(c *gin.Context) {
			// Public (unauthenticated) paths bypass auth
			path := c.Request.URL.Path
			if path == "/login" || path == "/api/auth/login" || path == "/health" || path == "/metrics" || path == "/favicon.ico" || strings.HasPrefix(path, "/static/") {
				c.Next()
				return
			}

			// Check for token in cookie (auth_token) or Authorization header
			token, err := c.Cookie("auth_token")
			if err != nil || token == "" {
				// Check Authorization header as fallback
				authHeader := c.GetHeader("Authorization")
				if authHeader != "" {
					parts := strings.Split(authHeader, " ")
					if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
						token = parts[1]
					}
				}
			}

            // If no token found, redirect for HTML requests, JSON for APIs
            if token == "" {
                accept := strings.ToLower(c.GetHeader("Accept"))
                if strings.Contains(accept, "text/html") || accept == "" {
                    // Browser navigation -> redirect to login
                    c.Redirect(http.StatusSeeOther, "/login")
                } else {
                    c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing authorization token"})
                }
                c.Abort()
                return
            }

			// Validate token
			jwtManager := shared.GetJWTManager()
			claims, err := jwtManager.ValidateToken(token)
            if err != nil {
                // Clear invalid cookie
                c.SetCookie("auth_token", "", -1, "/", "", false, true)
                accept := strings.ToLower(c.GetHeader("Accept"))
                if strings.Contains(accept, "text/html") || accept == "" {
                    c.Redirect(http.StatusSeeOther, "/login")
                } else {
                    c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
                }
                c.Abort()
                return
            }

			// Store user info in context
			c.Set("user_id", claims.UserID)
			c.Set("user_email", claims.Email)
			c.Set("user_role", claims.Role)
			c.Set("user_name", claims.Email)

			// Set is_customer based on role (for customer middleware compatibility)
			if claims.Role == "Customer" {
				c.Set("is_customer", true)
			} else {
				c.Set("is_customer", false)
			}

			c.Next()
		},

		"auth-optional": func(c *gin.Context) {
			c.Next()
		},

		"admin": func(c *gin.Context) {
			role, exists := c.Get("user_role")
			if !exists || role != "Admin" {
				c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
				c.Abort()
				return
			}
			c.Next()
		},

		"agent": func(c *gin.Context) {
			role, exists := c.Get("user_role")
			if !exists || (role != "Agent" && role != "Admin") {
				c.JSON(http.StatusForbidden, gin.H{"error": "Agent access required"})
				c.Abort()
				return
			}
			c.Next()
		},

		"customer": func(c *gin.Context) {
			isCustomer, exists := c.Get("is_customer")
			if !exists || !isCustomer.(bool) {
				c.JSON(http.StatusForbidden, gin.H{"error": "Customer access required"})
				c.Abort()
				return
			}
			c.Next()
		},

		"audit": func(c *gin.Context) {
			c.Next()
		},
	}

	// Register all middleware
	for name, handler := range middlewares {
		registry.RegisterMiddleware(name, handler)
	}

	// Register non-API handlers referenced by YAML
	registry.Override("HandleCustomerInfoPanel", HandleCustomerInfoPanel)
}

// RegisterAPIHandlers registers API handlers with the registry
func RegisterAPIHandlers(registry *HandlerRegistry, apiHandlers map[string]gin.HandlerFunc) {
	// Override existing handlers with API handlers
	registry.OverrideBatch(apiHandlers)
}

// HandleCustomerInfoPanel returns partial with customer details or unregistered notice
func HandleCustomerInfoPanel(c *gin.Context) {
	login := c.Param("login")
	if strings.TrimSpace(login) == "" { c.String(http.StatusBadRequest, "missing login"); return }
	orig := login
	if i := strings.Index(login, "("); i != -1 && strings.HasSuffix(login, ")") { inner := login[i+1:len(login)-1]; if strings.Contains(inner, "@") { login = inner } }
	if strings.Contains(login, "<") && strings.Contains(login, ">") { s := strings.Index(login, "<"); e := strings.LastIndex(login, ">"); if s!=-1 && e> s { inner:=login[s+1:e]; if strings.Contains(inner,"@") { login=inner } } }
	if login!=orig && os.Getenv("GOTRS_DEBUG") == "1" { log.Printf("customer-info: normalized '%s' -> '%s'", orig, login) }

	db, err := database.GetDB(); if err != nil || db == nil { c.String(http.StatusInternalServerError, "db not ready"); return }

	// Exact OTRS schema (customer_user + customer_company) join by customer_id
	// We look up by login first, falling back to email if no login match.
	var user struct {
		Login, Title, FirstName, LastName, Email, Phone, Mobile, Street, Zip, City, Country, CustomerID, Comment sql.NullString
		CompanyName, CompanyStreet, CompanyZip, CompanyCity, CompanyCountry, CompanyURL, CompanyComment sql.NullString
	}
	q := `SELECT cu.login, cu.title, cu.first_name, cu.last_name, cu.email, cu.phone, cu.mobile,
				 cu.street, cu.zip, cu.city, cu.country, cu.customer_id, cu.comments,
				 cc.name, cc.street, cc.zip, cc.city, cc.country, cc.url, cc.comments
		  FROM customer_user cu
		  LEFT JOIN customer_company cc ON cc.customer_id = cu.customer_id
		  WHERE cu.login = $1 LIMIT 1`
	if err = db.QueryRowContext(c.Request.Context(), database.ConvertPlaceholders(q), login).Scan(
		&user.Login, &user.Title, &user.FirstName, &user.LastName, &user.Email, &user.Phone, &user.Mobile,
		&user.Street, &user.Zip, &user.City, &user.Country, &user.CustomerID, &user.Comment,
		&user.CompanyName, &user.CompanyStreet, &user.CompanyZip, &user.CompanyCity, &user.CompanyCountry, &user.CompanyURL, &user.CompanyComment,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Try by email
			q2 := strings.Replace(q, "cu.login = $1", "cu.email = $1", 1)
			if err = db.QueryRowContext(c.Request.Context(), database.ConvertPlaceholders(q2), login).Scan(
				&user.Login, &user.Title, &user.FirstName, &user.LastName, &user.Email, &user.Phone, &user.Mobile,
				&user.Street, &user.Zip, &user.City, &user.Country, &user.CustomerID, &user.Comment,
				&user.CompanyName, &user.CompanyStreet, &user.CompanyZip, &user.CompanyCity, &user.CompanyCountry, &user.CompanyURL, &user.CompanyComment,
			); err != nil {
				api.GetPongo2Renderer().HTML(c, http.StatusOK, "partials/tickets/customer_info_unregistered.pongo2", gin.H{"email": login})
				return
			}
		} else {
			api.GetPongo2Renderer().HTML(c, http.StatusOK, "partials/tickets/customer_info_unregistered.pongo2", gin.H{"email": login})
			return
		}
	}

	// Map into structures expected by template (keep legacy names user/company fields)
	var tmplUser = map[string]interface{}{
		"Login": user.Login, "Title": user.Title, "FirstName": user.FirstName, "LastName": user.LastName,
		"Email": user.Email, "Phone": user.Phone, "Mobile": user.Mobile, "CompanyID": user.CustomerID, "Comment": user.Comment,
	}
	var tmplCompany = map[string]interface{}{
		"Name": user.CompanyName, "Street": user.CompanyStreet, "Postcode": user.CompanyZip, "City": user.CompanyCity,
		"Country": user.CompanyCountry, "URL": user.CompanyURL, "Comment": user.CompanyComment,
	}

	var openCount int
	_ = db.QueryRowContext(c.Request.Context(), database.ConvertPlaceholders(`SELECT count(*) FROM tickets WHERE customer_user_id = $1 AND state NOT IN ('closed','resolved')`), user.Login.String).Scan(&openCount)

	api.GetPongo2Renderer().HTML(c, http.StatusOK, "partials/tickets/customer_info.pongo2", gin.H{"user": tmplUser, "company": tmplCompany, "open": openCount})
}

func init() {
	// Best-effort registration; actual registry population occurs via RegisterExistingHandlers during setup
	// This provides the function symbol so YAML can reference "HandleCustomerInfoPanel"
}
