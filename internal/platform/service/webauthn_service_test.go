package service

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

func TestBeginRegistrationRequestsPasskeyCapableCredential(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT id, user_type, user_key, credential_id, credential_json").
		WithArgs(WebAuthnUserTypeAgent, "42").
		WillReturnRows(sqlmock.NewRows(webAuthnCredentialColumns()))
	expectCeremonyStore(mock, WebAuthnUserTypeAgent, "42", webAuthnPurposeRegistration)

	svc := newTestWebAuthnService(t, db)
	options, err := svc.BeginRegistration(WebAuthnUserTypeAgent, "42", "Ada Agent")
	if err != nil {
		t.Fatalf("BeginRegistration returned error: %v", err)
	}

	creation, ok := options.(*protocol.CredentialCreation)
	if !ok {
		t.Fatalf("BeginRegistration returned %T, want *protocol.CredentialCreation", options)
	}
	selection := creation.Response.AuthenticatorSelection
	if selection.ResidentKey != protocol.ResidentKeyRequirementRequired {
		t.Fatalf("ResidentKey = %q, want %q", selection.ResidentKey, protocol.ResidentKeyRequirementRequired)
	}
	if selection.RequireResidentKey == nil || !*selection.RequireResidentKey {
		t.Fatalf("RequireResidentKey = %v, want true", selection.RequireResidentKey)
	}
	if selection.UserVerification != protocol.VerificationRequired {
		t.Fatalf("UserVerification = %q, want %q", selection.UserVerification, protocol.VerificationRequired)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestWebAuthnCeremonyPersistsToDB(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	expectCeremonyStore(mock, WebAuthnUserTypeCustomer, "terry", webAuthnPurposeLogin)

	svc := newTestWebAuthnService(t, db)
	svc.storeCeremony(WebAuthnUserTypeCustomer, "terry", webAuthnPurposeLogin, webauthn.SessionData{
		Challenge:      "challenge",
		RelyingPartyID: "example.com",
		Expires:        time.Now().Add(time.Minute),
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestWebAuthnCeremonyCanBeConsumedFromDB(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	userType := WebAuthnUserTypeCustomer
	userKey := "terry"
	purpose := webAuthnPurposeLogin
	session := webauthn.SessionData{
		Challenge:      "challenge",
		RelyingPartyID: "example.com",
		Expires:        time.Now().Add(time.Minute),
	}
	sessionJSON, err := json.Marshal(session)
	if err != nil {
		t.Fatalf("marshal session: %v", err)
	}
	expires := time.Now().Add(time.Minute)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT user_type, user_key, purpose, session_json, expires_at").
		WithArgs(webAuthnCeremonyKey(userType, userKey, purpose)).
		WillReturnRows(sqlmock.NewRows([]string{"user_type", "user_key", "purpose", "session_json", "expires_at"}).
			AddRow(userType, userKey, purpose, string(sessionJSON), expires))
	mock.ExpectExec("DELETE FROM gk_webauthn_ceremony").
		WithArgs(webAuthnCeremonyKey(userType, userKey, purpose)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	svc := newTestWebAuthnService(t, db)
	ceremony, ok := svc.takeCeremony(userType, userKey, purpose)
	if !ok {
		t.Fatal("takeCeremony did not find DB-backed ceremony")
	}
	if ceremony.Session.Challenge != session.Challenge {
		t.Fatalf("challenge = %q, want %q", ceremony.Session.Challenge, session.Challenge)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestBeginPasskeyLoginCreatesDiscoverableChallenge(t *testing.T) {
	resetWebAuthnCeremonyStore()
	svc := newTestWebAuthnService(t, nil)

	options, token, err := svc.BeginPasskeyLogin(WebAuthnUserTypeAgent)
	if err != nil {
		t.Fatalf("BeginPasskeyLogin returned error: %v", err)
	}
	if token == "" {
		t.Fatal("BeginPasskeyLogin returned empty ceremony token")
	}

	assertion, ok := options.(*protocol.CredentialAssertion)
	if !ok {
		t.Fatalf("BeginPasskeyLogin returned %T, want *protocol.CredentialAssertion", options)
	}
	if len(assertion.Response.AllowedCredentials) != 0 {
		t.Fatalf("AllowedCredentials length = %d, want 0 for discoverable login", len(assertion.Response.AllowedCredentials))
	}
	if assertion.Response.UserVerification != protocol.VerificationRequired {
		t.Fatalf("UserVerification = %q, want %q", assertion.Response.UserVerification, protocol.VerificationRequired)
	}
	if _, ok := defaultWebAuthnCeremonies.Take(WebAuthnUserTypeAgent, token, webAuthnPurposePasskeyLogin); !ok {
		t.Fatal("passkey login ceremony was not stored")
	}
}

func TestBeginPasskeyLoginRejectsInvalidUserType(t *testing.T) {
	svc := newTestWebAuthnService(t, nil)
	if _, _, err := svc.BeginPasskeyLogin("operator"); err == nil {
		t.Fatal("BeginPasskeyLogin accepted invalid user type")
	}
}

func TestFinishPasskeyLoginRejectsMissingChallenge(t *testing.T) {
	resetWebAuthnCeremonyStore()
	svc := newTestWebAuthnService(t, nil)
	req := httptest.NewRequest(http.MethodPost, "https://example.com/api/auth/passkey/finish", strings.NewReader("{}"))

	_, err := svc.FinishPasskeyLogin(WebAuthnUserTypeAgent, "missing", req)
	if err == nil || !strings.Contains(err.Error(), "challenge expired") {
		t.Fatalf("FinishPasskeyLogin error = %v, want challenge expired", err)
	}
}

func TestPasskeyLoginUserAcceptsCredentialFromOtherLoginSurface(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	rawID := []byte("customer-passkey")
	credential := webauthn.Credential{ID: rawID}
	credentialJSON, err := json.Marshal(credential)
	if err != nil {
		t.Fatalf("marshal credential: %v", err)
	}
	now := time.Now()
	credentialRow := func() *sqlmock.Rows {
		return sqlmock.NewRows(webAuthnCredentialColumns()).
			AddRow(int64(7), WebAuthnUserTypeCustomer, "terry", encodeCredentialID(rawID), string(credentialJSON), "Terry key", int64(0), nil, now, now)
	}

	mock.ExpectQuery("SELECT id, user_type, user_key, credential_id, credential_json").
		WithArgs(encodeCredentialID(rawID)).
		WillReturnRows(credentialRow())
	mock.ExpectQuery("SELECT id, user_type, user_key, credential_id, credential_json").
		WithArgs(WebAuthnUserTypeCustomer, "terry").
		WillReturnRows(credentialRow())

	svc := newTestWebAuthnService(t, db)
	user, result, err := svc.passkeyLoginUser(rawID, webAuthnUserID(WebAuthnUserTypeCustomer, "terry"))
	if err != nil {
		t.Fatalf("passkeyLoginUser returned error: %v", err)
	}
	if result.UserType != WebAuthnUserTypeCustomer || result.UserKey != "terry" {
		t.Fatalf("result = (%q, %q), want customer/terry", result.UserType, result.UserKey)
	}
	if string(user.WebAuthnID()) != "customer:terry" {
		t.Fatalf("WebAuthnID = %q, want customer:terry", string(user.WebAuthnID()))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestParseWebAuthnUserID(t *testing.T) {
	userType, userKey, ok := parseWebAuthnUserID(webAuthnUserID(WebAuthnUserTypeCustomer, "bans@example.test"))
	if !ok {
		t.Fatal("parseWebAuthnUserID rejected valid handle")
	}
	if userType != WebAuthnUserTypeCustomer || userKey != "bans@example.test" {
		t.Fatalf("parseWebAuthnUserID = (%q, %q), want customer/bans@example.test", userType, userKey)
	}
	if _, _, ok := parseWebAuthnUserID([]byte("unknown:42")); ok {
		t.Fatal("parseWebAuthnUserID accepted invalid user type")
	}
}

func newTestWebAuthnService(t *testing.T, db *sql.DB) *WebAuthnService {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "https://example.com/webauthn", nil)
	svc, err := NewWebAuthnService(db, req)
	if err != nil {
		t.Fatalf("NewWebAuthnService: %v", err)
	}
	return svc
}

func resetWebAuthnCeremonyStore() {
	defaultWebAuthnCeremonies = &webAuthnCeremonyStore{sessions: map[string]webAuthnCeremony{}}
}

func expectCeremonyStore(mock sqlmock.Sqlmock, userType, userKey, purpose string) {
	mock.ExpectExec("DELETE FROM gk_webauthn_ceremony").
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO gk_webauthn_ceremony").
		WithArgs(
			webAuthnCeremonyKey(userType, userKey, purpose),
			userType,
			userKey,
			purpose,
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
}

func webAuthnCredentialColumns() []string {
	return []string{"id", "user_type", "user_key", "credential_id", "credential_json", "name", "sign_count", "last_used_at", "created_at", "updated_at"}
}
