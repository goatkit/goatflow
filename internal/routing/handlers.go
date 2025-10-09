package routing

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/api"
	"github.com/gotrs-io/gotrs-ce/internal/database"
    "github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/shared"
)

// buildUserContext produces a consistent User map and admin flags
func buildUserContext(c *gin.Context) (gin.H, bool) {
	user := api.GetUserMapForTemplate(c)
	isAdminGroup := false
	if v, ok := user["IsInAdminGroup"].(bool); ok {
		isAdminGroup = v
	}
	return user, isAdminGroup
}

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
				// Accept legacy cookie name used by non-YAML routes
				if alt, err2 := c.Cookie("access_token"); err2 == nil && alt != "" {
					token = alt
				}
			}
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
				c.SetCookie("access_token", "", -1, "/", "", false, true)
                accept := strings.ToLower(c.GetHeader("Accept"))
                if strings.Contains(accept, "text/html") || accept == "" {
                    c.Redirect(http.StatusSeeOther, "/login")
                } else {
                    c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
                }
                c.Abort()
                return
            }

			// Store user info in context (normalize and enrich)
			c.Set("user_email", claims.Email)
			c.Set("user_role", claims.Role)
			c.Set("user_name", claims.Email)

			// Try to resolve numeric user_id and set full user object for parity with non-YAML routes
			var resolvedID int64
			// claims.UserID is uint in our JWT implementation; convert directly
			resolvedID = int64(claims.UserID)

			// If still zero, try DB lookup by email or login
			var userObj *models.User
			if resolvedID == 0 {
				if db, dbErr := database.GetDB(); dbErr == nil && db != nil {
					var id int64
					var login, firstName, lastName sql.NullString
					// Our schema doesn't have users.email; login acts as email. Lookup by login.
					if err := db.QueryRowContext(c.Request.Context(), database.ConvertPlaceholders(`SELECT id, login, first_name, last_name FROM users WHERE login = $1 LIMIT 1`), claims.Email).Scan(&id, &login, &firstName, &lastName); err == nil {
						resolvedID = id
						userObj = &models.User{ID: uint(id), Login: login.String, FirstName: firstName.String, LastName: lastName.String, Email: login.String, ValidID: 1}
					}
				}
			} else {
				if db, dbErr := database.GetDB(); dbErr == nil && db != nil {
					var login, firstName, lastName sql.NullString
					if err := db.QueryRowContext(c.Request.Context(), database.ConvertPlaceholders(`SELECT login, first_name, last_name FROM users WHERE id = $1`), resolvedID).Scan(&login, &firstName, &lastName); err == nil {
						userObj = &models.User{ID: uint(resolvedID), Login: login.String, FirstName: firstName.String, LastName: lastName.String, Email: login.String, ValidID: 1}
					}
				}
			}

			// Determine role from group membership if possible
			if userObj != nil {
				if db, dbErr := database.GetDB(); dbErr == nil && db != nil {
					var cnt int
					_ = db.QueryRowContext(c.Request.Context(), database.ConvertPlaceholders(`SELECT COUNT(*) FROM group_user ug JOIN groups g ON ug.group_id = g.id WHERE ug.user_id = $1 AND LOWER(g.name) = 'admin'`), userObj.ID).Scan(&cnt)
					if cnt > 0 {
						c.Set("user_role", "Admin")
					} else if c.GetString("user_role") == "" {
						c.Set("user_role", "Agent")
					}
				}
			}

			if resolvedID > 0 {
				c.Set("user_id", uint(resolvedID))
			} else {
				// claims.UserID is already a uint in our implementation
				c.Set("user_id", claims.UserID)
			}
			if userObj != nil {
				c.Set("user", userObj)
				// Also provide a friendly name
				if userObj.FirstName != "" || userObj.LastName != "" {
					c.Set("user_name", strings.TrimSpace(userObj.FirstName+" "+userObj.LastName))
				}
			}

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
	registry.Override("HandleAgentNewTicket", HandleAgentNewTicket)
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

