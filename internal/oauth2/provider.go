package oauth2

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GrantType represents OAuth2 grant types
type GrantType string

const (
	GrantTypeAuthorizationCode GrantType = "authorization_code"
	GrantTypeClientCredentials GrantType = "client_credentials"
	GrantTypeRefreshToken      GrantType = "refresh_token"
)

// ResponseType represents OAuth2 response types
type ResponseType string

const (
	ResponseTypeCode  ResponseType = "code"
	ResponseTypeToken ResponseType = "token"
)

// TokenType represents OAuth2 token types
type TokenType string

const (
	TokenTypeBearer TokenType = "Bearer"
)

// Client represents an OAuth2 client application
type Client struct {
	ID           string    `json:"id" db:"id"`
	Secret       string    `json:"secret,omitempty" db:"secret"`
	Name         string    `json:"name" db:"name"`
	Description  string    `json:"description" db:"description"`
	RedirectURIs []string  `json:"redirect_uris" db:"redirect_uris"`
	Scopes       []string  `json:"scopes" db:"scopes"`
	GrantTypes   []GrantType `json:"grant_types" db:"grant_types"`
	IsActive     bool      `json:"is_active" db:"is_active"`
	IsConfidential bool    `json:"is_confidential" db:"is_confidential"` // true for server apps, false for SPAs/mobile
	
	// Metadata
	CreatedBy uint      `json:"created_by" db:"created_by"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
	LastUsed  *time.Time `json:"last_used,omitempty" db:"last_used"`
}

// AuthorizationCode represents an OAuth2 authorization code
type AuthorizationCode struct {
	Code        string    `json:"code" db:"code"`
	ClientID    string    `json:"client_id" db:"client_id"`
	UserID      uint      `json:"user_id" db:"user_id"`
	RedirectURI string    `json:"redirect_uri" db:"redirect_uri"`
	Scopes      []string  `json:"scopes" db:"scopes"`
	CodeChallenge string  `json:"code_challenge,omitempty" db:"code_challenge"` // PKCE
	CodeChallengeMethod string `json:"code_challenge_method,omitempty" db:"code_challenge_method"`
	ExpiresAt   time.Time `json:"expires_at" db:"expires_at"`
	Used        bool      `json:"used" db:"used"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// AccessToken represents an OAuth2 access token
type AccessToken struct {
	Token     string    `json:"token" db:"token"`
	ClientID  string    `json:"client_id" db:"client_id"`
	UserID    *uint     `json:"user_id,omitempty" db:"user_id"` // null for client credentials
	Scopes    []string  `json:"scopes" db:"scopes"`
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	LastUsed  *time.Time `json:"last_used,omitempty" db:"last_used"`
	IsActive  bool      `json:"is_active" db:"is_active"`
}

// RefreshToken represents an OAuth2 refresh token
type RefreshToken struct {
	Token         string    `json:"token" db:"token"`
	AccessTokenID string    `json:"access_token_id" db:"access_token_id"`
	ClientID      string    `json:"client_id" db:"client_id"`
	UserID        uint      `json:"user_id" db:"user_id"`
	Scopes        []string  `json:"scopes" db:"scopes"`
	ExpiresAt     time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	Used          bool      `json:"used" db:"used"`
}

// TokenResponse represents an OAuth2 token response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// ErrorResponse represents an OAuth2 error response
type ErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
	ErrorURI         string `json:"error_uri,omitempty"`
	State            string `json:"state,omitempty"`
}

// Scope represents OAuth2 scopes
type Scope string

const (
	ScopeRead         Scope = "read"
	ScopeWrite        Scope = "write"
	ScopeAdmin        Scope = "admin"
	ScopeTickets      Scope = "tickets"
	ScopeQueues       Scope = "queues"
	ScopeUsers        Scope = "users"
	ScopeReports      Scope = "reports"
	ScopeWebhooks     Scope = "webhooks"
	ScopeProfile      Scope = "profile"
	ScopeEmail        Scope = "email"
	ScopeOfflineAccess Scope = "offline_access" // for refresh tokens
)

// Provider implements OAuth2 authorization server
type Provider struct {
	clientRepo        ClientRepository
	codeRepo          AuthorizationCodeRepository
	accessTokenRepo   AccessTokenRepository
	refreshTokenRepo  RefreshTokenRepository
	
	// Configuration
	issuer             string
	authorizationURL   string
	tokenURL           string
	introspectionURL   string
	revocationURL      string
	
	// Token settings
	accessTokenTTL     time.Duration
	refreshTokenTTL    time.Duration
	authorizationCodeTTL time.Duration
}

