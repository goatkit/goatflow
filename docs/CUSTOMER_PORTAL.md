# Customer Portal

The customer portal is the part of GoatFlow that customer users (rows in `customer_user`) log in to.
Customers can open tickets, follow and answer their own tickets, see their own company, manage
their profile and second factors, and reset a forgotten password. This page describes GoatFlow
0.10.0.

## How it runs

The portal is part of the normal GoatFlow server image (Go + pongo2 templates). There is no
separate frontend build. You can run it in two ways.

| Mode | How | What it serves |
|------|-----|----------------|
| Main app | Default | Agent UI, admin UI, REST API and the portal under `/customer/*` |
| Customer-only instance | Same image with `CUSTOMER_FE_ONLY=true` | Only the portal and the paths it needs |

### What a customer-only instance allows

With `CUSTOMER_FE_ONLY=true` the server only answers these paths. Everything else answers 404.

| Allowed | Notes |
|---------|-------|
| `/` | Redirects to `ROOT_REDIRECT_PATH`, or to the global portal Landing Page when that is unset |
| `/customer`, `/customer/...` | The portal, self-service pages and `/customer/api/v1/tokens` |
| `/auth/customer` | Customer login page |
| `/login` | Redirects to `/customer/login` (exact path only; the agent 2FA page `/login/2fa` is blocked) |
| `/api/auth/customer/...` | Customer login, passkey and 2FA API calls. Agent login APIs (`/api/auth/login`, `/api/auth/2fa/...`, `/api/auth/passkey/...`) are blocked |
| `/api/languages`, `/api/themes` (and sub-paths) | Language and theme pickers |
| `/health`, `/healthz` | Health probes (exact paths; `/health/detailed` is blocked) |
| `/static/...`, `/assets/...`, `/runtime/...`, `/favicon.ico` | Static files |

Other effects of `CUSTOMER_FE_ONLY=true`:

- A request to an allowed path with no route redirects to the same target as `/`.
- Portal pages always need a customer login, whatever the "Require Login" setting says.
- The scheduler does not start. Scheduled jobs run only on the main app.
- The agent REST API (`/api/v1/*`) and all `/admin` pages are not served.

### Docker Compose

`docker-compose.yml` (development stack):

| Service | Role | Host port (env, default) |
|---------|------|--------------------------|
| `backend` | Main app | `BACKEND_PORT`, `8081` |
| `customer-fe` | Customer-only instance (`CUSTOMER_FE_ONLY: "true"`, `ROOT_REDIRECT_PATH: /customer`) | `CUSTOMER_FE_PORT`, `8083` |

`deploy/docker-compose.yml` (reference deployment) runs `app` and `customer-fe` behind Caddy.
Caddy sends `/customer/*`, `/auth/customer` and `/api/auth/customer/*` to `customer-fe`.
Everything else goes to `app`.

### Helm

The chart in `charts/goatflow` has no customer-only Deployment. The backend Deployment serves the
portal under `/customer` on its Service (port 8080).

The chart's Ingress (`ingress.enabled`, off by default) sends every path to the backend Service,
so `/customer` pages work through it. See [charts/goatflow/README.md](../charts/goatflow/README.md)
and [docs/deployment/kubernetes.md](deployment/kubernetes.md).

## Customer pages

All pages below, except login, need a logged-in customer. A customer only ever sees their own tickets
(`ticket.customer_user_id` = their login) and only articles marked visible for the customer.

