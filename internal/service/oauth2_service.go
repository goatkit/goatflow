package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
)

// OAuth2Service handles OAuth2 authorization and token management
type OAuth2Service struct {
	clientRepo     repository.OAuth2ClientRepository
	tokenRepo      repository.OAuth2TokenRepository
	userRepo       repository.UserRepository
	jwtSecret      []byte
	mu             sync.RWMutex
	authCodes      map[string]*AuthorizationCode
	accessTokens   map[string]*OAuth2Token
	refreshTokens  map[string]*OAuth2Token
}

// OAuth2Client represents an OAuth2 client application
type OAuth2Client struct {
	ID           string    `json:"client_id"`
	Secret       string    `json:"client_secret,omitempty"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	RedirectURIs []string  `json:"redirect_uris"`
	GrantTypes   []string  `json:"grant_types"`
	Scopes       []string  `json:"scopes"`
	Confidential bool      `json:"confidential"`
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	CreatedBy    int       `json:"created_by"`
}

// AuthorizationCode represents an OAuth2 authorization code
type AuthorizationCode struct {
	Code         string    `json:"code"`
	ClientID     string    `json:"client_id"`
	UserID       int       `json:"user_id"`
	RedirectURI  string    `json:"redirect_uri"`
	Scope        string    `json:"scope"`
	State        string    `json:"state"`
	CodeChallenge string   `json:"code_challenge,omitempty"`
	CodeChallengeMethod string `json:"code_challenge_method,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	UsedAt       *time.Time `json:"used_at,omitempty"`
}

// OAuth2Token represents an OAuth2 token
type OAuth2Token struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresIn    int       `json:"expires_in"`
	Scope        string    `json:"scope"`
	ClientID     string    `json:"client_id"`
	UserID       int       `json:"user_id"`
	IssuedAt     time.Time `json:"issued_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	RevokedAt    *time.Time `json:"revoked_at,omitempty"`
}

// AuthorizationRequest represents an OAuth2 authorization request
type AuthorizationRequest struct {
	ResponseType string `json:"response_type"`
	ClientID     string `json:"client_id"`
	RedirectURI  string `json:"redirect_uri"`
	Scope        string `json:"scope"`
	State        string `json:"state"`
	CodeChallenge string `json:"code_challenge,omitempty"`
	CodeChallengeMethod string `json:"code_challenge_method,omitempty"`
}

// TokenRequest represents an OAuth2 token request
type TokenRequest struct {
	GrantType    string `json:"grant_type"`
	Code         string `json:"code,omitempty"`
	RedirectURI  string `json:"redirect_uri,omitempty"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
	Username     string `json:"username,omitempty"`
	Password     string `json:"password,omitempty"`
	CodeVerifier string `json:"code_verifier,omitempty"`
}

// TokenResponse represents an OAuth2 token response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
}

// OpenIDConfiguration represents OpenID Connect discovery document
type OpenIDConfiguration struct {
	Issuer                 string   `json:"issuer"`
	AuthorizationEndpoint  string   `json:"authorization_endpoint"`
	TokenEndpoint          string   `json:"token_endpoint"`
	UserInfoEndpoint       string   `json:"userinfo_endpoint"`
	JWKSUri                string   `json:"jwks_uri"`
	RegistrationEndpoint   string   `json:"registration_endpoint,omitempty"`
	ScopesSupported        []string `json:"scopes_supported"`
	ResponseTypesSupported []string `json:"response_types_supported"`
	GrantTypesSupported    []string `json:"grant_types_supported"`
	SubjectTypesSupported  []string `json:"subject_types_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
	ClaimsSupported        []string `json:"claims_supported"`
	CodeChallengeMethodsSupported []string `json:"code_challenge_methods_supported"`
}

// UserInfo represents OpenID Connect UserInfo
type UserInfo struct {
	Sub           string `json:"sub"`
	Name          string `json:"name"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Picture       string `json:"picture,omitempty"`
	Locale        string `json:"locale,omitempty"`
}

// NewOAuth2Service creates a new OAuth2 service
func NewOAuth2Service(clientRepo repository.OAuth2ClientRepository, tokenRepo repository.OAuth2TokenRepository, userRepo repository.UserRepository, jwtSecret string) *OAuth2Service {
	return &OAuth2Service{
		clientRepo:    clientRepo,
		tokenRepo:     tokenRepo,
		userRepo:      userRepo,
		jwtSecret:     []byte(jwtSecret),
		authCodes:     make(map[string]*AuthorizationCode),
		accessTokens:  make(map[string]*OAuth2Token),
		refreshTokens: make(map[string]*OAuth2Token),
	}
}

