package api

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/auth"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/httpcookie"
	"github.com/goatkit/goatflow/internal/platform/models"
	"github.com/goatkit/goatflow/internal/platform/shared"
	"github.com/goatkit/goatflow/internal/repository"
)

func handleOIDCRedirect(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	providerID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil || providerID == 0 {
		c.Redirect(http.StatusFound, "/login?error=invalid_provider")
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		log.Printf("OIDC redirect: database unavailable: %v", err)
		c.Redirect(http.StatusFound, "/login?error=server_error")
		return
	}

	repo := repository.NewIdentityProviderRepository(db)
	provider, err := repo.GetProvider(uint(providerID))
	if err != nil {
		c.Redirect(http.StatusFound, "/login?error=provider_not_found")
		return
	}

	if !provider.Enabled {
		c.Redirect(http.StatusFound, "/login?error=provider_disabled")
		return
	}

	state := generateState()
	codeVerifier := generateCodeVerifier()

	stateStore := auth.GetStateStore()
	if stateStore != nil {
		entry := auth.StateData{
			ProviderID:     uint(providerID),
			ProviderType:   provider.ProviderType,
			CodeVerifier:   codeVerifier,
			BrowserBinding: bindSSOToBrowser(c, false),
		}
		if err := stateStore.StoreState(state, entry); err != nil {
			c.Redirect(http.StatusFound, "/login?error=server_error")
			return
		}
	}

	// Build provider config from DB
	redirectURL := shared.BuildRedirectURL(c, "/auth/"+idStr+"/callback")
	cfg := &auth.OidcConfig{
		DiscoveryURL:  provider.DiscoveryURL,
		ClientID:      provider.ClientID,
		ClientSecret:  provider.ClientSecret,
		RedirectURL:   redirectURL,
		Scopes:        provider.Scopes,
		ClaimEmail:    provider.UserClaimEmail,
		ClaimName:     provider.UserClaimName,
		ClaimGroups:   provider.UserClaimGroups,
		AutoProvision: provider.AutoProvision,
	}

	prov := auth.NewOidcProvider(cfg, auth.ProviderDependencies{
		OIDCClient: auth.GetOIDCClient(),
		StateStore: stateStore,
	})

	authURL, err := prov.StartAuthFlow(c.Request.Context(), state, codeVerifier)
	if err != nil {
		log.Printf("OIDC StartAuthFlow failed for provider %d: %v", providerID, err)
		c.Redirect(http.StatusFound, "/login?error=auth_failed")
		return
	}

	c.Redirect(http.StatusFound, authURL)
}

