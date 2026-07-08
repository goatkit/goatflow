package api

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/models"
	"github.com/goatkit/goatflow/internal/platform/routing"
	"github.com/goatkit/goatflow/internal/repository"
)

func init() {
	routing.RegisterHandler("handleAdminIdentityProviders", handleAdminIdentityProviders)
	routing.RegisterHandler("handleAdminIdentityProviderNew", handleAdminIdentityProviderNew)
	routing.RegisterHandler("handleAdminIdentityProviderCreate", handleAdminIdentityProviderCreate)
	routing.RegisterHandler("handleAdminIdentityProviderEdit", handleAdminIdentityProviderEdit)
	routing.RegisterHandler("handleAdminIdentityProviderUpdate", handleAdminIdentityProviderUpdate)
	routing.RegisterHandler("handleAdminIdentityProviderDelete", handleAdminIdentityProviderDelete)
	routing.RegisterHandler("handleAdminIdentityProviderToggle", handleAdminIdentityProviderToggle)
}

// handleAdminIdentityProviders lists all identity providers.
func handleAdminIdentityProviders(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil || db == nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	repo := repository.NewIdentityProviderRepository(db)
	providers, err := repo.ListProviders()
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to fetch identity providers")
		return
	}

	providerList := make([]gin.H, 0, len(providers))
	for _, p := range providers {
		orgStr := "Global"
		if p.OrgID != nil {
			orgStr = fmt.Sprintf("Org %d", *p.OrgID)
		}
		providerList = append(providerList, gin.H{
			"ID":            p.ID,
			"OrgID":         p.OrgID,
			"OrgStr":        orgStr,
			"Name":          p.Name,
			"ProviderType":  p.ProviderType,
			"ClientID":      p.ClientID,
			"DiscoveryURL":  p.DiscoveryURL,
			"Scopes":        p.Scopes,
			"Enabled":       p.Enabled,
			"AutoProvision": p.AutoProvision,
			"CreateTime":    p.CreateTime.Format("2006-01-02 15:04"),
		})
	}

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/identity_providers.pongo2", pongo2.Context{
		"Providers":  providerList,
		"ActivePage": "admin",
	})
}

// handleAdminIdentityProviderNew shows the new provider form.
func handleAdminIdentityProviderNew(c *gin.Context) {
	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/identity_provider_form.pongo2", pongo2.Context{
		"ActivePage": "admin",
		"FormAction": "/admin/identity-providers",
		"Method":     "POST",
	})
}