// CreateClient creates a new OAuth2 client
func (s *OAuth2Service) CreateClient(client *OAuth2Client) error {
	// Generate client ID
	client.ID = s.generateClientID()
	
	// Generate client secret for confidential clients
	if client.Confidential {
		client.Secret = s.generateClientSecret()
	}
	
	// Set defaults
	if len(client.GrantTypes) == 0 {
		client.GrantTypes = []string{"authorization_code", "refresh_token"}
	}
	
	if len(client.Scopes) == 0 {
		client.Scopes = []string{"read", "write"}
	}
	
	client.Active = true
	client.CreatedAt = time.Now()
	client.UpdatedAt = time.Now()
	
	// Save to repository
	return s.clientRepo.Create(client)
}

// GetClient gets an OAuth2 client
func (s *OAuth2Service) GetClient(clientID string) (*OAuth2Client, error) {
	return s.clientRepo.GetByID(clientID)
}

// ValidateAuthorizationRequest validates an authorization request
func (s *OAuth2Service) ValidateAuthorizationRequest(req *AuthorizationRequest) error {
	// Validate response type
	if req.ResponseType != "code" && req.ResponseType != "token" {
		return fmt.Errorf("unsupported response type: %s", req.ResponseType)
	}
	
	// Get client
	client, err := s.clientRepo.GetByID(req.ClientID)
	if err != nil {
		return fmt.Errorf("invalid client")
	}
	
	if !client.Active {
		return fmt.Errorf("client is inactive")
	}
	
	// Validate redirect URI
	if !s.validateRedirectURI(req.RedirectURI, client.RedirectURIs) {
		return fmt.Errorf("invalid redirect URI")
	}
	
	// Validate scope
	if !s.validateScope(req.Scope, client.Scopes) {
		return fmt.Errorf("invalid scope")
	}
	
	// Validate PKCE if present
	if req.CodeChallenge != "" {
		if req.CodeChallengeMethod != "S256" && req.CodeChallengeMethod != "plain" {
			return fmt.Errorf("unsupported code challenge method")
		}
	}
	
	return nil
}

// GenerateAuthorizationCode generates an authorization code
func (s *OAuth2Service) GenerateAuthorizationCode(req *AuthorizationRequest, userID int) (string, error) {
	code := &AuthorizationCode{
		Code:         s.generateCode(),
		ClientID:     req.ClientID,
		UserID:       userID,
		RedirectURI:  req.RedirectURI,
		Scope:        req.Scope,
		State:        req.State,
		CodeChallenge: req.CodeChallenge,
		CodeChallengeMethod: req.CodeChallengeMethod,
		ExpiresAt:    time.Now().Add(10 * time.Minute),
	}
	
	// Store code
	s.mu.Lock()
	s.authCodes[code.Code] = code
	s.mu.Unlock()
	
	// Schedule cleanup
	time.AfterFunc(10*time.Minute, func() {
		s.mu.Lock()
		delete(s.authCodes, code.Code)
		s.mu.Unlock()
	})
	
	return code.Code, nil
}

// ExchangeAuthorizationCode exchanges an authorization code for tokens
func (s *OAuth2Service) ExchangeAuthorizationCode(req *TokenRequest) (*TokenResponse, error) {
	// Get authorization code
	s.mu.Lock()
	code, exists := s.authCodes[req.Code]
	s.mu.Unlock()
	
	if !exists {
		return nil, fmt.Errorf("invalid authorization code")
	}
	
	// Check if code is expired
	if time.Now().After(code.ExpiresAt) {
		return nil, fmt.Errorf("authorization code expired")
	}
	
	// Check if code was already used
	if code.UsedAt != nil {
		return nil, fmt.Errorf("authorization code already used")
	}
	
	// Validate client
	client, err := s.clientRepo.GetByID(req.ClientID)
	if err != nil {
		return nil, fmt.Errorf("invalid client")
	}
	
	// Validate client secret for confidential clients
	if client.Confidential && client.Secret != req.ClientSecret {
		return nil, fmt.Errorf("invalid client credentials")
	}
	
	// Validate redirect URI
	if code.RedirectURI != req.RedirectURI {
		return nil, fmt.Errorf("redirect URI mismatch")
	}
	
	// Validate PKCE if present
	if code.CodeChallenge != "" {
		if !s.validatePKCE(code.CodeChallenge, code.CodeChallengeMethod, req.CodeVerifier) {
			return nil, fmt.Errorf("invalid PKCE verifier")
		}
	}
	
	// Mark code as used
	now := time.Now()
	code.UsedAt = &now
	
	// Generate tokens
	accessToken, refreshToken, err := s.generateTokens(code.ClientID, code.UserID, code.Scope)
	if err != nil {
		return nil, err
	}
	
	// Delete used code
	s.mu.Lock()
	delete(s.authCodes, req.Code)
	s.mu.Unlock()
	
	return &TokenResponse{
		AccessToken:  accessToken.AccessToken,
		TokenType:    accessToken.TokenType,
		ExpiresIn:    accessToken.ExpiresIn,
		RefreshToken: refreshToken.RefreshToken,
		Scope:        accessToken.Scope,
	}, nil
}