// Repository interfaces
type ClientRepository interface {
	Create(client *Client) error
	GetByID(id string) (*Client, error)
	GetByCredentials(id, secret string) (*Client, error)
	List() ([]*Client, error)
	Update(client *Client) error
	Delete(id string) error
}

type AuthorizationCodeRepository interface {
	Create(code *AuthorizationCode) error
	GetByCode(code string) (*AuthorizationCode, error)
	MarkUsed(code string) error
	CleanupExpired() error
}

type AccessTokenRepository interface {
	Create(token *AccessToken) error
	GetByToken(token string) (*AccessToken, error)
	Update(token *AccessToken) error
	Revoke(token string) error
	CleanupExpired() error
}

type RefreshTokenRepository interface {
	Create(token *RefreshToken) error
	GetByToken(token string) (*RefreshToken, error)
	MarkUsed(token string) error
	CleanupExpired() error
}

// NewProvider creates a new OAuth2 provider
func NewProvider(
	clientRepo ClientRepository,
	codeRepo AuthorizationCodeRepository,
	accessTokenRepo AccessTokenRepository,
	refreshTokenRepo RefreshTokenRepository,
	issuer string,
) *Provider {
	return &Provider{
		clientRepo:        clientRepo,
		codeRepo:          codeRepo,
		accessTokenRepo:   accessTokenRepo,
		refreshTokenRepo:  refreshTokenRepo,
		issuer:            issuer,
		authorizationURL:  issuer + "/oauth2/authorize",
		tokenURL:          issuer + "/oauth2/token",
		introspectionURL:  issuer + "/oauth2/introspect",
		revocationURL:     issuer + "/oauth2/revoke",
		accessTokenTTL:    1 * time.Hour,
		refreshTokenTTL:   24 * time.Hour,
		authorizationCodeTTL: 10 * time.Minute,
	}
}

// SetupOAuth2Routes sets up OAuth2 endpoints
func (p *Provider) SetupOAuth2Routes(r *gin.Engine) {
	oauth2 := r.Group("/oauth2")
	{
		// Authorization endpoint
		oauth2.GET("/authorize", p.handleAuthorize)
		oauth2.POST("/authorize", p.handleAuthorizePost)
		
		// Token endpoint
		oauth2.POST("/token", p.handleToken)
		
		// Token introspection
		oauth2.POST("/introspect", p.handleIntrospect)
		
		// Token revocation
		oauth2.POST("/revoke", p.handleRevoke)
		
		// OpenID Connect discovery
		oauth2.GET("/.well-known/openid_configuration", p.handleDiscovery)
		
		// JWKS endpoint (for JWT tokens)
		oauth2.GET("/jwks", p.handleJWKS)
	}
	
	// Client management endpoints (admin only)
	admin := r.Group("/admin/oauth2/clients")
	// TODO: Add admin middleware
	{
		admin.GET("", p.handleListClients)
		admin.POST("", p.handleCreateClient)
		admin.GET("/:id", p.handleGetClient)
		admin.PUT("/:id", p.handleUpdateClient)
		admin.DELETE("/:id", p.handleDeleteClient)
		admin.POST("/:id/regenerate-secret", p.handleRegenerateSecret)
	}
}

// handleAuthorize handles the authorization endpoint
func (p *Provider) handleAuthorize(c *gin.Context) {
	// Parse parameters
	clientID := c.Query("client_id")
	redirectURI := c.Query("redirect_uri")
	responseType := c.Query("response_type")
	scope := c.Query("scope")
	state := c.Query("state")
	codeChallenge := c.Query("code_challenge")
	codeChallengeMethod := c.Query("code_challenge_method")
	
	// Validate required parameters
	if clientID == "" || redirectURI == "" || responseType == "" {
		p.sendError(c, "invalid_request", "Missing required parameters", "", state)
		return
	}
	
	// Validate response type
	if ResponseType(responseType) != ResponseTypeCode {
		p.sendError(c, "unsupported_response_type", "Only 'code' response type is supported", redirectURI, state)
		return
	}
	
	// Validate client
	client, err := p.clientRepo.GetByID(clientID)
	if err != nil || !client.IsActive {
		p.sendError(c, "invalid_client", "Client not found or inactive", redirectURI, state)
		return
	}
	
	// Validate redirect URI
	if !p.isValidRedirectURI(client, redirectURI) {
		p.sendError(c, "invalid_request", "Invalid redirect URI", "", state)
		return
	}
	
	// Validate scopes
	requestedScopes := strings.Split(scope, " ")
	if !p.isValidScopes(client, requestedScopes) {
		p.sendError(c, "invalid_scope", "Invalid or unauthorized scope", redirectURI, state)
		return
	}
	
	// Check if user is authenticated
	userID, _, _, authenticated := p.getCurrentUser(c)
	if !authenticated {
		// Redirect to login with return URL
		loginURL := fmt.Sprintf("/login?return_to=%s", url.QueryEscape(c.Request.URL.String()))
		c.Redirect(http.StatusSeeOther, loginURL)
		return
	}
	
	// Show consent form (for now, auto-approve)
	// TODO: Implement proper consent UI
	p.handleAuthorizePost(c)
}

