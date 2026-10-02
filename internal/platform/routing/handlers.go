package routing

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/httpcookie"
	"github.com/goatkit/goatflow/internal/platform/middleware"
	platformmodels "github.com/goatkit/goatflow/internal/platform/models"
	"github.com/goatkit/goatflow/internal/platform/shared"
)

// wantsHTMLResponse returns true if the request expects HTML response (browser-like).
// API paths never do: an unauthenticated /api/** call gets a 401 JSON answer
// whatever its Accept header says, instead of a redirect to the login page.
func wantsHTMLResponse(c *gin.Context) bool {
	if p := c.Request.URL.Path; p == "/api" || strings.HasPrefix(p, "/api/") {
		return false
	}
	accept := strings.ToLower(c.GetHeader("Accept"))
	if accept == "" {
		return true
	}
	return strings.Contains(accept, "text/html") || strings.Contains(accept, "*/*")
}

func nullable(val sql.NullString) string {
	if val.Valid {
		return val.String
	}
	return ""
}

func isPublicAuthPath(path string) bool {
	// OIDC/SAML IdP redirect and callback endpoints must be public
	if strings.HasPrefix(path, "/auth/") {
		return true
	}
	switch path {
	case "/login",
		"/login/2fa",
		"/api/auth/login",
		"/api/auth/passkey/begin",
		"/api/auth/passkey/finish",
		"/api/auth/2fa/verify",
		"/api/auth/2fa/webauthn/begin",
		"/api/auth/2fa/webauthn/finish",
		"/customer/login",
		"/customer/login/2fa",
		"/api/auth/customer/login",
		"/api/auth/customer/passkey/begin",
		"/api/auth/customer/passkey/finish",
		"/api/auth/customer/2fa/verify",
		"/api/auth/customer/2fa/webauthn/begin",
		"/api/auth/customer/2fa/webauthn/finish":
		return true
	default:
		return false
	}
}