// RefreshAccessToken refreshes an access token
func (s *OAuth2Service) RefreshAccessToken(req *TokenRequest) (*TokenResponse, error) {
	// Get refresh token
	s.mu.RLock()
	refreshToken, exists := s.refreshTokens[req.RefreshToken]
	s.mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("invalid refresh token")
	}
	
	// Check if token is expired
	if time.Now().After(refreshToken.ExpiresAt) {
		return nil, fmt.Errorf("refresh token expired")
	}
	
	// Check if token is revoked
	if refreshToken.RevokedAt != nil {
		return nil, fmt.Errorf("refresh token revoked")
	}
	
	// Validate client
	if refreshToken.ClientID != req.ClientID {
		return nil, fmt.Errorf("client mismatch")
	}
	
	// Generate new access token
	accessToken := s.generateAccessToken(refreshToken.ClientID, refreshToken.UserID, refreshToken.Scope)
	
	return &TokenResponse{
		AccessToken:  accessToken.AccessToken,
		TokenType:    accessToken.TokenType,
		ExpiresIn:    accessToken.ExpiresIn,
		RefreshToken: req.RefreshToken, // Return same refresh token
		Scope:        accessToken.Scope,
	}, nil
}

// ValidateAccessToken validates an access token
func (s *OAuth2Service) ValidateAccessToken(tokenString string) (*OAuth2Token, error) {
	// Check cache first
	s.mu.RLock()
	token, exists := s.accessTokens[tokenString]
	s.mu.RUnlock()
	
	if exists {
		// Check if token is expired
		if time.Now().After(token.ExpiresAt) {
			return nil, fmt.Errorf("token expired")
		}
		
		// Check if token is revoked
		if token.RevokedAt != nil {
			return nil, fmt.Errorf("token revoked")
		}
		
		return token, nil
	}
	
	// Parse JWT token
	jwtToken, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return s.jwtSecret, nil
	})
	
	if err != nil {
		return nil, err
	}
	
	if !jwtToken.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	
	// Extract claims
	claims, ok := jwtToken.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}
	
	// Create token object
	token = &OAuth2Token{
		AccessToken: tokenString,
		TokenType:   "Bearer",
		ClientID:    claims["client_id"].(string),
		UserID:      int(claims["user_id"].(float64)),
		Scope:       claims["scope"].(string),
	}
	
	return token, nil
}

// RevokeToken revokes a token
func (s *OAuth2Service) RevokeToken(token string) error {
	now := time.Now()
	
	// Check if it's an access token
	s.mu.Lock()
	if accessToken, exists := s.accessTokens[token]; exists {
		accessToken.RevokedAt = &now
		s.mu.Unlock()
		return s.tokenRepo.RevokeToken(token)
	}
	
	// Check if it's a refresh token
	if refreshToken, exists := s.refreshTokens[token]; exists {
		refreshToken.RevokedAt = &now
		s.mu.Unlock()
		return s.tokenRepo.RevokeToken(token)
	}
	s.mu.Unlock()
	
	return fmt.Errorf("token not found")
}

// GetUserInfo gets user information for OpenID Connect
func (s *OAuth2Service) GetUserInfo(userID int) (*UserInfo, error) {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, err
	}
	
	return &UserInfo{
		Sub:           fmt.Sprintf("%d", user.ID),
		Name:          user.Name,
		Email:         user.Email,
		EmailVerified: true,
		Picture:       user.Avatar,
		Locale:        user.Locale,
	}, nil
}

// GetOpenIDConfiguration returns OpenID Connect configuration
func (s *OAuth2Service) GetOpenIDConfiguration(baseURL string) *OpenIDConfiguration {
	return &OpenIDConfiguration{
		Issuer:                 baseURL,
		AuthorizationEndpoint:  baseURL + "/api/v1/oauth/authorize",
		TokenEndpoint:          baseURL + "/api/v1/oauth/token",
		UserInfoEndpoint:       baseURL + "/api/v1/oauth/userinfo",
		JWKSUri:                baseURL + "/api/v1/oauth/.well-known/jwks.json",
		RegistrationEndpoint:   baseURL + "/api/v1/oauth/register",
		ScopesSupported:        []string{"openid", "profile", "email", "read", "write"},
		ResponseTypesSupported: []string{"code", "token", "id_token", "code id_token"},
		GrantTypesSupported:    []string{"authorization_code", "refresh_token", "password", "client_credentials"},
		SubjectTypesSupported:  []string{"public"},
		TokenEndpointAuthMethodsSupported: []string{"client_secret_basic", "client_secret_post"},
		ClaimsSupported:        []string{"sub", "name", "email", "email_verified", "picture", "locale"},
		CodeChallengeMethodsSupported: []string{"plain", "S256"},
	}
}

