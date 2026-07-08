package auth

import (
	"context"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/crewjam/saml"

	platformmodels "github.com/goatkit/goatflow/internal/platform/models"
)

const (
	saml2Name        = "saml2"
	saml2DisplayName = "SAML 2.0"
	saml2Priority    = 3
)

// SAMLConfig holds SAML2 service provider configuration derived from an IdentityProvider row.
type SAMLConfig struct {
	EntityID      string // SP entity ID
	AcsURL        string // Assertion Consumer Service URL on this host
	IdPMetadataURL string // IdP metadata endpoint URL
	SigningCert   string // PEM-encoded X.509 certificate for signing
	PrivateKey    string // PEM-encoded private key
	UserClaimEmail string // attribute name/oid mapped to email (default "email")
	UserClaimName  string // attribute name/oid mapped to display name (default "name")
	AutoProvision bool   // create users not found in the database
}

// samlProvider implements AuthProvider for SAML2 SP-initiated login flow.
type samlProvider struct {
	sp            *saml.ServiceProvider
	userRepo      UserLookup
	stateStore    StateStore
	autoProvision bool
	claimEmail    string
	claimName     string
}

// NewSAML2Provider creates a new SAML2 provider from the given configuration.
func NewSAML2Provider(cfg *SAMLConfig, deps ProviderDependencies) (*samlProvider, error) {
	if cfg == nil {
		return nil, fmt.Errorf("saml config required")
	}
	if cfg.PrivateKey == "" || cfg.SigningCert == "" {
		return nil, fmt.Errorf("signing certificate and private key required")
	}
	if cfg.IdPMetadataURL == "" {
		return nil, fmt.Errorf("IdP metadata URL required")
	}

	keySigner, err := parsePrivateKey([]byte(cfg.PrivateKey))
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	cert, err := parseCertificate([]byte(cfg.SigningCert))
	if err != nil {
		return nil, fmt.Errorf("parse certificate: %w", err)
	}

	metadata, err := fetchIdPMetadata(cfg.IdPMetadataURL)
	if err != nil {
		return nil, fmt.Errorf("fetch IdP metadata: %w", err)
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
		Key:         keySigner,
		Certificate: cert,
		MetadataURL: *metadataURL,
		AcsURL:      *acsURL,
		IDPMetadata: metadata,
	}

	emailClaim := cfg.UserClaimEmail
	if emailClaim == "" {
		emailClaim = "email"
	}
	nameClaim := cfg.UserClaimName
	if nameClaim == "" {
		nameClaim = "name"
	}

	return &samlProvider{
		sp:            sp,
		userRepo:      deps.UserRepo,
		stateStore:    deps.StateStore,
		autoProvision: cfg.AutoProvision,
		claimEmail:    emailClaim,
		claimName:     nameClaim,
	}, nil
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
func (p *samlProvider) Authenticate(_ context.Context, _, _ string) (*platformmodels.User, error) {
	return nil, ErrAuthBackendFailed
}

// GetUser retrieves a user by identifier (email or login).
func (p *samlProvider) GetUser(_ context.Context, identifier string) (*platformmodels.User, error) {
	if p.userRepo == nil {
		return nil, fmt.Errorf("user repository not available")
	}
	if strings.Contains(identifier, "@") {
		return p.userRepo.GetByEmail(identifier)
	}
	return p.userRepo.GetByLogin(identifier)
}

// ValidateToken is not implemented for SAML2.
func (p *samlProvider) ValidateToken(_ context.Context, _ string) (*platformmodels.User, error) {
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
func (p *samlProvider) CompleteAuthFlow(ctx context.Context, _ string, data map[string]interface{}) (*platformmodels.User, error) {
	_ = ctx

	req, ok := data["request"].(*http.Request)
	if !ok || req == nil {
		return nil, fmt.Errorf("missing *http.Request in auth data")
	}

	assertion, err := p.sp.ParseResponse(req, nil)
	if err != nil {
		return nil, fmt.Errorf("parse SAML response: %w", err)
	}

	email, name := extractUserAttributes(assertion, p.claimEmail, p.claimName)
	if email == "" {
		return nil, fmt.Errorf("no email attribute in SAML assertion")
	}

	return p.lookupOrProvisionUser(email, name)
}

// lookupOrProvisionUser finds an existing user by email or creates one if auto-provision is enabled.
func (p *samlProvider) lookupOrProvisionUser(email, name string) (*platformmodels.User, error) {
	if p.userRepo == nil {
		return nil, fmt.Errorf("user repository not available")
	}
	if user, err := p.userRepo.GetByEmail(email); err == nil && user != nil {
		return user, nil
	}
	if !p.autoProvision {
		return nil, fmt.Errorf("user %s not found and auto-provision is disabled", email)
	}
	return p.createOAuthUser(email, name)
}

// createOAuthUser creates a new user struct for SAML login.
func (p *samlProvider) createOAuthUser(email, name string) (*platformmodels.User, error) {
	login := email
	if idx := strings.Index(email, "@"); idx > 0 {
		login = email[:idx]
	}

	now := time.Now()
	return &platformmodels.User{
		Login:      login,
		Email:      email,
		Title:      name,
		Role:       "Agent",
		ValidID:    1,
		CreateTime: now,
		ChangeTime: now,
	}, nil
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
		// Try generic PKCS#8 parsing as fallback
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

// extractUserAttributes extracts email and display name from SAML assertion attributes.
func extractUserAttributes(assertion *saml.Assertion, emailAttrName, nameAttrName string) (email, displayName string) {
	email = extractAttributeValue(assertion, emailAttrName)
	firstName := extractAttributeValue(assertion, "givenName")
	surname := extractAttributeValue(assertion, "sn")
	fullName := extractAttributeValue(assertion, nameAttrName)

	if fullName != "" {
		displayName = fullName
	} else if firstName != "" || surname != "" {
		parts := []string{firstName, surname}
		validParts := make([]string, 0, len(parts))
		for _, p := range parts {
			if strings.TrimSpace(p) != "" {
				validParts = append(validParts, strings.TrimSpace(p))
			}
		}
		displayName = strings.Join(validParts, " ")
	}
	return email, displayName
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
