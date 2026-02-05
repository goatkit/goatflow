package api

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/apierrors"
	"github.com/gotrs-io/gotrs-ce/internal/middleware"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/service"
	"github.com/gotrs-io/gotrs-ce/internal/shared"
)

// apiTokenService is the global token service instance
var apiTokenService *service.APITokenService

// SetAPITokenService sets the global API token service
func SetAPITokenService(svc *service.APITokenService) {
	apiTokenService = svc
}

// InitAPITokenService initializes the API token service with a database connection
func InitAPITokenService(db *sql.DB) {
	log.Println("🔑 Initializing API token service...")
	apiTokenService = service.NewAPITokenService(db)
	// Register the service as the middleware's token verifier
	middleware.SetAPITokenVerifier(apiTokenService)
	log.Println("✅ API token service initialized (gf_* tokens enabled)")
}

// getUserContext extracts user ID and type from request context
// Works for both agents and customers based on middleware-set context values
func getUserContext(c *gin.Context) (userID int, userType models.APITokenUserType, ok bool) {
	// Check if this is a customer
	isCustomer := false
	if cust, exists := c.Get("is_customer"); exists {
		if b, ok := cust.(bool); ok {
			isCustomer = b
		}
	}
	// Also check user_role for customer
	if role, exists := c.Get("user_role"); exists {
		if r, ok := role.(string); ok && r == "Customer" {
			isCustomer = true
		}
	}

	if isCustomer {
		// Customer: try customer_user_id first, then fall back to standard user_id
		if id, exists := c.Get("customer_user_id"); exists {
			if uid, uok := id.(int); uok {
				return uid, models.APITokenUserCustomer, true
			}
		}
		userID = shared.GetUserIDFromCtx(c, 0)
		if userID == 0 {
			return 0, models.APITokenUserAgent, false // type doesn't matter when ok=false
		}
		return userID, models.APITokenUserCustomer, true
	}

	// Agent
	userID = shared.GetUserIDFromCtx(c, 0)
	if userID == 0 {
		return 0, models.APITokenUserAgent, false // type doesn't matter when ok=false
	}
	return userID, models.APITokenUserAgent, true
}

// HandleListTokens returns all tokens for the current user (agent or customer)
// GET /api/v1/tokens or /customer/api/v1/tokens
//
//	@Summary		List my API tokens
//	@Description	List all API tokens for the authenticated user
//	@Tags			API Tokens
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}	"List of tokens"
//	@Failure		401	{object}	map[string]interface{}	"Unauthorized"
//	@Security		BearerAuth
//	@Router			/tokens [get]
func HandleListTokens(c *gin.Context) {
	if apiTokenService == nil {
		apierrors.Error(c, apierrors.CodeServiceUnavailable)
		return
	}

	userID, userType, ok := getUserContext(c)
	if !ok {
		apierrors.Error(c, apierrors.CodeUnauthorized)
		return
	}

	tokens, err := apiTokenService.ListUserTokens(c.Request.Context(), userID, userType)
	if err != nil {
		apierrors.Error(c, apierrors.CodeInternalError)
		return
	}

	c.JSON(http.StatusOK, gin.H{"tokens": tokens})
}

