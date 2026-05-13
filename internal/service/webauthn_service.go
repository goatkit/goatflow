package service

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/goatkit/goatflow/internal/database"
)

const (
	WebAuthnUserTypeAgent    = "agent"
	WebAuthnUserTypeCustomer = "customer"

	webAuthnPurposeRegistration = "registration"
	webAuthnPurposeLogin        = "login"
	webAuthnPurposePasskeyLogin = "passkey-login"

	webAuthnCeremonyTTL = 5 * time.Minute
)

type WebAuthnCredentialRecord struct {
	ID             int64      `json:"id"`
	UserType       string     `json:"user_type"`
	UserKey        string     `json:"user_key"`
	CredentialID   string     `json:"credential_id"`
	Name           string     `json:"name"`
	SignCount      uint32     `json:"sign_count"`
	LastUsedAt     *time.Time `json:"last_used_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	credentialJSON []byte
}

type WebAuthnService struct {
	db       *sql.DB
	webauthn *webauthn.WebAuthn
}

type PasskeyLoginResult struct {
	UserType string
	UserKey  string
}

type webAuthnUser struct {
	id          []byte
	name        string
	displayName string
	credentials []webauthn.Credential
}

func (u webAuthnUser) WebAuthnID() []byte                         { return u.id }
func (u webAuthnUser) WebAuthnName() string                       { return u.name }
func (u webAuthnUser) WebAuthnDisplayName() string                { return u.displayName }
func (u webAuthnUser) WebAuthnCredentials() []webauthn.Credential { return u.credentials }

type webAuthnCeremony struct {
	UserType string               `json:"user_type"`
	UserKey  string               `json:"user_key"`
	Purpose  string               `json:"purpose"`
	Session  webauthn.SessionData `json:"session"`
	Expires  time.Time            `json:"expires"`
}

type webAuthnCeremonyStore struct {
	mu       sync.RWMutex
	sessions map[string]webAuthnCeremony
}

var defaultWebAuthnCeremonies = &webAuthnCeremonyStore{sessions: map[string]webAuthnCeremony{}}

func NewWebAuthnService(db *sql.DB, r *http.Request) (*WebAuthnService, error) {
	cfg := webAuthnConfigFromRequest(r)
	wa, err := webauthn.New(cfg)
	if err != nil {
		return nil, err
	}
	return &WebAuthnService{db: db, webauthn: wa}, nil
}

func webAuthnConfigFromRequest(r *http.Request) *webauthn.Config {
	rpName := strings.TrimSpace(os.Getenv("GOATFLOW_WEBAUTHN_RP_NAME"))
	if rpName == "" {
		rpName = "GoatFlow"
	}

	origin := requestOrigin(r)
	rpID := strings.TrimSpace(os.Getenv("GOATFLOW_WEBAUTHN_RP_ID"))
	if rpID == "" {
		rpID = hostWithoutPort(r.Host)
		if rpID == "" {
			rpID = "localhost"
		}
	}

	origins := splitCSV(os.Getenv("GOATFLOW_WEBAUTHN_ORIGINS"))
	if len(origins) == 0 && origin != "" {
		origins = []string{origin}
	}
	if len(origins) == 0 {
		origins = []string{"http://localhost", "http://localhost:8080"}
	}

	return &webauthn.Config{
		RPID:          rpID,
		RPDisplayName: rpName,
		RPOrigins:     origins,
	}
}

func requestOrigin(r *http.Request) string {
	if r == nil {
		return ""
	}
	proto := r.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		if r.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	if host == "" {
		return ""
	}
	return proto + "://" + host
}

func hostWithoutPort(host string) string {
	if host == "" {
		return ""
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	if u, err := url.Parse("//" + host); err == nil && u.Hostname() != "" {
		return u.Hostname()
	}
	return strings.Split(host, ":")[0]
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func (s *WebAuthnService) IsEnabled(userType, userKey string) bool {
	count, err := s.CountCredentials(userType, userKey)
	return err == nil && count > 0
}

func (s *WebAuthnService) CountCredentials(userType, userKey string) (int, error) {
	var count int
	query := database.ConvertPlaceholders("SELECT COUNT(*) FROM gk_webauthn_credential WHERE user_type = ? AND user_key = ?")
	err := s.db.QueryRow(query, userType, userKey).Scan(&count)
	return count, err
}

func (s *WebAuthnService) ListCredentials(userType, userKey string) ([]WebAuthnCredentialRecord, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, user_type, user_key, credential_id, credential_json, name, sign_count, last_used_at, created_at, updated_at
		FROM gk_webauthn_credential
		WHERE user_type = ? AND user_key = ?
		ORDER BY created_at ASC, id ASC`)
	rows, err := s.db.Query(query, userType, userKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []WebAuthnCredentialRecord
	for rows.Next() {
		var rec WebAuthnCredentialRecord
		if err := scanWebAuthnCredential(rows, &rec); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (s *WebAuthnService) BeginRegistration(userType, userKey, displayName string) (interface{}, error) {
	user, err := s.loadUser(userType, userKey, displayName)
	if err != nil {
		return nil, err
	}
	creation, session, err := s.webauthn.BeginRegistration(user,
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
			RequireResidentKey: protocol.ResidentKeyRequired(),
			ResidentKey:        protocol.ResidentKeyRequirementRequired,
			UserVerification:   protocol.VerificationRequired,
		}),
		webauthn.WithExclusions(webauthn.Credentials(user.WebAuthnCredentials()).CredentialDescriptors()),
		webauthn.WithExtensions(protocol.AuthenticationExtensions{"credProps": true}),
	)
	if err != nil {
		return nil, err
	}
	s.storeCeremony(userType, userKey, webAuthnPurposeRegistration, *session)
	return creation, nil
}

