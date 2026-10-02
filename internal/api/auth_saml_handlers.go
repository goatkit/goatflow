package api

import (
	"fmt"
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

// handleSAMLRedirect initiates the SAML2 SP-initiated login flow for a given provider by ID.
func handleSAMLRedirect(c *gin.Context) {
	idStr := c.Param("id")
	providerID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil || providerID == 0 {
		c.Redirect(http.StatusFound, "/login?error=invalid_provider")
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		log.Printf("SAML redirect: database unavailable: %v", err)
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

	stateStore := auth.GetStateStore()
	if stateStore == nil {
		c.Redirect(http.StatusFound, "/login?error=server_error")
		return
	}

	prov, err := auth.NewSAML2Provider(samlConfig(c, idStr, provider), auth.ProviderDependencies{
		DB:         db,
		UserRepo:   repository.NewUserRepository(db),
		StateStore: stateStore,
	})
	if err != nil {
		c.Redirect(http.StatusFound, "/login?error=saml_config_error")
		return
	}

	state := generateState()
	authURL, requestID, err := prov.StartAuthFlow(c.Request.Context(), state)
	if err != nil {
		c.Redirect(http.StatusFound, "/login?error=auth_failed")
		return
	}

	// The RelayState carries the AuthnRequest ID and the browser binding to
	// the ACS, which accepts only the response to this request from this browser.
	entry := auth.StateData{
		ProviderID:     uint(providerID),
		ProviderType:   provider.ProviderType,
		RequestID:      requestID,
		BrowserBinding: bindSSOToBrowser(c, true),
	}
	if err := stateStore.StoreState(state, entry); err != nil {
		c.Redirect(http.StatusFound, "/login?error=server_error")
		return
	}

	c.Redirect(http.StatusFound, authURL)
}

// samlConfig builds the SP configuration for a provider. The login, ACS and
// metadata endpoints must agree on it: the IdP is configured from the
// metadata and checks the AuthnRequest issuer against it, and the ACS checks
// the assertion audience against the entity ID. Without an explicit entity ID
// the SP's metadata URL is used.
func samlConfig(c *gin.Context, idStr string, provider *models.IdentityProvider) *auth.SAMLConfig {
	entityID := provider.EntityID
	if entityID == "" {
		entityID = shared.BuildRedirectURL(c, "/auth/"+idStr+"/metadata")
	}
	return &auth.SAMLConfig{
		EntityID:        entityID,
		AcsURL:          shared.BuildRedirectURL(c, "/auth/"+idStr+"/acs"),
		IdPMetadataURL:  provider.DiscoveryURL,
		IdPMetadataXML:  provider.IdPMetadataXML,
		SigningCert:     provider.SigningCert,
		PrivateKey:      provider.PrivateKey,
		UserClaimEmail:  provider.UserClaimEmail,
		UserClaimName:   provider.UserClaimName,
		UserClaimGroups: provider.UserClaimGroups,
		AutoProvision:   provider.AutoProvision,
		UserTable:       provider.UserTable,
	}
}

// handleSAMLCallback processes the SAML2 POST response from the IdP at the ACS endpoint.
func handleSAMLCallback(c *gin.Context) {
	idStr := c.Param("id")
	providerID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil || providerID == 0 {
		c.Redirect(http.StatusFound, "/login?error=missing_provider")
		return
	}

	samlResponse := c.PostForm("SAMLResponse")
	if samlResponse == "" {
		c.Redirect(http.StatusFound, "/login?error=missing_params")
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		log.Printf("SAML callback: database unavailable: %v", err)
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

	stateStore := auth.GetStateStore()
	if stateStore == nil {
		c.Redirect(http.StatusFound, "/login?error=server_error")
		return
	}
	token := c.PostForm("RelayState")
	if token == "" {
		c.Redirect(http.StatusFound, "/login?error=missing_state")
		return
	}
	// The RelayState must come back from the browser that started the login,
	// and the response must answer the AuthnRequest issued for it. The state
	// is consumed, so a response cannot be replayed.
	entry, ok := stateStore.ConsumeState(token)
	if !ok || entry.ProviderID != uint(providerID) || entry.ProviderType != provider.ProviderType ||
		!ssoBrowserMatches(c, entry.BrowserBinding) {
		c.Redirect(http.StatusFound, "/login?error=invalid_state")
		return
	}

	prov, err := auth.NewSAML2Provider(samlConfig(c, idStr, provider), auth.ProviderDependencies{
		DB:         db,
		UserRepo:   repository.NewUserRepository(db),
		StateStore: stateStore,
	})
	if err != nil {
		c.Redirect(http.StatusFound, "/login?error=server_error")
		return
	}

	user, err := prov.CompleteAuthFlow(c.Request.Context(), c.Request, entry.RequestID)
	if err != nil {
		log.Printf("SAML login for provider %d refused: %v", providerID, err)
		c.Redirect(http.StatusFound, "/login?error=auth_failed")
		return
	}

	mfaRequired, err := needs2FA(user.ID)
	if err != nil {
		log.Printf("SAML login: second-factor status for user %d unavailable: %v", user.ID, err)
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

// handleSAMLMetadata serves the SP metadata XML for auto-configuration in IdPs.
func handleSAMLMetadata(c *gin.Context) {
	idStr := c.Param("id")
	providerID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil || providerID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid provider ID"})
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database unavailable"})
		return
	}

	repo := repository.NewIdentityProviderRepository(db)
	provider, err := repo.GetProvider(uint(providerID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Provider not found"})
		return
	}

	// Generate SP metadata XML from config
	xmlBytes, err := auth.GenerateSPMetadata(samlConfig(c, idStr, provider))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to generate metadata: %v", err)})
		return
	}

	c.Data(http.StatusOK, "application/xml", xmlBytes)
}