// handleAuthorizePost handles consent form submission
func (p *Provider) handleAuthorizePost(c *gin.Context) {
	clientID := c.Query("client_id")
	redirectURI := c.Query("redirect_uri")
	scope := c.Query("scope")
	state := c.Query("state")
	codeChallenge := c.Query("code_challenge")
	codeChallengeMethod := c.Query("code_challenge_method")
	
	userID, _, _, authenticated := p.getCurrentUser(c)
	if !authenticated {
		p.sendError(c, "access_denied", "User not authenticated", redirectURI, state)
		return
	}
	
	// Generate authorization code
	code := p.generateCode()
	authCode := &AuthorizationCode{
		Code:        code,
		ClientID:    clientID,
		UserID:      userID,
		RedirectURI: redirectURI,
		Scopes:      strings.Split(scope, " "),
		CodeChallenge: codeChallenge,
		CodeChallengeMethod: codeChallengeMethod,
		ExpiresAt:   time.Now().Add(p.authorizationCodeTTL),
		CreatedAt:   time.Now(),
	}
	
	err := p.codeRepo.Create(authCode)
	if err != nil {
		p.sendError(c, "server_error", "Failed to create authorization code", redirectURI, state)
		return
	}
	
	// Redirect back to client with authorization code
	redirectURL, _ := url.Parse(redirectURI)
	query := redirectURL.Query()
	query.Set("code", code)
	if state != "" {
		query.Set("state", state)
	}
	redirectURL.RawQuery = query.Encode()
	
	c.Redirect(http.StatusSeeOther, redirectURL.String())
}

// handleToken handles the token endpoint
func (p *Provider) handleToken(c *gin.Context) {
	grantType := c.PostForm("grant_type")
	
	switch GrantType(grantType) {
	case GrantTypeAuthorizationCode:
		p.handleAuthorizationCodeGrant(c)
	case GrantTypeClientCredentials:
		p.handleClientCredentialsGrant(c)
	case GrantTypeRefreshToken:
		p.handleRefreshTokenGrant(c)
	default:
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:            "unsupported_grant_type",
			ErrorDescription: "Unsupported grant type",
		})
	}
}

