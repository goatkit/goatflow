# OIDC Identity Provider Integration

## Purpose

GoatFlow integrates with external identity providers (IdPs) as an **OIDC client**. Each organization configures one or more external IdPs (Google, GitHub, Keycloak, Azure AD, etc.). Users authenticate via those IdPs, and GoatFlow provisions or maps local user accounts from the IdP's claims.

**GoatFlow is NOT an OAuth2/OIDC provider.** The existing dormant `internal/platform/oauth2/provider.go` (1000+ lines) implements an authorization server — a fundamentally different product. That code is a dead skeleton. This spec repurposes the auth provider registry to make GoatFlow an OIDC *client*, not a provider.

## Roadmap Alignment

| Version | Timeline | Scope |
|---------|----------|-------|
| **0.8.4** | Jul 2026 | Auth provider registry extended with OIDC client. Generic OIDC + Google support. Per-org config. Login page IdP buttons. Basic user provisioning. |
| **0.9.0** | Aug 2026 | GitHub provider, SAML2 (future), group mapping from claims, admin UI for provider management. |

The current 0.8.4 roadmap entry "OAuth2/OIDC Provider & Client Management" is **misnamed**. It should read "OIDC Identity Provider Integration (Client Mode)".

---

## 1. Concept

GoatFlow operates as a standard OIDC relying party (client). The flow:

1. Org admin configures IdPs in GoatFlow (client_id, client_secret, discovery URL, etc.)
2. User visits GoatFlow login page
3. User clicks an IdP button (Google, GitHub, Keycloak, etc.)
4. GoatFlow redirects to the IdP's authorization endpoint
5. User authenticates with the IdP
6. IdP redirects back with an authorization code
7. GoatFlow exchanges the code for tokens via the token endpoint
8. GoatFlow validates the ID token (signature, issuer, expiry, audience)
9. GoatFlow extracts user claims (email, name, groups)
10. GoatFlow creates or finds the local user account
11. GoatFlow issues its own JWT session token
12. If TOTP is enabled for the user, TOTP verification is required post-auth