// RegisterExistingHandlers registers existing handlers with the registry.
func RegisterExistingHandlers(registry *HandlerRegistry) {
	// Register middleware only - all route handlers are now in YAML
	middlewares := map[string]gin.HandlerFunc{
		"auth": func(c *gin.Context) {
			// Public (unauthenticated) paths bypass auth. /health is the probe
			// target (Docker HEALTHCHECK, TrueNAS, k8s) and must stay open;
			// /health/detailed and /metrics are auth-gated at route level
			// (routes/basic.yaml) and intentionally have no bypass. /swagger/
			// stays public: the OpenAPI spec ships in the open-source repo and
			// every user can create API tokens, so the API docs are open to all.
			path := c.Request.URL.Path
			if isPublicAuthPath(path) ||
				path == "/health" || path == "/favicon.ico" || path == "/manifest.json" || path == "/sw.js" || path == "/sw-config.json" || strings.HasPrefix(path, "/static/") ||
				path == "/api/languages" || path == "/api/themes" || strings.HasPrefix(path, "/swagger/") {
				c.Next()
				return
			}

			// API tokens (gf_*) are accepted by unified_auth routes only.
			// Refuse them here instead of passing the request on with no
			// identity: handlers behind `auth` expect a session user.
			token := middleware.ExtractToken(c)
			if middleware.IsAPIToken(token) {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "API tokens are not accepted on this route"})
				c.Abort()
				return
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
				// Clear invalid cookies (both standard and customer-specific)
				httpcookie.SetAuth(c, "auth_token", "", -1)
				httpcookie.SetAuth(c, "access_token", "", -1)
				httpcookie.SetAuth(c, "customer_auth_token", "", -1)
				httpcookie.SetAuth(c, "customer_access_token", "", -1)
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

			// A killed session revokes the token, whatever the client sends.
			if !middleware.VerifySession(c, claims) {
				for _, name := range []string{"auth_token", "access_token", "session_id",
					"customer_auth_token", "customer_access_token", "customer_session_id"} {
					httpcookie.SetAuth(c, name, "", -1)
				}
				if wantsHTMLResponse(c) {
					loginPath := "/login"
					if strings.HasPrefix(path, "/customer") {
						loginPath = "/customer/login"
					}
					c.Redirect(http.StatusSeeOther, loginPath)
				} else {
					c.JSON(http.StatusUnauthorized, gin.H{"error": "Session has been terminated"})
				}
				c.Abort()
				return
			}

			// Store user info in context (normalize and enrich)
			c.Set("user_email", claims.Email)
			c.Set("user_role", claims.Role)
			c.Set("user_name", claims.Email)
			c.Set("is_customer", claims.Role == "Customer")

			// Try to resolve numeric user_id and set full user object for parity with non-YAML routes
			var resolvedID int64
			// claims.UserID is uint in our JWT implementation; convert directly
			resolvedID = int64(claims.UserID) // #nosec G115 -- UserID is a users/customer_user primary key we signed into the JWT; never above MaxInt64

			// SECURITY: the agent `users` and `customer_user` tables share
			// an id space (both auto-increment from 1). If we naively look
			// up claims.UserID in `users`, a Customer JWT whose UserID
			// happens to match an agent row (almost certain at low ids)
			// hijacks that agent's identity — including admin group
			// membership. Branch on claims.Role so customer JWTs stay on
			// the customer_user side, agent JWTs on the users side. The
			// email-fallback path is similarly scoped.
			var userObj *platformmodels.User
			isCustomerClaims := claims.Role == "Customer"
			db, dbErr := database.GetDB()
			if dbErr == nil && db != nil {
				if isCustomerClaims {
					// Customer branch — resolve via customer_user.login
					// (claims.Login carries the login, claims.UserID carries
					// the customer_user.id). Never read `users` here.
					resolveLogin := claims.Login
					if resolveLogin == "" {
						resolveLogin = claims.Email
					}
					var cuID int64
					var login, firstName, lastName sql.NullString
					query := `SELECT id, login, first_name, last_name FROM customer_user WHERE login = ? LIMIT 1`
					if err := db.QueryRowContext(c.Request.Context(), database.ConvertPlaceholders(query), resolveLogin).Scan(&cuID, &login, &firstName, &lastName); err == nil {
						resolvedID = cuID
						userObj = &platformmodels.User{ID: uint(cuID), Login: login.String, FirstName: firstName.String, LastName: lastName.String, Email: login.String, Role: "Customer", ValidID: 1} // #nosec G115 -- customer_user.id is a positive auto-increment key
						c.Set("customer_login", login.String)
					}
				} else if resolvedID == 0 {
					var id int64
					var login, firstName, lastName, title sql.NullString
					// Our schema doesn't have users.email; login acts as email. Lookup by login.
					if err := db.QueryRowContext(c.Request.Context(), database.ConvertPlaceholders(`SELECT id, login, first_name, last_name, title FROM users WHERE login = ? LIMIT 1`), claims.Email).Scan(&id, &login, &firstName, &lastName, &title); err == nil {
						resolvedID = id
						userObj = &platformmodels.User{ID: uint(id), Login: login.String, FirstName: firstName.String, LastName: lastName.String, Title: title.String, Email: login.String, ValidID: 1} // #nosec G115 -- users.id is a positive auto-increment key
					}
				} else {
					var login, firstName, lastName, title sql.NullString
					if err := db.QueryRowContext(c.Request.Context(), database.ConvertPlaceholders(`SELECT login, first_name, last_name, title FROM users WHERE id = ?`), resolvedID).Scan(&login, &firstName, &lastName, &title); err == nil {
						userObj = &platformmodels.User{ID: uint(resolvedID), Login: login.String, FirstName: firstName.String, LastName: lastName.String, Title: title.String, Email: login.String, ValidID: 1} // #nosec G115 -- resolvedID came from a uint claim and matched an existing users.id row
					}
				}
			}

			// Determine role from group membership if possible. Skip for
			// customer JWTs — group_user is the agent-side join table and
			// would trigger the same cross-class escalation this branching
			// defends against.
			if userObj != nil && !isCustomerClaims {
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

		// Admins are session/JWT users with role "Admin" or callers the auth
		// layer marked as admin-group members (isInAdminGroup=true: admin
		// group membership in JWT claims, '*' / 'admin:*' scoped API tokens).
		"admin": func(c *gin.Context) {
			if role, exists := c.Get("user_role"); exists && role == "Admin" {
				c.Next()
				return
			}
			if inAdminGroup, exists := c.Get("isInAdminGroup"); exists {
				if b, isBool := inAdminGroup.(bool); isBool && b {
					c.Next()
					return
				}
			}
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			c.Abort()
		},

		// agent admits agent JWTs/sessions and agent API tokens; customers
		// (whose customer_user ids overlap users ids) get 403.
		"agent": middleware.RequireAgent(),

		// customer admits customer JWTs/sessions and customer API tokens.
		"customer": middleware.RequireCustomer(),

		"audit": func(c *gin.Context) {
			c.Next()
		},

		"customer-portal":           middleware.CustomerPortalGate(shared.GetJWTManager()),
		"customer-captive-redirect": middleware.CustomerCaptiveRedirect(shared.GetJWTManager()),

		// Demo-mode guard — passthrough unless App.DemoMode=true, at which
		// point it blocks non-admin password / MFA changes on a shared
		// demo instance. Referenced by settings / agent / customer
		// routes; without this registration the YAML loader logs
		// `Warning: Middleware 'demo-guard' not found` and the
		// protection is silently unwired.
		"demo-guard": middleware.DemoGuard(),

		// Queue permission middleware - for routes requiring access to ANY queue with permission
		"queue_ro":     middleware.RequireAnyQueueAccess("ro"),
		"queue_rw":     middleware.RequireAnyQueueAccess("rw"),
		"queue_create": middleware.RequireAnyQueueAccess("create"),

		// Queue permission middleware - for routes with queue_id in path/query
		"queue_access_ro":        middleware.RequireQueueAccess("ro"),
		"queue_access_rw":        middleware.RequireQueueAccess("rw"),
		"queue_access_create":    middleware.RequireQueueAccess("create"),
		"queue_access_move_into": middleware.RequireQueueAccess("move_into"),
		"queue_access_note":      middleware.RequireQueueAccess("note"),
		"queue_access_owner":     middleware.RequireQueueAccess("owner"),
		"queue_access_priority":  middleware.RequireQueueAccess("priority"),

		// Ticket permission middleware - for routes with ticket_id/id in path (checks ticket's queue)
		"ticket_access_ro":        middleware.RequireQueueAccessFromTicket("ro"),
		"ticket_access_rw":        middleware.RequireQueueAccessFromTicket("rw"),
		"ticket_access_note":      middleware.RequireQueueAccessFromTicket("note"),
		"ticket_access_owner":     middleware.RequireQueueAccessFromTicket("owner"),
		"ticket_access_priority":  middleware.RequireQueueAccessFromTicket("priority"),
		"ticket_access_move_into": middleware.RequireQueueAccessFromTicket("move_into"),
		// Ticket read for routes customers may also use: agents need ro on
		// the ticket's queue, customers must own the ticket.
		"ticket_access_customer_ro": middleware.RequireTicketReadOrCustomerOwner(),
		// Any-queue ro for agents; customers pass (handler limits to own tickets).
		"customer_or_queue_ro": middleware.RequireCustomerOrAnyQueueAccess("ro"),

		// API token authentication
		"api_token":    middleware.APITokenAuthMiddleware(),
		"unified_auth": middleware.UnifiedAuthMiddleware(shared.GetJWTManager()),

		// API token scope middleware - restricts API token access
		"scope_tickets_read":   middleware.RequireScope("tickets:read"),
		"scope_tickets_write":  middleware.RequireScope("tickets:write"),
		"scope_tickets_delete": middleware.RequireScope("tickets:delete"),
		"scope_articles_read":  middleware.RequireScope("articles:read"),
		"scope_articles_write": middleware.RequireScope("articles:write"),
		"scope_users_read":     middleware.RequireScope("users:read"),
		"scope_queues_read":    middleware.RequireScope("queues:read"),
		"scope_admin":          middleware.RequireScope("admin:*"),
	}

	// Register all middleware
	for name, handler := range middlewares {
		if err := registry.RegisterMiddleware(name, handler); err != nil {
			log.Printf("routing: %v", err)
		}
	}

	// Register non-API handlers referenced by YAML
	registry.Override("HandleCustomerInfoPanel", HandleCustomerInfoPanel)
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
	if login != orig && os.Getenv("GOATFLOW_DEBUG") == "1" {
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

	// Open = any state whose type is not closed (OTRS state types new/open/pending*).
	var openCount int
	if err := db.QueryRowContext(c.Request.Context(), database.ConvertPlaceholders(`
		SELECT COUNT(*)
		FROM ticket t
		JOIN ticket_state ts ON ts.id = t.ticket_state_id
		JOIN ticket_state_type tst ON tst.id = ts.type_id
		WHERE t.customer_user_id = ?
		  AND tst.name IN ('new', 'open', 'pending reminder', 'pending auto')`), user.Login.String).Scan(&openCount); err != nil {
		log.Printf("customer-info: count open tickets for %q: %v", user.Login.String, err)
		c.String(http.StatusInternalServerError, "failed to load customer tickets")
		return
	}

	shared.GetGlobalRenderer().HTML(c, http.StatusOK, "partials/tickets/customer_info.pongo2", gin.H{"user": tmplUser, "company": tmplCompany, "open": openCount})
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
