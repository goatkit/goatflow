package auth

import (
	"context"
	"crypto"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/crewjam/saml"

	"github.com/goatkit/goatflow/internal/platform/models"
)

const (
	saml2Name        = "saml2"
	saml2DisplayName = "SAML 2.0"
	saml2Priority    = 3
)

// SAMLConfig holds SAML2 service provider configuration derived from an IdentityProvider row.
type SAMLConfig struct {
	EntityID       string // SP entity ID
	AcsURL         string // Assertion Consumer Service URL on this host
	IdPMetadataURL string // IdP metadata endpoint URL
	IdPMetadataXML string // raw IdP metadata XML (alternative to URL)
	SigningCert    string // PEM-encoded X.509 certificate for signing
	PrivateKey     string // PEM-encoded private key
	UserClaimEmail string // attribute name/oid mapped to email (default "email")
	UserClaimName  string // attribute name/oid mapped to display name (default "name")
	UserClaimGroups string // attribute name mapped to groups (default "groups")
	AutoProvision  bool   // create users not found in the database
	UserTable      string // "users" (agent) or "customer" (service_customer_user)
}

// samlProvider implements AuthProvider for SAML2 SP-initiated login flow.
type samlProvider struct {
	sp            *saml.ServiceProvider
	userRepo      UserLookup
	stateStore    StateStore
	autoProvision bool
	claimEmail    string
	claimName     string
	claimGroups   string
	userTable     string
	db            *sql.DB
}

// NewSAML2Provider creates a new SAML2 provider from the given configuration.
func NewSAML2Provider(cfg *SAMLConfig, deps ProviderDependencies) (*samlProvider, error) {
	if cfg == nil {
		return nil, fmt.Errorf("saml config required")
	}

	metadata, err := loadIdPMetadata(cfg.IdPMetadataXML, cfg.IdPMetadataURL)
	if err != nil {
		return nil, err
	}

	entityID := cfg.EntityID
	if entityID == "" {
		entityID = cfg.AcsURL + "/saml/metadata"
	}
	acsURL, err := parseURL(cfg.AcsURL)
	if err != nil {
		return nil, fmt.Errorf("parse ACS URL: %w", err)
	}
	metadataURL, err := parseURL(entityID + "/metadata")
	if err != nil {
		return nil, fmt.Errorf("parse metadata URL: %w", err)
	}

	sp := &saml.ServiceProvider{
		EntityID:    entityID,
		MetadataURL: *metadataURL,
		AcsURL:      *acsURL,
		IDPMetadata: metadata,
	}

	if cfg.PrivateKey != "" && cfg.SigningCert != "" {
		keySigner, err := parsePrivateKey([]byte(cfg.PrivateKey))
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		cert, err := parseCertificate([]byte(cfg.SigningCert))
		if err != nil {
			return nil, fmt.Errorf("parse certificate: %w", err)
		}
		sp.Key = keySigner
		sp.Certificate = cert
	}

	emailClaim := cfg.UserClaimEmail
	if emailClaim == "" {
		emailClaim = "email"
	}
	nameClaim := cfg.UserClaimName
	if nameClaim == "" {
		nameClaim = "name"
	}
	groupsClaim := cfg.UserClaimGroups
	if groupsClaim == "" {
		groupsClaim = "groups"
	}
	userTable := cfg.UserTable
	if userTable == "" {
		userTable = "users"
	}

	return &samlProvider{
		sp:            sp,
		userRepo:      deps.UserRepo,
		stateStore:    deps.StateStore,
		autoProvision: cfg.AutoProvision,
		claimEmail:    emailClaim,
		claimName:     nameClaim,
		claimGroups:   groupsClaim,
		userTable:     userTable,
		db:            deps.DB,
	}, nil
}

// loadIdPMetadata loads IdP metadata from inline XML first, then falls back to URL fetch.
func loadIdPMetadata(metadataXML, metadataURL string) (*saml.EntityDescriptor, error) {
	if metadataXML != "" {
		var ed saml.EntityDescriptor
		if err := xml.Unmarshal([]byte(metadataXML), &ed); err != nil {
			return nil, fmt.Errorf("parse IdP metadata XML: %w", err)
		}
		return &ed, nil
	}
	if metadataURL != "" {
		return fetchIdPMetadata(metadataURL)
	}
	return nil, fmt.Errorf("IdP metadata XML or URL required")
}