This is the standard [OIDC Authorization Code Flow with PKCE](https://openid.net/specs/openid-connect-core-1_0.html#CodeFlowAuth).

### Multi-Tenancy

- Each `gk_organisation` can configure its own set of IdPs
- An IdP config with `org_id = NULL` is **global** — available to all organisations
- An IdP config with `org_id = N` is **org-specific** — only organisation N sees it
- When resolving which IdPs to show on the login page for a given org:
  1. First, list global IdPs (`org_id IS NULL`), filtered by `enabled = true`
  2. Then, list org-specific IdPs (`org_id = ?`), filtered by `enabled = true`
  3. Render all buttons on the login page

---

## 2. Data Model

### Table: `gk_identity_provider`

| Column | Type | Notes |
|--------|------|-------|
| `id` | `BIGINT` | PK, auto-increment |
| `org_id` | `BIGINT NULL` | FK → `gk_organisation.id`. NULL = global (all orgs) |
| `name` | `VARCHAR(100)` | Human-readable name, e.g. "Company Google", "GitHub Enterprise" |
| `provider_type` | `ENUM('oidc','google','github','saml2')` | Pre-configured or generic OIDC |
| `client_id` | `TEXT` | OAuth2/OIDC client ID |
| `client_secret` | `TEXT` | Encrypted (same scheme as `SECURE_SETTINGS`) |
| `discovery_url` | `TEXT` | OIDC discovery endpoint (`/.well-known/openid-configuration`). Required for `oidc` type |
| `scopes` | `TEXT` | Comma-separated scope list, default `openid,profile,email` |
| `user_claim_email` | `VARCHAR(50)` | JSON path or claim key for email, default `email` |
| `user_claim_name` | `VARCHAR(50)` | JSON path or claim key for display name, default `name` |
| `user_claim_groups` | `VARCHAR(50)` | JSON path or claim key for groups, default `groups` |
| `enabled` | `BOOLEAN` | Default `false`. Only enabled providers appear on login |
| `auto_provision` | `BOOLEAN` | Default `false`. If true, create local user on first login |
| `auto_add_to_group` | `BIGINT NULL` | FK → `gk_group.id`. Auto-assign newly provisioned users to this group |
| `created_by` | `BIGINT NULL` | FK → `gk_user.id` |
| `create_time` | `TIMESTAMP` | Auto-set on insert |
| `change_time` | `TIMESTAMP` | Auto-updated on row change |

### Indexes

```sql
CREATE INDEX idx_idp_org_id ON gk_identity_provider (org_id);
CREATE INDEX idx_idp_enabled ON gk_identity_provider (enabled) WHERE enabled = true;
CREATE INDEX idx_idp_org_enabled ON gk_identity_provider (org_id, enabled);
```

### Type-Specific Field Requirements

| Type | Required Fields | Default Values |
|------|----------------|----------------|
| `oidc` | `client_id`, `client_secret`, `discovery_url` | `scopes=openid,profile,email`; `user_claim_email=email` |
| `google` | `client_id`, `client_secret` | `discovery_url=https://accounts.google.com/.well-known/openid-configuration`; `scopes=openid,profile,email` |
| `github` | `client_id`, `client_secret` | Uses GitHub's OIDC endpoint; `discovery_url=https://github.com/.well-known/openid-configuration` |
| `saml2` | `metadata_url` (future), `entity_id` | Not yet implemented |

### Encrypted Fields

`client_secret` is stored encrypted using the same encryption scheme as `SECURE_SETTINGS` in the existing codebase. The key is derived from the system's master encryption key. Decryption happens at query time, before the secret is used in HTTP requests to the IdP.

---

## 3. Config Schema

GoatFlow's `sysconfig` uses hierarchical keys. Identity providers are configured under the `IdentityProviders` top-level key.

### Provider Configuration Keys

Per-provider keys follow the pattern `IdentityProviders::{slug}::{property}`:

```yaml
# Google IdP — global (all orgs)
IdentityProviders::google::enabled=true
IdentityProviders::google::client_id=goatflow-google.apps.googleusercontent.com
IdentityProviders::google::client_secret=encrypted:gAAAA...

# GitHub IdP — org-specific (org 123)
IdentityProviders::github::org::123::enabled=true
IdentityProviders::github::org::123::client_id=gho_xxxx
IdentityProviders::github::org::123::client_secret=encrypted:gAAAA...

# Generic OIDC — org-specific (org 456)
IdentityProviders::oidc::org::456::enabled=true
IdentityProviders::oidc::org::456::client_id=goatflow-org456
IdentityProviders::oidc::org::456::client_secret=encrypted:gAAAA...
IdentityProviders::oidc::org::456::discovery_url=https://keycloak.internal.realms/goatflow/.well-known/openid-configuration
IdentityProviders::oidc::org::456::scopes=openid,profile,email,groups
```

### Discovery URL Resolution

For pre-configured types (`google`, `github`), the discovery URL is auto-populated from the `gk_identity_provider` table on first use. If the org-specific discovery URL is set in config, it overrides the default.

### Environment Variable Overrides

All config values can be overridden via environment variables:

```bash
# Enable Google for all orgs
GOATFLOW_IDENTITYPROVIDERS_GOOGLE_ENABLED=true

# Google credentials (not recommended for secrets — use config file or vault)
GOATFLOW_IDENTITYPROVIDERS_GOOGLE_CLIENT_ID=...
GOATFLOW_IDENTITYPROVIDERS_GOOGLE_CLIENT_SECRET=...
```

### Per-Org Resolution at Runtime

When a user accesses GoatFlow in the context of an organisation:

1. Load all IdPs from `gk_identity_provider` where `(org_id IS NULL AND enabled=true) OR (org_id = ? AND enabled=true)`
2. For each, resolve config overrides:
   - Global IdP: `IdentityProviders::{slug}::*`
   - Org-specific IdP: `IdentityProviders::{slug}::org::{org_id}::*`
3. Return the merged configuration to the login handler

---

## 4. Auth Provider Interface

The existing `AuthProvider` interface in `internal/platform/auth/authenticator.go` is the foundation. The current interface is:

```go
type AuthProvider interface {
    Authenticate(ctx context.Context, username, password string) (*platformmodels.User, error)
    GetUser(ctx context.Context, identifier string) (*platformmodels.User, error)
    ValidateToken(ctx context.Context, token string) (*platformmodels.User, error)
    Name() string
    Priority() int
}
```

The current `Authenticate(ctx, username, password)` method signature is **password-based**. OIDC providers use **code exchange**. We need to extend the interface without breaking the existing `database` and `ldap` providers.

### Option A: Add OIDC-specific method to interface

```go
type AuthProvider interface {
    Authenticate(ctx context.Context, username, password string) (*platformmodels.User, error)
    GetUser(ctx context.Context, identifier string) (*platformmodels.User, error)
    ValidateToken(ctx context.Context, token string) (*platformmodels.User, error)
    Name() string
    Priority() int

    // OIDC-specific methods — only relevant for OIDC/OAuth2 providers.
    // The default authenticator delegates to these only when the login
    // payload contains an OIDC auth code rather than a password.
    StartAuthFlow(ctx context.Context, state string) (authURL string, err error)
    CompleteAuthFlow(ctx context.Context, code, state string) (*platformmodels.User, error)
}
```

This adds two methods to the interface. The `database` and `ldap` providers return `ErrNotImplemented` or `panic` for these. This is acceptable because the login handler only calls these methods when the user submitted via an IdP button (not password).

### Option B: Type assertion

Keep the interface unchanged and use type assertion in the login handler:

```go
type OIDCProvider interface {
    StartAuthFlow(ctx context.Context, state string) (authURL string, err error)
    CompleteAuthFlow(ctx context.Context, code, state string) (*platformmodels.User, error)
}

// In login handler:
if p, ok := provider.(OIDCProvider); ok {
    user, err = p.CompleteAuthFlow(ctx, code, state)
} else {
    user, err = provider.Authenticate(ctx, username, password)
}
```

**Recommendation: Option B.** Cleaner separation. The `Authenticator` struct doesn't need to know about OIDC. The login handler is the dispatcher that decides which auth method to invoke.

---

## 5. Provider Registry

The existing registry in `internal/platform/auth/registry.go` uses:

```go
type ProviderFactory func(deps ProviderDependencies) (AuthProvider, error)
var providerRegistry = map[string]ProviderFactory{}

func RegisterProvider(name string, factory ProviderFactory) error
func CreateProvider(name string, deps ProviderDependencies) (AuthProvider, error)
```

We register OIDC providers under their slug names:

```go
auth.RegisterProvider("google", newGoogleProvider)
auth.RegisterProvider("github", newGitHubProvider)
auth.RegisterProvider("oidc", newOIDCProvider)
auth.RegisterProvider("saml2", newSAML2Provider) // stub
```

### ProviderDependencies Extension

Extend `ProviderDependencies` with OIDC-specific resources:

```go
type ProviderDependencies struct {
    DB       *sql.DB
    UserRepo UserLookup

    // OIDC client — shared HTTP client with PKCE state store
    OIDCClient *http.Client
    StateStore *OIDCStateStore  // in-memory map[code]state
}
```

The `StateStore` is an in-memory LRU cache mapping short-lived authorization codes to state strings, with a TTL of 5 minutes. It is used to prevent CSRF during the OIDC redirect flow.

---

## 6. Auth Flow

### 6.1 Login Page — IdP Buttons

The login page (currently `POST /login` with username/password) is extended to render IdP buttons:

```html
<!-- Login page -->
<h2>Sign in</h2>

<!-- IdP buttons -->
<div class="idp-buttons">
  <a href="/auth/google" class="btn btn-google">Sign in with Google</a>
  <a href="/auth/github" class="btn btn-github">Sign in with GitHub</a>
  <a href="/auth/oidc/456" class="btn">Sign in with Keycloak</a>
</div>

<div class="divider">or</div>

<!-- Existing password form -->
<form method="post" action="/login">
  <input name="username" ...>
  <input name="password" ...>
  <button type="submit">Sign in</button>
</form>
```

Each button points to a handler that initiates the OIDC flow:

```
GET /auth/google        → redirect to Google's authorization URL
GET /auth/github        → redirect to GitHub's authorization URL
GET /auth/oidc/:id      → redirect to generic OIDC IdP's authorization URL
```

### 6.2 Authorization Request

The login handler for an IdP button:

1. Look up the IdP config from `gk_identity_provider`
2. Generate a cryptographically random `state` string
3. Store `state` → `org_id` in `StateStore` (5 min TTL)
4. Build the authorization URL:
   ```
   {discovery_url}/authorize
     ?response_type=code
     &client_id={client_id}
     &redirect_uri={goatflow_url}/auth/{provider}/callback
     &scope={scopes}
     &state={state}
     &code_challenge={pkce_challenge}
     &code_challenge_method=S256
   ```
5. Redirect user to that URL

### 6.3 Callback

```
GET /auth/{provider}/callback?code=xxx&state=yyy
```

The callback handler:

1. Look up `state` in `StateStore`, delete it (one-time use)
2. If not found → error: "Invalid or expired state"
3. Look up the IdP config by provider name (and org_id from state lookup)
4. Exchange code for tokens:
   ```
   POST {discovery_url}/token
     grant_type=authorization_code
     &code=xxx
     &redirect_uri={goatflow_url}/auth/{provider}/callback
     &client_id={client_id}
     &client_secret={client_secret}
     &code_verifier={pkce_verifier}
   ```
5. Parse the response — extract `id_token`, `access_token`, `token_type`
6. Validate the `id_token`:
   - Verify JWT signature against the IdP's JWKS endpoint (`{discovery_url}/jwks`)
   - Verify `iss` matches the IdP's issuer
   - Verify `aud` contains GoatFlow's client_id
   - Verify `exp` is not expired
   - Verify `nonce` matches (if sent)
7. Decode the ID token payload (JWT body) to extract user claims
8. Extract email from the configured claim key (`user_claim_email`)
9. Look up or create the local user (see User Provisioning)
10. Issue GoatFlow JWT session token
11. Redirect to the dashboard

### 6.4 Flow Diagram

```
User              GoatFlow            IdP (Google/etc.)
 |                  |                      |
 |--- login page -->|                      |
 |<-- IdP buttons --|                      |
 |--- /auth/google -|                      |
 |                  |--- auth request ------>|
 |<-- redirect ------|                      |
 |--- browser ------>|                      |
 |                  |--- authorize -------->|
 |                  |<-- consent/login ------|
 |                  |<-- redirect + code ----|
 |--- browser ------>|                      |
 |                  |--- token exchange ---->|
 |                  |<-- id_token + access --|
 |                  |--- validate ID token --|
 |                  |--- find/create user ---|
 |<-- redirect ---->|                      |
 |--- dashboard ---->|                      |
```

---

## 7. User Provisioning

### 7.1 auto_provision = false (existing user required)

1. Extract email from IdP claims
2. Call `UserRepo.GetByEmail(email)`
3. If user not found → return `ErrUserNotFound` with message "Account not found. Contact your administrator."
4. If user found, check `disabled` flag → return `ErrUserDisabled` if applicable
5. Continue to session creation

### 7.2 auto_provision = true (first-time login creates user)

1. Extract email, name from IdP claims
2. Check `UserRepo.GetByEmail(email)` first
3. If user exists → use existing account (skip creation)
4. If user not found → create new local user:
   ```go
   user := &User{
       Login:       extractedEmail,
       Email:       extractedEmail,
       Name:        extractedName,
       Role:        "customer",  // default role
       Enabled:     true,
       IdentityID:  extractedIDFromIdP,  // store external ID for link tracking
   }
   db.Insert(user)
   ```
5. If `auto_add_to_group` is set, add the new user to that group
6. Continue to session creation

### 7.3 Account Linking

Store the external IdP identifier on the user record (`identity_id`) so subsequent logins from the same IdP can be matched even before the email claim is extracted. This also supports future multi-IdP linking per user.

### 7.4 Group Mapping

When the IdP returns a groups claim (configured via `user_claim_groups`):

1. Extract groups array from the claim
2. For each group string, attempt to match against local `gk_group` entries
3. Matching logic (configurable per provider):
   - Exact match on group name
   - Prefix/suffix match (e.g., `goatflow-admin-*` → admin role)
   - Regex match (advanced)
4. Add matched users to matched groups
5. If no groups config is set, skip group mapping (user retains default role)

Group mapping is a **0.9.0** feature. In 0.8.4, groups are logged but not acted upon.

---

## 8. Provider Types

### 8.1 Generic OIDC (`provider_type = 'oidc'`)

Fully generic — uses the OIDC discovery protocol to learn all endpoints:

- Authorization endpoint: from discovery `authorization_endpoint`
- Token endpoint: from discovery `token_endpoint`
- JWKS endpoint: from discovery `jwks_uri`
- Userinfo endpoint: from discovery `userinfo_endpoint` (fallback if ID token doesn't contain all claims)

**Required config**: `client_id`, `client_secret`, `discovery_url`

### 8.2 Google (`provider_type = 'google'`)

Pre-configured with known values:

- Discovery URL: `https://accounts.google.com/.well-known/openid-configuration`
- Standard scopes: `openid`, `profile`, `email`
- Standard claim keys: `email`, `name`
- Known JWKS URL: `https://www.googleapis.com/oauth2/v3/certs`
- Known token endpoint: `https://oauth2.googleapis.com/token`

**Required config**: `client_id`, `client_secret` only

### 8.3 GitHub (`provider_type = 'github'`)

Pre-configured with known values:

- Discovery URL: `https://github.com/.well-known/openid-configuration`
- Standard scopes: `openid`, `profile`, `email`
- Standard claim keys: `email`, `name`
- Known JWKS URL: `https://github.com/.well-known/openid-configuration` (GitHub's OIDC JWKs)

**Required config**: `client_id`, `client_secret` only

### 8.4 SAML2 (`provider_type = 'saml2'`) — Future

Not implemented in 0.8.4. Will use a SAML2 library (e.g., `go-saml2` or `saml2go`) for:

- Metadata URL fetch
- SSO redirect/binding handling
- SAML response validation and signature checking
- Attribute mapping

---

## 9. Admin UI

### 9.1 Provider Management (0.9.0)

A new admin section under `/admin/identity-providers/`:

| Route | Action |
|-------|--------|
| `GET /admin/identity-providers` | List all providers (paginated) |
| `GET /admin/identity-providers/new` | Create provider form |
| `POST /admin/identity-providers` | Create provider handler |
| `GET /admin/identity-providers/:id/edit` | Edit provider form |
| `POST /admin/identity-providers/:id` | Update provider handler |
| `POST /admin/identity-providers/:id/disable` | Toggle enabled |
| `POST /admin/identity-providers/:id/delete` | Delete provider |

### 9.2 Create/Edit Form Fields

```
┌─────────────────────────────────────────────┐
│ Identity Provider                           │
├─────────────────────────────────────────────┤
│ Name:         [Company Google      ]        │
│ Type:         [Google ▼         ]           │
│ Organisation: [Global (all orgs) ▼]         │
│                                               │
│ Client ID:    [AIzaSy...            ]       │
│ Client Secret: [••••••••••••••••    ]       │
│ Discovery URL: [auto (https://...)]         │
│                                               │
│ Scopes:       [openid,profile,email     ]    │
│ Email Claim:  [email                   ]    │
│ Name Claim:   [name                    ]    │
│ Groups Claim: [groups                  ]    │
│                                               │
│ ☑ Enabled                                    │
│ ☐ Auto-provision users on first login        │
│ Auto-add to group: [  ▼  ]                   │
│                                               │
│ [Save]  [Cancel]                              │
└─────────────────────────────────────────────┘
```

---

## 10. TOTP Integration

The existing TOTP mechanism is unaffected. OIDC-authenticated users who have TOTP enabled receive a TOTP challenge after the external auth completes:

1. OIDC flow completes → user resolved
2. System checks `user.totp_enabled`
3. If enabled → redirect to `/auth/totp/verify` with a session token
4. If not enabled → issue JWT, redirect to dashboard

This is a seamless handoff — the user doesn't know two auth steps happened.

---

## 11. Security Considerations

### 11.1 PKCE

All OIDC flows MUST use PKCE (Code Challenge + Code Verifier). This is mandatory for public clients and good practice for confidential clients too.

### 11.2 State Parameter

The `state` parameter is a cryptographically random string stored server-side with a 5-minute TTL. It prevents CSRF attacks on the redirect.

### 11.3 ID Token Validation

The ID token MUST be validated before trusting any claims:

- **Signature**: Verify against the IdP's public keys (JWKS). Cache keys with their `cache-control` header TTL.
- **Issuer**: Verify `iss` matches the expected IdP issuer
- **Audience**: Verify `aud` contains GoatFlow's client_id
- **Expiry**: Verify `exp` is in the future
- **Nonce**: If present, verify against the nonce sent in the auth request
- **Authorization Header**: Use `Authorization: Bearer <access_token>` for userinfo endpoint (not public URLs)

### 11.4 Client Secret Storage

`client_secret` is stored encrypted at rest using the same `SECURE_SETTINGS` encryption scheme. It is never logged, never returned in API responses, and only decrypted in memory during the token exchange.

### 11.5 Redirect URI Validation

GoatFlow's redirect URI is fixed per provider: `{base_url}/auth/{provider}/callback`. The IdP must be configured to accept this URI. The callback handler validates that the redirect URI in the config matches the incoming request's `redirect_uri` parameter.

### 11.6 Brute Force Protection

IdP login buttons do not have brute-force risk (they redirect to the IdP). However, the callback handler should rate-limit rapid code exchanges to prevent token replay.

### 11.7 Session Security

The JWT session token issued after OIDC auth follows the same security model as password auth:
- Secure flag
- HttpOnly flag
- SameSite=Strict
- Configurable TTL and idle timeout

---

## 12. API Endpoints

### 12.1 Auth Flow Endpoints

| Method | Path | Handler |
|--------|------|---------|
| GET | `/auth/{provider}` | Start OIDC flow — redirect to IdP |
| GET | `/auth/{provider}/callback` | Complete OIDC flow — exchange code, create session |
| GET | `/auth/{provider}/callback` with `error=...` | Handle IdP errors (consent required, access denied) |

### 12.2 Admin Endpoints (0.9.0)

| Method | Path | Permission |
|--------|------|-----------|
| GET | `/admin/identity-providers` | `system:config` |
| POST | `/admin/identity-providers` | `system:config` |
| PUT | `/admin/identity-providers/:id` | `system:config` |
| DELETE | `/admin/identity-providers/:id` | `system:config` |
| POST | `/admin/identity-providers/:id/disable` | `system:config` |

### 12.3 API Response — Available Providers

```
GET /api/v1/auth/providers
```

Returns the list of enabled IdPs visible to the current context (based on the org from session cookie or subdomain):

```json
{
  "providers": [
    {
      "id": 1,
      "name": "Company Google",
      "type": "google",
      "enabled": true
    },
    {
      "id": 2,
      "name": "Keycloak",
      "type": "oidc",
      "enabled": true
    }
  ]
}
```

This endpoint supports frontend SPA navigation for the login page to dynamically render buttons.

---

## 13. Implementation Plan

### 0.8.4 Scope

1. **Schema & Migration**: Add `gk_identity_provider` table
2. **Config Schema**: Register `IdentityProviders::*` config keys in default config
3. **OIDC Provider Implementation**: `internal/platform/auth/oidc_provider.go`
   - Generic OIDC provider (discovery URL-based)
   - Google pre-configured provider
   - PKCE support, state store, JWKS validation
4. **Auth Flow Handlers**: New routes in the auth module
   - `/auth/{provider}` — start flow
   - `/auth/{provider}/callback` — complete flow
5. **Login Page Template**: Add IdP buttons section, render from enabled providers
6. **User Provisioning**: Basic create/find logic with `auto_provision` flag
7. **Multi-tenancy**: org_id filtering in provider lookup

### 0.9.0 Scope

1. **GitHub Provider**: Pre-configured GitHub OIDC provider
2. **Group Mapping**: Map IdP claims to local groups
3. **Admin UI**: Full CRUD for identity providers in admin panel
4. **SAML2 Stub**: Register but defer implementation
5. **API Endpoint**: `GET /api/v1/auth/providers` for dynamic rendering
6. **Testing**: Integration tests against test IdPs (Keycloak testcontainer)

---

## 14. Existing Code Touchpoints

| File | Change |
|------|--------|
| `internal/platform/auth/authenticator.go` | No interface change — use type assertion for OIDC methods |
| `internal/platform/auth/registry.go` | Register `oidc`, `google`, `github` providers |
| `internal/platform/auth/registry.go` | Extend `ProviderDependencies` with `OIDCClient` and `StateStore` |
| `internal/config/default.yaml` | Add `IdentityProviders::*` config entries |
| `internal/db/migrations/` | Add migration for `gk_identity_provider` table |
| `internal/platform/auth/oidc_provider.go` | **New file** — OIDC provider implementation |
| `internal/platform/auth/google_provider.go` | **New file** — Google pre-configured provider |
| `internal/platform/auth/github_provider.go` | **New file** — GitHub pre-configured provider (0.9.0) |
| `internal/handlers/auth_handler.go` | New routes and callbacks |
| `templates/pages/auth/login.pongo2` | Add IdP buttons |
| `internal/platform/auth/saml2_provider.go` | **New file** — stub (0.9.0) |

No changes to:
- `internal/platform/oauth2/provider.go` — remains dead code, scheduled for removal
- `internal/platform/models/user.go` — add `IdentityID` field for external ID tracking
- TOTP module — no changes needed

---

## 15. Configuration Reference

### Minimal Configuration (Google, global, enabled)

```yaml
IdentityProviders:
  google:
    enabled: true
    client_id: "goatflow.apps.googleusercontent.com"
    client_secret: "encrypted:gAAAA..."
```

### Generic OIDC (org-specific, disabled by default)

```yaml
IdentityProviders:
  oidc:
    org:
      "123":
        enabled: true
        client_id: "goatflow-org123"
        client_secret: "encrypted:gAAAA..."
        discovery_url: "https://keycloak.internal/realms/goatflow"
        scopes: "openid,profile,email,groups"
        user_claim_email: "email"
        user_claim_name: "preferred_username"
        user_claim_groups: "groups"
        auto_provision: true
        auto_add_to_group: 5
```

### All Available Configuration Keys

| Config Key | Type | Default | Description |
|------------|------|---------|-------------|
| `IdentityProviders::{slug}::enabled` | bool | `false` | Enable/disable this provider |
| `IdentityProviders::{slug}::client_id` | string | — | OAuth2 client ID |
| `IdentityProviders::{slug}::client_secret` | string (encrypted) | — | OAuth2 client secret |
| `IdentityProviders::{slug}::discovery_url` | string | (type-dependent) | OIDC discovery endpoint |
| `IdentityProviders::{slug}::scopes` | string | `openid,profile,email` | Comma-separated OIDC scopes |
| `IdentityProviders::{slug}::user_claim_email` | string | `email` | Claim key for email |
| `IdentityProviders::{slug}::user_claim_name` | string | `name` | Claim key for display name |
| `IdentityProviders::{slug}::user_claim_groups` | string | `groups` | Claim key for groups |
| `IdentityProviders::{slug}::auto_provision` | bool | `false` | Auto-create users on first login |
| `IdentityProviders::{slug}::auto_add_to_group` | int | `0` | Group ID to assign new users |

Org-specific variants use `::org::{id}::` between the slug and property:

```
IdentityProviders::{slug}::org::{org_id}::{property}
```

Example:
```
IdentityProviders::google::org::42::enabled=true
IdentityProviders::google::org::42::client_id=...
```

---

*Author: GoatFlow Engineering*
*Version: 1.0*
*Last Updated: 2026-07-07*