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

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/middleware"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/shared"
)

// DynamicFieldLoader is a function type for loading dynamic fields to avoid import cycles.
type DynamicFieldLoader func(screenKey, objectType string) ([]interface{}, error)

// dynamicFieldLoader is set externally by api package during init.
var dynamicFieldLoader DynamicFieldLoader

// wantsHTMLResponse returns true if the request expects HTML response (browser-like).
func wantsHTMLResponse(c *gin.Context) bool {
	accept := strings.ToLower(c.GetHeader("Accept"))
	if accept == "" {
		return true
	}
	return strings.Contains(accept, "text/html") || strings.Contains(accept, "*/*")
}

// SetDynamicFieldLoader sets the function used to load dynamic fields (called by api package).
func SetDynamicFieldLoader(loader DynamicFieldLoader) {
	dynamicFieldLoader = loader
}

// buildUserContext produces a consistent User map and admin flags.
func buildUserContext(c *gin.Context) (gin.H, bool) {
	user := shared.GetUserMapForTemplate(c)
	isAdminGroup := false
	if v, ok := user["IsInAdminGroup"].(bool); ok {
		isAdminGroup = v
	}
	return user, isAdminGroup
}

func nullable(val sql.NullString) string {
	if val.Valid {
		return val.String
	}
	return ""
}

