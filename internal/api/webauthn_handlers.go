package api

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/auth"
	"github.com/goatkit/goatflow/internal/platform/constants"
	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/platform/httpcookie"
	"github.com/goatkit/goatflow/internal/platform/service"
	"github.com/goatkit/goatflow/internal/repository"
	"github.com/goatkit/goatflow/internal/routing"
	"github.com/goatkit/goatflow/internal/shared"
)

func init() {
	routing.RegisterHandler("handleWebAuthnRegisterBegin", handleWebAuthnRegisterBegin)
	routing.RegisterHandler("handleWebAuthnRegisterFinish", handleWebAuthnRegisterFinish)
	routing.RegisterHandler("handleWebAuthnCredentials", handleWebAuthnCredentials)
	routing.RegisterHandler("handleWebAuthnCredentialRename", handleWebAuthnCredentialRename)
	routing.RegisterHandler("handleWebAuthnCredentialDelete", handleWebAuthnCredentialDelete)
	routing.RegisterHandler("handleWebAuthnLoginBegin", handleWebAuthnLoginBegin)
	routing.RegisterHandler("handleWebAuthnLoginFinish", handleWebAuthnLoginFinish)
	routing.RegisterHandler("handlePasskeyLoginBegin", handlePasskeyLoginBegin)
	routing.RegisterHandler("handlePasskeyLoginFinish", handlePasskeyLoginFinish)

	routing.RegisterHandler("handleCustomerWebAuthnRegisterBegin", handleCustomerWebAuthnRegisterBegin)
	routing.RegisterHandler("handleCustomerWebAuthnRegisterFinish", handleCustomerWebAuthnRegisterFinish)
	routing.RegisterHandler("handleCustomerWebAuthnCredentials", handleCustomerWebAuthnCredentials)
	routing.RegisterHandler("handleCustomerWebAuthnCredentialRename", handleCustomerWebAuthnCredentialRename)
	routing.RegisterHandler("handleCustomerWebAuthnCredentialDelete", handleCustomerWebAuthnCredentialDelete)
	routing.RegisterHandler("handleCustomerWebAuthnLoginBegin", handleCustomerWebAuthnLoginBegin)
	routing.RegisterHandler("handleCustomerWebAuthnLoginFinish", handleCustomerWebAuthnLoginFinish)
	routing.RegisterHandler("handleCustomerPasskeyLoginBegin", handleCustomerPasskeyLoginBegin)
	routing.RegisterHandler("handleCustomerPasskeyLoginFinish", handleCustomerPasskeyLoginFinish)
}

func handleWebAuthnRegisterBegin(c *gin.Context) {
	userID := getTOTPUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}
	var req struct {
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "password is required"})
		return
	}
	db, ok := webAuthnDB(c)
	if !ok {
		return
	}
	displayName, err := verifyAgentPasswordAndDisplayName(db, userID, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": err.Error()})
		return
	}
	wa, err := service.NewWebAuthnService(db, c.Request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "security key setup unavailable"})
		return
	}
	options, err := wa.BeginRegistration(service.WebAuthnUserTypeAgent, service.AgentWebAuthnUserKey(userID), displayName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "options": options})
}

func handleWebAuthnRegisterFinish(c *gin.Context) {
	userID := getTOTPUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}
	db, ok := webAuthnDB(c)
	if !ok {
		return
	}
	displayName := getTOTPUserEmail(c)
	keyName := c.Query("name")
	wa, err := service.NewWebAuthnService(db, c.Request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "security key setup unavailable"})
		return
	}
	rec, err := wa.FinishRegistration(service.WebAuthnUserTypeAgent, service.AgentWebAuthnUserKey(userID), displayName, keyName, c.Request)
	if err != nil {
		log.Printf("[SECURITY] WebAuthn registration failed user_type=agent user_id=%d ip=%s err=%v", userID, c.ClientIP(), err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	auth.LogTOTPAuditEvent(auth.TOTPAuditEvent{EventType: "WEBAUTHN_REGISTERED", UserID: userID, UserLogin: displayName, ClientIP: c.ClientIP(), Success: true, Details: "security key registered"})
	c.JSON(http.StatusOK, gin.H{"success": true, "credential": publicWebAuthnCredential(rec)})
}

func handleWebAuthnCredentials(c *gin.Context) {
	userID := getTOTPUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}
	listWebAuthnCredentials(c, service.WebAuthnUserTypeAgent, service.AgentWebAuthnUserKey(userID))
}

