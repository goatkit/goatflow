# Docker Deployment Guide

GOTRS provides two deployment methods depending on your needs.

## Method 1: Quick Deploy (Production)

Download just the deployment files and run. Best for production servers where you want to run pre-built images.

```bash
# Create deployment directory
mkdir gotrs && cd gotrs

# Download deployment files
curl -O https://raw.githubusercontent.com/gotrs-io/gotrs-ce/main/deploy/docker-compose.yml
curl -O https://raw.githubusercontent.com/gotrs-io/gotrs-ce/main/deploy/.env.example

# Configure environment
cp .env.example .env
# Edit .env with your values (DOMAIN, DB_PASSWORD, JWT_SECRET, etc.)

# Start GOTRS
docker compose up -d
```

### Required Environment Variables

Edit `.env` before starting:

| Variable | Description | Example |
|----------|-------------|---------|
| `DOMAIN` | Your domain name | `tickets.example.com` |
| `ACME_EMAIL` | Email for Let's Encrypt | `admin@example.com` |
| `DB_PASSWORD` | Database password | (generate a secure password) |
| `DB_ROOT_PASSWORD` | MariaDB root password | (generate a secure password) |
| `JWT_SECRET` | JWT signing secret | (generate with `openssl rand -hex 32`) |

### What's Included

The deployment stack includes:
- **Caddy** - Reverse proxy with automatic HTTPS via Let's Encrypt
- **MariaDB** - Database server
- **Valkey** - Cache server (Redis-compatible)
- **GOTRS App** - Main application (agent interface)
- **GOTRS Customer-FE** - Customer portal
- **GOTRS Runner** - Background job processor

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
git clone https://github.com/gotrs-io/gotrs-ce.git
cd gotrs-ce

# Copy environment template
cp .env.example .env
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
| `make restart` | Rebuild and restart |
| `make logs` | Follow container logs |
| `make ps` | Show running containers |
| `make build` | Build all containers |

### Development Features

The development setup includes:
- Hot reload for development
- Toolbox container for running Go commands
- Test database support
- Migration tools

See [Development Guide](../development/MVP.md) for more details.

---

## System Requirements

- **OS**: Linux (64-bit) - Ubuntu, Debian, RHEL, CentOS
- **RAM**: Minimum 4GB, Recommended 8GB+
- **CPU**: 2+ cores
- **Disk**: 20GB+ available space
- **Container Runtime**: Docker 24.0+ or Podman 4.0+ with Compose

---

## Podman Support

GOTRS works with Podman as a drop-in replacement for Docker. Podman runs rootless by default, which aligns with GOTRS's security model.

### Quick Deploy with Podman

```bash
# Download deployment files
mkdir gotrs && cd gotrs
curl -O https://raw.githubusercontent.com/gotrs-io/gotrs-ce/main/deploy/docker-compose.yml
curl -O https://raw.githubusercontent.com/gotrs-io/gotrs-ce/main/deploy/.env.example

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