// handleAdminIdentityProviderCreate creates a new identity provider.
func handleAdminIdentityProviderCreate(c *gin.Context) {
	name := strings.TrimSpace(c.PostForm("name"))
	providerType := c.PostForm("provider_type")
	clientID := strings.TrimSpace(c.PostForm("client_id"))
	discoveryURL := strings.TrimSpace(c.PostForm("discovery_url"))
	signingCert := strings.TrimSpace(c.PostForm("signing_cert"))
	privateKey := strings.TrimSpace(c.PostForm("private_key"))
	entityID := strings.TrimSpace(c.PostForm("entity_id"))
	acsURL := strings.TrimSpace(c.PostForm("acs_url"))
	scopes := strings.TrimSpace(c.PostForm("scopes"))
	clientSecret := c.PostForm("client_secret")
	claimEmail := strings.TrimSpace(c.PostForm("user_claim_email"))
	claimName := strings.TrimSpace(c.PostForm("user_claim_name"))
	claimGroups := strings.TrimSpace(c.PostForm("user_claim_groups"))
	enabled := c.PostForm("enabled") == "1"
	autoProvision := c.PostForm("auto_provision") == "1"
	userTable := c.PostForm("user_table")
	if userTable == "" {
		userTable = "users"
	}

	if name == "" || providerType == "" || clientID == "" {
		sendErrorResponse(c, http.StatusBadRequest, "Name, provider type, and client ID are required")
		return
	}

	switch providerType {
	case "oidc", "google", "github", "saml2":
		// valid
	default:
		sendErrorResponse(c, http.StatusBadRequest, "Invalid provider type")
		return
	}

	var orgID *uint
	if orgIDStr := c.PostForm("org_id"); orgIDStr != "" {
		if id, err := strconv.ParseUint(orgIDStr, 10, 64); err == nil && id > 0 {
			u := uint(id)
			orgID = &u
		}
	}

	// Validate discovery URL for OIDC providers
	if providerType == "oidc" && discoveryURL != "" {
		if !isValidURL(discoveryURL) {
			sendErrorResponse(c, http.StatusBadRequest, "Invalid discovery URL format")
			return
		}
	}

	// Validate SAML2 specific fields (required)
	if providerType == "saml2" {
		if discoveryURL == "" {
			sendErrorResponse(c, http.StatusBadRequest, "Metadata URL is required for SAML2 providers")
			return
		}
		if !isValidURL(discoveryURL) {
			sendErrorResponse(c, http.StatusBadRequest, "Invalid metadata URL format")
			return
		}
		if signingCert == "" {
			sendErrorResponse(c, http.StatusBadRequest, "Signing certificate is required for SAML2 providers")
			return
		}
		if privateKey == "" {
			sendErrorResponse(c, http.StatusBadRequest, "Private key is required for SAML2 providers")
			return
		}
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	now := time.Now()
	p := &models.IdentityProvider{
		Name:            name,
		ProviderType:    providerType,
		ClientID:        clientID,
		ClientSecret:    clientSecret,
		DiscoveryURL:    discoveryURL,
		SigningCert:     signingCert,
		PrivateKey:      privateKey,
		EntityID:        entityID,
		ACSURL:          acsURL,
		Scopes:          scopes,
		UserClaimEmail:  claimEmail,
		UserClaimName:   claimName,
		UserClaimGroups: claimGroups,
		OrgID:           orgID,
		Enabled:         enabled,
		AutoProvision:   autoProvision,
		UserTable:       userTable,
		CreateTime:      now,
		CreateBy:        0,
		ChangeTime:      now,
		ChangeBy:        0,
	}

	repo := repository.NewIdentityProviderRepository(db)
	if err := repo.CreateProvider(p); err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to create provider: %v", err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "id": p.ID})
}

// handleAdminIdentityProviderEdit shows the edit form for an existing provider.
func handleAdminIdentityProviderEdit(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		sendErrorResponse(c, http.StatusBadRequest, "Invalid provider ID")
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	repo := repository.NewIdentityProviderRepository(db)
	p, err := repo.GetProvider(uint(id))
	if err != nil {
		sendErrorResponse(c, http.StatusNotFound, "Identity provider not found")
		return
	}

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/identity_provider_form.pongo2", pongo2.Context{
		"ActivePage": "admin",
		"FormAction": "/admin/identity-providers/" + idStr,
		"Method":     "PUT",
		"Provider": gin.H{
			"ID":              p.ID,
			"OrgID":           p.OrgID,
			"Name":            p.Name,
			"ProviderType":    p.ProviderType,
			"ClientID":        p.ClientID,
			"ClientSecret":    p.ClientSecret,
			"DiscoveryURL":    p.DiscoveryURL,
			"SigningCert":     p.SigningCert,
			"EntityID":        p.EntityID,
			"ACSURL":          p.ACSURL,
			"Scopes":          p.Scopes,
			"UserClaimEmail":  p.UserClaimEmail,
			"UserClaimName":   p.UserClaimName,
			"UserClaimGroups": p.UserClaimGroups,
			"Enabled":         p.Enabled,
			"AutoProvision":   p.AutoProvision,
		},
	})
}