func handleWebAuthnCredentialRename(c *gin.Context) {
	userID := getTOTPUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}
	renameWebAuthnCredential(c, service.WebAuthnUserTypeAgent, service.AgentWebAuthnUserKey(userID))
}

func handleWebAuthnCredentialDelete(c *gin.Context) {
	userID := getTOTPUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}
	var req struct {
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "password is required"})
		return
	}
	db, ok := webAuthnDB(c)
	if !ok {
		return
	}
	if _, err := verifyAgentPasswordAndDisplayName(db, userID, req.Password); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": err.Error()})
		return
	}
	deleteWebAuthnCredential(c, db, service.WebAuthnUserTypeAgent, service.AgentWebAuthnUserKey(userID))
}

func handleWebAuthnLoginBegin(c *gin.Context) {
	session, ok := pendingAgent2FASession(c)
	if !ok {
		return
	}
	db, ok := webAuthnDB(c)
	if !ok {
		return
	}
	wa, err := service.NewWebAuthnService(db, c.Request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "security key login unavailable"})
		return
	}
	options, err := wa.BeginLogin(service.WebAuthnUserTypeAgent, service.AgentWebAuthnUserKey(session.UserID), session.Username)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "options": options})
}

func handleWebAuthnLoginFinish(c *gin.Context) {
	token, err := c.Cookie("2fa_pending")
	if err != nil || token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "no pending 2FA session"})
		return
	}
	session, ok := pendingAgent2FASession(c)
	if !ok {
		return
	}
	db, ok := webAuthnDB(c)
	if !ok {
		return
	}
	wa, err := service.NewWebAuthnService(db, c.Request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "security key login unavailable"})
		return
	}
	if err := wa.FinishLogin(service.WebAuthnUserTypeAgent, service.AgentWebAuthnUserKey(session.UserID), session.Username, c.Request); err != nil {
		remaining := auth.GetTOTPSessionManager().RecordFailedAttempt(token)
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "security key verification failed", "attempts_remaining": remaining})
		return
	}
	auth.GetTOTPSessionManager().InvalidateSession(token)
	httpcookie.SetAuth(c, "2fa_pending", "", -1)
	completeAgentSecondFactorLogin(c, db, session)
}

func handlePasskeyLoginBegin(c *gin.Context) {
	beginPasskeyLogin(c, service.WebAuthnUserTypeAgent, "passkey_login_pending")
}

func handlePasskeyLoginFinish(c *gin.Context) {
	finishPasskeyLogin(c, service.WebAuthnUserTypeAgent, "passkey_login_pending")
}

func handleCustomerWebAuthnRegisterBegin(c *gin.Context) {
	customerLogin := getCustomerLogin(c)
	if customerLogin == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}
	var req struct {
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "password is required"})
		return
	}
	db, ok := webAuthnDB(c)
	if !ok {
		return
	}
	if !verifyCustomerPassword(db, customerLogin, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "incorrect password"})
		return
	}
	wa, err := service.NewWebAuthnService(db, c.Request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "security key setup unavailable"})
		return
	}
	options, err := wa.BeginRegistration(service.WebAuthnUserTypeCustomer, customerLogin, customerLogin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "options": options})
}

func handleCustomerWebAuthnRegisterFinish(c *gin.Context) {
	customerLogin := getCustomerLogin(c)
	if customerLogin == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}
	db, ok := webAuthnDB(c)
	if !ok {
		return
	}
	wa, err := service.NewWebAuthnService(db, c.Request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "security key setup unavailable"})
		return
	}
	rec, err := wa.FinishRegistration(service.WebAuthnUserTypeCustomer, customerLogin, customerLogin, c.Query("name"), c.Request)
	if err != nil {
		log.Printf("[SECURITY] WebAuthn registration failed user_type=customer user=%s ip=%s err=%v", customerLogin, c.ClientIP(), err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	auth.LogTOTPAuditEvent(auth.TOTPAuditEvent{EventType: "WEBAUTHN_REGISTERED", UserLogin: customerLogin, IsCustomer: true, ClientIP: c.ClientIP(), Success: true, Details: "customer security key registered"})
	c.JSON(http.StatusOK, gin.H{"success": true, "credential": publicWebAuthnCredential(rec)})
}

func handleCustomerWebAuthnCredentials(c *gin.Context) {
	customerLogin := getCustomerLogin(c)
	if customerLogin == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}
	listWebAuthnCredentials(c, service.WebAuthnUserTypeCustomer, customerLogin)
}

