package selfservice

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/goatkit/goatflow/internal/database"
)

// Repository provides CRUD for auth tokens and registration requests.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a repository using the global DB.
func NewRepository() (*Repository, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	return &Repository{db: db}, nil
}

// NewRepositoryWithDB creates a repository with an explicit DB.
func NewRepositoryWithDB(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// --- Auth Tokens ---

// GenerateToken creates a cryptographically random 32-byte hex token.
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// CreateToken creates a new auth token.
func (r *Repository) CreateToken(t *AuthToken) error {
	query := database.ConvertPlaceholders(`
		INSERT INTO gk_auth_token (token, token_type, user_type, user_id, customer_login, email, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	_, err := r.db.Exec(query,
		t.Token, t.TokenType, t.UserType, t.UserID, t.CustomerLogin,
		t.Email, t.ExpiresAt, t.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create auth token: %w", err)
	}
	return nil
}

// GetToken retrieves and validates a token. Returns nil if not found.
func (r *Repository) GetToken(token string) (*AuthToken, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, token, token_type, user_type, user_id, customer_login, email, expires_at, used_at, created_at
		FROM gk_auth_token WHERE token = ?
	`)
	var t AuthToken
	err := r.db.QueryRow(query, token).Scan(
		&t.ID, &t.Token, &t.TokenType, &t.UserType, &t.UserID, &t.CustomerLogin,
		&t.Email, &t.ExpiresAt, &t.UsedAt, &t.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get token: %w", err)
	}
	return &t, nil
}

// ConsumeToken marks a token as used.
func (r *Repository) ConsumeToken(token string) error {
	now := time.Now()
	query := database.ConvertPlaceholders("UPDATE gk_auth_token SET used_at = ? WHERE token = ? AND used_at IS NULL")
	result, err := r.db.Exec(query, now, token)
	if err != nil {
		return fmt.Errorf("consume token: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("token already used or not found")
	}
	return nil
}

// CleanupExpired removes expired and used tokens older than 24 hours.
func (r *Repository) CleanupExpired() (int64, error) {
	cutoff := time.Now().Add(-24 * time.Hour)
	query := database.ConvertPlaceholders("DELETE FROM gk_auth_token WHERE expires_at < ? OR (used_at IS NOT NULL AND used_at < ?)")
	result, err := r.db.Exec(query, cutoff, cutoff)
	if err != nil {
		return 0, fmt.Errorf("cleanup tokens: %w", err)
	}
	return result.RowsAffected()
}

// --- Registration Requests ---

// CreateRegistration creates a new registration request.
func (r *Repository) CreateRegistration(req *RegistrationRequest) (int64, error) {
	query := database.ConvertPlaceholders(`
		INSERT INTO gk_registration_request (email, first_name, last_name, customer_id, status, approval_token, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	result, err := r.db.Exec(query,
		req.Email, req.FirstName, req.LastName, req.CustomerID,
		req.Status, req.ApprovalToken, req.CreatedAt,
	)
	if err != nil {
		return 0, fmt.Errorf("create registration: %w", err)
	}
	return result.LastInsertId()
}

// GetRegistration retrieves a registration request by ID.
func (r *Repository) GetRegistration(id int64) (*RegistrationRequest, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, email, first_name, last_name, customer_id, status, approval_token, approved_by, approved_at, rejected_reason, created_at
		FROM gk_registration_request WHERE id = ?
	`)
	var req RegistrationRequest
	err := r.db.QueryRow(query, id).Scan(
		&req.ID, &req.Email, &req.FirstName, &req.LastName, &req.CustomerID,
		&req.Status, &req.ApprovalToken, &req.ApprovedBy, &req.ApprovedAt,
		&req.RejectedReason, &req.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get registration: %w", err)
	}
	return &req, nil
}

// ListPendingRegistrations lists registration requests with pending status.
func (r *Repository) ListPendingRegistrations() ([]RegistrationRequest, error) {
	query := database.ConvertPlaceholders(
		"SELECT id, email, first_name, last_name, customer_id, status, approval_token, approved_by, approved_at, rejected_reason, created_at FROM gk_registration_request WHERE status = ? ORDER BY created_at DESC")
	rows, err := r.db.Query(query, StatusPending)
	if err != nil {
		return nil, fmt.Errorf("list pending: %w", err)
	}
	defer rows.Close()

	var reqs []RegistrationRequest
	for rows.Next() {
		var req RegistrationRequest
		if err := rows.Scan(&req.ID, &req.Email, &req.FirstName, &req.LastName, &req.CustomerID,
			&req.Status, &req.ApprovalToken, &req.ApprovedBy, &req.ApprovedAt,
			&req.RejectedReason, &req.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan registration: %w", err)
		}
		reqs = append(reqs, req)
	}
	return reqs, rows.Err()
}

// ApproveRegistration approves a registration request.
func (r *Repository) ApproveRegistration(id int64, approvedBy int) error {
	now := time.Now()
	query := database.ConvertPlaceholders(
		"UPDATE gk_registration_request SET status = ?, approved_by = ?, approved_at = ? WHERE id = ? AND status = ?")
	result, err := r.db.Exec(query, StatusApproved, approvedBy, now, id, StatusPending)
	if err != nil {
		return fmt.Errorf("approve registration: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("registration not found or already processed")
	}
	return nil
}

// RejectRegistration rejects a registration request.
func (r *Repository) RejectRegistration(id int64, reason string, rejectedBy int) error {
	query := database.ConvertPlaceholders(
		"UPDATE gk_registration_request SET status = ?, rejected_reason = ?, approved_by = ?, approved_at = ? WHERE id = ? AND status = ?")
	now := time.Now()
	result, err := r.db.Exec(query, StatusRejected, reason, rejectedBy, now, id, StatusPending)
	if err != nil {
		return fmt.Errorf("reject registration: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("registration not found or already processed")
	}
	return nil
}