// HandleCreateToken creates a new API token (agent or customer)
// POST /api/v1/tokens or /customer/api/v1/tokens
//
//	@Summary		Create API token
//	@Description	Create a new API token for the authenticated user
//	@Tags			API Tokens
//	@Accept			json
//	@Produce		json
//	@Param			token	body		object	true	"Token data (name, scopes, expires_at)"
//	@Success		201		{object}	map[string]interface{}	"Created token (includes raw token - save it!)"
//	@Failure		400		{object}	map[string]interface{}	"Invalid request"
//	@Failure		401		{object}	map[string]interface{}	"Unauthorized"
//	@Security		BearerAuth
//	@Router			/tokens [post]
func HandleCreateToken(c *gin.Context) {
	if apiTokenService == nil {
		apierrors.Error(c, apierrors.CodeServiceUnavailable)
		return
	}

	userID, userType, ok := getUserContext(c)
	if !ok {
		apierrors.Error(c, apierrors.CodeUnauthorized)
		return
	}

	var req models.APITokenCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.ErrorWithMessage(c, apierrors.CodeInvalidRequest, "Invalid request body: "+err.Error())
		return
	}

	// Filter out admin scopes for customers
	if userType == models.APITokenUserCustomer {
		filteredScopes := make([]string, 0, len(req.Scopes))
		for _, s := range req.Scopes {
			if strings.HasPrefix(s, "admin:") {
				continue
			}
			filteredScopes = append(filteredScopes, s)
		}
		req.Scopes = filteredScopes
	}

	resp, err := apiTokenService.GenerateToken(c.Request.Context(), &req, userID, userType, userID)
	if err != nil {
		// Determine specific error code based on error message
		code := apierrors.CodeInvalidRequest
		if strings.Contains(err.Error(), "scope") {
			code = apierrors.CodeInvalidScope
		} else if strings.Contains(err.Error(), "expir") {
			code = apierrors.CodeInvalidExpiration
		}
		apierrors.ErrorWithMessage(c, code, err.Error())
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// HandleRevokeToken revokes a token by ID (agent or customer)
// DELETE /api/v1/tokens/:id or /customer/api/v1/tokens/:id
//
//	@Summary		Revoke API token
//	@Description	Revoke an API token (user can only revoke own tokens)
//	@Tags			API Tokens
//	@Accept			json
//	@Produce		json
//	@Param			id	path		int	true	"Token ID"
//	@Success		200	{object}	map[string]interface{}	"Token revoked"
//	@Failure		401	{object}	map[string]interface{}	"Unauthorized"
//	@Failure		404	{object}	map[string]interface{}	"Token not found"
//	@Security		BearerAuth
//	@Router			/tokens/{id} [delete]
func HandleRevokeToken(c *gin.Context) {
	if apiTokenService == nil {
		apierrors.Error(c, apierrors.CodeServiceUnavailable)
		return
	}

	userID, userType, ok := getUserContext(c)
	if !ok {
		apierrors.Error(c, apierrors.CodeUnauthorized)
		return
	}

	tokenID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		apierrors.ErrorWithMessage(c, apierrors.CodeInvalidID, "Invalid token ID format")
		return
	}

	err = apiTokenService.RevokeToken(c.Request.Context(), tokenID, userID, userType, userID)
	if err != nil {
		apierrors.Error(c, apierrors.CodeTokenNotFound)
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "revoked"})
}

// HandleGetScopes returns available scopes for token creation
// GET /api/v1/tokens/scopes
// Scopes are filtered based on the user's role and type
//
//	@Summary		Get available scopes
//	@Description	List available scopes for token creation (filtered by user role)
//	@Tags			API Tokens
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}	"List of available scopes"
//	@Failure		401	{object}	map[string]interface{}	"Unauthorized"
//	@Security		BearerAuth
//	@Router			/tokens/scopes [get]
func HandleGetScopes(c *gin.Context) {
	// Get user context
	userRole := "User" // Default
	if role, exists := c.Get("user_role"); exists {
		if r, ok := role.(string); ok {
			userRole = r
		}
	}

	isCustomer := false
	if cust, exists := c.Get("is_customer"); exists {
		if b, ok := cust.(bool); ok {
			isCustomer = b
		}
	}
	// Also check user_role for customer
	if userRole == "Customer" {
		isCustomer = true
	}

	// Get available scopes for this user
	scopeDefs := models.GetAvailableScopes(userRole, isCustomer)

	// Convert to response format
	type scopeInfo struct {
		Scope       string `json:"scope"`
		Description string `json:"description"`
		Category    string `json:"category,omitempty"`
	}

	scopes := make([]scopeInfo, 0, len(scopeDefs))
	for _, def := range scopeDefs {
		scopes = append(scopes, scopeInfo{
			Scope:       def.Scope,
			Description: def.Description,
			Category:    def.Category,
		})
	}

	c.JSON(http.StatusOK, gin.H{"scopes": scopes})
}

// Customer handlers are aliases to the unified handlers above
// The handlers detect agent vs customer from context automatically
var (
	HandleCustomerListTokens   = HandleListTokens
	HandleCustomerCreateToken  = HandleCreateToken
	HandleCustomerRevokeToken  = HandleRevokeToken
)

// === Admin Token Handlers ===