func handleCustomerWebAuthnCredentialRename(c *gin.Context) {
	customerLogin := getCustomerLogin(c)
	if customerLogin == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}
	renameWebAuthnCredential(c, service.WebAuthnUserTypeCustomer, customerLogin)
}

func handleCustomerWebAuthnCredentialDelete(c *gin.Context) {
	customerLogin := getCustomerLogin(c)
	if customerLogin == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}
	var req struct {
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "password is required"})
		return
	}
	db, ok := webAuthnDB(c)
	if !ok {
		return
	}
	if !verifyCustomerPassword(db, customerLogin, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "incorrect password"})
		return
	}
	deleteWebAuthnCredential(c, db, service.WebAuthnUserTypeCustomer, customerLogin)
}

func handleCustomerWebAuthnLoginBegin(c *gin.Context) {
	session, ok := pendingCustomer2FASession(c)
	if !ok {
		return
	}
	db, ok := webAuthnDB(c)
	if !ok {
		return
	}
	wa, err := service.NewWebAuthnService(db, c.Request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "security key login unavailable"})
		return
	}
	options, err := wa.BeginLogin(service.WebAuthnUserTypeCustomer, session.UserLogin, session.UserLogin)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "options": options})
}

func handleCustomerWebAuthnLoginFinish(c *gin.Context) {
	token, err := c.Cookie("customer_2fa_pending")
	if err != nil || token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "no pending 2FA session"})
		return
	}
	session, ok := pendingCustomer2FASession(c)
	if !ok {
		return
	}
	db, ok := webAuthnDB(c)
	if !ok {
		return
	}
	wa, err := service.NewWebAuthnService(db, c.Request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "security key login unavailable"})
		return
	}
	if err := wa.FinishLogin(service.WebAuthnUserTypeCustomer, session.UserLogin, session.UserLogin, c.Request); err != nil {
		remaining := auth.GetTOTPSessionManager().RecordFailedAttempt(token)
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "security key verification failed", "attempts_remaining": remaining})
		return
	}
	auth.GetTOTPSessionManager().InvalidateSession(token)
	httpcookie.SetAuth(c, "customer_2fa_pending", "", -1)
	completeCustomerSecondFactorLogin(c, db, session)
}

func handleCustomerPasskeyLoginBegin(c *gin.Context) {
	beginPasskeyLogin(c, service.WebAuthnUserTypeCustomer, "customer_passkey_login_pending")
}

func handleCustomerPasskeyLoginFinish(c *gin.Context) {
	finishPasskeyLogin(c, service.WebAuthnUserTypeCustomer, "customer_passkey_login_pending")
}

func webAuthnDB(c *gin.Context) (*sql.DB, bool) {
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "database unavailable"})
		return nil, false
	}
	return db, true
}

func beginPasskeyLogin(c *gin.Context, userType, cookieName string) {
	limiterKey := passkeyLimiterKey(userType)
	if blocked, remaining := auth.DefaultLoginRateLimiter.IsBlocked(c.ClientIP(), limiterKey); blocked {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"success":         false,
			"error":           fmt.Sprintf("too many failed attempts, try again in %d seconds", int(remaining.Seconds())),
			"retry_after_sec": int(remaining.Seconds()),
		})
		return
	}
	db, ok := webAuthnDB(c)
	if !ok {
		return
	}
	wa, err := service.NewWebAuthnService(db, c.Request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "passkey login unavailable"})
		return
	}
	options, token, err := wa.BeginPasskeyLogin(userType)
	if err != nil {
		auth.DefaultLoginRateLimiter.RecordFailure(c.ClientIP(), limiterKey)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "passkey login unavailable"})
		return
	}
	httpcookie.SetAuth(c, cookieName, token, 300)
	c.JSON(http.StatusOK, gin.H{"success": true, "options": options})
}

