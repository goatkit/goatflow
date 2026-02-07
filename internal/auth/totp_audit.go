// Package auth provides authentication utilities for GoatFlow.
package auth

import (
	"fmt"
	"log"
	"time"
)

// TOTPAuditEvent represents a 2FA-related security event.
type TOTPAuditEvent struct {
	Timestamp  time.Time
	EventType  string
	UserID     int
	UserLogin  string
	IsCustomer bool
	ClientIP   string
	UserAgent  string
	Success    bool
	Details    string
}

// TOTP audit event types
const (
	AuditTOTPSetupStarted    = "2FA_SETUP_STARTED"
	AuditTOTPSetupCompleted  = "2FA_SETUP_COMPLETED"
	AuditTOTPSetupFailed     = "2FA_SETUP_FAILED"
	AuditTOTPDisabled        = "2FA_DISABLED"
	AuditTOTPVerifySuccess   = "2FA_VERIFY_SUCCESS"
	AuditTOTPVerifyFailed    = "2FA_VERIFY_FAILED"
	AuditTOTPSessionCreated  = "2FA_SESSION_CREATED"
	AuditTOTPSessionExpired  = "2FA_SESSION_EXPIRED"
	AuditTOTPSessionLocked   = "2FA_SESSION_LOCKED"
	AuditTOTPRecoveryUsed    = "2FA_RECOVERY_CODE_USED"
)

// LogTOTPAuditEvent logs a 2FA security event.
// V8 FIX: Provides audit trail for 2FA events.
func LogTOTPAuditEvent(event TOTPAuditEvent) {
	event.Timestamp = time.Now()

	userIdentifier := event.UserLogin
	if userIdentifier == "" && event.UserID > 0 {
		userIdentifier = string(rune(event.UserID))
	}

	userType := "agent"
	if event.IsCustomer {
		userType = "customer"
	}

	status := "FAILURE"
	if event.Success {
		status = "SUCCESS"
	}

	// Log in structured format for easy parsing
	log.Printf("[SECURITY] [2FA] event=%s status=%s user_type=%s user=%s ip=%s details=%q",
		event.EventType,
		status,
		userType,
		userIdentifier,
		event.ClientIP,
		event.Details,
	)

	// TODO: In production, this should write to:
	// 1. Security-specific log file
	// 2. SIEM system (Splunk, ELK, etc.)
	// 3. Database audit table
	// For now, we use standard logging which can be captured by log aggregators.
}

// Convenience functions for common audit events

// LogTOTPSetupStarted logs when a user starts 2FA setup.
func LogTOTPSetupStarted(userID int, userLogin string, isCustomer bool, clientIP string) {
	LogTOTPAuditEvent(TOTPAuditEvent{
		EventType:  AuditTOTPSetupStarted,
		UserID:     userID,
		UserLogin:  userLogin,
		IsCustomer: isCustomer,
		ClientIP:   clientIP,
		Success:    true,
		Details:    "2FA setup initiated",
	})
}

// LogTOTPSetupCompleted logs when 2FA setup is successfully completed.
func LogTOTPSetupCompleted(userID int, userLogin string, isCustomer bool, clientIP string) {
	LogTOTPAuditEvent(TOTPAuditEvent{
		EventType:  AuditTOTPSetupCompleted,
		UserID:     userID,
		UserLogin:  userLogin,
		IsCustomer: isCustomer,
		ClientIP:   clientIP,
		Success:    true,
		Details:    "2FA enabled successfully",
	})
}

// LogTOTPDisabled logs when 2FA is disabled.
func LogTOTPDisabled(userID int, userLogin string, isCustomer bool, clientIP string) {
	LogTOTPAuditEvent(TOTPAuditEvent{
		EventType:  AuditTOTPDisabled,
		UserID:     userID,
		UserLogin:  userLogin,
		IsCustomer: isCustomer,
		ClientIP:   clientIP,
		Success:    true,
		Details:    "2FA disabled by user",
	})
}

// LogTOTPVerifySuccess logs a successful 2FA verification.
func LogTOTPVerifySuccess(userID int, userLogin string, isCustomer bool, clientIP string) {
	LogTOTPAuditEvent(TOTPAuditEvent{
		EventType:  AuditTOTPVerifySuccess,
		UserID:     userID,
		UserLogin:  userLogin,
		IsCustomer: isCustomer,
		ClientIP:   clientIP,
		Success:    true,
		Details:    "2FA verification successful",
	})
}

// LogTOTPVerifyFailed logs a failed 2FA verification attempt.
func LogTOTPVerifyFailed(userID int, userLogin string, isCustomer bool, clientIP string, attemptsRemaining int) {
	LogTOTPAuditEvent(TOTPAuditEvent{
		EventType:  AuditTOTPVerifyFailed,
		UserID:     userID,
		UserLogin:  userLogin,
		IsCustomer: isCustomer,
		ClientIP:   clientIP,
		Success:    false,
		Details:    fmt.Sprintf("2FA verification failed, %d attempts remaining", attemptsRemaining),
	})
}

// LogTOTPSessionLocked logs when a 2FA session is locked due to too many failures.
func LogTOTPSessionLocked(userID int, userLogin string, isCustomer bool, clientIP string) {
	LogTOTPAuditEvent(TOTPAuditEvent{
		EventType:  AuditTOTPSessionLocked,
		UserID:     userID,
		UserLogin:  userLogin,
		IsCustomer: isCustomer,
		ClientIP:   clientIP,
		Success:    false,
		Details:    "2FA session locked due to too many failed attempts",
	})
}

// LogTOTPRecoveryCodeUsed logs when a recovery code is used instead of TOTP.
func LogTOTPRecoveryCodeUsed(userID int, userLogin string, isCustomer bool, clientIP string, codesRemaining int) {
	LogTOTPAuditEvent(TOTPAuditEvent{
		EventType:  AuditTOTPRecoveryUsed,
		UserID:     userID,
		UserLogin:  userLogin,
		IsCustomer: isCustomer,
		ClientIP:   clientIP,
		Success:    true,
		Details:    fmt.Sprintf("Recovery code used, %d codes remaining", codesRemaining),
	})
}
