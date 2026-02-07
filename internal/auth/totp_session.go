// Package auth provides authentication utilities for GoatFlow.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"
)

// TOTPSessionManager handles pending 2FA sessions with security controls.
// Addresses: V3 (rate limiting), V5 (HMAC verification), V7 (session invalidation).
type TOTPSessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*PendingTOTPSession
	secret   []byte // HMAC secret key
}

// PendingTOTPSession tracks a pending 2FA verification.
type PendingTOTPSession struct {
	Token      string    // Random token (cookie value)
	UserID     int       // For agents (numeric)
	UserLogin  string    // For customers (email) - stored here, not in cookie
	Username   string    // Display name
	IsCustomer bool      // true for customer, false for agent
	CreatedAt  time.Time // Session creation time
	ExpiresAt  time.Time // Expiration (5 minutes)
	Attempts   int       // Failed attempt count
	MaxAttempts int      // Max allowed (default 5)
	ClientIP   string    // Bind to IP for security
	UserAgent  string    // Bind to User-Agent
}

const (
	// MaxTOTPAttempts before session is invalidated
	MaxTOTPAttempts = 5
	// TOTPSessionTTL is how long a pending session lasts
	TOTPSessionTTL = 5 * time.Minute
	// CleanupInterval for expired sessions
	CleanupInterval = 1 * time.Minute
)

var (
	// DefaultTOTPSessionManager is the global instance
	DefaultTOTPSessionManager *TOTPSessionManager
	initOnce                  sync.Once
)

// GetTOTPSessionManager returns the singleton instance.
func GetTOTPSessionManager() *TOTPSessionManager {
	initOnce.Do(func() {
		secret := make([]byte, 32)
		rand.Read(secret)
		DefaultTOTPSessionManager = NewTOTPSessionManager(secret)
		go DefaultTOTPSessionManager.cleanupLoop()
	})
	return DefaultTOTPSessionManager
}

// NewTOTPSessionManager creates a new session manager with the given HMAC secret.
func NewTOTPSessionManager(secret []byte) *TOTPSessionManager {
	return &TOTPSessionManager{
		sessions: make(map[string]*PendingTOTPSession),
		secret:   secret,
	}
}

// CreateAgentSession creates a pending 2FA session for an agent.
// Returns the token to store in cookie.
func (m *TOTPSessionManager) CreateAgentSession(userID int, username, clientIP, userAgent string) (string, error) {
	token, err := m.generateToken()
	if err != nil {
		return "", err
	}

	session := &PendingTOTPSession{
		Token:       token,
		UserID:      userID,
		Username:    username,
		IsCustomer:  false,
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(TOTPSessionTTL),
		Attempts:    0,
		MaxAttempts: MaxTOTPAttempts,
		ClientIP:    clientIP,
		UserAgent:   userAgent,
	}

	m.mu.Lock()
	m.sessions[token] = session
	m.mu.Unlock()

	return token, nil
}

// CreateCustomerSession creates a pending 2FA session for a customer.
// Returns the token to store in cookie. Login is stored server-side, NOT in cookie (V4 fix).
func (m *TOTPSessionManager) CreateCustomerSession(userLogin, clientIP, userAgent string) (string, error) {
	token, err := m.generateToken()
	if err != nil {
		return "", err
	}

	session := &PendingTOTPSession{
		Token:       token,
		UserLogin:   userLogin, // Stored server-side, not in cookie!
		IsCustomer:  true,
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(TOTPSessionTTL),
		Attempts:    0,
		MaxAttempts: MaxTOTPAttempts,
		ClientIP:    clientIP,
		UserAgent:   userAgent,
	}

	m.mu.Lock()
	m.sessions[token] = session
	m.mu.Unlock()

	return token, nil
}