// GenerateSPMetadata builds the SP metadata XML from the given configuration.
// The metadata advertises this service provider's entity ID, ACS URL, and
// optional signing certificate so an IdP can be configured to trust it.
func GenerateSPMetadata(cfg *SAMLConfig) ([]byte, error) {
	if cfg == nil {
		return nil, fmt.Errorf("saml config required")
	}
	entityID := cfg.EntityID
	if entityID == "" {
		entityID = cfg.AcsURL + "/saml/metadata"
	}
	acsURL, err := parseURL(cfg.AcsURL)
	if err != nil {
		return nil, fmt.Errorf("parse ACS URL: %w", err)
	}
	metadataURL, err := parseURL(entityID + "/metadata")
	if err != nil {
		return nil, fmt.Errorf("parse metadata URL: %w", err)
	}

	sp := &saml.ServiceProvider{
		EntityID:    entityID,
		MetadataURL: *metadataURL,
		AcsURL:      *acsURL,
	}

	if cfg.PrivateKey != "" && cfg.SigningCert != "" {
		keySigner, err := parsePrivateKey([]byte(cfg.PrivateKey))
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		cert, err := parseCertificate([]byte(cfg.SigningCert))
		if err != nil {
			return nil, fmt.Errorf("parse certificate: %w", err)
		}
		sp.Key = keySigner
		sp.Certificate = cert
	}

	metadata := sp.Metadata()
	data, err := xml.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal SP metadata: %w", err)
	}
	return append(data, '\n'), nil
}

// Name returns the provider name.
func (p *samlProvider) Name() string {
	return saml2DisplayName
}

// Priority returns the provider priority (lower = higher priority).
func (p *samlProvider) Priority() int {
	return saml2Priority
}

// Authenticate returns an error — SAML uses redirects, not passwords.
func (p *samlProvider) Authenticate(_ context.Context, _, _ string) (*models.User, error) {
	return nil, ErrAuthBackendFailed
}

// GetUser retrieves a user by identifier (email or login).
func (p *samlProvider) GetUser(_ context.Context, identifier string) (*models.User, error) {
	if p.userRepo == nil {
		return nil, fmt.Errorf("user repository not available")
	}
	return p.userRepo.GetByLogin(identifier)
}

// ValidateToken is not implemented for SAML2.
func (p *samlProvider) ValidateToken(_ context.Context, _ string) (*models.User, error) {
	return nil, fmt.Errorf("ValidateToken not supported for SAML")
}

// StartAuthFlow initiates the SAML2 SP-initiated login by generating an AuthnRequest
// and returning the IdP redirect URL. relayState is stored in StateStore for CSRF validation.
func (p *samlProvider) StartAuthFlow(_ context.Context, state string, _ string) (string, error) {
	authURL, err := p.sp.MakeRedirectAuthenticationRequest(state)
	if err != nil {
		return "", fmt.Errorf("create SAML auth request: %w", err)
	}
	return authURL.String(), nil
}

// CompleteAuthFlow completes the SAML2 authentication by parsing and validating the
// IdP POST response from the ACS endpoint. The data map must contain a *http.Request
// under the key "request".
func (p *samlProvider) CompleteAuthFlow(ctx context.Context, _ string, data map[string]interface{}) (*models.User, error) {
	req, ok := data["request"].(*http.Request)
	if !ok || req == nil {
		return nil, fmt.Errorf("missing *http.Request in auth data")
	}

	assertion, err := p.sp.ParseResponse(req, nil)
	if err != nil {
		return nil, fmt.Errorf("parse SAML response: %w", err)
	}

	email, givenName, familyName, groups := extractUserAttributes(assertion, p.claimEmail, p.claimName, p.claimGroups)
	if email == "" {
		return nil, fmt.Errorf("no email attribute in SAML assertion")
	}

	return p.lookupOrProvisionUser(ctx, email, givenName, familyName, groups)
}

// lookupOrProvisionUser finds an existing user by login or creates one if auto-provision is enabled.
func (p *samlProvider) lookupOrProvisionUser(_ context.Context, email, givenName, familyName string, groups []string) (*models.User, error) {
	if p.userRepo == nil {
		return nil, fmt.Errorf("user repository not available")
	}

	var user *models.User
	var err error

	if p.userTable == "customer" {
		if !p.autoProvision {
			return nil, fmt.Errorf("user not found and auto-provision is disabled")
		}
		user, err = p.createOAuthUser(email, givenName, familyName)
		if err != nil {
			return nil, fmt.Errorf("provision customer: %w", err)
		}
	} else {
		existing, repoErr := p.userRepo.GetByLogin(email)
		if repoErr == nil && existing != nil {
			user = existing
		} else if !p.autoProvision {
			return nil, fmt.Errorf("user not found and auto-provision is disabled")
		} else {
			user, err = p.createOAuthUser(email, givenName, familyName)
			if err != nil {
				return nil, fmt.Errorf("provision agent: %w", err)
			}
		}
	}

	if user == nil {
		return nil, fmt.Errorf("failed to lookup or provision user")
	}

	if len(groups) > 0 && p.userRepo != nil {
		p.userRepo.SyncGroups(user.ID, groups)
	}

	return user, nil
}