func (s *WebAuthnService) FinishRegistration(userType, userKey, displayName, keyName string, r *http.Request) (*WebAuthnCredentialRecord, error) {
	ceremony, ok := s.takeCeremony(userType, userKey, webAuthnPurposeRegistration)
	if !ok {
		return nil, errors.New("registration challenge expired")
	}
	user, err := s.loadUser(userType, userKey, displayName)
	if err != nil {
		return nil, err
	}
	credential, err := s.webauthn.FinishRegistration(user, ceremony.Session, r)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(keyName) == "" {
		keyName = "Security key"
	}
	return s.storeCredential(userType, userKey, keyName, credential)
}

func (s *WebAuthnService) BeginLogin(userType, userKey, displayName string) (interface{}, error) {
	user, err := s.loadUser(userType, userKey, displayName)
	if err != nil {
		return nil, err
	}
	if len(user.credentials) == 0 {
		return nil, errors.New("no security keys registered")
	}
	assertion, session, err := s.webauthn.BeginLogin(user)
	if err != nil {
		return nil, err
	}
	s.storeCeremony(userType, userKey, webAuthnPurposeLogin, *session)
	return assertion, nil
}

func (s *WebAuthnService) FinishLogin(userType, userKey, displayName string, r *http.Request) error {
	ceremony, ok := s.takeCeremony(userType, userKey, webAuthnPurposeLogin)
	if !ok {
		return errors.New("login challenge expired")
	}
	user, err := s.loadUser(userType, userKey, displayName)
	if err != nil {
		return err
	}
	credential, err := s.webauthn.FinishLogin(user, ceremony.Session, r)
	if err != nil {
		return err
	}
	return s.updateCredentialAfterLogin(credential)
}

func (s *WebAuthnService) BeginPasskeyLogin(userType string) (interface{}, string, error) {
	if !validWebAuthnUserType(userType) {
		return nil, "", errors.New("invalid user type")
	}
	assertion, session, err := s.webauthn.BeginDiscoverableLogin(
		webauthn.WithUserVerification(protocol.VerificationRequired),
	)
	if err != nil {
		return nil, "", err
	}
	token, err := randomWebAuthnToken()
	if err != nil {
		return nil, "", err
	}
	s.storeCeremony(userType, token, webAuthnPurposePasskeyLogin, *session)
	return assertion, token, nil
}

