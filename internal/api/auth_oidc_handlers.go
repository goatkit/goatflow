package api

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/auth"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/httpcookie"
	"github.com/goatkit/goatflow/internal/platform/models"
	"github.com/goatkit/goatflow/internal/repository"
	"github.com/goatkit/goatflow/internal/platform/shared"
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
		if err := stateStore.StoreState(uint(providerID), provider.ProviderType, state, 0, codeVerifier); err != nil {
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

	// Peek at state to get provider ID — CompleteAuthFlow will consume it
	providerID, providerType, _, _, ok := stateStore.GetState(state)
	if !ok {
		c.Redirect(http.StatusFound, "/login?error=invalid_state")
		return
	}

	// Validate provider exists in DB
	db, err := database.GetDB()
	if err != nil || db == nil {
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

	if needs2FA(user.ID) {
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

func needs2FA(userID uint) bool {
	if db, err := database.GetDB(); err == nil && db != nil {
		var enabled int
		query := database.ConvertPlaceholders("SELECT COUNT(*) FROM totp_pending_session WHERE user_id = ? AND expires_at > ?")
		_ = db.QueryRow(query, userID, time.Now().Format("2006-01-02 15:04:05")).Scan(&enabled)
		return enabled > 0
	}
	return false
}

func createSession(c *gin.Context, user *models.User) {
	jwtMgr := shared.GetJWTManager()
	if jwtMgr == nil {
		c.Redirect(http.StatusFound, "/login?error=server_error")
		return
	}

	role := user.Role
	isAdmin := user.Role == "Admin"
	token, err := jwtMgr.GenerateTokenWithLogin(user.ID, user.Login, user.Email, role, isAdmin, 0)
	if err != nil {
		c.Redirect(http.StatusFound, "/login?error=server_error")
		return
	}

	sessionTimeout := 86400
	httpcookie.SetAuth(c, "access_token", token, sessionTimeout)
	httpcookie.SetAuth(c, "auth_token", token, sessionTimeout)
	httpcookie.SetAuthState(c, "goatflow_logged_in", "1", sessionTimeout)
	c.Redirect(http.StatusFound, "/dashboard")
}