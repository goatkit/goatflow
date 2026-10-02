# Docker Deployment Guide

GoatFlow provides two deployment methods depending on your needs.

## Method 1: Quick Deploy (Production)

Download just the deployment files and run. Best for production servers where you want to run pre-built images.

```bash
# Create deployment directory
mkdir goatflow && cd goatflow

# Download deployment files
curl -O https://raw.githubusercontent.com/goatkit/goatflow/main/deploy/docker-compose.yml
curl -O https://raw.githubusercontent.com/goatkit/goatflow/main/deploy/.env.example

# Configure environment
cp .env.example .env
# Edit .env with your values (DOMAIN, ACME_EMAIL, DB passwords, JWT_SECRET, GOATFLOW_SECURE_KEY, BASE_URL)

# Start GoatFlow
docker compose up -d
```

### Environment Variables

Edit `.env` before starting. The compose file refuses to start without the variables marked
"Yes".

| Variable | Required | Description | Example |
|----------|----------|-------------|---------|
| `DOMAIN` | Yes | Your domain name (Caddy gets a certificate for it) | `tickets.example.com` |
| `ACME_EMAIL` | Yes | Email for Let's Encrypt | `admin@example.com` |
| `DB_PASSWORD` | Yes | Database password | (generate a secure password) |
| `DB_ROOT_PASSWORD` | Yes | MariaDB root password | (generate a secure password) |
| `JWT_SECRET` | Yes | JWT signing secret, at least 32 characters | (generate with `openssl rand -hex 32`) |
| `GOATFLOW_SECURE_KEY` | Strongly recommended | Encrypts stored secrets (plugin secure settings, webhook signing secrets). 64 hex characters. The app and the runner must use the same value. If it is empty, each container makes its own random key at start, so those secrets cannot be read after a restart and webhooks fail. Never change it once set. | (generate with `openssl rand -hex 32`) |
| `BASE_URL` | Recommended | Public URL, e.g. `https://tickets.example.com`. Password-reset and sign-up email links are built from it. Default `http://localhost:8080`. | `https://tickets.example.com` |
| `GOATFLOW_TAG` | No | Image tag for app, customer portal and runner. Default `latest`. | `0.10.0` |

### Image Tags

Images are published to `ghcr.io/goatkit/goatflow` (app and customer portal) and
`ghcr.io/goatkit/goatflow-runner` (runner).

| Tag | Meaning | Runner image too? |
|-----|---------|-------------------|
| `0.10.0` | That release (recommended for production) | Yes |
| `0.10`, `0` | Newest release in that minor / major line | Yes |
| `latest` | Newest build of the `main` branch | Yes |
| `main` | Newest build of the `main` branch | Yes |
| `dev` | Newest build of the `dev` branch (unstable, amd64 only) | No |

Tags have no `v` prefix. `GOATFLOW_TAG` sets the tag for both images, so use a tag that exists
for the runner as well.

### What's Included

The deployment stack includes:
- **Caddy** - Reverse proxy with automatic HTTPS via Let's Encrypt
- **MariaDB** - Database server
- **Valkey** - Cache server (Redis-compatible)
- **GoatFlow App** - Main application (agent interface)
- **GoatFlow Customer-FE** - Customer portal
- **GoatFlow Runner** - Background tasks: sends queued outgoing email, delivers webhooks, cleans up expired sessions

All services are configured with `restart: unless-stopped` so they automatically start on boot.

### Management Commands

```bash
# View logs
docker compose logs -f

# Stop services
docker compose down

# Update to latest version
docker compose pull
docker compose up -d

# View running containers
docker compose ps
```

---

## Method 2: Development Setup (Full Repository)

Clone the full repository for development or customization. Uses Makefile targets for all operations.

```bash
# Clone repository
git clone https://github.com/goatkit/goatflow.git
cd goatflow

# Copy environment template
cp .env.development .env
# Edit .env if needed

# Start all services (builds containers locally)
make up-d

# View logs
make logs

# Stop services
make down
```

### Makefile Targets

| Target | Description |
|--------|-------------|
| `make up` | Start services (attached) |
| `make up-d` | Start services (detached) |
| `make down` | Stop all services |
| `make restart` | Stop and start all services (`down` then `up-d`) |
| `make logs` | Follow container logs |
| `make ps` | Show running containers |
| `make build` | Build all containers |

### Development Features

- Toolbox container for running Go commands
- Test database support
- Migration tools

See the [Developer Guide](../developer-guide/README.md) for more details.

---

## System Requirements

- **OS**: Linux (64-bit) - Ubuntu, Debian, RHEL, CentOS
- **RAM**: Minimum 4GB, Recommended 8GB+
- **CPU**: 2+ cores
- **Disk**: 20GB+ available space
- **Container Runtime**: Docker 24.0+ or Podman 4.0+ with Compose

---

## Podman Support

GoatFlow works with Podman as a drop-in replacement for Docker. Podman runs rootless by default, which aligns with GoatFlow's security model.

### Quick Deploy with Podman

```bash
# Download deployment files
mkdir goatflow && cd goatflow
curl -O https://raw.githubusercontent.com/goatkit/goatflow/main/deploy/docker-compose.yml
curl -O https://raw.githubusercontent.com/goatkit/goatflow/main/deploy/.env.example

# Configure environment
cp .env.example .env
# Edit .env with your values

# Start with Podman Compose
podman-compose up -d
```

### Development with Podman

The Makefile auto-detects Podman vs Docker. If you prefer Podman explicitly:

```bash
# Set container command (add to .env or export)
export CONTAINER_CMD=podman

# Then use make targets as normal
make up-d
make logs
```

### Podman Notes

- **Rootless**: Podman runs without root by default - no daemon required
- **Systemd Integration**: Use `podman generate systemd` for service files
- **Socket Activation**: Enable `podman.socket` for Docker API compatibility
- **SELinux**: On RHEL/Fedora, volumes may need `:Z` suffix for SELinux labels

```bash
# Enable Podman socket for Docker compatibility
systemctl --user enable --now podman.socket
export DOCKER_HOST=unix:///run/user/$(id -u)/podman/podman.sock
```

---

## See Also

- [Kubernetes Deployment](kubernetes.md)
- [Architecture Overview](../ARCHITECTURE.md)
- [Migration Guide](../MIGRATION.md)