func finishPasskeyLogin(c *gin.Context, userType, cookieName string) {
	limiterKey := passkeyLimiterKey(userType)
	if blocked, remaining := auth.DefaultLoginRateLimiter.IsBlocked(c.ClientIP(), limiterKey); blocked {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"success":         false,
			"error":           fmt.Sprintf("too many failed attempts, try again in %d seconds", int(remaining.Seconds())),
			"retry_after_sec": int(remaining.Seconds()),
		})
		return
	}
	token, err := c.Cookie(cookieName)
	if err != nil || token == "" {
		auth.DefaultLoginRateLimiter.RecordFailure(c.ClientIP(), limiterKey)
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "passkey login challenge expired"})
		return
	}
	db, ok := webAuthnDB(c)
	if !ok {
		return
	}
	wa, err := service.NewWebAuthnService(db, c.Request)
	if err != nil {
		httpcookie.SetAuth(c, cookieName, "", -1)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "passkey login unavailable"})
		return
	}
	result, err := wa.FinishPasskeyLogin(userType, token, c.Request)
	httpcookie.SetAuth(c, cookieName, "", -1)
	if err != nil {
		auth.DefaultLoginRateLimiter.RecordFailure(c.ClientIP(), limiterKey)
		log.Printf("[SECURITY] passkey login failed user_type=%s ip=%s err=%v", userType, c.ClientIP(), err)
		auth.LogTOTPAuditEvent(auth.TOTPAuditEvent{EventType: "PASSKEY_LOGIN_FAILED", IsCustomer: userType == service.WebAuthnUserTypeCustomer, ClientIP: c.ClientIP(), Success: false, Details: "passkey assertion failed"})
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "passkey login failed"})
		return
	}

	switch result.UserType {
	case service.WebAuthnUserTypeAgent:
		session, err := activeAgentPasskeySession(db, result.UserKey, c)
		if err != nil {
			auth.DefaultLoginRateLimiter.RecordFailure(c.ClientIP(), limiterKey)
			auth.LogTOTPAuditEvent(auth.TOTPAuditEvent{EventType: "PASSKEY_LOGIN_FAILED", UserLogin: result.UserKey, ClientIP: c.ClientIP(), Success: false, Details: "agent account inactive or unavailable"})
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "passkey login failed"})
			return
		}
		auth.DefaultLoginRateLimiter.RecordSuccess(c.ClientIP(), limiterKey)
		auth.LogTOTPAuditEvent(auth.TOTPAuditEvent{EventType: "PASSKEY_LOGIN_SUCCESS", UserID: session.UserID, UserLogin: session.Username, ClientIP: c.ClientIP(), Success: true, Details: "agent passkey login"})
		completeAgentSecondFactorLogin(c, db, session)
	case service.WebAuthnUserTypeCustomer:
		session, err := activeCustomerPasskeySession(db, result.UserKey, c)
		if err != nil {
			auth.DefaultLoginRateLimiter.RecordFailure(c.ClientIP(), limiterKey)
			auth.LogTOTPAuditEvent(auth.TOTPAuditEvent{EventType: "PASSKEY_LOGIN_FAILED", UserLogin: result.UserKey, IsCustomer: true, ClientIP: c.ClientIP(), Success: false, Details: "customer account inactive or unavailable"})
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "passkey login failed"})
			return
		}
		auth.DefaultLoginRateLimiter.RecordSuccess(c.ClientIP(), limiterKey)
		auth.LogTOTPAuditEvent(auth.TOTPAuditEvent{EventType: "PASSKEY_LOGIN_SUCCESS", UserLogin: session.UserLogin, IsCustomer: true, ClientIP: c.ClientIP(), Success: true, Details: "customer passkey login"})
		completeCustomerSecondFactorLogin(c, db, session)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "passkey login failed"})
	}
}

func passkeyLimiterKey(userType string) string {
	return "__passkey_login__:" + userType
}