// handleAuthorizationCodeGrant handles authorization code grant
func (p *Provider) handleAuthorizationCodeGrant(c *gin.Context) {
	code := c.PostForm("code")
	clientID := c.PostForm("client_id")
	clientSecret := c.PostForm("client_secret")
	redirectURI := c.PostForm("redirect_uri")
	codeVerifier := c.PostForm("code_verifier")
	
	// Validate client credentials
	client, err := p.authenticateClient(clientID, clientSecret)
	if err != nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:            "invalid_client",
			ErrorDescription: "Client authentication failed",
		})
		return
	}
	
	// Get authorization code
	authCode, err := p.codeRepo.GetByCode(code)
	if err != nil || authCode.Used || time.Now().After(authCode.ExpiresAt) {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:            "invalid_grant",
			ErrorDescription: "Invalid or expired authorization code",
		})
		return
	}
	
	// Validate redirect URI matches
	if authCode.RedirectURI != redirectURI {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:            "invalid_grant",
			ErrorDescription: "Redirect URI mismatch",
		})
		return
	}
	
	// Validate PKCE if used
	if authCode.CodeChallenge != "" {
		if !p.validatePKCE(authCode.CodeChallenge, authCode.CodeChallengeMethod, codeVerifier) {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:            "invalid_grant",
				ErrorDescription: "Invalid code verifier",
			})
			return
		}
	}
	
	// Mark code as used
	p.codeRepo.MarkUsed(code)
	
	// Generate tokens
	accessToken := p.generateToken()
	refreshToken := p.generateToken()
	
	// Create access token record
	accessTokenRecord := &AccessToken{
		Token:     accessToken,
		ClientID:  client.ID,
		UserID:    &authCode.UserID,
		Scopes:    authCode.Scopes,
		ExpiresAt: time.Now().Add(p.accessTokenTTL),
		CreatedAt: time.Now(),
		IsActive:  true,
	}
	
	err = p.accessTokenRepo.Create(accessTokenRecord)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:            "server_error",
			ErrorDescription: "Failed to create access token",
		})
		return
	}
	
	// Create refresh token if offline_access scope is present
	var refreshTokenStr string
	if p.hasOfflineAccess(authCode.Scopes) {
		refreshTokenRecord := &RefreshToken{
			Token:         refreshToken,
			AccessTokenID: accessToken,
			ClientID:      client.ID,
			UserID:        authCode.UserID,
			Scopes:        authCode.Scopes,
			ExpiresAt:     time.Now().Add(p.refreshTokenTTL),
			CreatedAt:     time.Now(),
		}
		
		err = p.refreshTokenRepo.Create(refreshTokenRecord)
		if err == nil {
			refreshTokenStr = refreshToken
		}
	}
	
	// Return token response
	response := TokenResponse{
		AccessToken:  accessToken,
		TokenType:    string(TokenTypeBearer),
		ExpiresIn:    int(p.accessTokenTTL.Seconds()),
		RefreshToken: refreshTokenStr,
		Scope:        strings.Join(authCode.Scopes, " "),
	}
	
	c.JSON(http.StatusOK, response)
}

// handleClientCredentialsGrant handles client credentials grant
func (p *Provider) handleClientCredentialsGrant(c *gin.Context) {
	clientID := c.PostForm("client_id")
	clientSecret := c.PostForm("client_secret")
	scope := c.PostForm("scope")
	
	// Authenticate client
	client, err := p.authenticateClient(clientID, clientSecret)
	if err != nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:            "invalid_client",
			ErrorDescription: "Client authentication failed",
		})
		return
	}
	
	// Validate scopes
	requestedScopes := strings.Split(scope, " ")
	if !p.isValidScopes(client, requestedScopes) {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:            "invalid_scope",
			ErrorDescription: "Invalid or unauthorized scope",
		})
		return
	}
	
	// Generate access token
	accessToken := p.generateToken()
	
	accessTokenRecord := &AccessToken{
		Token:     accessToken,
		ClientID:  client.ID,
		UserID:    nil, // No user for client credentials
		Scopes:    requestedScopes,
		ExpiresAt: time.Now().Add(p.accessTokenTTL),
		CreatedAt: time.Now(),
		IsActive:  true,
	}
	
	err = p.accessTokenRepo.Create(accessTokenRecord)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:            "server_error",
			ErrorDescription: "Failed to create access token",
		})
		return
	}
	
	response := TokenResponse{
		AccessToken: accessToken,
		TokenType:   string(TokenTypeBearer),
		ExpiresIn:   int(p.accessTokenTTL.Seconds()),
		Scope:       scope,
	}
	
	c.JSON(http.StatusOK, response)
}