// ValidateAndGetSession checks if a token is valid and returns the session.
// Returns nil if invalid, expired, or too many attempts.
// Security: Enforces strict IP binding to prevent session hijacking during 2FA.
func (m *TOTPSessionManager) ValidateAndGetSession(token, clientIP, userAgent string) *PendingTOTPSession {
	m.mu.RLock()
	session, exists := m.sessions[token]
	m.mu.RUnlock()

	if !exists {
		log.Printf("[SECURITY] 2FA session validation failed: token not found")
		return nil
	}

	// Check expiration
	if time.Now().After(session.ExpiresAt) {
		log.Printf("[SECURITY] 2FA session expired for user %d/%s", session.UserID, session.UserLogin)
		m.InvalidateSession(token)
		return nil
	}

	// Check max attempts (V7)
	if session.Attempts >= session.MaxAttempts {
		log.Printf("[SECURITY] 2FA session locked due to max attempts for user %d/%s", session.UserID, session.UserLogin)
		m.InvalidateSession(token)
		return nil
	}

	// T3: Strict IP binding - reject if client IP changed since session creation
	// This prevents session hijacking during the 2FA window
	if session.ClientIP != "" && session.ClientIP != clientIP {
		log.Printf("[SECURITY] 2FA session REJECTED - IP mismatch: expected=%s, got=%s, user=%d/%s",
			session.ClientIP, clientIP, session.UserID, session.UserLogin)
		m.InvalidateSession(token)
		return nil
	}

	// Log UA mismatches for monitoring (soft enforcement - UA can change legitimately)
	if session.UserAgent != "" && session.UserAgent != userAgent {
		if !isSimilarUserAgent(session.UserAgent, userAgent) {
			log.Printf("[SECURITY] 2FA session User-Agent mismatch: user=%d/%s (allowing)",
				session.UserID, session.UserLogin)
		}
	}

	return session
}

// isSimilarUserAgent checks if two user agents are "similar enough" (same browser/OS family).
// This avoids false positives from minor version updates during a session.
func isSimilarUserAgent(expected, got string) bool {
	if expected == got {
		return true
	}
	// Simple heuristic: check if first 50 chars match (browser/OS usually in prefix)
	if len(expected) > 50 && len(got) > 50 {
		return expected[:50] == got[:50]
	}
	return false
}

// RecordFailedAttempt increments the attempt counter (V3 + V7).
// Returns remaining attempts, or 0 if session is now invalid.
func (m *TOTPSessionManager) RecordFailedAttempt(token string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[token]
	if !exists {
		return 0
	}

	session.Attempts++
	remaining := session.MaxAttempts - session.Attempts

	if remaining <= 0 {
		delete(m.sessions, token)
		return 0
	}

	return remaining
}

// InvalidateSession removes a session (after successful auth or too many failures).
func (m *TOTPSessionManager) InvalidateSession(token string) {
	m.mu.Lock()
	delete(m.sessions, token)
	m.mu.Unlock()
}

// GetRemainingAttempts returns how many attempts are left for a session.
func (m *TOTPSessionManager) GetRemainingAttempts(token string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[token]
	if !exists {
		return 0
	}
	return session.MaxAttempts - session.Attempts
}

// GenerateHMAC creates an HMAC signature for session data (V5).
func (m *TOTPSessionManager) GenerateHMAC(data string) string {
	h := hmac.New(sha256.New, m.secret)
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

// VerifyHMAC checks if an HMAC signature is valid (V5).
func (m *TOTPSessionManager) VerifyHMAC(data, signature string) bool {
	expected := m.GenerateHMAC(data)
	return hmac.Equal([]byte(expected), []byte(signature))
}

// generateToken creates a cryptographically secure random token.
func (m *TOTPSessionManager) generateToken() (string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("failed to generate secure token: %w", err)
	}
	return base64.URLEncoding.EncodeToString(tokenBytes), nil
}

// cleanupLoop periodically removes expired sessions.
func (m *TOTPSessionManager) cleanupLoop() {
	ticker := time.NewTicker(CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		m.cleanup()
	}
}

func (m *TOTPSessionManager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for token, session := range m.sessions {
		if now.After(session.ExpiresAt) {
			delete(m.sessions, token)
		}
	}
}

// Stats returns current session manager statistics.
func (m *TOTPSessionManager) Stats() map[string]int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]int{
		"active_sessions": len(m.sessions),
	}
}
