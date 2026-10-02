# GoatFlow Deployment

Reference Docker Compose configuration for deploying GoatFlow.

## Quick Start

```bash
# Download files
curl -O https://raw.githubusercontent.com/goatkit/goatflow/main/deploy/docker-compose.yml
curl -O https://raw.githubusercontent.com/goatkit/goatflow/main/deploy/.env.example

# Configure
cp .env.example .env
# Edit .env - set DOMAIN, ACME_EMAIL, DB_ROOT_PASSWORD, DB_PASSWORD, JWT_SECRET,
# GOATFLOW_SECURE_KEY, GOATFLOW_ADMIN_PASSWORD and the SMTP_* settings

# Start
docker compose up -d
```

## Configuration

All configuration is via environment variables in `.env`:

| Variable | Required | Description | Default |
|----------|----------|-------------|---------|
| `DOMAIN` | **Yes** | Domain name Caddy serves and gets a certificate for | - |
| `ACME_EMAIL` | **Yes** | Email for Let's Encrypt registration | - |
| `GOATFLOW_TAG` | No | Image tag for app, customer portal and runner | `latest` |
| `DB_ROOT_PASSWORD` | **Yes** | MariaDB root password | - |
| `DB_USER` | No | MariaDB username | `goatflow` |
| `DB_PASSWORD` | **Yes** | MariaDB user password | - |
| `DB_NAME` | No | Database name | `goatflow` |
| `JWT_SECRET` | **Yes** | JWT signing secret (32+ chars) | - |
| `GOATFLOW_SECURE_KEY` | **Yes** | Encrypts stored secrets (plugin secure settings, webhook signing secrets); exactly 64 hex characters (`openssl rand -hex 32`). App, customer portal and runner get the same value, so the runner can decrypt webhook signing secrets. Compose refuses to start without it. Never change it once set. | - |
| `GOATFLOW_ADMIN_PASSWORD` | Recommended | First-boot password for the admin account `root@localhost`. Applied once, while the seeded account is still disabled; change it in GoatFlow afterwards | empty (admin stays disabled) |
| `BASE_URL` | No | Public URL for the application; password-reset and customer sign-up email links are built from it | `https://$DOMAIN` |
| `EMAIL_ENABLED` | No | Send outgoing email; `false` keeps it in the queue | `true` |
| `EMAIL_FROM` | No | Sender address | `noreply@example.com` |
| `SMTP_HOST` | **Yes** | SMTP server | - |
| `SMTP_PORT` | No | SMTP port | `587` |
| `SMTP_USER` / `SMTP_PASSWORD` | No | SMTP login (no login when empty) | empty |
| `SMTP_TLS` | No | STARTTLS (`true`/`false`) | `true` |
| `SMTP_AUTH_TYPE` | No | `plain` or `login` | `plain` |
| `APP_ENV` | No | Application environment | `production` |
| `GIN_MODE` | No | Web framework mode | `release` |
| `LOG_LEVEL` | No | Log level | `warn` |

## Image Tags

`app` and `customer-fe` use `ghcr.io/goatkit/goatflow`; `runner` uses
`ghcr.io/goatkit/goatflow-runner`. Tags have no `v` prefix.

| Tag | Description | Runner image too? |
|-----|-------------|-------------------|
| `0.10.0` | Specific release (recommended for production) | Yes |
| `0.10`, `0` | Newest release in that minor / major line | Yes |
| `latest` | Newest build of the `main` branch | Yes |
| `main` | Newest build of the `main` branch | Yes |
| `dev` | Newest build of the `dev` branch (unstable, amd64 only) | No |

`GOATFLOW_TAG` sets the tag for all three services, so pick a tag that exists for the runner.

## Outgoing Email

Ticket emails to customers and password-reset and sign-up links are put in the `mail_queue`
table and sent by the `runner`; a few messages (for example two-factor codes) are sent by the
app directly. Both get the `SMTP_*` / `EMAIL_*` settings from `.env` (passed as
`GOATFLOW_EMAIL_*`).

## URLs

Caddy sends `/customer/*` and the customer login API to `customer-fe` and everything else to
`app`. `/c/<path>` redirects to `/customer/<path>`.

## Services

| Service | Purpose |
|---------|---------|
| `app` | Main GoatFlow application |
| `runner` | Background tasks: sends queued outgoing email, delivers webhooks, cleans up expired sessions |
| `mariadb` | MariaDB database |
| `valkey` | Redis-compatible cache |
| `caddy` | Reverse proxy - serves 80/443 |
| `customer-fe` | Customer portal frontend (same image, customer-only mode) |

## Operations

```bash
# View logs
docker compose logs -f app

# Stop
docker compose down

# Update to latest images
docker compose pull
docker compose up -d

# Backup database
docker compose exec mariadb mariadb-dump -u root -p goatflow > backup.sql
```

## Production Considerations

This is a **reference configuration**. For production, consider:

- External database (managed MariaDB/MySQL)
- Secrets management (not plain `.env` files)
- Reverse proxy with TLS (Caddy included, or Traefik, etc.)
- Volume backups
- Resource limits
- Monitoring and alerting

## License

See [LICENSE](../LICENSE) in the main repository.