// HandleAdminListAllTokens returns all tokens (admin only)
// GET /api/v1/admin/tokens
func HandleAdminListAllTokens(c *gin.Context) {
	if apiTokenService == nil {
		apierrors.Error(c, apierrors.CodeServiceUnavailable)
		return
	}

	includeRevoked := c.Query("include_revoked") == "true"

	tokens, err := apiTokenService.ListAllTokens(c.Request.Context(), includeRevoked)
	if err != nil {
		apierrors.Error(c, apierrors.CodeInternalError)
		return
	}

	// Convert to list items (hide token hashes)
	items := make([]*models.APITokenListItem, 0, len(tokens))
	for _, t := range tokens {
		item := &models.APITokenListItem{
			ID:        t.ID,
			Name:      t.Name,
			Prefix:    t.Prefix,
			Scopes:    t.Scopes,
			CreatedAt: t.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			IsActive:  t.IsActive(),
		}
		if t.ExpiresAt.Valid {
			exp := t.ExpiresAt.Time.Format("2006-01-02T15:04:05Z07:00")
			item.ExpiresAt = &exp
		}
		if t.LastUsedAt.Valid {
			lu := t.LastUsedAt.Time.Format("2006-01-02T15:04:05Z07:00")
			item.LastUsedAt = &lu
		}
		items = append(items, item)
	}

	c.JSON(http.StatusOK, gin.H{"tokens": items})
}

// HandleAdminRevokeToken revokes any token (admin only)
// DELETE /api/v1/admin/tokens/:id
func HandleAdminRevokeToken(c *gin.Context) {
	if apiTokenService == nil {
		apierrors.Error(c, apierrors.CodeServiceUnavailable)
		return
	}

	adminID := shared.GetUserIDFromCtx(c, 0)

	tokenID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		apierrors.ErrorWithMessage(c, apierrors.CodeInvalidID, "Invalid token ID format")
		return
	}

	err = apiTokenService.RevokeTokenAdmin(c.Request.Context(), tokenID, adminID)
	if err != nil {
		apierrors.ErrorWithMessage(c, apierrors.CodeTokenNotFound, "Token not found or already revoked")
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "revoked"})
}

// getAdminTargetContext extracts target user ID and type from route params
// Supports both /users/:userId and /customer-users/:customerId routes
func getAdminTargetContext(c *gin.Context) (targetID int, userType models.APITokenUserType, idKey string, ok bool) {
	// Check for customer route first
	if customerIDStr := c.Param("customerId"); customerIDStr != "" {
		id, err := strconv.Atoi(customerIDStr)
		if err != nil {
			apierrors.ErrorWithMessage(c, apierrors.CodeInvalidID, "Invalid customer ID format")
			return 0, models.APITokenUserAgent, "", false
		}
		return id, models.APITokenUserCustomer, "customer_id", true
	}

	// Fall back to agent route
	if userIDStr := c.Param("userId"); userIDStr != "" {
		id, err := strconv.Atoi(userIDStr)
		if err != nil {
			apierrors.ErrorWithMessage(c, apierrors.CodeInvalidID, "Invalid user ID format")
			return 0, models.APITokenUserAgent, "", false
		}
		return id, models.APITokenUserAgent, "user_id", true
	}

	apierrors.ErrorWithMessage(c, apierrors.CodeInvalidID, "Missing user or customer ID")
	return 0, models.APITokenUserAgent, "", false
}

// HandleAdminListTargetTokens returns tokens for a specific user or customer (admin only)
// GET /api/v1/admin/users/:userId/tokens or /api/v1/admin/customer-users/:customerId/tokens
func HandleAdminListTargetTokens(c *gin.Context) {
	if apiTokenService == nil {
		apierrors.Error(c, apierrors.CodeServiceUnavailable)
		return
	}

	targetID, userType, idKey, ok := getAdminTargetContext(c)
	if !ok {
		return // error already sent
	}

	tokens, err := apiTokenService.ListUserTokens(c.Request.Context(), targetID, userType)
	if err != nil {
		apierrors.Error(c, apierrors.CodeInternalError)
		return
	}

	c.JSON(http.StatusOK, gin.H{"tokens": tokens, idKey: targetID})
}