func (s *WebAuthnService) FinishPasskeyLogin(userType, token string, r *http.Request) (*PasskeyLoginResult, error) {
	if !validWebAuthnUserType(userType) {
		return nil, errors.New("invalid user type")
	}
	ceremony, ok := s.takeCeremony(userType, token, webAuthnPurposePasskeyLogin)
	if !ok {
		return nil, errors.New("passkey login challenge expired")
	}

	var result *PasskeyLoginResult
	handler := func(rawID, userHandle []byte) (webauthn.User, error) {
		user, loginResult, err := s.passkeyLoginUser(rawID, userHandle)
		if err != nil {
			return nil, err
		}
		result = loginResult
		return user, nil
	}

	_, credential, err := s.webauthn.FinishPasskeyLogin(handler, ceremony.Session, r)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, errors.New("passkey login user was not resolved")
	}
	if err := s.updateCredentialAfterLogin(credential); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *WebAuthnService) passkeyLoginUser(rawID, userHandle []byte) (webauthn.User, *PasskeyLoginResult, error) {
	rec, err := s.credentialByRawID(rawID)
	if err != nil {
		return nil, nil, err
	}
	handleUserType, handleUserKey, ok := parseWebAuthnUserID(userHandle)
	if !ok || handleUserType != rec.UserType || handleUserKey != rec.UserKey {
		return nil, nil, errors.New("credential user handle mismatch")
	}
	user, err := s.loadUser(rec.UserType, rec.UserKey, rec.UserKey)
	if err != nil {
		return nil, nil, err
	}
	return user, &PasskeyLoginResult{UserType: rec.UserType, UserKey: rec.UserKey}, nil
}

func (s *WebAuthnService) RenameCredential(userType, userKey string, id int64, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("name is required")
	}
	query := database.ConvertPlaceholders("UPDATE gk_webauthn_credential SET name = ?, updated_at = ? WHERE id = ? AND user_type = ? AND user_key = ?")
	res, err := s.db.Exec(query, name, time.Now(), id, userType, userKey)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *WebAuthnService) DeleteCredential(userType, userKey string, id int64) error {
	query := database.ConvertPlaceholders("DELETE FROM gk_webauthn_credential WHERE id = ? AND user_type = ? AND user_key = ?")
	res, err := s.db.Exec(query, id, userType, userKey)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *WebAuthnService) DeleteAllCredentials(userType, userKey string) error {
	query := database.ConvertPlaceholders("DELETE FROM gk_webauthn_credential WHERE user_type = ? AND user_key = ?")
	_, err := s.db.Exec(query, userType, userKey)
	return err
}

func (s *WebAuthnService) loadUser(userType, userKey, displayName string) (*webAuthnUser, error) {
	records, err := s.ListCredentials(userType, userKey)
	if err != nil {
		return nil, err
	}
	credentials := make([]webauthn.Credential, 0, len(records))
	for _, rec := range records {
		var cred webauthn.Credential
		if err := json.Unmarshal(rec.credentialJSON, &cred); err != nil {
			return nil, fmt.Errorf("decode credential %d: %w", rec.ID, err)
		}
		credentials = append(credentials, cred)
	}
	if displayName == "" {
		displayName = userKey
	}
	return &webAuthnUser{
		id:          webAuthnUserID(userType, userKey),
		name:        userKey,
		displayName: displayName,
		credentials: credentials,
	}, nil
}

