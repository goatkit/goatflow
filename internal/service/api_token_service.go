package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
)

// APITokenService handles API token operations
type APITokenService struct {
	repo *repository.APITokenRepository
}

// NewAPITokenService creates a new API token service
func NewAPITokenService(db *sql.DB) *APITokenService {
	return &APITokenService{
		repo: repository.NewAPITokenRepository(db),
	}
}

// GenerateToken creates a new API token for a user
func (s *APITokenService) GenerateToken(ctx context.Context, req *models.APITokenCreateRequest, userID int, userType models.APITokenUserType, createdBy int) (*models.APITokenCreateResponse, error) {
	// Validate scopes
	if err := s.validateScopes(req.Scopes, userType); err != nil {
		return nil, err
	}

	// Generate random token
	randomBytes := make([]byte, models.TokenRandomLength)
	if _, err := rand.Read(randomBytes); err != nil {
		return nil, fmt.Errorf("generate random: %w", err)
	}
	randomPart := hex.EncodeToString(randomBytes)

	// Extract prefix (first 8 chars of random part)
	prefix := randomPart[:models.TokenPrefixLength]

	// Full token: gf_<prefix>_<remaining>
	fullToken := fmt.Sprintf("%s%s_%s", models.TokenPrefix, prefix, randomPart[models.TokenPrefixLength:])

	// Hash the full token for storage
	hash, err := bcrypt.GenerateFromPassword([]byte(fullToken), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash token: %w", err)
	}

	// Calculate expiration
	var expiresAt sql.NullTime
	if req.ExpiresIn != "" && req.ExpiresIn != "never" {
		duration, err := parseExpiration(req.ExpiresIn)
		if err != nil {
			return nil, fmt.Errorf("invalid expiration: %w", err)
		}
		expiresAt = sql.NullTime{Time: time.Now().Add(duration), Valid: true}
	}

	// Create token record
	token := &models.APIToken{
		UserID:    userID,
		UserType:  userType,
		Name:      req.Name,
		Prefix:    prefix,
		TokenHash: string(hash),
		Scopes:    req.Scopes,
		ExpiresAt: expiresAt,
		RateLimit: models.DefaultRateLimit,
		CreatedAt: time.Now(),
		CreatedBy: sql.NullInt64{Int64: int64(createdBy), Valid: createdBy > 0},
	}

	// Insert into database
	id, err := s.repo.Create(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("create token: %w", err)
	}

	// Build response
	resp := &models.APITokenCreateResponse{
		ID:        id,
		Name:      req.Name,
		Prefix:    prefix,
		Token:     fullToken,
		Scopes:    req.Scopes,
		CreatedAt: token.CreatedAt,
		Warning:   "Save this token now. It won't be shown again.",
	}

	if expiresAt.Valid {
		exp := expiresAt.Time.Format(time.RFC3339)
		resp.ExpiresAt = &exp
	}

	return resp, nil
}

// VerifyToken verifies an API token and returns the token record if valid
func (s *APITokenService) VerifyToken(ctx context.Context, rawToken string) (*models.APIToken, error) {
	log.Printf("DEBUG VerifyToken: rawToken length=%d, starts=%q", len(rawToken), rawToken[:min(20, len(rawToken))])

	// Validate format: gf_<prefix>_<random>
	if !strings.HasPrefix(rawToken, models.TokenPrefix) {
		log.Printf("DEBUG VerifyToken: missing gf_ prefix")
		return nil, fmt.Errorf("invalid token format")
	}

	// Extract prefix (8 chars after "gf_")
	tokenPart := rawToken[len(models.TokenPrefix):]
	if len(tokenPart) < models.TokenPrefixLength+1 {
		log.Printf("DEBUG VerifyToken: token too short")
		return nil, fmt.Errorf("invalid token format")
	}

	prefix := tokenPart[:models.TokenPrefixLength]
	log.Printf("DEBUG VerifyToken: prefix=%s", prefix)

	// Look up tokens by prefix
	tokens, err := s.repo.GetByPrefix(ctx, prefix)
	if err != nil {
		log.Printf("DEBUG VerifyToken: lookup error: %v", err)
		return nil, fmt.Errorf("lookup token: %w", err)
	}

	log.Printf("DEBUG VerifyToken: found %d tokens with prefix %s", len(tokens), prefix)

	if len(tokens) == 0 {
		return nil, fmt.Errorf("token not found")
	}

	// Verify against each matching token (usually just one)
	for _, token := range tokens {
		log.Printf("DEBUG VerifyToken: comparing against token ID=%d, hash length=%d", token.ID, len(token.TokenHash))
		if err := bcrypt.CompareHashAndPassword([]byte(token.TokenHash), []byte(rawToken)); err == nil {
			log.Printf("DEBUG VerifyToken: bcrypt match for token ID=%d", token.ID)
			// Check if active
			if !token.IsActive() {
				if token.IsRevoked() {
					return nil, fmt.Errorf("token revoked")
				}
				if token.IsExpired() {
					return nil, fmt.Errorf("token expired")
				}
			}
			return token, nil
		} else {
			log.Printf("DEBUG VerifyToken: bcrypt mismatch for token ID=%d: %v", token.ID, err)
		}
	}

	return nil, fmt.Errorf("invalid token")
}

