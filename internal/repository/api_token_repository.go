package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/models"
)

// APITokenRepository handles database operations for API tokens
type APITokenRepository struct {
	db *sql.DB
}

// NewAPITokenRepository creates a new API token repository
func NewAPITokenRepository(db *sql.DB) *APITokenRepository {
	return &APITokenRepository{db: db}
}

// Create inserts a new API token
func (r *APITokenRepository) Create(ctx context.Context, token *models.APIToken) (int64, error) {
	var scopesJSON sql.NullString
	if len(token.Scopes) > 0 {
		data, err := json.Marshal(token.Scopes)
		if err != nil {
			return 0, fmt.Errorf("marshal scopes: %w", err)
		}
		scopesJSON = sql.NullString{String: string(data), Valid: true}
	}

	query := database.ConvertPlaceholders(`
		INSERT INTO user_api_tokens (
			user_id, user_type, name, prefix, token_hash, scopes,
			expires_at, rate_limit, created_at, created_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)

	result, err := r.db.ExecContext(ctx, query,
		token.UserID,
		token.UserType,
		token.Name,
		token.Prefix,
		token.TokenHash,
		scopesJSON,
		token.ExpiresAt,
		token.RateLimit,
		token.CreatedAt,
		token.CreatedBy,
	)
	if err != nil {
		return 0, fmt.Errorf("insert token: %w", err)
	}

	return result.LastInsertId()
}

// GetByPrefix retrieves tokens matching a prefix (for verification)
func (r *APITokenRepository) GetByPrefix(ctx context.Context, prefix string) ([]*models.APIToken, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, user_id, user_type, name, prefix, token_hash, scopes,
			   expires_at, last_used_at, last_used_ip, rate_limit,
			   created_at, created_by, revoked_at, revoked_by
		FROM user_api_tokens
		WHERE prefix = ? AND revoked_at IS NULL
	`)

	rows, err := r.db.QueryContext(ctx, query, prefix)
	if err != nil {
		return nil, fmt.Errorf("query tokens: %w", err)
	}
	defer rows.Close()

	var tokens []*models.APIToken
	for rows.Next() {
		token, err := r.scanToken(rows)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}

	return tokens, rows.Err()
}

// GetByID retrieves a token by ID
func (r *APITokenRepository) GetByID(ctx context.Context, id int64) (*models.APIToken, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, user_id, user_type, name, prefix, token_hash, scopes,
			   expires_at, last_used_at, last_used_ip, rate_limit,
			   created_at, created_by, revoked_at, revoked_by
		FROM user_api_tokens
		WHERE id = ?
	`)

	row := r.db.QueryRowContext(ctx, query, id)
	return r.scanTokenRow(row)
}

// ListByUser retrieves all tokens for a user
func (r *APITokenRepository) ListByUser(ctx context.Context, userID int, userType models.APITokenUserType) ([]*models.APIToken, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, user_id, user_type, name, prefix, token_hash, scopes,
			   expires_at, last_used_at, last_used_ip, rate_limit,
			   created_at, created_by, revoked_at, revoked_by
		FROM user_api_tokens
		WHERE user_id = ? AND user_type = ?
		ORDER BY created_at DESC
	`)

	rows, err := r.db.QueryContext(ctx, query, userID, userType)
	if err != nil {
		return nil, fmt.Errorf("query tokens: %w", err)
	}
	defer rows.Close()

	var tokens []*models.APIToken
	for rows.Next() {
		token, err := r.scanToken(rows)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}

	return tokens, rows.Err()
}

// UpdateLastUsed updates the last used timestamp and IP
func (r *APITokenRepository) UpdateLastUsed(ctx context.Context, id int64, ip string) error {
	query := database.ConvertPlaceholders(`
		UPDATE user_api_tokens
		SET last_used_at = ?, last_used_ip = ?
		WHERE id = ?
	`)

	_, err := r.db.ExecContext(ctx, query, time.Now(), ip, id)
	return err
}

// Revoke soft-deletes a token
func (r *APITokenRepository) Revoke(ctx context.Context, id int64, revokedBy int) error {
	query := database.ConvertPlaceholders(`
		UPDATE user_api_tokens
		SET revoked_at = ?, revoked_by = ?
		WHERE id = ? AND revoked_at IS NULL
	`)

	result, err := r.db.ExecContext(ctx, query, time.Now(), revokedBy, id)
	if err != nil {
		return fmt.Errorf("revoke token: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("token not found or already revoked")
	}

	return nil
}

// ListAll retrieves all tokens (admin only)
func (r *APITokenRepository) ListAll(ctx context.Context, includeRevoked bool) ([]*models.APIToken, error) {
	var query string
	if includeRevoked {
		query = `
			SELECT id, user_id, user_type, name, prefix, token_hash, scopes,
				   expires_at, last_used_at, last_used_ip, rate_limit,
				   created_at, created_by, revoked_at, revoked_by
			FROM user_api_tokens
			ORDER BY created_at DESC
		`
	} else {
		query = `
			SELECT id, user_id, user_type, name, prefix, token_hash, scopes,
				   expires_at, last_used_at, last_used_ip, rate_limit,
				   created_at, created_by, revoked_at, revoked_by
			FROM user_api_tokens
			WHERE revoked_at IS NULL
			ORDER BY created_at DESC
		`
	}

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query tokens: %w", err)
	}
	defer rows.Close()

	var tokens []*models.APIToken
	for rows.Next() {
		token, err := r.scanToken(rows)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}

	return tokens, rows.Err()
}

// scanToken scans a token from rows
func (r *APITokenRepository) scanToken(rows *sql.Rows) (*models.APIToken, error) {
	var token models.APIToken
	err := rows.Scan(
		&token.ID,
		&token.UserID,
		&token.UserType,
		&token.Name,
		&token.Prefix,
		&token.TokenHash,
		&token.ScopesJSON,
		&token.ExpiresAt,
		&token.LastUsedAt,
		&token.LastUsedIP,
		&token.RateLimit,
		&token.CreatedAt,
		&token.CreatedBy,
		&token.RevokedAt,
		&token.RevokedBy,
	)
	if err != nil {
		return nil, fmt.Errorf("scan token: %w", err)
	}

	// Parse scopes JSON
	if token.ScopesJSON.Valid && token.ScopesJSON.String != "" {
		if err := json.Unmarshal([]byte(token.ScopesJSON.String), &token.Scopes); err != nil {
			return nil, fmt.Errorf("parse scopes: %w", err)
		}
	}

	return &token, nil
}

// scanTokenRow scans a single row
func (r *APITokenRepository) scanTokenRow(row *sql.Row) (*models.APIToken, error) {
	var token models.APIToken
	err := row.Scan(
		&token.ID,
		&token.UserID,
		&token.UserType,
		&token.Name,
		&token.Prefix,
		&token.TokenHash,
		&token.ScopesJSON,
		&token.ExpiresAt,
		&token.LastUsedAt,
		&token.LastUsedIP,
		&token.RateLimit,
		&token.CreatedAt,
		&token.CreatedBy,
		&token.RevokedAt,
		&token.RevokedBy,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan token: %w", err)
	}

	// Parse scopes JSON
	if token.ScopesJSON.Valid && token.ScopesJSON.String != "" {
		if err := json.Unmarshal([]byte(token.ScopesJSON.String), &token.Scopes); err != nil {
			return nil, fmt.Errorf("parse scopes: %w", err)
		}
	}

	return &token, nil
}