| Page | URL | Notes |
|------|-----|-------|
| Login | `/customer/login` (also `/auth/customer`) | Password or passkey ("Use security key"). Form posts to `/api/auth/customer/login`. |
| 2FA step | `/customer/login/2fa` | Code, security key, or recovery code under "Other ways to sign in" |
| Log out | `/customer/logout` | |
| Dashboard | `/customer` | |
| My tickets | `/customer/tickets` | `?status=all` (default), `open` or `closed`. `?search=` matches ticket number or title. |
| New ticket | `/customer/tickets/new` | Posts to `/customer/tickets/create`. See below. |
| Ticket view | `/customer/tickets/:id` | |
| Reply | `POST /customer/tickets/:id/reply` | A reply to a pending ticket sets it to `open` |
| Close | `POST /customer/tickets/:id/close` | Sets the state to `closed successful` |
| Company | `/customer/company` | See [Company pages](#company-pages) |
| Company users | `/customer/company/users` | See [Company pages](#company-pages) |
| Profile | `/customer/profile` | Name and details, language, session timeout, 2FA |
| Change password | `/customer/password/form` | Posts to `/customer/password/change` |
| Knowledge base | `/customer/kb` | Only with the goat-kb plugin. See below. |

### New tickets

- Title and message are required.
- The service list shows the services assigned to the customer user (`service_customer_user`).
- Priority defaults to ID `3` (normal).
- The ticket number comes from the configured ticket number generator.
- The ticket goes to the first queue whose group is linked to the customer's company in
  `group_customer` (Admin -> Customer Groups). With no link it goes to queue ID `1`.

### Attachments

Customers can add files to new tickets, replies, or an existing ticket. They can open
attachments of customer-visible articles on their own tickets.

| Action | URL |
|--------|-----|
| List | `GET /customer/tickets/:id/attachments` |
| Upload | `POST /customer/tickets/:id/attachments` |
| Download | `GET /customer/tickets/:id/articles/:article_id/attachments/:file_id` (`?download=1` forces a download) |
| Thumbnail | `GET .../attachments/:file_id/thumbnail` |
| Viewer | `GET .../attachments/:file_id/view` (`?raw=1` serves the file for embedding) |

Thumbnails are PNG images:

- Images are scaled down to fit 320 x 240 pixels.
- PDFs show page one, about 400 pixels wide. This uses `pdftoppm` from poppler, which is in the
  server image.
- Anything else, or a file that cannot be rendered, gets a placeholder image for its type.

### Profile, password and 2FA

The profile page saves through these endpoints:

| Setting | Endpoint |
|---------|----------|
| Profile details | `POST /customer/profile/update` |
| Language | `GET`/`POST /customer/api/preferences/language` |
| Session timeout | `GET`/`POST /customer/api/preferences/session-timeout` |
| Theme | `GET`/`POST /customer/api/preferences/theme` |
| Wallpaper | `POST /customer/api/preferences/wallpaper` |

The new password must pass the customer password policy. Its rules come from the sysconfig keys
`CustomerPreferencesGroups###Password::PasswordMinSize`, `...PasswordMin2Lower2UpperCharacters`,
`...PasswordNeedDigit`, `...PasswordMin2Characters` and `...PasswordRegExp`.

Customers can set up two-factor authentication on the profile page. All endpoints are under
`/customer/api/preferences/2fa`:

| Feature | Endpoints |
|---------|-----------|
| Authenticator app (TOTP) | `GET /status`, `POST /setup`, `POST /confirm`, `POST /disable` |
| Passkeys and security keys | `POST /webauthn/register/begin`, `POST /webauthn/register/finish`, `GET /webauthn/credentials`, `PATCH`/`DELETE /webauthn/credentials/:id` |
| New recovery codes | `POST /recovery-codes` (password required) |

### API tokens

Customers can manage their own API tokens at `/customer/api/v1/tokens`:

| Method | Path | Action |
|--------|------|--------|
| `GET` | `/customer/api/v1/tokens` | List tokens |
| `POST` | `/customer/api/v1/tokens` | Create a token |
| `DELETE` | `/customer/api/v1/tokens/:id` | Revoke a token |
| `GET` | `/customer/api/v1/tokens/scopes` | List scopes |

There is no portal page for this yet; it is an API only. See [docs/api/README.md](api/README.md).

### Knowledge base

The customer knowledge base comes from the goat-kb plugin (`/customer/kb`). GoatFlow itself has
no customer KB. The dashboard shows the "Knowledge Base" card only when the plugin registers its
`/customer/kb` menu item.

## Company pages

| URL | Shows |
|-----|-------|
| `/customer/company` | Company name, customer ID, address, website, and a link to the user list with the number of users |
| `/customer/company/users` | Valid customer users of the company: name, title, email. The customer's own row is marked "You". |

Rules:

- Read-only.
- Only the customer's own company (`customer_user.customer_id`), and only if that company is valid.
  Otherwise the page says "Your account is not linked to a company."
- Agent-only company comments are never shown.
- The website only becomes a link if it is an `http` or `https` URL.
- The "Company Info" link shows in the portal navigation when the customer's customer ID matches a
  customer company.

## Forgotten password

Flow: `/customer/login` -> "Forgot password?" -> `/customer/forgot-password` -> email ->
`/customer/reset-password?token=...` -> new password.

Agents have the same flow at `/forgot-password` and `/reset-password`.

| Rule | Value |
|------|-------|
| Customer enters | Login or email address (matched without case, valid accounts only) |
| Answer on the page | Always the same, whether or not an account matches |
| Accounts per request | At most 5 matching accounts get an email |
| Link lifetime | 1 hour |
| Use | Once. A newer request revokes older unused links for the same account. |
| Storage | Only a SHA-256 hash of the token is stored (`gk_auth_token`) |
| Binding | The link works only for the account and email address it was sent to |
| New password | Must pass the customer password policy |
| After reset | All open sessions of the account are ended |

The emails go on `mail_queue` and are sent by the runner, from the configured system sender.

Switch: `features.lost_password` in `config/default.yaml`, default `true`. When it is off, the
pages answer 404 and the "Forgot password?" link is hidden on the login page.

## Self-registration

Switch: `features.registration` in `config/default.yaml`, default `false`. When it is off,
`/customer/register` answers 404 and the "Sign up" link is hidden on the login page.

When it is on:

1. The customer opens `/customer/register` and enters first name, last name and email address.
2. GoatFlow emails a confirmation link to `/customer/register/complete?token=...`. The link is
   valid for 24 hours and works once.
3. The customer follows the link and chooses a password (customer password policy applies).
4. GoatFlow creates a valid `customer_user` with login = email address and customer ID = email
   address.

If the address already belongs to a customer, the page gives the same answer, and the email
points to `/customer/forgot-password` instead (only while `features.lost_password` is on).

## Rate limits

The forgotten-password, set-new-password (reset link) and registration forms share these limits.

| Limit | Value | When exceeded |
|-------|-------|---------------|
| Form posts per client IP | 10 per hour | HTTP 429 with `Retry-After: 3600` |
| Emails per recipient | 3 per hour (per account for resets, per address for sign-ups) | Email is not sent; the page answer does not change |

The counters are kept in memory in each running server.

## BASE_URL

Links in these emails are built from the `BASE_URL` environment variable. GoatFlow never uses
the request `Host` header for this, because a client can fake it.

- `BASE_URL` must be an absolute `http` or `https` URL, for example
  `https://helpdesk.example.com`. A trailing `/` is removed.
- If it is unset or invalid, no reset or confirmation email is sent. The server logs an error.
- The link is built by the server that handled the form. With a customer-only instance, set
  `BASE_URL` on that instance too.

| Setup | Where `BASE_URL` comes from |
|-------|-----------------------------|
| `docker-compose.yml`, `backend` | `APP_URL` (default `http://localhost:${BACKEND_PORT:-8080}`) |
| `docker-compose.yml`, `customer-fe` | Fixed to `http://localhost:${CUSTOMER_FE_PORT:-8083}` |
| `deploy/docker-compose.yml`, `app` and `customer-fe` | `BASE_URL` in `.env` (default `http://localhost:8080`) |
| Helm | `config.baseUrl` (not set while empty) |

## Portal settings

### Global settings

Menu: Admin -> Customer Administration -> Customer Portal (`/admin/customer/portal/settings`).
The page is titled "Default Customer Portal Settings".

| Field | Sysconfig key | Default |
|-------|---------------|---------|
| Enable | `CustomerPortal::Enabled` | `true` |
| Require Login | `CustomerPortal::LoginRequired` | `true` |
| Portal Title | `CustomerPortal::Title` | `Customer Portal` |
| Footer Text | `CustomerPortal::FooterText` | `Powered by GoatFlow` |
| Landing Page | `CustomerPortal::LandingPage` | `/customer/tickets` |

The defaults are seeded by migration `000003_customer_portal_sysconfig`. Saved values go to
`sysconfig_modified`.

What they do:

- **Enable** off: the portal pages from `routes/customer.yaml` (dashboard, tickets, company,
  profile, password, preferences) answer 503. Browsers see "`<Portal Title>` is currently
  disabled". Login, forgotten-password, registration and API token URLs are not affected.
- **Require Login** on: visitors without a customer login are sent to `/customer/login`, and so
  are agents (API calls get 403). The built-in portal pages need a customer login even when this
  is off.
- **Portal Title** is used in page titles.
- **Footer Text** is shown at the bottom of portal pages.
- **Landing Page** is where a customer goes after signing in (password, authenticator code or
  passkey), and when a signed-in customer opens `/customer/login`. On a customer-only instance
  with no `ROOT_REDIRECT_PATH`, `/` also redirects here. Use a path such as `/customer/tickets`;
  a value without a leading `/` (for example `tickets`) is relative to `/customer`. A value that
  is not a local path (a URL, `//host`, backslashes) is ignored and `/customer/tickets` is used.
  The dashboard stays at `/customer`. A customer whose organisation is captive to a plugin goes
  to that plugin's landing page instead.

### Per-company settings

Menu: Admin -> Customer Administration -> Customer Organizations -> Edit a company ->
"Portal Settings" tab (`/admin/customer/companies/:id/edit?tab=portal`). The form posts to
`/admin/customer/companies/:id/portal-settings`.

- Each field has an "Override default" box.
- Ticked: the value is stored as `CustomerPortal::<Setting>::<customer_id>`, for example
  `CustomerPortal::Title::ACME`.
- Not ticked: the company's row is deleted and the field inherits the global value.
- After saving, the browser returns to the Portal Settings tab.

How they apply:

- A signed-in customer gets the settings of their company (`customer_user.customer_id`): the
  company's overrides, and the global value for every field it does not override.
- Visitors who are not signed in (the login page, anonymous portal requests, `/` on a
  customer-only instance) get the global settings, because their company is not known yet.
- **Enable** works both ways: a company with Enable off is refused (503 with the company's title)
  while other companies keep the portal; a company with Enable on keeps the portal while it is off
  globally. Customers can sign in either way, because the login page is not gated.

## Admin pages for a company

Linked from the company list and the company edit form:

- `/admin/customer/companies/:id/users`: the company's customer users with ticket counts and status.
- `/admin/customer/companies/:id/tickets`: the company's tickets, newest first, 50 per page.
- `/admin/customer/companies/:id/services`: which valid services each of the company's users has.

## Configuration reference

| Setting | Where | Default | Effect |
|---------|-------|---------|--------|
| `CUSTOMER_FE_ONLY` | Environment | unset | `true`, `1`, `yes` or `on` (any case) makes a customer-only instance |
| `ROOT_REDIRECT_PATH` | Environment | `/login` (the global portal Landing Page on a customer-only instance) | Where `/` redirects |
| `BASE_URL` | Environment | unset | Public URL for email links |
| `features.lost_password` | `config/default.yaml`, env `GOATFLOW_FEATURES_LOST_PASSWORD` | `true` | Forgotten-password pages |
| `features.registration` | `config/default.yaml`, env `GOATFLOW_FEATURES_REGISTRATION` | `false` | Customer self-registration |
| `config.baseUrl` | Helm values | `""` | Sets `BASE_URL` on the backend |
| `backend.extraEnv` | Helm values | `[]` | Use it to set the `GOATFLOW_FEATURES_*` variables |
| `CustomerPortal::*` | Sysconfig (admin page) | see [Global settings](#global-settings) | Portal on/off, login, title, footer, landing page; per company with `::<customer_id>` |

See also [docs/configuration.md](configuration.md).