// HandleAdminCreateTargetToken creates a token for a specific user or customer (admin only)
// POST /api/v1/admin/users/:userId/tokens or /api/v1/admin/customer-users/:customerId/tokens
func HandleAdminCreateTargetToken(c *gin.Context) {
	if apiTokenService == nil {
		apierrors.Error(c, apierrors.CodeServiceUnavailable)
		return
	}

	targetID, userType, _, ok := getAdminTargetContext(c)
	if !ok {
		return // error already sent
	}

	adminID := shared.GetUserIDFromCtx(c, 0)

	var req models.APITokenCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.ErrorWithMessage(c, apierrors.CodeInvalidRequest, "Invalid request: "+err.Error())
		return
	}

	// Validate scopes
	for _, scope := range req.Scopes {
		// Customers can't have admin scopes
		if userType == models.APITokenUserCustomer && strings.HasPrefix(scope, "admin:") {
			apierrors.ErrorWithMessage(c, apierrors.CodeInvalidRequest, "Customers cannot have admin scopes")
			return
		}
		if _, valid := models.ValidScopes[scope]; !valid {
			if !isValidWildcardScope(scope) {
				apierrors.ErrorWithMessage(c, apierrors.CodeInvalidRequest, "Invalid scope: "+scope)
				return
			}
		}
	}

	tenantID := 1 // Default tenant

	resp, err := apiTokenService.GenerateTokenForUser(c.Request.Context(), &req, targetID, userType, tenantID, adminID)
	if err != nil {
		apierrors.ErrorWithMessage(c, apierrors.CodeInternalError, "Failed to create token: "+err.Error())
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// HandleAdminRevokeTargetToken revokes a specific user's or customer's token (admin only)
// DELETE /api/v1/admin/users/:userId/tokens/:tokenId or /api/v1/admin/customer-users/:customerId/tokens/:tokenId
func HandleAdminRevokeTargetToken(c *gin.Context) {
	if apiTokenService == nil {
		apierrors.Error(c, apierrors.CodeServiceUnavailable)
		return
	}

	targetID, userType, _, ok := getAdminTargetContext(c)
	if !ok {
		return // error already sent
	}

	tokenID, err := strconv.ParseInt(c.Param("tokenId"), 10, 64)
	if err != nil {
		apierrors.ErrorWithMessage(c, apierrors.CodeInvalidID, "Invalid token ID format")
		return
	}

	adminID := shared.GetUserIDFromCtx(c, 0)

	// Verify token belongs to the specified user/customer before revoking
	token, err := apiTokenService.GetToken(c.Request.Context(), tokenID)
	if err != nil || token == nil {
		apierrors.ErrorWithMessage(c, apierrors.CodeTokenNotFound, "Token not found")
		return
	}

	if token.UserID != targetID || token.UserType != userType {
		apierrors.ErrorWithMessage(c, apierrors.CodeForbidden, "Token does not belong to specified user")
		return
	}

	err = apiTokenService.RevokeTokenAdmin(c.Request.Context(), tokenID, adminID)
	if err != nil {
		apierrors.ErrorWithMessage(c, apierrors.CodeTokenNotFound, "Token not found or already revoked")
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "revoked"})
}

// isValidWildcardScope checks if a scope is a valid wildcard pattern
func isValidWildcardScope(scope string) bool {
	// Check patterns like "tickets:*", "admin:*", etc.
	parts := strings.Split(scope, ":")
	if len(parts) == 2 && parts[1] == "*" {
		validPrefixes := []string{"tickets", "articles", "users", "queues", "admin"}
		for _, prefix := range validPrefixes {
			if parts[0] == prefix {
				return true
			}
		}
	}
	return false
}

// Admin handler aliases - unified handlers work for both agents and customers
var (
	HandleAdminListUserTokens     = HandleAdminListTargetTokens
	HandleAdminCreateUserToken    = HandleAdminCreateTargetToken
	HandleAdminRevokeUserToken    = HandleAdminRevokeTargetToken
	HandleAdminListCustomerTokens = HandleAdminListTargetTokens
	HandleAdminCreateCustomerToken = HandleAdminCreateTargetToken
	HandleAdminRevokeCustomerToken = HandleAdminRevokeTargetToken
)

// Note: Routes are defined in routes/api-tokens.yaml
// Handlers are registered automatically via the YAML routing system
