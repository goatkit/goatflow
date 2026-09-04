# OIDC Identity Provider Integration Guide

## Overview

GoatFlow integrates with external identity providers as an **OIDC client** (Relying Party). Users authenticate via their IdP, and GoatFlow creates or maps local user accounts from the IdP's claims. This guide covers setup, configuration, and troubleshooting for Okta (the process is similar for any OIDC-compliant provider like Keycloak, Azure AD, Google Workspace, etc.).

---
## Identity Provider Compatibility & Testing

GoatFlow's OIDC client uses the standard Authorization Code Flow with PKCE and JWKS token verification. Any compliant IdP should work — you swap out the discovery URL, credentials, and adjust claim names as needed. Configuration steps vary slightly between providers; the flow is the same.

| Provider | Tested | Discovery URL | Notes |
|----------|--------|---------------|-------|
| **Okta** | ✅ Yes — this guide's examples | `https://your-org.okta.com/oauth2/default` | Groups claim added via Authorization Server → Claims tab |
| Keycloak | Not yet tested (CI test container exists) | `https://keycloak.example.com/realms/<realm>` | Groups sent by default in ID token if mappers configured |
| Azure AD / Entra ID | Should work | `https://login.microsoftonline.com/{tenant}/v2.0` | Uses standard OIDC; claim names match defaults |
| Google Workspace | Should work | `https://accounts.google.com/.well-known/openid-configuration` | Built-in Google provider (shipped in 0.9.0) with pre-configured discovery URL |
| Generic SAML2 providers | Supported | — | SAML2 client shipped in 0.9.0 (configure via the admin identity-provider form) |

> **Okta is the tested reference implementation.** Other providers should work by substituting their discovery URL and adjusting claim names. If a provider uses non-standard claim keys, override them in the GoatFlow provider form (User Claim Email/Name/Groups fields).


## What Happens During Login

1. User clicks "Sign in with {Provider}" on the GoatFlow login page
2. GoatFlow redirects to your IdP's authorization endpoint
3. User authenticates with their IdP (password, MFA, etc.)
4. IdP redirects back to GoatFlow with an authorization code
5. GoatFlow exchanges the code for tokens and validates the ID token
6. GoatFlow extracts user claims (email, name, groups) from the ID token
7. **First login**: GoatFlow creates a local user record and assigns matching groups
8. **Subsequent logins**: GoatFlow finds the existing user by email and syncs any new group memberships
9. If TOTP is enabled for the user, they're challenged to verify their second factor
10. User is redirected to the dashboard

