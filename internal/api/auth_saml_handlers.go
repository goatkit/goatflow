package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/auth"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/httpcookie"
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

	stateStore := auth.GetStateStore()
	if stateStore != nil {
		if err := stateStore.StoreState(uint(providerID), provider.ProviderType, state, 0, ""); err != nil {
			c.Redirect(http.StatusFound, "/login?error=server_error")
			return
		}
	}

	acsURL := shared.BuildRedirectURL(c, "/auth/"+idStr+"/acs")

	cfg := &auth.SAMLConfig{
		EntityID:        provider.EntityID,
		AcsURL:          acsURL,
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

	prov, err := auth.NewSAML2Provider(cfg, auth.ProviderDependencies{
		DB:         db,
		UserRepo:   repository.NewUserRepository(db),
		StateStore: stateStore,
	})
	if err != nil {
		c.Redirect(http.StatusFound, "/login?error=saml_config_error")
		return
	}

	authURL, err := prov.StartAuthFlow(c.Request.Context(), state, acsURL)
	if err != nil {
		c.Redirect(http.StatusFound, "/login?error=auth_failed")
		return
	}

	c.Redirect(http.StatusFound, authURL)
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
	if stateStore != nil {
		token := c.PostForm("RelayState")
		if token == "" {
			c.Redirect(http.StatusFound, "/login?error=missing_state")
			return
		}
		storedProviderID, storedType, _, _, ok := stateStore.ConsumeState(token)
		if !ok || storedProviderID != uint(providerID) || storedType != provider.ProviderType {
			c.Redirect(http.StatusFound, "/login?error=invalid_state")
			return
		}
	}

	acsURL := shared.BuildRedirectURL(c, "/auth/"+idStr+"/acs")
	cfg := &auth.SAMLConfig{
		EntityID:        provider.EntityID,
		AcsURL:          acsURL,
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

	prov, err := auth.NewSAML2Provider(cfg, auth.ProviderDependencies{
		DB:         db,
		UserRepo:   repository.NewUserRepository(db),
		StateStore: stateStore,
	})
	if err != nil {
		c.Redirect(http.StatusFound, "/login?error=server_error")
		return
	}

	user, err := prov.CompleteAuthFlow(c.Request.Context(), "", map[string]interface{}{
		"request": c.Request,
	})
	if err != nil {
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

	entityID := provider.EntityID
	if entityID == "" {
		entityID = shared.BuildRedirectURL(c, "/auth/"+idStr+"/metadata")
	}
	acsURL := shared.BuildRedirectURL(c, "/auth/"+idStr+"/acs")

	cfg := &auth.SAMLConfig{
		EntityID:      entityID,
		AcsURL:        acsURL,
		SigningCert:   provider.SigningCert,
		PrivateKey:    provider.PrivateKey,
		IdPMetadataURL: provider.DiscoveryURL,
	}

	// Generate SP metadata XML from config
	xmlBytes, err := auth.GenerateSPMetadata(cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to generate metadata: %v", err)})
		return
	}

	c.Data(http.StatusOK, "application/xml", xmlBytes)
}