// createOAuthUser persists a new SAML2 user.
func (p *samlProvider) createOAuthUser(email, givenName, familyName string) (*models.User, error) {
	now := time.Now()
	user := &models.User{
		Login:      email,
		FirstName:  givenName,
		LastName:   familyName,
		Role:       "Agent",
		ValidID:    1,
		CreateTime: now,
		CreateBy:   1,
		ChangeTime: now,
		ChangeBy:   1,
	}
	if err := p.userRepo.Create(user); err != nil {
		return nil, fmt.Errorf("create SAML user: %w", err)
	}
	user.Email = email

	groupName := "users"
	if p.db != nil {
		var gid int64
		err := p.db.QueryRow("SELECT id FROM groups WHERE name = ?", groupName).Scan(&gid)
		if err == nil && gid > 0 {
			p.db.Exec("INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by) VALUES (?, ?, 'rw', NOW(), 1, NOW(), 1)", int(user.ID), gid)
		}
	}

	return user, nil
}

// parsePrivateKey parses a PEM-encoded private key and returns it as a crypto.Signer.
func parsePrivateKey(pemBytes []byte) (crypto.Signer, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("invalid PEM data for private key")
	}

	switch strings.ToLower(block.Headers["Type"]) {
	case "ec private key", "private key":
		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse EC private key: %w", err)
		}
		return key, nil
	case "rsa private key":
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse RSA private key: %w", err)
		}
		return key, nil
	default:
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse private key (unknown type %q): %w", block.Type, err)
		}
		signer, ok := key.(crypto.Signer)
		if !ok {
			return nil, fmt.Errorf("private key type %T does not support signing", key)
		}
		return signer, nil
	}
}

// parseCertificate parses a PEM-encoded X.509 certificate.
func parseCertificate(pemBytes []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("invalid PEM data for certificate")
	}
	return x509.ParseCertificate(block.Bytes)
}

// fetchIdPMetadata downloads and parses the IdP XML metadata from the given URL.
func fetchIdPMetadata(metadataURL string) (*saml.EntityDescriptor, error) {
	resp, err := http.Get(metadataURL)
	if err != nil {
		return nil, fmt.Errorf("http GET: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d fetching IdP metadata", resp.StatusCode)
	}

	var ed saml.EntityDescriptor
	if err := xml.NewDecoder(resp.Body).Decode(&ed); err != nil {
		return nil, fmt.Errorf("decode IdP metadata: %w", err)
	}
	return &ed, nil
}

// parseURL parses a URL string. Returns nil on error.
func parseURL(rawURL string) (*url.URL, error) {
	if rawURL == "" {
		return nil, fmt.Errorf("empty URL")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}
	return parsed, nil
}

// extractUserAttributes extracts email, display name components, and groups from SAML assertion attributes.
func extractUserAttributes(assertion *saml.Assertion, emailAttrName, nameAttrName, groupsAttrName string) (email, givenName, familyName string, groups []string) {
	email = extractAttributeValue(assertion, emailAttrName)

	givenName = extractAttributeValue(assertion, "givenName")
	familyName = extractAttributeValue(assertion, "sn")
	fullName := extractAttributeValue(assertion, nameAttrName)

	if givenName == "" && familyName == "" && fullName != "" {
		if idx := strings.Index(fullName, " "); idx > 0 {
			givenName = fullName[:idx]
			familyName = fullName[idx+1:]
		} else {
			givenName = fullName
		}
	}

	groups = extractAttributeValues(assertion, groupsAttrName)
	return email, givenName, familyName, groups
}

// extractAttributeValue retrieves a single attribute value from all AttributeStatements in the assertion.
func extractAttributeValue(assertion *saml.Assertion, attrName string) string {
	for _, stmt := range assertion.AttributeStatements {
		for _, attr := range stmt.Attributes {
			if strings.EqualFold(attr.Name, attrName) || strings.EqualFold(attr.FriendlyName, attrName) {
				if len(attr.Values) > 0 && attr.Values[0].Value != "" {
					return attr.Values[0].Value
				}
			}
		}
	}
	return ""
}

// extractAttributeValues collects all values for a multi-valued attribute across all AttributeStatements.
// If a single value contains commas, it is split into individual entries.
func extractAttributeValues(assertion *saml.Assertion, attrName string) []string {
	var values []string
	for _, stmt := range assertion.AttributeStatements {
		for _, attr := range stmt.Attributes {
			if !strings.EqualFold(attr.Name, attrName) && !strings.EqualFold(attr.FriendlyName, attrName) {
				continue
			}
			for _, v := range attr.Values {
				if v.Value == "" {
					continue
				}
				if strings.Contains(v.Value, ",") {
					for _, part := range strings.Split(v.Value, ",") {
						part = strings.TrimSpace(part)
						if part != "" {
							values = append(values, part)
						}
					}
				} else {
					values = append(values, strings.TrimSpace(v.Value))
				}
			}
		}
	}
	return values
}

// Register SAML2 provider factory.
func init() {
	RegisterProvider("saml2", func(deps ProviderDependencies) (AuthProvider, error) {
		return NewSAML2Provider(&SAMLConfig{}, deps)
	})
}
