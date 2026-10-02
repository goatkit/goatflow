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
# GOATFLOW_SECURE_KEY and BASE_URL

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
| `GOATFLOW_SECURE_KEY` | Strongly recommended | Encrypts stored secrets (plugin secure settings, webhook signing secrets); 64 hex characters (`openssl rand -hex 32`). App and runner get the same value. If empty, each container makes its own random key at start, so stored secrets cannot be read after a restart and webhooks fail. Never change it once set. | empty |
| `BASE_URL` | Recommended | Public URL for the application; password-reset and customer sign-up email links are built from it (they are not sent while it is unset) | `http://localhost:8080` |
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

All outgoing email (ticket emails to customers, password-reset and sign-up links) is put in the
`mail_queue` table and sent by the `runner`. The runner reads its SMTP settings from the
`email.*` config, which you can set with environment variables. This compose file passes none,
so add them to the `runner` service:

| Variable | Meaning |
|----------|---------|
| `GOATFLOW_EMAIL_SMTP_HOST` / `GOATFLOW_EMAIL_SMTP_PORT` | SMTP server |
| `GOATFLOW_EMAIL_SMTP_USER` / `GOATFLOW_EMAIL_SMTP_PASSWORD` | SMTP login |
| `GOATFLOW_EMAIL_SMTP_TLS` | Use TLS (`true`/`false`) |
| `GOATFLOW_EMAIL_FROM` | Sender address |

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