func activeAgentPasskeySession(db *sql.DB, userKey string, c *gin.Context) (*auth.PendingTOTPSession, error) {
	userID, err := strconv.Atoi(userKey)
	if err != nil || userID <= 0 {
		return nil, errorsText("invalid passkey account")
	}
	userRepo := repository.NewUserRepository(db)
	user, err := userRepo.GetByID(uint(userID))
	if err != nil || user == nil || !user.IsActive() {
		return nil, errorsText("account inactive")
	}
	return &auth.PendingTOTPSession{
		UserID:     userID,
		Username:   user.Login,
		IsCustomer: false,
		ClientIP:   c.ClientIP(),
		UserAgent:  c.Request.UserAgent(),
	}, nil
}

func activeCustomerPasskeySession(db *sql.DB, login string, c *gin.Context) (*auth.PendingTOTPSession, error) {
	var validID int
	query := database.ConvertPlaceholders("SELECT valid_id FROM customer_user WHERE login = ?")
	if err := db.QueryRow(query, login).Scan(&validID); err != nil {
		return nil, err
	}
	if validID != 1 {
		return nil, errorsText("account inactive")
	}
	return &auth.PendingTOTPSession{
		UserLogin:  login,
		IsCustomer: true,
		ClientIP:   c.ClientIP(),
		UserAgent:  c.Request.UserAgent(),
	}, nil
}

func verifyAgentPasswordAndDisplayName(db *sql.DB, userID int, password string) (string, error) {
	userRepo := repository.NewUserRepository(db)
	user, err := userRepo.GetByID(uint(userID))
	if err != nil || user == nil {
		return "", errorsText("user not found")
	}
	if !auth.NewPasswordHasher().VerifyPassword(password, user.Password) {
		return "", errorsText("incorrect password")
	}
	display := strings.TrimSpace(user.FirstName + " " + user.LastName)
	if display == "" {
		display = user.Login
	}
	return display, nil
}

type errorsText string

func (e errorsText) Error() string { return string(e) }

func listWebAuthnCredentials(c *gin.Context, userType, userKey string) {
	db, ok := webAuthnDB(c)
	if !ok {
		return
	}
	wa, err := service.NewWebAuthnService(db, c.Request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "security key management unavailable"})
		return
	}
	records, err := wa.ListCredentials(userType, userKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to load security keys"})
		return
	}
	credentials := make([]gin.H, 0, len(records))
	for i := range records {
		credentials = append(credentials, publicWebAuthnCredential(&records[i]))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "credentials": credentials})
}

func renameWebAuthnCredential(c *gin.Context, userType, userKey string) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "name is required"})
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid credential id"})
		return
	}
	db, ok := webAuthnDB(c)
	if !ok {
		return
	}
	wa, _ := service.NewWebAuthnService(db, c.Request)
	if err := wa.RenameCredential(userType, userKey, id, req.Name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "failed to rename security key"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func deleteWebAuthnCredential(c *gin.Context, db *sql.DB, userType, userKey string) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid credential id"})
		return
	}
	wa, _ := service.NewWebAuthnService(db, c.Request)
	if err := wa.DeleteCredential(userType, userKey, id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "failed to remove security key"})
		return
	}
	auth.LogTOTPAuditEvent(auth.TOTPAuditEvent{EventType: "WEBAUTHN_REMOVED", UserLogin: userKey, IsCustomer: userType == service.WebAuthnUserTypeCustomer, ClientIP: c.ClientIP(), Success: true, Details: "security key removed"})
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func publicWebAuthnCredential(rec *service.WebAuthnCredentialRecord) gin.H {
	out := gin.H{
		"id":            rec.ID,
		"name":          rec.Name,
		"sign_count":    rec.SignCount,
		"credential_id": rec.CredentialID,
		"created_at":    rec.CreatedAt,
		"updated_at":    rec.UpdatedAt,
	}
	if rec.LastUsedAt != nil {
		out["last_used_at"] = rec.LastUsedAt
	}
	return out
}

func pendingAgent2FASession(c *gin.Context) (*auth.PendingTOTPSession, bool) {
	token, err := c.Cookie("2fa_pending")
	if err != nil || token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "no pending 2FA session"})
		return nil, false
	}
	session := auth.GetTOTPSessionManager().ValidateAndGetSession(token, c.ClientIP(), c.Request.UserAgent())
	if session == nil || session.IsCustomer {
		httpcookie.SetAuth(c, "2fa_pending", "", -1)
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "invalid or expired 2FA session"})
		return nil, false
	}
	return session, true
}