func (s *WebAuthnService) storeCredential(userType, userKey, name string, credential *webauthn.Credential) (*WebAuthnCredentialRecord, error) {
	credentialJSON, err := json.Marshal(credential)
	if err != nil {
		return nil, err
	}
	credentialID := encodeCredentialID(credential.ID)
	now := time.Now()
	query := database.ConvertPlaceholders(`
		INSERT INTO gk_webauthn_credential
			(user_type, user_key, credential_id, credential_json, name, sign_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	res, err := s.db.Exec(query, userType, userKey, credentialID, string(credentialJSON), name, credential.Authenticator.SignCount, now, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	if id == 0 {
		lookup := database.ConvertPlaceholders("SELECT id FROM gk_webauthn_credential WHERE credential_id = ?")
		_ = s.db.QueryRow(lookup, credentialID).Scan(&id)
	}
	return &WebAuthnCredentialRecord{
		ID:           id,
		UserType:     userType,
		UserKey:      userKey,
		CredentialID: credentialID,
		Name:         name,
		SignCount:    credential.Authenticator.SignCount,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func (s *WebAuthnService) updateCredentialAfterLogin(credential *webauthn.Credential) error {
	credentialJSON, err := json.Marshal(credential)
	if err != nil {
		return err
	}
	now := time.Now()
	query := database.ConvertPlaceholders(`
		UPDATE gk_webauthn_credential
		SET credential_json = ?, sign_count = ?, last_used_at = ?, updated_at = ?
		WHERE credential_id = ?`)
	_, err = s.db.Exec(query, string(credentialJSON), credential.Authenticator.SignCount, now, now, encodeCredentialID(credential.ID))
	return err
}

func (s *WebAuthnService) credentialByRawID(rawID []byte) (*WebAuthnCredentialRecord, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, user_type, user_key, credential_id, credential_json, name, sign_count, last_used_at, created_at, updated_at
		FROM gk_webauthn_credential
		WHERE credential_id = ?
		LIMIT 1`)
	var rec WebAuthnCredentialRecord
	if err := scanWebAuthnCredential(s.db.QueryRow(query, encodeCredentialID(rawID)), &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

type webAuthnCredentialScanner interface {
	Scan(dest ...interface{}) error
}

func scanWebAuthnCredential(scanner webAuthnCredentialScanner, rec *WebAuthnCredentialRecord) error {
	var signCount int64
	var lastUsed sql.NullTime
	if err := scanner.Scan(&rec.ID, &rec.UserType, &rec.UserKey, &rec.CredentialID, &rec.credentialJSON, &rec.Name, &signCount, &lastUsed, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
		return err
	}
	const maxUint32 = int64(1<<32 - 1)
	if signCount < 0 || signCount > maxUint32 {
		return fmt.Errorf("credential %d sign count out of range", rec.ID)
	}
	rec.SignCount = uint32(signCount)
	if lastUsed.Valid {
		rec.LastUsedAt = &lastUsed.Time
	}
	return nil
}

func webAuthnUserID(userType, userKey string) []byte {
	return []byte(userType + ":" + userKey)
}

func parseWebAuthnUserID(handle []byte) (string, string, bool) {
	userType, userKey, ok := strings.Cut(string(handle), ":")
	return userType, userKey, ok && validWebAuthnUserType(userType) && userKey != ""
}

func validWebAuthnUserType(userType string) bool {
	return userType == WebAuthnUserTypeAgent || userType == WebAuthnUserTypeCustomer
}

func randomWebAuthnToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func encodeCredentialID(id []byte) string {
	return base64.RawURLEncoding.EncodeToString(id)
}

func AgentWebAuthnUserKey(userID int) string {
	return strconv.Itoa(userID)
}

func (s *WebAuthnService) storeCeremony(userType, userKey, purpose string, session webauthn.SessionData) {
	ceremony := newWebAuthnCeremony(userType, userKey, purpose, session)
	if err := s.storeCeremonyDB(ceremony); err != nil {
		defaultWebAuthnCeremonies.PutCeremony(ceremony)
	}
}

func (s *WebAuthnService) takeCeremony(userType, userKey, purpose string) (webAuthnCeremony, bool) {
	if ceremony, ok, err := s.takeCeremonyDB(userType, userKey, purpose); err == nil {
		return ceremony, ok
	}
	return defaultWebAuthnCeremonies.Take(userType, userKey, purpose)
}

func (s *WebAuthnService) storeCeremonyDB(ceremony webAuthnCeremony) error {
	if s.db == nil {
		return errors.New("database unavailable")
	}
	sessionJSON, err := json.Marshal(ceremony.Session)
	if err != nil {
		return err
	}
	now := time.Now()
	cleanupQuery := database.ConvertPlaceholders("DELETE FROM gk_webauthn_ceremony WHERE expires_at < ?")
	if _, err := s.db.Exec(cleanupQuery, now); err != nil {
		return err
	}

	var query string
	if database.IsMySQL() {
		query = `
			INSERT INTO gk_webauthn_ceremony
				(ceremony_key, user_type, user_key, purpose, session_json, expires_at, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
				user_type = VALUES(user_type),
				user_key = VALUES(user_key),
				purpose = VALUES(purpose),
				session_json = VALUES(session_json),
				expires_at = VALUES(expires_at),
				created_at = VALUES(created_at)`
	} else {
		query = `
			INSERT INTO gk_webauthn_ceremony
				(ceremony_key, user_type, user_key, purpose, session_json, expires_at, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (ceremony_key) DO UPDATE SET
				user_type = EXCLUDED.user_type,
				user_key = EXCLUDED.user_key,
				purpose = EXCLUDED.purpose,
				session_json = EXCLUDED.session_json,
				expires_at = EXCLUDED.expires_at,
				created_at = EXCLUDED.created_at`
	}
	_, err = s.db.Exec(database.ConvertPlaceholders(query),
		webAuthnCeremonyKey(ceremony.UserType, ceremony.UserKey, ceremony.Purpose),
		ceremony.UserType,
		ceremony.UserKey,
		ceremony.Purpose,
		string(sessionJSON),
		ceremony.Expires,
		now,
	)
	return err
}

func (s *WebAuthnService) takeCeremonyDB(userType, userKey, purpose string) (webAuthnCeremony, bool, error) {
	if s.db == nil {
		return webAuthnCeremony{}, false, errors.New("database unavailable")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return webAuthnCeremony{}, false, err
	}
	defer func() { _ = tx.Rollback() }()

	var ceremony webAuthnCeremony
	var sessionJSON []byte
	key := webAuthnCeremonyKey(userType, userKey, purpose)
	query := database.ConvertPlaceholders(`
		SELECT user_type, user_key, purpose, session_json, expires_at
		FROM gk_webauthn_ceremony
		WHERE ceremony_key = ?
		FOR UPDATE`)
	err = tx.QueryRow(query, key).Scan(&ceremony.UserType, &ceremony.UserKey, &ceremony.Purpose, &sessionJSON, &ceremony.Expires)
	if err == sql.ErrNoRows {
		return webAuthnCeremony{}, false, nil
	}
	if err != nil {
		return webAuthnCeremony{}, false, err
	}

	deleteQuery := database.ConvertPlaceholders("DELETE FROM gk_webauthn_ceremony WHERE ceremony_key = ?")
	if _, err := tx.Exec(deleteQuery, key); err != nil {
		return webAuthnCeremony{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return webAuthnCeremony{}, false, err
	}

	if ceremony.UserType != userType || ceremony.UserKey != userKey || ceremony.Purpose != purpose {
		return webAuthnCeremony{}, false, nil
	}
	if time.Now().After(ceremony.Expires) {
		return webAuthnCeremony{}, false, nil
	}
	if err := json.Unmarshal(sessionJSON, &ceremony.Session); err != nil {
		return webAuthnCeremony{}, false, err
	}
	return ceremony, true, nil
}

func newWebAuthnCeremony(userType, userKey, purpose string, session webauthn.SessionData) webAuthnCeremony {
	return webAuthnCeremony{
		UserType: userType,
		UserKey:  userKey,
		Purpose:  purpose,
		Session:  session,
		Expires:  time.Now().Add(webAuthnCeremonyTTL),
	}
}

func webAuthnCeremonyKey(userType, userKey, purpose string) string {
	sum := sha256.Sum256([]byte(userType + "\x00" + userKey + "\x00" + purpose))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func (s *webAuthnCeremonyStore) Put(userType, userKey, purpose string, session webauthn.SessionData) {
	s.PutCeremony(newWebAuthnCeremony(userType, userKey, purpose, session))
}

func (s *webAuthnCeremonyStore) PutCeremony(ceremony webAuthnCeremony) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked()
	s.sessions[s.key(ceremony.UserType, ceremony.UserKey, ceremony.Purpose)] = ceremony
}

func (s *webAuthnCeremonyStore) Take(userType, userKey, purpose string) (webAuthnCeremony, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := s.key(userType, userKey, purpose)
	ceremony, ok := s.sessions[key]
	if !ok || time.Now().After(ceremony.Expires) {
		delete(s.sessions, key)
		return webAuthnCeremony{}, false
	}
	delete(s.sessions, key)
	return ceremony, true
}

func (s *webAuthnCeremonyStore) key(userType, userKey, purpose string) string {
	return userType + "\x00" + userKey + "\x00" + purpose
}

func (s *webAuthnCeremonyStore) cleanupLocked() {
	now := time.Now()
	for key, ceremony := range s.sessions {
		if now.After(ceremony.Expires) {
			delete(s.sessions, key)
		}
	}
}