// Helper methods

// generateTokens generates access and refresh tokens
func (s *OAuth2Service) generateTokens(clientID string, userID int, scope string) (*OAuth2Token, *OAuth2Token, error) {
	accessToken := s.generateAccessToken(clientID, userID, scope)
	refreshToken := s.generateRefreshToken(clientID, userID, scope)
	
	// Store tokens
	s.mu.Lock()
	s.accessTokens[accessToken.AccessToken] = accessToken
	s.refreshTokens[refreshToken.RefreshToken] = refreshToken
	s.mu.Unlock()
	
	// Save to repository
	if err := s.tokenRepo.CreateToken(accessToken); err != nil {
		return nil, nil, err
	}
	
	if err := s.tokenRepo.CreateToken(refreshToken); err != nil {
		return nil, nil, err
	}
	
	return accessToken, refreshToken, nil
}

// generateAccessToken generates an access token
func (s *OAuth2Service) generateAccessToken(clientID string, userID int, scope string) *OAuth2Token {
	now := time.Now()
	expiresAt := now.Add(1 * time.Hour)
	
	// Create JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"type":      "access",
		"client_id": clientID,
		"user_id":   userID,
		"scope":     scope,
		"iat":       now.Unix(),
		"exp":       expiresAt.Unix(),
		"jti":       s.generateTokenID(),
	})
	
	tokenString, _ := token.SignedString(s.jwtSecret)
	
	return &OAuth2Token{
		AccessToken: tokenString,
		TokenType:   "Bearer",
		ExpiresIn:   3600,
		Scope:       scope,
		ClientID:    clientID,
		UserID:      userID,
		IssuedAt:    now,
		ExpiresAt:   expiresAt,
	}
}

// generateRefreshToken generates a refresh token
func (s *OAuth2Service) generateRefreshToken(clientID string, userID int, scope string) *OAuth2Token {
	now := time.Now()
	expiresAt := now.Add(30 * 24 * time.Hour) // 30 days
	
	// Create JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"type":      "refresh",
		"client_id": clientID,
		"user_id":   userID,
		"scope":     scope,
		"iat":       now.Unix(),
		"exp":       expiresAt.Unix(),
		"jti":       s.generateTokenID(),
	})
	
	tokenString, _ := token.SignedString(s.jwtSecret)
	
	return &OAuth2Token{
		RefreshToken: tokenString,
		TokenType:    "Bearer",
		Scope:        scope,
		ClientID:     clientID,
		UserID:       userID,
		IssuedAt:     now,
		ExpiresAt:    expiresAt,
	}
}

// validateRedirectURI validates a redirect URI
func (s *OAuth2Service) validateRedirectURI(uri string, allowedURIs []string) bool {
	for _, allowed := range allowedURIs {
		if uri == allowed {
			return true
		}
		
		// Allow wildcard matching for development
		if strings.HasSuffix(allowed, "*") {
			prefix := strings.TrimSuffix(allowed, "*")
			if strings.HasPrefix(uri, prefix) {
				return true
			}
		}
	}
	return false
}

// validateScope validates requested scopes
func (s *OAuth2Service) validateScope(requested string, allowed []string) bool {
	requestedScopes := strings.Split(requested, " ")
	
	for _, scope := range requestedScopes {
		found := false
		for _, allowedScope := range allowed {
			if scope == allowedScope {
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

// validatePKCE validates PKCE challenge
func (s *OAuth2Service) validatePKCE(challenge, method, verifier string) bool {
	switch method {
	case "plain":
		return challenge == verifier
	case "S256":
		h := sha256.Sum256([]byte(verifier))
		computed := base64.RawURLEncoding.EncodeToString(h[:])
		return challenge == computed
	default:
		return false
	}
}

// generateClientID generates a client ID
func (s *OAuth2Service) generateClientID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// generateClientSecret generates a client secret
func (s *OAuth2Service) generateClientSecret() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// generateCode generates an authorization code
func (s *OAuth2Service) generateCode() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// generateTokenID generates a unique token ID
func (s *OAuth2Service) generateTokenID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}