func pendingCustomer2FASession(c *gin.Context) (*auth.PendingTOTPSession, bool) {
	token, err := c.Cookie("customer_2fa_pending")
	if err != nil || token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "no pending 2FA session"})
		return nil, false
	}
	session := auth.GetTOTPSessionManager().ValidateAndGetSession(token, c.ClientIP(), c.Request.UserAgent())
	if session == nil || !session.IsCustomer {
		httpcookie.SetAuth(c, "customer_2fa_pending", "", -1)
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "invalid or expired 2FA session"})
		return nil, false
	}
	return session, true
}

func completeAgentSecondFactorLogin(c *gin.Context, db *sql.DB, session *auth.PendingTOTPSession) {
	jwtManager := shared.GetJWTManager()
	var token string
	if jwtManager != nil {
		role, isAdmin := resolveUserRole(uint(session.UserID))
		tokenStr, err := jwtManager.GenerateTokenWithLogin(uint(session.UserID), session.Username, session.Username, role, isAdmin, 1)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to generate token"})
			return
		}
		token = tokenStr
	} else {
		token = fmt.Sprintf("demo_session_%d_%d", session.UserID, time.Now().Unix())
	}

	sessionTimeout := constants.DefaultSessionTimeout
	prefService := service.NewUserPreferencesService(db)
	if userTimeout := prefService.GetSessionTimeout(session.UserID); userTimeout > 0 {
		sessionTimeout = userTimeout
	}
	httpcookie.SetAuth(c, "customer_access_token", "", -1)
	httpcookie.SetAuth(c, "customer_auth_token", "", -1)
	httpcookie.SetAuth(c, "customer_session_id", "", -1)
	httpcookie.SetAuthState(c, "goatflow_customer_logged_in", "", -1)
	httpcookie.SetAuth(c, "access_token", token, sessionTimeout)
	httpcookie.SetAuth(c, "auth_token", token, sessionTimeout)
	httpcookie.SetAuthState(c, "goatflow_logged_in", "1", sessionTimeout)
	if userTheme := prefService.GetTheme(session.UserID); userTheme != "" {
		c.SetCookie("goatflow_theme", userTheme, sessionTimeout, "/", "", false, false)
	}
	if userThemeMode := prefService.GetThemeMode(session.UserID); userThemeMode != "" {
		c.SetCookie("goatflow_mode", userThemeMode, sessionTimeout, "/", "", false, false)
	}
	if sessionSvc := shared.GetSessionService(); sessionSvc != nil {
		sessionID, err := sessionSvc.CreateSession(session.UserID, session.Username, "User", c.ClientIP(), c.Request.UserAgent())
		if err != nil {
			log.Printf("Failed to create session record: %v", err)
		} else {
			httpcookie.SetAuth(c, "session_id", sessionID, sessionTimeout)
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "redirect": "/dashboard"})
}

func completeCustomerSecondFactorLogin(c *gin.Context, db *sql.DB, session *auth.PendingTOTPSession) {
	jwtManager := shared.GetJWTManager()
	if jwtManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "authentication not configured"})
		return
	}
	var userID uint
	var email, firstName, lastName string
	query := database.ConvertPlaceholders("SELECT id, email, first_name, last_name FROM customer_user WHERE login = ?")
	if err := db.QueryRow(query, session.UserLogin).Scan(&userID, &email, &firstName, &lastName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to load user"})
		return
	}
	_ = firstName
	_ = lastName
	jwtToken, err := jwtManager.GenerateTokenWithLogin(userID, session.UserLogin, email, "Customer", false, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to generate token"})
		return
	}
	sessionTimeout := 86400
	httpcookie.SetAuth(c, "access_token", "", -1)
	httpcookie.SetAuth(c, "auth_token", "", -1)
	httpcookie.SetAuth(c, "session_id", "", -1)
	httpcookie.SetAuthState(c, "goatflow_logged_in", "", -1)
	httpcookie.SetAuth(c, "customer_access_token", jwtToken, sessionTimeout)
	httpcookie.SetAuth(c, "customer_auth_token", jwtToken, sessionTimeout)
	httpcookie.SetAuthState(c, "goatflow_customer_logged_in", "1", sessionTimeout)
	c.JSON(http.StatusOK, gin.H{"success": true, "access_token": jwtToken, "redirect": "/customer"})
}
