# LDAP / Active Directory Authentication

GoatFlow can verify **agent** passwords against an LDAP directory (OpenLDAP, 389 Directory Server,
Active Directory). It works like OTRS/Znuny's `AuthModule` LDAP backend:

1. Connect to the server (plain LDAP, StartTLS or LDAPS) and bind with a read-only service account
   (or search anonymously when no service account is configured).
2. Search for exactly one entry matching `LDAP_USER_FILTER`, with the typed login escaped (RFC 4515)
   so it can never change the filter.
3. Optionally read the user's groups (`LDAP_GROUP_*`).
4. Bind as that entry with the typed password. An empty password is always rejected (LDAP would
   treat it as an anonymous bind that "succeeds").
5. Map the entry to the GoatFlow agent whose `login` is the directory username, optionally creating
   the account on first login and syncing name, email and administrator rights on every login.

Customer-portal logins are not authenticated against LDAP; customers keep using database
authentication.

## Enabling LDAP

Two settings are needed:

- `LDAP_ENABLED=true` plus the connection settings below.
- `ldap` in the authentication provider order. Set it with the `AUTH_PROVIDERS` environment
  variable (comma separated, e.g. `AUTH_PROVIDERS=ldap,database`) or with the `Auth::Providers`
  setting in `config/Config.yaml`; the environment variable wins.

Providers are tried in order until one accepts the login:

- A login that has no directory entry is passed on to the next provider, so with
  `ldap,database` local accounts such as `root@localhost` keep working.
- A wrong LDAP password is also passed on. Agents created by LDAP have no local password, so the
  database provider cannot accept them; an agent that already had a local password can still use it.
  List only `ldap` (or clear the local passwords) to make the directory the only authority.

At startup GoatFlow validates every `LDAP_*` setting when `LDAP_ENABLED=true` and refuses to start
with an error naming each invalid variable. It does not contact the directory at startup, so an LDAP
outage never prevents GoatFlow from starting; during an outage LDAP logins fail and the next provider
is tried. A mismatch between `LDAP_ENABLED` and the provider list is logged as a warning.

## Settings

| Variable | Default | Meaning |
|----------|---------|---------|
| `LDAP_ENABLED` | `false` | Turns the LDAP provider on (it must also be in the provider order). |
| `LDAP_TYPE` | — | `openldap`, `389ds` or `active_directory`: fills in the filter and attribute defaults below. |
| `LDAP_HOST` | — (required) | Server host name, without scheme or port. Used for certificate name checking. |
| `LDAP_PORT` | `389`, or `636` with `LDAP_USE_SSL` | Server port. |
| `LDAP_USE_SSL` | `false` | Connect with LDAPS (TLS from the first byte). |
| `LDAP_USE_TLS` | `false` | Upgrade a plain connection with StartTLS. Cannot be combined with `LDAP_USE_SSL`. |
| `LDAP_TLS_CA_FILE` | system roots | PEM file with the CA certificate(s) that signed the server certificate. |
| `LDAP_SKIP_TLS_VERIFY` | `false` | Disables certificate verification. For testing only; a warning is logged. |
| `LDAP_TIMEOUT` | `10` | Seconds allowed for connecting and for each LDAP operation (1–300). The login request's own deadline is honoured too. |
| `LDAP_BIND_DN` | — | Service account DN used for searches. Empty means anonymous search. |
| `LDAP_BIND_PASSWORD` | — | Service account password; required when `LDAP_BIND_DN` is set. |
| `LDAP_BASE_DN` | — (required) | Base DN of the directory. |
| `LDAP_USER_BASE_DN` | `LDAP_BASE_DN` | Subtree searched for users. |
| `LDAP_USER_FILTER` | from `LDAP_TYPE` | User search filter with exactly one `%s`, replaced by the escaped login. Must match a single entry; a filter matching several entries refuses the login. |
| `LDAP_IS_ACTIVE_DIRECTORY` | from `LDAP_TYPE` | Also match `userPrincipalName`: logins containing `@` are tried as a UPN. |
| `LDAP_DOMAIN` | — | Active Directory only: logins without `@` are also tried as `<login>@<LDAP_DOMAIN>`. |
| `LDAP_USERNAME_ATTRIBUTE` | from `LDAP_TYPE` | Attribute whose value is the GoatFlow agent login (e.g. `uid`, `sAMAccountName`). Empty keeps the login as typed. |
| `LDAP_EMAIL_ATTRIBUTE` | from `LDAP_TYPE` | Attribute copied to the agent's email (`UserEmail` preference). |
| `LDAP_FIRST_NAME_ATTRIBUTE` | from `LDAP_TYPE` | Attribute copied to the agent's first name. |
| `LDAP_LAST_NAME_ATTRIBUTE` | from `LDAP_TYPE` | Attribute copied to the agent's last name. |
| `LDAP_DISPLAY_NAME_ATTRIBUTE` | from `LDAP_TYPE` | Used for the names when first/last name are empty. |
| `LDAP_GROUP_BASE_DN` | — | Subtree searched for the user's groups. Required for `LDAP_AGENT_GROUPS` / `LDAP_ADMIN_GROUPS`. |
| `LDAP_GROUP_FILTER` | from `LDAP_TYPE` | Group search filter with exactly one `%s`, replaced by the escaped member value. |
| `LDAP_GROUP_MEMBER_VALUE` | `dn` | What replaces `%s` in the group filter: `dn` (user DN, for `member`/`uniqueMember`) or `username` (for `memberUid`). |
| `LDAP_GROUP_ATTRIBUTE` | `cn` | Group attribute compared with the names in the two lists below. |
| `LDAP_AGENT_GROUPS` | — | Comma-separated group names. When set, only members of these groups (or of `LDAP_ADMIN_GROUPS`) may log in via LDAP. |
| `LDAP_ADMIN_GROUPS` | — | Comma-separated group names. When set, membership of the GoatFlow `admin` group follows LDAP on every login: added for members, removed for everyone else. |
| `LDAP_AUTO_CREATE_USERS` | `false` | Create the GoatFlow agent on first successful LDAP login. Without it, the agent must already exist with a matching login. |
| `LDAP_AUTO_UPDATE_USERS` | `false` | Overwrite first name, last name and email from the directory on every login. |
| `LDAP_INITIAL_GROUPS` | `users` | Comma-separated GoatFlow groups (rw) given to agents created by `LDAP_AUTO_CREATE_USERS`. Every group must exist. |