// RegisterExistingHandlers registers existing handlers with the registry.
func RegisterExistingHandlers(registry *HandlerRegistry) {
	// Register middleware only - all route handlers are now in YAML
	middlewares := map[string]gin.HandlerFunc{
		"auth": func(c *gin.Context) {
			if testAuthBypassAllowed() {
				if _, exists := c.Get("user_id"); !exists {
					c.Set("user_id", uint(1))
				}
				if _, exists := c.Get("user_email"); !exists {
					c.Set("user_email", "demo@example.com")
				}
				if _, exists := c.Get("user_role"); !exists {
					c.Set("user_role", "Admin")
				}
				if _, exists := c.Get("user_name"); !exists {
					c.Set("user_name", "Demo User")
				}
				c.Next()
				return
			}

			// Public (unauthenticated) paths bypass auth
			path := c.Request.URL.Path
			if path == "/login" || path == "/api/auth/login" || path == "/api/auth/customer/login" || path == "/health" || path == "/metrics" || path == "/favicon.ico" || strings.HasPrefix(path, "/static/") || path == "/customer/login" || path == "/auth/customer" {
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
				if wantsHTMLResponse(c) {
					loginPath := "/login"
					if strings.HasPrefix(path, "/customer") {
						loginPath = "/customer/login"
					}
					c.Redirect(http.StatusSeeOther, loginPath)
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
				if wantsHTMLResponse(c) {
					loginPath := "/login"
					if strings.HasPrefix(path, "/customer") {
						loginPath = "/customer/login"
					}
					c.Redirect(http.StatusSeeOther, loginPath)
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
					if err := db.QueryRowContext(c.Request.Context(), database.ConvertPlaceholders(`SELECT id, login, first_name, last_name FROM users WHERE login = ? LIMIT 1`), claims.Email).Scan(&id, &login, &firstName, &lastName); err == nil {
						resolvedID = id
						userObj = &models.User{ID: uint(id), Login: login.String, FirstName: firstName.String, LastName: lastName.String, Email: login.String, ValidID: 1}
					}
				}
			} else {
				if db, dbErr := database.GetDB(); dbErr == nil && db != nil {
					var login, firstName, lastName sql.NullString
					if err := db.QueryRowContext(c.Request.Context(), database.ConvertPlaceholders(`SELECT login, first_name, last_name FROM users WHERE id = ?`), resolvedID).Scan(&login, &firstName, &lastName); err == nil {
						userObj = &models.User{ID: uint(resolvedID), Login: login.String, FirstName: firstName.String, LastName: lastName.String, Email: login.String, ValidID: 1}
					}
				}
			}

			// Determine role from group membership if possible
			if userObj != nil {
				if db, dbErr := database.GetDB(); dbErr == nil && db != nil {
					var cnt int
					_ = db.QueryRowContext(c.Request.Context(), database.ConvertPlaceholders(`SELECT COUNT(*) FROM group_user ug JOIN groups g ON ug.group_id = g.id WHERE ug.user_id = ? AND LOWER(g.name) = 'admin'`), userObj.ID).Scan(&cnt)
					if cnt > 0 {
						c.Set("user_role", "Admin")
					} else if c.GetString("user_role") == "" {
						c.Set("user_role", "Agent")
					}
				}
			}

			// Set isInAdminGroup from JWT claim (no DB query needed)
			if claims.IsAdmin {
				c.Set("isInAdminGroup", true)
				if userObj != nil {
					userObj.IsInAdminGroup = true
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

		"customer-portal": middleware.CustomerPortalGate(shared.GetJWTManager()),
	}

	// Register all middleware
	for name, handler := range middlewares {
		registry.RegisterMiddleware(name, handler)
	}

	// Register non-API handlers referenced by YAML
	registry.Override("HandleCustomerInfoPanel", HandleCustomerInfoPanel)
	if !registry.HandlerExists("HandleAgentNewTicket") {
		registry.Override("HandleAgentNewTicket", HandleAgentNewTicket)
	}
}

func testAuthBypassAllowed() bool {
	disable := strings.ToLower(strings.TrimSpace(os.Getenv("GOTRS_DISABLE_TEST_AUTH_BYPASS")))
	switch disable {
	case "1", "true", "yes", "on":
		return false
	}

	env := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	switch env {
	case "production", "prod":
		return false
	}

	if gin.Mode() == gin.TestMode {
		return true
	}

	switch env {
	case "", "test", "testing", "unit", "unit-test", "unit_real", "unit-real":
		return true
	}

	return false
}

// RegisterAPIHandlers registers API handlers with the registry.
func RegisterAPIHandlers(registry *HandlerRegistry, apiHandlers map[string]gin.HandlerFunc) {
	// Override existing handlers with API handlers
	registry.OverrideBatch(apiHandlers)
}

// HandleCustomerInfoPanel returns partial with customer details or unregistered notice.
func HandleCustomerInfoPanel(c *gin.Context) {
	login := c.Param("login")
	if strings.TrimSpace(login) == "" {
		c.String(http.StatusBadRequest, "missing login")
		return
	}
	orig := login
	if i := strings.Index(login, "("); i != -1 && strings.HasSuffix(login, ")") {
		inner := login[i+1 : len(login)-1]
		if strings.Contains(inner, "@") {
			login = inner
		}
	}
	if strings.Contains(login, "<") && strings.Contains(login, ">") {
		s := strings.Index(login, "<")
		e := strings.LastIndex(login, ">")
		if s != -1 && e > s {
			inner := login[s+1 : e]
			if strings.Contains(inner, "@") {
				login = inner
			}
		}
	}
	if login != orig && os.Getenv("GOTRS_DEBUG") == "1" {
		log.Printf("customer-info: normalized '%s' -> '%s'", orig, login)
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.String(http.StatusInternalServerError, "db not ready")
		return
	}

	// Exact OTRS schema (customer_user + customer_company) join by customer_id
	// We look up by login first, falling back to email if no login match.
	var user struct {
		Login, Title, FirstName, LastName, Email, Phone, Mobile, Street, Zip, City, Country, CustomerID, Comment sql.NullString
		CompanyName, CompanyStreet, CompanyZip, CompanyCity, CompanyCountry, CompanyURL, CompanyComment          sql.NullString
	}
	q := `SELECT cu.login, cu.title, cu.first_name, cu.last_name, cu.email, cu.phone, cu.mobile,
				 cu.street, cu.zip, cu.city, cu.country, cu.customer_id, cu.comments,
				 cc.name, cc.street, cc.zip, cc.city, cc.country, cc.url, cc.comments
		  FROM customer_user cu
		  LEFT JOIN customer_company cc ON cc.customer_id = cu.customer_id
		  WHERE cu.login = ? LIMIT 1`
	if err = db.QueryRowContext(c.Request.Context(), database.ConvertPlaceholders(q), login).Scan(
		&user.Login, &user.Title, &user.FirstName, &user.LastName, &user.Email, &user.Phone, &user.Mobile,
		&user.Street, &user.Zip, &user.City, &user.Country, &user.CustomerID, &user.Comment,
		&user.CompanyName, &user.CompanyStreet, &user.CompanyZip, &user.CompanyCity, &user.CompanyCountry, &user.CompanyURL, &user.CompanyComment,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Try by email
			q2 := strings.Replace(q, "cu.login = ?", "cu.email = ?", 1)
			if err = db.QueryRowContext(c.Request.Context(), database.ConvertPlaceholders(q2), login).Scan(
				&user.Login, &user.Title, &user.FirstName, &user.LastName, &user.Email, &user.Phone, &user.Mobile,
				&user.Street, &user.Zip, &user.City, &user.Country, &user.CustomerID, &user.Comment,
				&user.CompanyName, &user.CompanyStreet, &user.CompanyZip, &user.CompanyCity, &user.CompanyCountry, &user.CompanyURL, &user.CompanyComment,
			); err != nil {
				shared.GetGlobalRenderer().HTML(c, http.StatusOK, "partials/tickets/customer_info_unregistered.pongo2", gin.H{"email": login})
				return
			}
		} else {
			shared.GetGlobalRenderer().HTML(c, http.StatusOK, "partials/tickets/customer_info_unregistered.pongo2", gin.H{"email": login})
			return
		}
	}

	// Map into structures expected by template (keep legacy names user/company fields)
	var tmplUser = map[string]interface{}{
		"Login":     nullable(user.Login),
		"Title":     nullable(user.Title),
		"FirstName": nullable(user.FirstName),
		"LastName":  nullable(user.LastName),
		"Email":     nullable(user.Email),
		"Phone":     nullable(user.Phone),
		"Mobile":    nullable(user.Mobile),
		"CompanyID": nullable(user.CustomerID),
		"Comment":   nullable(user.Comment),
	}
	var tmplCompany = map[string]interface{}{
		"Name":     nullable(user.CompanyName),
		"Street":   nullable(user.CompanyStreet),
		"Postcode": nullable(user.CompanyZip),
		"City":     nullable(user.CompanyCity),
		"Country":  nullable(user.CompanyCountry),
		"URL":      nullable(user.CompanyURL),
		"Comment":  nullable(user.CompanyComment),
	}

	var openCount int
	_ = db.QueryRowContext(c.Request.Context(), database.ConvertPlaceholders(`SELECT count(*) FROM tickets WHERE customer_user_id = ? AND state NOT IN ('closed','resolved')`), nullable(user.Login)).Scan(&openCount)

	shared.GetGlobalRenderer().HTML(c, http.StatusOK, "partials/tickets/customer_info.pongo2", gin.H{"user": tmplUser, "company": tmplCompany, "open": openCount})
}

// HandleAgentNewTicket renders the new ticket form with proper nav context.
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
			var id int
			var name string
			if err := rows.Scan(&id, &name); err == nil {
				queues = append(queues, gin.H{"ID": id, "Name": name})
			}
		}
		_ = rows.Err() // Check for iteration errors
	}
	// Priorities
	priorities := []gin.H{}
	if rows, err := db.QueryContext(c.Request.Context(), `SELECT id, name FROM ticket_priority WHERE valid_id = 1 ORDER BY id`); err == nil {
		defer rows.Close()
		for rows.Next() {
			var id int
			var name string
			if err := rows.Scan(&id, &name); err == nil {
				priorities = append(priorities, gin.H{"ID": id, "Name": name})
			}
		}
		_ = rows.Err() // Check for iteration errors
	}
	// Types
	types := []gin.H{}
	if rows, err := db.QueryContext(c.Request.Context(), `SELECT id, name FROM ticket_type WHERE valid_id = 1 ORDER BY name`); err == nil {
		defer rows.Close()
		for rows.Next() {
			var id int
			var name string
			if err := rows.Scan(&id, &name); err == nil {
				types = append(types, gin.H{"ID": id, "Name": name})
			}
		}
		_ = rows.Err() // Check for iteration errors
	}
	// Customer users seed (limited)
	customerUsers := []gin.H{}
	if rows, err := db.QueryContext(c.Request.Context(), `SELECT login, email, first_name, last_name, customer_id FROM customer_user WHERE valid_id = 1 ORDER BY last_name, first_name, email LIMIT 250`); err == nil {
		defer rows.Close()
		for rows.Next() {
			var login, email, fn, ln, cid sql.NullString
			if err := rows.Scan(&login, &email, &fn, &ln, &cid); err == nil {
				customerUsers = append(customerUsers, gin.H{"Login": login.String, "Email": email.String, "FirstName": fn.String, "LastName": ln.String, "CustomerID": cid.String})
			}
		}
		_ = rows.Err() // Check for iteration errors
	}
	// Ticket states
	ticketStates := []gin.H{}
	ticketStateLookup := map[string]gin.H{}
	if opts, lookup, stateErr := shared.LoadTicketStatesForForm(db); stateErr != nil {
		log.Printf("agent route new ticket: failed to load ticket states: %v", stateErr)
	} else {
		ticketStates = opts
		ticketStateLookup = lookup
	}

	// Render template set by YAML (pages/tickets/new.pongo2)
	// Derive admin role similar to getUserMapForTemplate for consistency
	isAdmin := false
	// From group membership
	if isInAdminGroup {
		isAdmin = true
	}
	// From user ID heuristic (1/2)
	if uid, ok := user["ID"]; ok {
		switch v := uid.(type) {
		case int:
			if v == 1 || v == 2 {
				isAdmin = true
			}
		case int32:
			if v == 1 || v == 2 {
				isAdmin = true
			}
		case int64:
			if v == 1 || v == 2 {
				isAdmin = true
			}
		case uint:
			if v == 1 || v == 2 {
				isAdmin = true
			}
		case uint32:
			if v == 1 || v == 2 {
				isAdmin = true
			}
		case uint64:
			if v == 1 || v == 2 {
				isAdmin = true
			}
		case float64:
			if int(v) == 1 || int(v) == 2 {
				isAdmin = true
			}
		case string:
			if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
				if n == 1 || n == 2 {
					isAdmin = true
				}
			}
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

	// Dynamic fields for AgentTicketPhone screen
	var dynamicFields []interface{}
	if dynamicFieldLoader != nil {
		var dfErr error
		dynamicFields, dfErr = dynamicFieldLoader("AgentTicketPhone", "Ticket")
		if dfErr != nil {
			log.Printf("Warning: failed to load dynamic fields for ticket create: %v", dfErr)
		}
	}

	shared.GetGlobalRenderer().HTML(c, http.StatusOK, "pages/tickets/new.pongo2", gin.H{
		"User":              user,
		"IsInAdminGroup":    isInAdminGroup,
		"ActivePage":        "tickets",
		"Queues":            queues,
		"Priorities":        priorities,
		"Types":             types,
		"CustomerUsers":     customerUsers,
		"TicketStates":      ticketStates,
		"TicketStateLookup": ticketStateLookup,
		"DynamicFields":     dynamicFields,
	})
}

func init() {
	// Best-effort registration; actual registry population occurs via RegisterExistingHandlers during setup
	// This provides the function symbol so YAML can reference "HandleCustomerInfoPanel"
}

// This ensures YAML routes can find handlers registered via RegisterAPIHandlers.
func SyncHandlersToGlobalMap(registry *HandlerRegistry) {
	if registry == nil {
		log.Printf("Warning: HandlerRegistry is nil, cannot sync to GlobalHandlerMap")
		return
	}

	handlers := registry.GetAllHandlers()
	for name, handler := range handlers {
		GlobalHandlerMap[name] = handler
		log.Printf("DEBUG: Synced handler %s to GlobalHandlerMap", name)
	}
	log.Printf("INFO: Synced %d handlers to GlobalHandlerMap", len(handlers))
}
