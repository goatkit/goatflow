# GoatFlow Deployment

Reference Docker Compose configuration for deploying GoatFlow.

## Quick Start

```bash
# Download files
curl -O https://raw.githubusercontent.com/goatkit/goatflow/main/deploy/docker-compose.yml
curl -O https://raw.githubusercontent.com/goatkit/goatflow/main/deploy/.env.example

# Configure
cp .env.example .env
# Edit .env - change DB_ROOT_PASSWORD, DB_PASSWORD, and JWT_SECRET at minimum

# Start
docker compose up -d
```

## Configuration

All configuration is via environment variables in `.env`:

| Variable | Required | Description | Default |
|----------|----------|-------------|---------|
| `GOATFLOW_TAG` | No | Image tag to use | `latest` |
| `DB_ROOT_PASSWORD` | **Yes** | MariaDB root password | - |
| `DB_USER` | No | MariaDB username | `goatflow` |
| `DB_PASSWORD` | **Yes** | MariaDB user password | - |
| `DB_NAME` | No | Database name | `goatflow` |
| `JWT_SECRET` | **Yes** | JWT signing secret (32+ chars) | - |
| `APP_PORT` | No | Host port for web UI | `8080` |
| `BASE_URL` | No | Public URL for the application | `http://localhost:8080` |

## Image Tags

| Tag | Description |
|-----|-------------|
| `latest` | Latest release from main branch |
| `stable` | Alias for latest stable release |
| `v1.2.3` | Specific version (recommended for production) |
| `dev` | Latest development build (unstable) |

## Services

| Service | Purpose |
|---------|---------|
| `app` | Main GoatFlow application |
| `runner` | Background job processor |
| `mariadb` | MariaDB database |
| `valkey` | Redis-compatible cache |

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