// handleOIDCCallback processes the callback from the OIDC/OAuth2 provider.
func handleOIDCCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	if code == "" || state == "" {
		c.Redirect(http.StatusFound, "/login?error=missing_params")
		return
	}

	stateStore := auth.GetStateStore()
	if stateStore == nil {
		c.Redirect(http.StatusFound, "/login?error=server_error")
		return
	}

	// Peek at state to get provider ID — CompleteAuthFlow will consume it.
	// The state must come back from the browser that started the login; a
	// mismatch burns the state so the captured callback URL cannot be retried.
	entry, ok := stateStore.GetState(state)
	if !ok || !ssoBrowserMatches(c, entry.BrowserBinding) {
		stateStore.ConsumeState(state)
		c.Redirect(http.StatusFound, "/login?error=invalid_state")
		return
	}
	providerID, providerType := entry.ProviderID, entry.ProviderType

	// Validate provider exists in DB
	db, err := database.GetDB()
	if err != nil || db == nil {
		log.Printf("OIDC callback: database unavailable: %v", err)
		c.Redirect(http.StatusFound, "/login?error=server_error")
		return
	}

	repo := repository.NewIdentityProviderRepository(db)
	provider, err := repo.GetProvider(uint(providerID))
	if err != nil {
		c.Redirect(http.StatusFound, "/login?error=provider_not_found")
		return
	}

	if !provider.Enabled {
		c.Redirect(http.StatusFound, "/login?error=provider_disabled")
		return
	}

	// Verify provider type matches
	if providerType != provider.ProviderType {
		c.Redirect(http.StatusFound, "/login?error=provider_mismatch")
		return
	}

	// Build provider config from DB — must include same RedirectURL used in redirect
	callbackRedirectURL := shared.BuildRedirectURL(c, "/auth/"+strconv.FormatUint(uint64(providerID), 10)+"/callback")
	cfg := &auth.OidcConfig{
		DiscoveryURL:  provider.DiscoveryURL,
		ClientID:      provider.ClientID,
		ClientSecret:  provider.ClientSecret,
		RedirectURL:   callbackRedirectURL,
		Scopes:        provider.Scopes,
		ClaimEmail:    provider.UserClaimEmail,
		ClaimName:     provider.UserClaimName,
		ClaimGroups:   provider.UserClaimGroups,
		AutoProvision: provider.AutoProvision,
		UserTable:     provider.UserTable,
	}

	prov := auth.NewOidcProvider(cfg, auth.ProviderDependencies{
		DB:         db,
		UserRepo:   repository.NewUserRepository(db),
		OIDCClient: auth.GetOIDCClient(),
		StateStore: stateStore,
	})

	user, err := prov.CompleteAuthFlow(c.Request.Context(), code, state)
	if err != nil {
		log.Printf("OIDC CompleteAuthFlow failed for provider %d: %v", providerID, err)
		c.Redirect(http.StatusFound, "/login?error=auth_failed")
		return
	}

	mfaRequired, err := needs2FA(user.ID)
	if err != nil {
		log.Printf("OIDC login: second-factor status for user %d unavailable: %v", user.ID, err)
		c.Redirect(http.StatusFound, "/login?error=server_error")
		return
	}
	if mfaRequired {
		sessionMgr := auth.GetTOTPSessionManager()
		if sessionMgr != nil {
			token, err := sessionMgr.CreateAgentSession(int(user.ID), user.Login, c.ClientIP(), c.Request.UserAgent())
			if err != nil {
				c.Redirect(http.StatusFound, "/login?error=session_error")
				return
			}
			httpcookie.SetAuth(c, "2fa_pending", token, 300)
		}
		c.Redirect(http.StatusFound, "/login/2fa")
		return
	}

	createSession(c, user)
}

func generateState() string {
	buf := make([]byte, 32)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

func generateCodeVerifier() string {
	buf := make([]byte, 32)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

// needs2FA reports whether an SSO-authenticated agent must still pass the
// second factor, using the same TOTP/passkey status as the password login.
// Lookup failures are returned so callers refuse the login (fail closed).
func needs2FA(userID uint) (bool, error) {
	db, err := database.GetDB()
	if err != nil {
		return false, err
	}
	status, err := agentMFAStatus(db, int(userID))
	if err != nil {
		return false, err
	}
	return status.Enabled(), nil
}

// createSession completes an SSO (OIDC, SAML) login: it creates the sessions
// row, so the login is listed and killable under /admin/sessions like a
// password login, and issues the token bound to it.
func createSession(c *gin.Context, user *models.User) {
	jwtMgr := shared.GetJWTManager()
	if jwtMgr == nil {
		c.Redirect(http.StatusFound, "/login?error=server_error")
		return
	}

	sessionID, err := newLoginSession(c, int(user.ID), user.Login, user.Role) // #nosec G115 -- users.id fits int
	if err != nil {
		log.Printf("sso login: create session for %s: %v", user.Login, err)
		c.Redirect(http.StatusFound, "/login?error=server_error")
		return
	}
	role := user.Role
	isAdmin := user.Role == "Admin"
	token, err := jwtMgr.GenerateTokenWithLogin(sessionID, user.ID, user.Login, user.Email, role, isAdmin, 0)
	if err != nil {
		c.Redirect(http.StatusFound, "/login?error=server_error")
		return
	}

	sessionTimeout := 86400
	httpcookie.SetAuth(c, "access_token", token, sessionTimeout)
	httpcookie.SetAuth(c, "auth_token", token, sessionTimeout)
	httpcookie.SetAuth(c, "session_id", sessionID, sessionTimeout)
	httpcookie.SetAuthState(c, "goatflow_logged_in", "1", sessionTimeout)
	c.Redirect(http.StatusFound, "/dashboard")
}