// ListUserTokens returns all tokens for a user
func (s *APITokenService) ListUserTokens(ctx context.Context, userID int, userType models.APITokenUserType) ([]*models.APITokenListItem, error) {
	tokens, err := s.repo.ListByUser(ctx, userID, userType)
	if err != nil {
		return nil, err
	}

	items := make([]*models.APITokenListItem, 0, len(tokens))
	for _, t := range tokens {
		item := &models.APITokenListItem{
			ID:        t.ID,
			Name:      t.Name,
			Prefix:    t.Prefix,
			Scopes:    t.Scopes,
			CreatedAt: t.CreatedAt.Format(time.RFC3339),
			IsActive:  t.IsActive(),
		}
		if t.ExpiresAt.Valid {
			exp := t.ExpiresAt.Time.Format(time.RFC3339)
			item.ExpiresAt = &exp
		}
		if t.LastUsedAt.Valid {
			lu := t.LastUsedAt.Time.Format(time.RFC3339)
			item.LastUsedAt = &lu
		}
		items = append(items, item)
	}

	return items, nil
}

// GenerateTokenForUser creates a token for a specific user (admin use)
// This is used when an admin creates a token on behalf of another user.
// The adminID is recorded as created_by for audit purposes.
func (s *APITokenService) GenerateTokenForUser(ctx context.Context, req *models.APITokenCreateRequest, targetUserID int, userType models.APITokenUserType, tenantID int, adminID int) (*models.APITokenCreateResponse, error) {
	// Use the existing GenerateToken logic but with adminID as creator
	return s.GenerateToken(ctx, req, targetUserID, userType, adminID)
}

// GetToken returns a token by ID (for admin verification)
func (s *APITokenService) GetToken(ctx context.Context, tokenID int64) (*models.APIToken, error) {
	return s.repo.GetByID(ctx, tokenID)
}

// RevokeToken revokes a token by ID
func (s *APITokenService) RevokeToken(ctx context.Context, tokenID int64, userID int, userType models.APITokenUserType, revokedBy int) error {
	// Get token to verify ownership
	token, err := s.repo.GetByID(ctx, tokenID)
	if err != nil {
		return fmt.Errorf("get token: %w", err)
	}
	if token == nil {
		return fmt.Errorf("token not found")
	}

	// Verify ownership (unless admin)
	if token.UserID != userID || token.UserType != userType {
		return fmt.Errorf("token not found") // Don't reveal existence
	}

	return s.repo.Revoke(ctx, tokenID, revokedBy)
}

// RevokeTokenAdmin revokes any token (admin only)
func (s *APITokenService) RevokeTokenAdmin(ctx context.Context, tokenID int64, revokedBy int) error {
	return s.repo.Revoke(ctx, tokenID, revokedBy)
}

// UpdateLastUsed updates the last used timestamp
func (s *APITokenService) UpdateLastUsed(ctx context.Context, tokenID int64, ip string) error {
	return s.repo.UpdateLastUsed(ctx, tokenID, ip)
}

// ListAllTokens returns all tokens (admin only)
func (s *APITokenService) ListAllTokens(ctx context.Context, includeRevoked bool) ([]*models.APIToken, error) {
	return s.repo.ListAll(ctx, includeRevoked)
}

// validateScopes validates that scopes are valid and appropriate for user type
func (s *APITokenService) validateScopes(scopes []string, userType models.APITokenUserType) error {
	for _, scope := range scopes {
		if _, ok := models.ValidScopes[scope]; !ok {
			return fmt.Errorf("invalid scope: %s", scope)
		}

		// Customers can't have admin scopes
		if userType == models.APITokenUserCustomer && strings.HasPrefix(scope, "admin:") {
			return fmt.Errorf("customers cannot have admin scopes")
		}
	}
	return nil
}

// parseExpiration parses expiration strings like "30d", "90d", "1y"
func parseExpiration(exp string) (time.Duration, error) {
	exp = strings.ToLower(strings.TrimSpace(exp))

	if exp == "never" || exp == "" {
		return 0, fmt.Errorf("never does not have a duration")
	}

	var multiplier time.Duration
	var value int

	if strings.HasSuffix(exp, "d") {
		multiplier = 24 * time.Hour
		fmt.Sscanf(exp, "%dd", &value)
	} else if strings.HasSuffix(exp, "y") {
		multiplier = 365 * 24 * time.Hour
		fmt.Sscanf(exp, "%dy", &value)
	} else if strings.HasSuffix(exp, "m") {
		multiplier = 30 * 24 * time.Hour // Approximate month
		fmt.Sscanf(exp, "%dm", &value)
	} else {
		return 0, fmt.Errorf("invalid expiration format: %s (use 30d, 90d, 1y, etc.)", exp)
	}

	if value <= 0 {
		return 0, fmt.Errorf("expiration must be positive")
	}

	return time.Duration(value) * multiplier, nil
}