// HandleAgentNewTicket renders the new ticket form with proper nav context
func HandleAgentNewTicket(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "System unavailable"})
		return
	}

	// Build user context consistently
	user, isInAdminGroup := buildUserContext(c)

	// Queues
	queues := []gin.H{}
	if rows, err := db.QueryContext(c.Request.Context(), `SELECT id, name FROM queue WHERE valid_id = 1 ORDER BY name`); err == nil {
		defer rows.Close()
		for rows.Next() {
			var id int; var name string
			if err := rows.Scan(&id, &name); err == nil { queues = append(queues, gin.H{"ID": id, "Name": name}) }
		}
	}
	// Priorities
	priorities := []gin.H{}
	if rows, err := db.QueryContext(c.Request.Context(), `SELECT id, name FROM ticket_priority WHERE valid_id = 1 ORDER BY id`); err == nil {
		defer rows.Close()
		for rows.Next() { var id int; var name string; if err := rows.Scan(&id,&name); err==nil { priorities=append(priorities, gin.H{"ID": id, "Name": name}) } }
	}
	// Types
	types := []gin.H{}
	if rows, err := db.QueryContext(c.Request.Context(), `SELECT id, name FROM ticket_type WHERE valid_id = 1 ORDER BY name`); err == nil {
		defer rows.Close()
		for rows.Next() { var id int; var name string; if err := rows.Scan(&id,&name); err==nil { types=append(types, gin.H{"ID": id, "Name": name}) } }
	}
	// Customer users seed (limited)
	customerUsers := []gin.H{}
	if rows, err := db.QueryContext(c.Request.Context(), `SELECT login, email, first_name, last_name FROM customer_user ORDER BY last_name, first_name LIMIT 250`); err == nil {
		defer rows.Close()
		for rows.Next() { var login, email, fn, ln sql.NullString; if err := rows.Scan(&login,&email,&fn,&ln); err==nil { customerUsers=append(customerUsers, gin.H{"Login": login.String, "Email": email.String, "FirstName": fn.String, "LastName": ln.String}) } }
	}

	// Render template set by YAML (pages/tickets/new.pongo2)
	// Derive admin role similar to getUserMapForTemplate for consistency
	isAdmin := false
	// From group membership
	if isInAdminGroup { isAdmin = true }
	// From user ID heuristic (1/2)
	if uid, ok := user["ID"]; ok {
		switch v := uid.(type) {
		case int:
			if v == 1 || v == 2 { isAdmin = true }
		case int32:
			if v == 1 || v == 2 { isAdmin = true }
		case int64:
			if v == 1 || v == 2 { isAdmin = true }
		case uint:
			if v == 1 || v == 2 { isAdmin = true }
		case uint32:
			if v == 1 || v == 2 { isAdmin = true }
		case uint64:
			if v == 1 || v == 2 { isAdmin = true }
		case float64:
			if int(v) == 1 || int(v) == 2 { isAdmin = true }
		case string:
			if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil { if n == 1 || n == 2 { isAdmin = true } }
		}
	}
	// From login naming
	if !isAdmin {
		if loginVal, ok := user["Login"].(string); ok {
			if strings.Contains(strings.ToLower(loginVal), "admin") || loginVal == "root@localhost" {
				isAdmin = true
			}
		}
	}
	user["IsAdmin"] = isAdmin
	user["IsInAdminGroup"] = isInAdminGroup
	user["Role"] = map[bool]string{true: "Admin", false: "Agent"}[isAdmin]
	api.GetPongo2Renderer().HTML(c, http.StatusOK, "pages/tickets/new.pongo2", gin.H{
		"User":           user,
		"IsInAdminGroup": isInAdminGroup,
		"ActivePage":     "tickets",
		"Queues":         queues,
		"Priorities":     priorities,
		"Types":          types,
		"CustomerUsers":  customerUsers,
	})
}

func init() {
	// Best-effort registration; actual registry population occurs via RegisterExistingHandlers during setup
	// This provides the function symbol so YAML can reference "HandleCustomerInfoPanel"
}
