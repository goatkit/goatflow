// Package service provides business logic services for GoatFlow.
package service

import (
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"

	"github.com/goatkit/goatflow/internal/database"
)

// PreferencesBackend abstracts preference storage for TOTP.
// Allows the same TOTP logic to work with different storage backends.
type PreferencesBackend interface {
	Get(key string) (string, error)
	Set(key, value string) error
	Delete(key string) error
	// SetAndDelete performs multiple set and delete operations atomically in a single transaction.
	// This ensures all-or-nothing semantics for multi-step preference changes.
	SetAndDelete(sets map[string]string, deletes []string) error
}

// UserPreferencesBackend stores preferences in user_preferences table (numeric user_id).
type UserPreferencesBackend struct {
	db     *sql.DB
	userID int
}

// NewUserPreferencesBackend creates a backend for agent/admin users.
func NewUserPreferencesBackend(db *sql.DB, userID int) *UserPreferencesBackend {
	return &UserPreferencesBackend{db: db, userID: userID}
}

func (b *UserPreferencesBackend) Get(key string) (string, error) {
	var value string
	query := database.ConvertPlaceholders(
		"SELECT preferences_value FROM user_preferences WHERE user_id = ? AND preferences_key = ?",
	)
	err := b.db.QueryRow(query, b.userID, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (b *UserPreferencesBackend) Set(key, value string) error {
	// Try update first
	query := database.ConvertPlaceholders(
		"UPDATE user_preferences SET preferences_value = ? WHERE user_id = ? AND preferences_key = ?",
	)
	result, err := b.db.Exec(query, value, b.userID, key)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		// Insert if not exists
		insertQuery := database.ConvertPlaceholders(
			"INSERT INTO user_preferences (user_id, preferences_key, preferences_value) VALUES (?, ?, ?)",
		)
		_, err = b.db.Exec(insertQuery, b.userID, key, value)
		return err
	}
	return nil
}

func (b *UserPreferencesBackend) Delete(key string) error {
	query := database.ConvertPlaceholders(
		"DELETE FROM user_preferences WHERE user_id = ? AND preferences_key = ?",
	)
	_, err := b.db.Exec(query, b.userID, key)
	return err
}

// SetAndDelete performs multiple set and delete operations atomically.
func (b *UserPreferencesBackend) SetAndDelete(sets map[string]string, deletes []string) error {
	tx, err := b.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Perform all deletes first
	deleteQuery := database.ConvertPlaceholders(
		"DELETE FROM user_preferences WHERE user_id = ? AND preferences_key = ?",
	)
	for _, key := range deletes {
		if _, err = tx.Exec(deleteQuery, b.userID, key); err != nil {
			return err
		}
	}

	// Perform all sets (upsert pattern: delete then insert)
	setDeleteQuery := database.ConvertPlaceholders(
		"DELETE FROM user_preferences WHERE user_id = ? AND preferences_key = ?",
	)
	insertQuery := database.ConvertPlaceholders(
		"INSERT INTO user_preferences (user_id, preferences_key, preferences_value) VALUES (?, ?, ?)",
	)
	for key, value := range sets {
		if _, err = tx.Exec(setDeleteQuery, b.userID, key); err != nil {
			return err
		}
		if _, err = tx.Exec(insertQuery, b.userID, key, value); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// CustomerPreferencesBackend stores preferences in customer_preferences table (string user_id/login).
type CustomerPreferencesBackend struct {
	db        *sql.DB
	userLogin string
}

// NewCustomerPreferencesBackend creates a backend for customer users.
func NewCustomerPreferencesBackend(db *sql.DB, userLogin string) *CustomerPreferencesBackend {
	return &CustomerPreferencesBackend{db: db, userLogin: userLogin}
}

func (b *CustomerPreferencesBackend) Get(key string) (string, error) {
	var value string
	query := database.ConvertPlaceholders(
		"SELECT preferences_value FROM customer_preferences WHERE user_id = ? AND preferences_key = ?",
	)
	err := b.db.QueryRow(query, b.userLogin, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (b *CustomerPreferencesBackend) Set(key, value string) error {
	// Delete then insert (customer_preferences may have duplicates in legacy data)
	tx, err := b.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	deleteQuery := database.ConvertPlaceholders(
		"DELETE FROM customer_preferences WHERE user_id = ? AND preferences_key = ?",
	)
	if _, err = tx.Exec(deleteQuery, b.userLogin, key); err != nil {
		return err
	}

	insertQuery := database.ConvertPlaceholders(
		"INSERT INTO customer_preferences (user_id, preferences_key, preferences_value) VALUES (?, ?, ?)",
	)
	if _, err = tx.Exec(insertQuery, b.userLogin, key, value); err != nil {
		return err
	}

	return tx.Commit()
}

func (b *CustomerPreferencesBackend) Delete(key string) error {
	query := database.ConvertPlaceholders(
		"DELETE FROM customer_preferences WHERE user_id = ? AND preferences_key = ?",
	)
	_, err := b.db.Exec(query, b.userLogin, key)
	return err
}

// SetAndDelete performs multiple set and delete operations atomically.
func (b *CustomerPreferencesBackend) SetAndDelete(sets map[string]string, deletes []string) error {
	tx, err := b.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Perform all deletes first
	deleteQuery := database.ConvertPlaceholders(
		"DELETE FROM customer_preferences WHERE user_id = ? AND preferences_key = ?",
	)
	for _, key := range deletes {
		if _, err = tx.Exec(deleteQuery, b.userLogin, key); err != nil {
			return err
		}
	}

	// Perform all sets (delete then insert for customer_preferences)
	insertQuery := database.ConvertPlaceholders(
		"INSERT INTO customer_preferences (user_id, preferences_key, preferences_value) VALUES (?, ?, ?)",
	)
	for key, value := range sets {
		if _, err = tx.Exec(deleteQuery, b.userLogin, key); err != nil {
			return err
		}
		if _, err = tx.Exec(insertQuery, b.userLogin, key, value); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// TOTPService handles two-factor authentication using TOTP.
type TOTPService struct {
	db      *sql.DB
	issuer  string
	backend PreferencesBackend
}

// NewTOTPService creates a TOTP service for agent/admin users (numeric ID).
func NewTOTPService(db *sql.DB, issuer string) *TOTPService {
	if issuer == "" {
		issuer = "GoatFlow"
	}
	return &TOTPService{db: db, issuer: issuer}
}

// NewCustomerTOTPService creates a TOTP service for customer users (string login).
func NewCustomerTOTPService(db *sql.DB, issuer string, userLogin string) *TOTPService {
	if issuer == "" {
		issuer = "GoatFlow"
	}
	return &TOTPService{
		db:      db,
		issuer:  issuer,
		backend: NewCustomerPreferencesBackend(db, userLogin),
	}
}

// ForUser returns a TOTPService bound to a specific agent user ID.
func (s *TOTPService) ForUser(userID int) *TOTPService {
	return &TOTPService{
		db:      s.db,
		issuer:  s.issuer,
		backend: NewUserPreferencesBackend(s.db, userID),
	}
}

// ForCustomer returns a TOTPService bound to a specific customer login.
func (s *TOTPService) ForCustomer(userLogin string) *TOTPService {
	return &TOTPService{
		db:      s.db,
		issuer:  s.issuer,
		backend: NewCustomerPreferencesBackend(s.db, userLogin),
	}
}

// TOTPSetupData contains data needed for 2FA setup.
type TOTPSetupData struct {
	Secret        string   `json:"secret"`
	URL           string   `json:"url"`
	RecoveryCodes []string `json:"recovery_codes"`
}

// GenerateSetup creates a new TOTP secret and recovery codes for a user.
// The secret is NOT saved until ConfirmSetup is called with a valid code.
func (s *TOTPService) GenerateSetup(userID int, userEmail string) (*TOTPSetupData, error) {
	backend := s.getBackend(userID)
	return s.generateSetupWithBackend(backend, userEmail)
}

// GenerateSetupForCustomer creates a new TOTP secret for a customer.
func (s *TOTPService) GenerateSetupForCustomer(userLogin string) (*TOTPSetupData, error) {
	backend := NewCustomerPreferencesBackend(s.db, userLogin)
	return s.generateSetupWithBackend(backend, userLogin)
}

func (s *TOTPService) generateSetupWithBackend(backend PreferencesBackend, accountName string) (*TOTPSetupData, error) {
	// Generate TOTP key
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      s.issuer,
		AccountName: accountName,
		Algorithm:   otp.AlgorithmSHA1,
		Digits:      otp.DigitsSix,
		Period:      30,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate TOTP key: %w", err)
	}

	// Generate recovery codes
	recoveryCodes, err := generateRecoveryCodes(8)
	if err != nil {
		return nil, fmt.Errorf("failed to generate recovery codes: %w", err)
	}

	// Store pending setup (not enabled yet)
	if err := backend.Set("UserTOTPPendingSecret", key.Secret()); err != nil {
		return nil, fmt.Errorf("failed to store pending setup: %w", err)
	}
	codesJSON, _ := json.Marshal(recoveryCodes)
	if err := backend.Set("UserTOTPPendingRecoveryCodes", string(codesJSON)); err != nil {
		return nil, fmt.Errorf("failed to store recovery codes: %w", err)
	}

	return &TOTPSetupData{
		Secret:        key.Secret(),
		URL:           key.URL(),
		RecoveryCodes: recoveryCodes,
	}, nil
}

// ConfirmSetup verifies the TOTP code and enables 2FA for the user.
func (s *TOTPService) ConfirmSetup(userID int, code string) error {
	backend := s.getBackend(userID)
	return s.confirmSetupWithBackend(backend, code)
}

// ConfirmSetupForCustomer verifies and enables 2FA for a customer.
func (s *TOTPService) ConfirmSetupForCustomer(userLogin string, code string) error {
	backend := NewCustomerPreferencesBackend(s.db, userLogin)
	return s.confirmSetupWithBackend(backend, code)
}

func (s *TOTPService) confirmSetupWithBackend(backend PreferencesBackend, code string) error {
	// Get pending secret
	secret, err := backend.Get("UserTOTPPendingSecret")
	if err != nil || secret == "" {
		return fmt.Errorf("no pending 2FA setup found")
	}

	// Validate the code
	if !totp.Validate(code, secret) {
		return fmt.Errorf("invalid verification code")
	}

	// Get recovery codes from pending
	codes, _ := backend.Get("UserTOTPPendingRecoveryCodes")

	// Atomically: set active values and delete pending values
	sets := map[string]string{
		"UserTOTPSecret":  secret,
		"UserTOTPEnabled": "1",
	}
	if codes != "" {
		sets["UserTOTPRecoveryCodes"] = codes
	}

	deletes := []string{
		"UserTOTPPendingSecret",
		"UserTOTPPendingRecoveryCodes",
	}

	return backend.SetAndDelete(sets, deletes)
}

// ValidateCode checks if a TOTP code is valid for the user.
func (s *TOTPService) ValidateCode(userID int, code string) (bool, error) {
	backend := s.getBackend(userID)
	return s.validateCodeWithBackend(backend, code)
}

// ValidateCodeForCustomer checks if a TOTP code is valid for a customer.
func (s *TOTPService) ValidateCodeForCustomer(userLogin string, code string) (bool, error) {
	backend := NewCustomerPreferencesBackend(s.db, userLogin)
	return s.validateCodeWithBackend(backend, code)
}

func (s *TOTPService) validateCodeWithBackend(backend PreferencesBackend, code string) (bool, error) {
	secret, err := backend.Get("UserTOTPSecret")
	if err != nil || secret == "" {
		return false, fmt.Errorf("2FA not configured for user")
	}

	// Try TOTP code first
	totpValid := totp.Validate(code, secret)
	if totpValid {
		return true, nil
	}

	// Try recovery code
	valid, err := s.useRecoveryCodeWithBackend(backend, code)
	if err == nil && valid {
		return true, nil
	}

	return false, nil
}

// IsEnabled checks if 2FA is enabled for a user.
func (s *TOTPService) IsEnabled(userID int) bool {
	backend := s.getBackend(userID)
	return s.isEnabledWithBackend(backend)
}

// IsEnabledForCustomer checks if 2FA is enabled for a customer.
func (s *TOTPService) IsEnabledForCustomer(userLogin string) bool {
	backend := NewCustomerPreferencesBackend(s.db, userLogin)
	return s.isEnabledWithBackend(backend)
}

func (s *TOTPService) isEnabledWithBackend(backend PreferencesBackend) bool {
	enabled, err := backend.Get("UserTOTPEnabled")
	if err != nil {
		return false
	}
	return enabled == "1"
}

// Disable turns off 2FA for a user.
func (s *TOTPService) Disable(userID int, code string) error {
	backend := s.getBackend(userID)
	return s.disableWithBackend(backend, code)
}

// DisableForCustomer turns off 2FA for a customer.
func (s *TOTPService) DisableForCustomer(userLogin string, code string) error {
	backend := NewCustomerPreferencesBackend(s.db, userLogin)
	return s.disableWithBackend(backend, code)
}

// ForceDisable turns off 2FA for a user without requiring a code (admin override).
func (s *TOTPService) ForceDisable(userID int) error {
	backend := s.getBackend(userID)
	return s.forceDisableWithBackend(backend)
}

// ForceDisableForCustomer turns off 2FA for a customer without requiring a code (admin override).
func (s *TOTPService) ForceDisableForCustomer(userLogin string) error {
	backend := NewCustomerPreferencesBackend(s.db, userLogin)
	return s.forceDisableWithBackend(backend)
}

func (s *TOTPService) forceDisableWithBackend(backend PreferencesBackend) error {
	// Atomically clear all TOTP preferences (no code verification)
	deletes := []string{
		"UserTOTPSecret",
		"UserTOTPEnabled",
		"UserTOTPRecoveryCodes",
		"UserTOTPPendingSecret",
		"UserTOTPPendingRecoveryCodes",
	}
	return backend.SetAndDelete(nil, deletes)
}

func (s *TOTPService) disableWithBackend(backend PreferencesBackend, code string) error {
	// Require valid code to disable
	valid, err := s.validateCodeWithBackend(backend, code)
	if err != nil || !valid {
		return fmt.Errorf("invalid code - cannot disable 2FA")
	}

	// Atomically clear all TOTP preferences
	deletes := []string{
		"UserTOTPSecret",
		"UserTOTPEnabled",
		"UserTOTPRecoveryCodes",
		"UserTOTPPendingSecret",
		"UserTOTPPendingRecoveryCodes",
	}

	return backend.SetAndDelete(nil, deletes)
}

// GetRemainingRecoveryCodes returns count of unused recovery codes.
func (s *TOTPService) GetRemainingRecoveryCodes(userID int) int {
	backend := s.getBackend(userID)
	return s.getRemainingRecoveryCodesWithBackend(backend)
}

// GetRemainingRecoveryCodesForCustomer returns count for a customer.
func (s *TOTPService) GetRemainingRecoveryCodesForCustomer(userLogin string) int {
	backend := NewCustomerPreferencesBackend(s.db, userLogin)
	return s.getRemainingRecoveryCodesWithBackend(backend)
}

func (s *TOTPService) getRemainingRecoveryCodesWithBackend(backend PreferencesBackend) int {
	codesJSON, err := backend.Get("UserTOTPRecoveryCodes")
	if err != nil || codesJSON == "" {
		return 0
	}

	var codes []string
	if err := json.Unmarshal([]byte(codesJSON), &codes); err != nil {
		return 0
	}
	return len(codes)
}

// Internal helpers

func (s *TOTPService) getBackend(userID int) PreferencesBackend {
	if s.backend != nil {
		return s.backend
	}
	return NewUserPreferencesBackend(s.db, userID)
}

func (s *TOTPService) useRecoveryCodeWithBackend(backend PreferencesBackend, code string) (bool, error) {
	codesJSON, err := backend.Get("UserTOTPRecoveryCodes")
	if err != nil || codesJSON == "" {
		return false, nil
	}

	var codes []string
	if err := json.Unmarshal([]byte(codesJSON), &codes); err != nil {
		return false, err
	}

	// Normalize input code (remove dashes/spaces, lowercase)
	code = strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(code), "-", ""), " ", "")

	// Find and remove the code
	for i, c := range codes {
		if strings.ToLower(c) == code {
			// Remove used code
			codes = append(codes[:i], codes[i+1:]...)
			newJSON, _ := json.Marshal(codes)
			_ = backend.Set("UserTOTPRecoveryCodes", string(newJSON))
			return true, nil
		}
	}

	return false, nil
}

// Legacy helpers for backward compatibility (deprecated, use methods above)

func (s *TOTPService) getPreference(userID int, key string) (string, error) {
	return s.getBackend(userID).Get(key)
}

func (s *TOTPService) setPreference(userID int, key, value string) error {
	return s.getBackend(userID).Set(key, value)
}

func (s *TOTPService) deletePreference(userID int, key string) error {
	return s.getBackend(userID).Delete(key)
}

func (s *TOTPService) storePendingSetup(userID int, secret string, codes []string) error {
	backend := s.getBackend(userID)
	if err := backend.Set("UserTOTPPendingSecret", secret); err != nil {
		return err
	}
	codesJSON, err := json.Marshal(codes)
	if err != nil {
		return err
	}
	return backend.Set("UserTOTPPendingRecoveryCodes", string(codesJSON))
}

func (s *TOTPService) useRecoveryCode(userID int, code string) (bool, error) {
	return s.useRecoveryCodeWithBackend(s.getBackend(userID), code)
}

func generateRecoveryCodes(count int) ([]string, error) {
	codes := make([]string, count)
	for i := 0; i < count; i++ {
		// V6 FIX: Increased from 5 bytes (40 bits) to 16 bytes (128 bits)
		bytes := make([]byte, 16)
		if _, err := rand.Read(bytes); err != nil {
			return nil, err
		}
		// Generate 12-character code from 128 bits of entropy
		// Format: xxxx-xxxx-xxxx for readability
		code := base32.StdEncoding.EncodeToString(bytes)[:12]
		codes[i] = strings.ToLower(code)
	}
	return codes, nil
}