// handleRefreshTokenGrant handles refresh token grant
func (p *Provider) handleRefreshTokenGrant(c *gin.Context) {
	refreshTokenStr := c.PostForm("refresh_token")
	clientID := c.PostForm("client_id")
	clientSecret := c.PostForm("client_secret")
	
	// Authenticate client
	client, err := p.authenticateClient(clientID, clientSecret)
	if err != nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:            "invalid_client",
			ErrorDescription: "Client authentication failed",
		})
		return
	}
	
	// Get refresh token
	refreshTokenRecord, err := p.refreshTokenRepo.GetByToken(refreshTokenStr)
	if err != nil || refreshTokenRecord.Used || time.Now().After(refreshTokenRecord.ExpiresAt) {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:            "invalid_grant",
			ErrorDescription: "Invalid or expired refresh token",
		})
		return
	}
	
	// Validate client matches
	if refreshTokenRecord.ClientID != client.ID {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:            "invalid_grant",
			ErrorDescription: "Client mismatch",
		})
		return
	}
	
	// Mark refresh token as used
	p.refreshTokenRepo.MarkUsed(refreshTokenStr)
	
	// Generate new tokens
	accessToken := p.generateToken()
	newRefreshToken := p.generateToken()
	
	// Create new access token
	accessTokenRecord := &AccessToken{
		Token:     accessToken,
		ClientID:  client.ID,
		UserID:    &refreshTokenRecord.UserID,
		Scopes:    refreshTokenRecord.Scopes,
		ExpiresAt: time.Now().Add(p.accessTokenTTL),
		CreatedAt: time.Now(),
		IsActive:  true,
	}
	
	err = p.accessTokenRepo.Create(accessTokenRecord)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:            "server_error",
			ErrorDescription: "Failed to create access token",
		})
		return
	}
	
	// Create new refresh token
	newRefreshTokenRecord := &RefreshToken{
		Token:         newRefreshToken,
		AccessTokenID: accessToken,
		ClientID:      client.ID,
		UserID:        refreshTokenRecord.UserID,
		Scopes:        refreshTokenRecord.Scopes,
		ExpiresAt:     time.Now().Add(p.refreshTokenTTL),
		CreatedAt:     time.Now(),
	}
	
	p.refreshTokenRepo.Create(newRefreshTokenRecord)
	
	response := TokenResponse{
		AccessToken:  accessToken,
		TokenType:    string(TokenTypeBearer),
		ExpiresIn:    int(p.accessTokenTTL.Seconds()),
		RefreshToken: newRefreshToken,
		Scope:        strings.Join(refreshTokenRecord.Scopes, " "),
	}
	
	c.JSON(http.StatusOK, response)
}

// Helper methods

func (p *Provider) authenticateClient(clientID, clientSecret string) (*Client, error) {
	if clientID == "" {
		return nil, fmt.Errorf("missing client_id")
	}
	
	client, err := p.clientRepo.GetByID(clientID)
	if err != nil {
		return nil, err
	}
	
	if !client.IsActive {
		return nil, fmt.Errorf("client is inactive")
	}
	
	// For confidential clients, verify secret
	if client.IsConfidential {
		if clientSecret == "" || client.Secret != clientSecret {
			return nil, fmt.Errorf("invalid client secret")
		}
	}
	
	return client, nil
}

func (p *Provider) isValidRedirectURI(client *Client, redirectURI string) bool {
	for _, uri := range client.RedirectURIs {
		if uri == redirectURI {
			return true
		}
	}
	return false
}

func (p *Provider) isValidScopes(client *Client, requestedScopes []string) bool {
	for _, requested := range requestedScopes {
		if requested == "" {
			continue
		}
		
		found := false
		for _, allowed := range client.Scopes {
			if allowed == requested {
				found = true
				break
			}
		}
		
		if !found {
			return false
		}
	}
	
	return true
}

func (p *Provider) hasOfflineAccess(scopes []string) bool {
	for _, scope := range scopes {
		if scope == string(ScopeOfflineAccess) {
			return true
		}
	}
	return false
}

func (p *Provider) generateCode() string {
	return p.generateToken()
}

func (p *Provider) generateToken() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return base64.URLEncoding.EncodeToString(bytes)
}

func (p *Provider) validatePKCE(challenge, method, verifier string) bool {
	// TODO: Implement PKCE validation
	// For now, just check if verifier is provided when challenge exists
	return verifier != ""
}

func (p *Provider) getCurrentUser(c *gin.Context) (uint, string, string, bool) {
	// TODO: Integrate with existing middleware
	// For now, return mock data
	return 1, "demo@example.com", "Admin", true
}

func (p *Provider) sendError(c *gin.Context, error, description, redirectURI, state string) {
	if redirectURI != "" {
		// Redirect error to client
		redirectURL, _ := url.Parse(redirectURI)
		query := redirectURL.Query()
		query.Set("error", error)
		if description != "" {
			query.Set("error_description", description)
		}
		if state != "" {
			query.Set("state", state)
		}
		redirectURL.RawQuery = query.Encode()
		c.Redirect(http.StatusSeeOther, redirectURL.String())
	} else {
		// Return JSON error
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:            error,
			ErrorDescription: description,
			State:            state,
		})
	}
}

// Stub implementations for remaining handlers

func (p *Provider) handleIntrospect(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"active": false,
		"message": "Token introspection not yet implemented",
	})
}