`LDAP_TYPE` defaults:

| | `openldap` / `389ds` | `active_directory` |
|---|---|---|
| `LDAP_USER_FILTER` | `(&(objectClass=inetOrgPerson)(uid=%s))` | `(&(objectClass=user)(sAMAccountName=%s))` |
| `LDAP_USERNAME_ATTRIBUTE` | `uid` | `sAMAccountName` |
| `LDAP_EMAIL_ATTRIBUTE` | `mail` | `mail` |
| `LDAP_FIRST_NAME_ATTRIBUTE` / `LDAP_LAST_NAME_ATTRIBUTE` | `givenName` / `sn` | `givenName` / `sn` |
| `LDAP_DISPLAY_NAME_ATTRIBUTE` | `cn` | `displayName` |
| `LDAP_GROUP_FILTER` | openldap: `(&(objectClass=groupOfNames)(member=%s))`; 389ds: `(&(objectClass=groupOfUniqueNames)(uniqueMember=%s))` | `(&(objectClass=group)(member=%s))` |
| `LDAP_IS_ACTIVE_DIRECTORY` | `false` | `true` |

## Account mapping

- The agent login is the value of `LDAP_USERNAME_ATTRIBUTE` from the directory entry, so `JDoe` and
  `jdoe` map to the same agent.
- An agent marked invalid in GoatFlow cannot log in, whatever the directory says.
- Agents created by LDAP get an empty local password, the configured initial groups and their email
  as the `UserEmail` preference. `first_name` falls back to the display name, then the login.
- Group membership other than the `admin` group (`LDAP_ADMIN_GROUPS`) is managed in GoatFlow.
- Directory errors (unreachable server, TLS failure, wrong service-account password, failed group
  search) fail the LDAP attempt closed and are logged with the cause; the user only sees
  "invalid credentials".

## Examples

OpenLDAP with StartTLS and a private CA:

```bash
AUTH_PROVIDERS=ldap,database
LDAP_ENABLED=true
LDAP_TYPE=openldap
LDAP_HOST=ldap.example.com
LDAP_USE_TLS=true
LDAP_TLS_CA_FILE=/etc/goatflow/ldap-ca.pem
LDAP_BIND_DN=cn=goatflow,ou=system,dc=example,dc=com
LDAP_BIND_PASSWORD=change-me
LDAP_BASE_DN=dc=example,dc=com
LDAP_USER_BASE_DN=ou=people,dc=example,dc=com
LDAP_GROUP_BASE_DN=ou=groups,dc=example,dc=com
LDAP_AGENT_GROUPS=helpdesk
LDAP_ADMIN_GROUPS=goatflow-admins
LDAP_AUTO_CREATE_USERS=true
LDAP_AUTO_UPDATE_USERS=true
```

Active Directory over LDAPS:

```bash
AUTH_PROVIDERS=ldap,database
LDAP_ENABLED=true
LDAP_TYPE=active_directory
LDAP_HOST=dc01.corp.example.com
LDAP_USE_SSL=true
LDAP_BIND_DN=CN=svc-goatflow,OU=Service Accounts,DC=corp,DC=example,DC=com
LDAP_BIND_PASSWORD=change-me
LDAP_BASE_DN=DC=corp,DC=example,DC=com
LDAP_DOMAIN=corp.example.com
LDAP_GROUP_BASE_DN=OU=Groups,DC=corp,DC=example,DC=com
# Nested groups:
# LDAP_GROUP_FILTER=(&(objectClass=group)(member:1.2.840.113556.1.4.1941:=%s))
LDAP_AGENT_GROUPS=GoatFlow Agents
LDAP_ADMIN_GROUPS=GoatFlow Admins
LDAP_AUTO_CREATE_USERS=true
```

Kubernetes: see the `ldap` section of the Helm chart's `values.yaml`; the bind password is read from
a Secret.

## Testing

Unit tests (configuration validation, filter escaping):

```bash
make toolbox-exec ARGS="go test ./internal/platform/ldap/"
```

Integration tests start an OpenLDAP container with testcontainers, then log in through
`POST /api/auth/login` against the test database. They need Docker and the test database
(`make test-db-up`), and fail rather than skip when either is missing:

```bash
make test-ldap-integration
```