This is the standard [OIDC Authorization Code Flow with PKCE](https://openid.net/specs/openid-connect-core-1_0.html#CodeFlowAuth).

---

## Prerequisites

- GoatFlow 0.9.0+ running and accessible (OIDC/SAML identity providers shipped in 0.9.0)
- Access to your IdP admin console (Okta Admin Console in these instructions)
- Ability to create OIDC applications in your IdP
- Network connectivity from GoatFlow's backend to your IdP's public endpoints

---

## Okta Configuration

### Step 1: Create an OIDC Application

In the [Okta Admin Console](https://dev-xxxxx-admin.okta.com/):

1. Navigate to **Applications** → **Applications**
2. Click **Create App Integration**
3. Select **OIDC - OpenID Connect** as the sign-in method
4. Select **Web Application** as the platform (NOT "Single-page application" — GoatFlow needs client credentials)
5. Fill in:
   - **App integration name**: `GoatFlow OIDC`
   - **Grant types allowed**: Check ✅ **Authorization Code** and ✅ **Refresh Token**
6. Click **Next**

### Step 2: Configure Redirect URIs

On the General settings page, add these as login redirect URIs:

```
https://your-goatflow-domain.com/auth/:provider-id/callback
```

Replace `:provider-id` with the actual GoatFlow provider ID number (you'll find this after creating the provider in GoatFlow — it's shown in the URL when editing). For example, if your Okta provider is ID 2:

```
https://goatflow.example.com/auth/2/callback
```

**Important**: If you plan to use multiple providers later, add each one separately. The redirect URI **must match exactly** — including scheme (`https`), host, and path.

### Step 3: Configure Token Claims

1. In your Okta app, go to the **Sign On** tab
2. Under **OpenID Connect ID Token**, click **Edit**
3. Verify that `email` is included (it should be by default)
4. Click **Save**

### Step 4: Add Groups Claim

GoatFlow uses group claims to automatically assign users to local groups for queue access permissions. To include user groups in the ID token:

1. Go to **Security** → **API** → **Authorization Servers** (left sidebar)
2. Click on **default** under Authorization Servers
3. Go to the **Claims** tab
4. Check that a claim named `groups` exists with value type set to Groups filter and included in ID Token
5. If not, click **Add Claim**:
   - **Name**: `groups`
   - **Include in token type**: Check ✅ **ID Token**
   - Add a rule: **Value Type/Expression** → select **Groups** from the dropdown
6. Click **Save**

This sends all Okta group memberships for authenticated users in every ID token as an array of strings, like `["users", "stats"]`.

### Step 5: Note Your Credentials

After setup, go to your app's **General** tab and copy down:

| Field | Example Value | Where Used Later |
|-------|---------------|------------------|
| Client ID | `0oa123456789ABCDEF` | GoatFlow admin form |
| Client Secret | (eye icon to reveal) | GoatFlow admin form |
| Issuer URL | `https://your-org.okta.com/oauth2/default` | Used for discovery — **do NOT append** `/.well-known/openid-configuration` |

---

## GoatFlow Configuration

### Step 1: Create an Identity Provider

1. Log into GoatFlow as an admin
2. Go to **Admin** → **Identity Providers** (or navigate to `/admin/identity-providers`)
3. Click **+ New Provider**
4. Fill in the form:

| Field | Value | Notes |
|-------|-------|-------|
| Name | `Okta OIDC` | Displayed on login page |
| Provider Type | `OIDC` | Generic OIDC provider |
| Client ID | *(from Okta)* | Copy from app General tab |
| Client Secret | *(from Okta)* | Paste — use [REVEAL] button to toggle visibility |
| Discovery URL | `https://your-org.okta.com/oauth2/default` | **Do NOT include** `/.well-known/openid-configuration` — GoatFlow auto-appends it |
| Scopes | `openid profile email` | Default is fine for most setups |
| User Table | `Agent (users)` or `Customer (service_customer_user)` | Determines which user table new accounts are created in |
| Auto Provision | ✅ Enabled | Creates local accounts on first login automatically |

5. Click **Save**

### Step 2: Verify Configuration

1. Log out of GoatFlow
2. Open an incognito/private browser window
3. Navigate to your GoatFlow login page
4. You should see a "Sign in with Okta OIDC" button (or whatever name you gave the provider)
5. Click it — you should be redirected to Okta's login page

### Step 3: Test Full Flow

1. Log in via Okta with a test user account
2. After authentication, you should land on the GoatFlow dashboard
3. Check your profile (`/profile`) — verify fields populated correctly

---

## Profile Field Mapping

When Okta creates or updates a local user, these fields are mapped from ID token claims:

| GoatFlow Field | Okta Claim | Behavior |
|----------------|------------|----------|
| **Login (username)** | `email` | Full email address stored as login. Existing users matched by this value. |
| **Email** | — | Users table doesn't have a separate email column; Login field serves this purpose |
| **First Name** | `given_name` (or first word of `name`) | Populated on creation and updated each login |
| **Last Name** | `family_name` (or remaining words of `name`) | Same as above |
| **Title / Salutation** | `honorificPrefix` (falls back to `title`) | e.g., "Mr", "Ms", "Dr" |
| **Role** | — | Always set to "Agent" for agents, "Customer" for customers. Cannot be changed by OIDC. |

### Custom Claim Names

If your IdP uses non-standard claim names (e.g., `preferred_username` instead of `email`), you can configure alternative claim keys in the provider settings:

- **User Claim Email**: override from default `email`
- **User Claim Name**: override from default `name`
- **User Claim Groups**: override from default `groups`

---

## Group Mapping

### How It Works

When a user logs in via OIDC, GoatFlow reads the groups claim from their ID token (default: `"groups"`). Each group name is matched against local groups in the `groups` table. If a match exists and the group is active (`valid_id = 1`), a `group_user` entry is created with read-write (`rw`) permissions.

### Rules

| Scenario | Behavior |
|----------|----------|
| Okta group name matches a local group | ✅ User gets assigned to that group automatically |
| Okta group name does NOT match any local group | Silently ignored — no error shown to user |
| User removed from an Okta group | **NOT** revoked locally. We only add, never remove. This prevents accidental permission loss from transient sync issues. |
| Multiple groups claimed | All matching groups are added |

### Example Setup

If your local GoatFlow has these groups: `users`, `stats`, `admin`:

1. In Okta Admin Console, create groups with the **exact same names** (case-sensitive)
2. Assign users to those Okta groups
3. On next login via OIDC, the user will have all three groups assigned locally
4. The "no queue access" error disappears because the user now belongs to at least one group

### Why Groups Matter

In OTRS-style systems like GoatFlow, queue access is gated by group membership. Without any `group_user` entries, every API call through the queue access middleware returns `"You do not have access to any queues"`. Group mapping ensures OIDC-authenticated users can actually use the system immediately after first login.

---

## Agent vs Customer Provisioning

GoatFlow supports two user provisioning paths for OIDC:

| User Table | Destination Table | Use Case |
|------------|------------------|----------|
| `Agent (users)` | `users` table | Staff, support agents, administrators — full system access |
| `Customer (service_customer_user)` | `service_customer_user` table | External users who submit tickets but don't manage them |

You create **separate provider entries** for each path. Each points to the same Okta app (or different ones) and differs only in the User Table setting:

1. Create "Okta Agents" provider → set User Table to `Agent`
2. Create "Okta Customers" provider → set User Table to `Customer`

Each provider gets its own redirect URI (`/auth/:id/callback`) so Okta needs both registered as allowed callback URLs.

---

## Troubleshooting

### "Authentication failed. Please check your provider configuration or try again."

Check the GoatFlow backend logs:
```bash
docker compose logs backend --tail=50 | grep -i OIDC
```

Common causes:
| Error in Logs | Fix |
|---------------|-----|
| `invalid_client` — Client authentication failed | Client secret is wrong, empty, or not saved. Re-enter it in the provider edit form and click Save |
| `no email claim in ID token` | Okta isn't sending an email claim. Verify in app config that email scope is requested |
| `exchange code for token: ... expired_token` | Clock skew between GoatFlow server and IdP — check NTP sync on the host |
| `discover OIDC provider: Get ... 405 Method Not Allowed` | Your discovery URL has `/.well-known/openid-configuration` appended — remove it; GoatFlow adds it automatically |

### "You do not have access to any queues"

The user was provisioned but assigned to no groups. Check:
1. Is the Okta app configured to send group claims? (Security → API → Authorization Servers → Claims)
2. Do the Okta group names match local group names exactly?
3. Are the users actually in those Okta groups?

Check what groups a user has locally:
```sql
SELECT gu.user_id, u.login, g.name as group_name, gu.permission_key
FROM group_user gu
JOIN users u ON gu.user_id = u.id
JOIN groups g ON gu.group_id = g.id
WHERE u.email LIKE '%@okta-domain%';
```

### Duplicate user records created on first login

If multiple logins created different local accounts for the same Okta user, check that:
1. The `email` claim is consistent across all tokens (same value every time)
2. There are no mismatched entries in the `users` table where some use email prefix and others use full email

---

## Reference: ID Token Example

A typical Okta ID token with groups enabled looks like:

```json
{
  "sub": "00u1abc2def3ghi4jkl5",
  "email": "peter.parker@goatflow.io",
  "given_name": "Peter",
  "family_name": "Parker",
  "honorificPrefix": "Mr",
  "name": "Peter Parker",
  "groups": ["users", "stats"],
  "iss": "https://integrator-3758985.okta.com/oauth2/default",
  "aud": "0oa153xmg39PSbNN3698",
  "exp": 1720784400,
  "iat": 1720780800
}
```

---

*Last Updated: 2026-07-12*
