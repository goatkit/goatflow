package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/models"
)

// SessionRepository defines the interface for session operations.
type SessionRepository interface {
	Create(session *models.Session) error
	GetByID(sessionID string) (*models.Session, error)
	GetByUserID(userID int) ([]*models.Session, error)
	List() ([]*models.Session, error)
	UpdateLastRequest(sessionID string) error
	Delete(sessionID string) error
	DeleteByUserID(userID int) (int, error)
	DeleteExpired(maxAge time.Duration) (int, error)
	DeleteByMaxAge(maxAge time.Duration) (int, error)
}

// SessionSQLRepository handles database operations for the OTRS sessions table.
// The sessions table uses a key-value store format with columns:
// session_id, data_key, data_value, serialized
type SessionSQLRepository struct {
	db *sql.DB
}

// NewSessionRepository creates a new session repository.
func NewSessionRepository(db *sql.DB) *SessionSQLRepository {
	return &SessionSQLRepository{db: db}
}

// Create stores a new session in the key-value sessions table.
func (r *SessionSQLRepository) Create(session *models.Session) error {
	if session.SessionID == "" {
		return errors.New("session ID is required")
	}

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Insert each key-value pair
	insertQuery := database.ConvertPlaceholders(`
		INSERT INTO sessions (session_id, data_key, data_value, serialized)
		VALUES (?, ?, ?, ?)`)

	// UserID
	if _, err = tx.Exec(insertQuery, session.SessionID, models.SessionKeyUserID, strconv.Itoa(session.UserID), 0); err != nil {
		return fmt.Errorf("failed to insert UserID: %w", err)
	}

	// UserLogin
	if _, err = tx.Exec(insertQuery, session.SessionID, models.SessionKeyUserLogin, session.UserLogin, 0); err != nil {
		return fmt.Errorf("failed to insert UserLogin: %w", err)
	}

	// UserType
	if _, err = tx.Exec(insertQuery, session.SessionID, models.SessionKeyUserType, session.UserType, 0); err != nil {
		return fmt.Errorf("failed to insert UserType: %w", err)
	}

	// CreateTime
	createTimeStr := session.CreateTime.Format(time.RFC3339)
	if _, err = tx.Exec(insertQuery, session.SessionID, models.SessionKeyCreateTime, createTimeStr, 0); err != nil {
		return fmt.Errorf("failed to insert CreateTime: %w", err)
	}

	// LastRequest
	lastRequestStr := session.LastRequest.Format(time.RFC3339)
	if _, err = tx.Exec(insertQuery, session.SessionID, models.SessionKeyLastRequest, lastRequestStr, 0); err != nil {
		return fmt.Errorf("failed to insert LastRequest: %w", err)
	}

	// UserRemoteAddr
	if _, err = tx.Exec(insertQuery, session.SessionID, models.SessionKeyUserRemoteAddr, session.RemoteAddr, 0); err != nil {
		return fmt.Errorf("failed to insert UserRemoteAddr: %w", err)
	}

	// UserRemoteUserAgent
	if _, err = tx.Exec(insertQuery, session.SessionID, models.SessionKeyUserRemoteAgent, session.UserAgent, 0); err != nil {
		return fmt.Errorf("failed to insert UserRemoteUserAgent: %w", err)
	}

	// UserTitle (optional)
	if session.UserTitle != "" {
		if _, err = tx.Exec(insertQuery, session.SessionID, models.SessionKeyUserTitle, session.UserTitle, 0); err != nil {
			return fmt.Errorf("failed to insert UserTitle: %w", err)
		}
	}

	// UserFullname (optional)
	if session.UserFullName != "" {
		if _, err = tx.Exec(insertQuery, session.SessionID, models.SessionKeyUserFullname, session.UserFullName, 0); err != nil {
			return fmt.Errorf("failed to insert UserFullname: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetByID retrieves a session by its ID.
func (r *SessionSQLRepository) GetByID(sessionID string) (*models.Session, error) {
	query := database.ConvertPlaceholders(`
		SELECT data_key, data_value
		FROM sessions
		WHERE session_id = ?`)

	rows, err := r.db.Query(query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query session: %w", err)
	}
	defer rows.Close()

	session := &models.Session{SessionID: sessionID}
	found := false

	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		found = true
		r.applyKeyValue(session, key, value)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	if !found {
		return nil, errors.New("session not found")
	}

	return session, nil
}

// GetByUserID retrieves all sessions for a specific user.
func (r *SessionSQLRepository) GetByUserID(userID int) ([]*models.Session, error) {
	// First, get all session IDs for this user
	query := database.ConvertPlaceholders(`
		SELECT DISTINCT session_id
		FROM sessions
		WHERE data_key = ? AND data_value = ?`)

	rows, err := r.db.Query(query, models.SessionKeyUserID, strconv.Itoa(userID))
	if err != nil {
		return nil, fmt.Errorf("failed to query session IDs: %w", err)
	}
	defer rows.Close()

	var sessionIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan session ID: %w", err)
		}
		sessionIDs = append(sessionIDs, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	// Now load each session
	sessions := make([]*models.Session, 0, len(sessionIDs))
	for _, id := range sessionIDs {
		session, err := r.GetByID(id)
		if err != nil {
			continue // Skip sessions that fail to load
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// List retrieves all sessions.
func (r *SessionSQLRepository) List() ([]*models.Session, error) {
	// First, get all unique session IDs
	query := database.ConvertPlaceholders(`SELECT DISTINCT session_id FROM sessions`)

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query session IDs: %w", err)
	}
	defer rows.Close()

	var sessionIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan session ID: %w", err)
		}
		sessionIDs = append(sessionIDs, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	// Now load each session
	sessions := make([]*models.Session, 0, len(sessionIDs))
	for _, id := range sessionIDs {
		session, err := r.GetByID(id)
		if err != nil {
			continue // Skip sessions that fail to load
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// UpdateLastRequest updates the last request time for a session.
func (r *SessionSQLRepository) UpdateLastRequest(sessionID string) error {
	query := database.ConvertPlaceholders(`
		UPDATE sessions
		SET data_value = ?
		WHERE session_id = ? AND data_key = ?`)

	result, err := r.db.Exec(query, time.Now().Format(time.RFC3339), sessionID, models.SessionKeyLastRequest)
	if err != nil {
		return fmt.Errorf("failed to update last request: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("session not found")
	}

	return nil
}

// Delete removes a session by its ID.
func (r *SessionSQLRepository) Delete(sessionID string) error {
	query := database.ConvertPlaceholders(`DELETE FROM sessions WHERE session_id = ?`)

	result, err := r.db.Exec(query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("session not found")
	}

	return nil
}

// DeleteByUserID removes all sessions for a specific user.
func (r *SessionSQLRepository) DeleteByUserID(userID int) (int, error) {
	// First, get all session IDs for this user
	query := database.ConvertPlaceholders(`
		SELECT session_id
		FROM sessions
		WHERE data_key = ? AND data_value = ?`)

	rows, err := r.db.Query(query, models.SessionKeyUserID, strconv.Itoa(userID))
	if err != nil {
		return 0, fmt.Errorf("failed to query session IDs: %w", err)
	}
	defer rows.Close()

	var sessionIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, fmt.Errorf("failed to scan session ID: %w", err)
		}
		sessionIDs = append(sessionIDs, id)
	}

	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("row iteration error: %w", err)
	}

	// Delete each session
	deleteQuery := database.ConvertPlaceholders(`DELETE FROM sessions WHERE session_id = ?`)
	count := 0
	for _, id := range sessionIDs {
		if _, err := r.db.Exec(deleteQuery, id); err != nil {
			continue // Skip sessions that fail to delete
		}
		count++
	}

	return count, nil
}

// DeleteExpired removes all sessions older than the specified duration.
func (r *SessionSQLRepository) DeleteExpired(maxAge time.Duration) (int, error) {
	cutoff := time.Now().Add(-maxAge).Format(time.RFC3339)

	// Get session IDs with LastRequest older than cutoff
	query := database.ConvertPlaceholders(`
		SELECT session_id
		FROM sessions
		WHERE data_key = ? AND data_value < ?`)

	rows, err := r.db.Query(query, models.SessionKeyLastRequest, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to query expired sessions: %w", err)
	}
	defer rows.Close()

	var sessionIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, fmt.Errorf("failed to scan session ID: %w", err)
		}
		sessionIDs = append(sessionIDs, id)
	}

	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("row iteration error: %w", err)
	}

	// Delete each expired session
	deleteQuery := database.ConvertPlaceholders(`DELETE FROM sessions WHERE session_id = ?`)
	count := 0
	for _, id := range sessionIDs {
		if _, err := r.db.Exec(deleteQuery, id); err != nil {
			continue
		}
		count++
	}

	return count, nil
}

// DeleteByMaxAge removes all sessions created more than maxAge ago.
// This enforces the maximum session lifetime regardless of activity.
func (r *SessionSQLRepository) DeleteByMaxAge(maxAge time.Duration) (int, error) {
	cutoff := time.Now().Add(-maxAge).Format(time.RFC3339)

	// Get session IDs with CreateTime older than cutoff
	query := database.ConvertPlaceholders(`
		SELECT session_id
		FROM sessions
		WHERE data_key = ? AND data_value < ?`)

	rows, err := r.db.Query(query, models.SessionKeyCreateTime, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to query old sessions: %w", err)
	}
	defer rows.Close()

	var sessionIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, fmt.Errorf("failed to scan session ID: %w", err)
		}
		sessionIDs = append(sessionIDs, id)
	}

	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("row iteration error: %w", err)
	}

	// Delete each old session
	deleteQuery := database.ConvertPlaceholders(`DELETE FROM sessions WHERE session_id = ?`)
	count := 0
	for _, id := range sessionIDs {
		if _, err := r.db.Exec(deleteQuery, id); err != nil {
			continue
		}
		count++
	}

	return count, nil
}

// parseTime tries to parse a time string in various formats.
// OTRS uses "2006-01-02 15:04:05" format, while GoatFlow uses RFC3339.
func parseTime(value string) (time.Time, bool) {
	// Try RFC3339 first (GoatFlow format)
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, true
	}
	// Try OTRS format "2006-01-02 15:04:05"
	if t, err := time.Parse("2006-01-02 15:04:05", value); err == nil {
		return t, true
	}
	// Try date only
	if t, err := time.Parse("2006-01-02", value); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// applyKeyValue sets a session field based on the key-value pair.
func (r *SessionSQLRepository) applyKeyValue(session *models.Session, key, value string) {
	switch key {
	case models.SessionKeyUserID:
		if id, err := strconv.Atoi(value); err == nil {
			session.UserID = id
		}
	case models.SessionKeyUserLogin:
		session.UserLogin = value
	case models.SessionKeyUserType:
		session.UserType = value
	case models.SessionKeyUserTitle:
		session.UserTitle = value
	case models.SessionKeyUserFullname:
		session.UserFullName = value
	case models.SessionKeyCreateTime:
		if t, ok := parseTime(value); ok {
			session.CreateTime = t
		}
	case models.SessionKeyLastRequest:
		if t, ok := parseTime(value); ok {
			session.LastRequest = t
		}
	// OTRS uses "ChangeTime" instead of "LastRequest"
	case "ChangeTime":
		if t, ok := parseTime(value); ok {
			session.LastRequest = t
		}
	case models.SessionKeyUserRemoteAddr:
		session.RemoteAddr = value
	case models.SessionKeyUserRemoteAgent:
		session.UserAgent = value
	}
}