// handleAdminIdentityProviderUpdate updates an existing identity provider.
func handleAdminIdentityProviderUpdate(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		sendErrorResponse(c, http.StatusBadRequest, "Invalid provider ID")
		return
	}

	name := strings.TrimSpace(c.PostForm("name"))
	providerType := c.PostForm("provider_type")
	clientID := strings.TrimSpace(c.PostForm("client_id"))
	discoveryURL := strings.TrimSpace(c.PostForm("discovery_url"))
	signingCert := strings.TrimSpace(c.PostForm("signing_cert"))
	privateKey := strings.TrimSpace(c.PostForm("private_key"))
	entityID := strings.TrimSpace(c.PostForm("entity_id"))
	acsURL := strings.TrimSpace(c.PostForm("acs_url"))
	scopes := strings.TrimSpace(c.PostForm("scopes"))
	clientSecret := c.PostForm("client_secret")
	claimEmail := strings.TrimSpace(c.PostForm("user_claim_email"))
	claimName := strings.TrimSpace(c.PostForm("user_claim_name"))
	claimGroups := strings.TrimSpace(c.PostForm("user_claim_groups"))
	enabled := c.PostForm("enabled") == "1"
	autoProvision := c.PostForm("auto_provision") == "1"
	userTable := c.PostForm("user_table")
	if userTable == "" {
		userTable = "users"
	}

	if name == "" || providerType == "" || clientID == "" {
		sendErrorResponse(c, http.StatusBadRequest, "Name, provider type, and client ID are required")
		return
	}

	switch providerType {
	case "oidc", "google", "github", "saml2":
		// valid
	default:
		sendErrorResponse(c, http.StatusBadRequest, "Invalid provider type")
		return
	}

	var orgID *uint
	if orgIDStr := c.PostForm("org_id"); orgIDStr != "" {
		if oid, err := strconv.ParseUint(orgIDStr, 10, 64); err == nil && oid > 0 {
			u := uint(oid)
			orgID = &u
		}
	}

	// Validate discovery URL for OIDC providers
	if providerType == "oidc" && discoveryURL != "" {
		if !isValidURL(discoveryURL) {
			sendErrorResponse(c, http.StatusBadRequest, "Invalid discovery URL format")
			return
		}
	}

	// Validate SAML2 specific fields (required)
	if providerType == "saml2" {
		if discoveryURL == "" {
			sendErrorResponse(c, http.StatusBadRequest, "Metadata URL is required for SAML2 providers")
			return
		}
		if !isValidURL(discoveryURL) {
			sendErrorResponse(c, http.StatusBadRequest, "Invalid metadata URL format")
			return
		}
		if signingCert == "" {
			sendErrorResponse(c, http.StatusBadRequest, "Signing certificate is required for SAML2 providers")
			return
		}
		if privateKey == "" {
			sendErrorResponse(c, http.StatusBadRequest, "Private key is required for SAML2 providers")
			return
		}
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	repo := repository.NewIdentityProviderRepository(db)
	p, err := repo.GetProvider(uint(id))
	if err != nil {
		sendErrorResponse(c, http.StatusNotFound, "Identity provider not found")
		return
	}

	p.Name = name
	p.ProviderType = providerType
	p.ClientID = clientID
	if clientSecret != "" {
		p.ClientSecret = clientSecret
	}
	p.DiscoveryURL = discoveryURL
	p.SigningCert = signingCert
	p.PrivateKey = privateKey
	p.EntityID = entityID
	p.ACSURL = acsURL
	p.Scopes = scopes
	p.UserClaimEmail = claimEmail
	p.UserClaimName = claimName
	p.UserClaimGroups = claimGroups
	p.OrgID = orgID
	p.Enabled = enabled
	p.AutoProvision = autoProvision
	p.UserTable = userTable
	p.ChangeTime = time.Now()
	p.ChangeBy = 0

	if err := repo.UpdateProvider(p); err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to update provider: %v", err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// handleAdminIdentityProviderDelete deletes an identity provider.
func handleAdminIdentityProviderDelete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		sendErrorResponse(c, http.StatusBadRequest, "Invalid provider ID")
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	repo := repository.NewIdentityProviderRepository(db)
	if err := repo.DeleteProvider(uint(id)); err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to delete provider")
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// handleAdminIdentityProviderToggle enables or disables a provider.
func handleAdminIdentityProviderToggle(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		sendErrorResponse(c, http.StatusBadRequest, "Invalid provider ID")
		return
	}

	action := c.Param("action")
	if action != "enable" && action != "disable" {
		sendErrorResponse(c, http.StatusBadRequest, "Invalid action")
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	repo := repository.NewIdentityProviderRepository(db)
	p, err := repo.GetProvider(uint(id))
	if err != nil {
		sendErrorResponse(c, http.StatusNotFound, "Provider not found")
		return
	}

	if action == "enable" {
		p.Enabled = true
	} else {
		p.Enabled = false
	}
	p.ChangeTime = time.Now()
	p.ChangeBy = 0

	if err := repo.UpdateProvider(p); err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to update provider")
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "enabled": p.Enabled})
}

// isValidURL checks if a string is a valid URL.
func isValidURL(u string) bool {
	_, err := url.ParseRequestURI(u)
	return err == nil
}