func (p *Provider) handleRevoke(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Token revocation not yet implemented",
	})
}

func (p *Provider) handleDiscovery(c *gin.Context) {
	discovery := gin.H{
		"issuer":                 p.issuer,
		"authorization_endpoint": p.authorizationURL,
		"token_endpoint":         p.tokenURL,
		"introspection_endpoint": p.introspectionURL,
		"revocation_endpoint":    p.revocationURL,
		"jwks_uri":               p.issuer + "/oauth2/jwks",
		"response_types_supported": []string{"code"},
		"grant_types_supported": []string{
			string(GrantTypeAuthorizationCode),
			string(GrantTypeClientCredentials),
			string(GrantTypeRefreshToken),
		},
		"token_endpoint_auth_methods_supported": []string{
			"client_secret_post",
			"client_secret_basic",
		},
		"scopes_supported": []string{
			string(ScopeRead),
			string(ScopeWrite),
			string(ScopeAdmin),
			string(ScopeTickets),
			string(ScopeQueues),
			string(ScopeUsers),
			string(ScopeReports),
			string(ScopeWebhooks),
			string(ScopeProfile),
			string(ScopeEmail),
			string(ScopeOfflineAccess),
		},
	}
	
	c.JSON(http.StatusOK, discovery)
}

func (p *Provider) handleJWKS(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"keys": []interface{}{},
		"message": "JWKS endpoint not yet implemented",
	})
}

// Client management handlers (stubs)

func (p *Provider) handleListClients(c *gin.Context) {
	clients, err := p.clientRepo.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"clients": clients})
}

func (p *Provider) handleCreateClient(c *gin.Context) {
	var req struct {
		Name         string      `json:"name" binding:"required"`
		Description  string      `json:"description"`
		RedirectURIs []string    `json:"redirect_uris" binding:"required"`
		Scopes       []string    `json:"scopes"`
		GrantTypes   []GrantType `json:"grant_types"`
		IsConfidential bool      `json:"is_confidential"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	client := &Client{
		ID:           uuid.New().String(),
		Secret:       p.generateToken(),
		Name:         req.Name,
		Description:  req.Description,
		RedirectURIs: req.RedirectURIs,
		Scopes:       req.Scopes,
		GrantTypes:   req.GrantTypes,
		IsActive:     true,
		IsConfidential: req.IsConfidential,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	
	err := p.clientRepo.Create(client)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusCreated, client)
}

func (p *Provider) handleGetClient(c *gin.Context) {
	clientID := c.Param("id")
	client, err := p.clientRepo.GetByID(clientID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Client not found"})
		return
	}
	c.JSON(http.StatusOK, client)
}

func (p *Provider) handleUpdateClient(c *gin.Context) {
	clientID := c.Param("id")
	
	client, err := p.clientRepo.GetByID(clientID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Client not found"})
		return
	}
	
	var req struct {
		Name         string      `json:"name"`
		Description  string      `json:"description"`
		RedirectURIs []string    `json:"redirect_uris"`
		Scopes       []string    `json:"scopes"`
		GrantTypes   []GrantType `json:"grant_types"`
		IsActive     *bool       `json:"is_active"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Update fields
	if req.Name != "" {
		client.Name = req.Name
	}
	client.Description = req.Description
	if len(req.RedirectURIs) > 0 {
		client.RedirectURIs = req.RedirectURIs
	}
	if len(req.Scopes) > 0 {
		client.Scopes = req.Scopes
	}
	if len(req.GrantTypes) > 0 {
		client.GrantTypes = req.GrantTypes
	}
	if req.IsActive != nil {
		client.IsActive = *req.IsActive
	}
	client.UpdatedAt = time.Now()
	
	err = p.clientRepo.Update(client)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, client)
}

func (p *Provider) handleDeleteClient(c *gin.Context) {
	clientID := c.Param("id")
	
	err := p.clientRepo.Delete(clientID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "Client deleted successfully"})
}

func (p *Provider) handleRegenerateSecret(c *gin.Context) {
	clientID := c.Param("id")
	
	client, err := p.clientRepo.GetByID(clientID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Client not found"})
		return
	}
	
	client.Secret = p.generateToken()
	client.UpdatedAt = time.Now()
	
	err = p.clientRepo.Update(client)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"client_id": client.ID,
		"client_secret": client.Secret,
		"message": "Secret regenerated successfully",
	})
}