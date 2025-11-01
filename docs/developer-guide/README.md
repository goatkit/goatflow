# GOTRS Developer Guide

## Coming Soon

Comprehensive guide for developers working with and extending GOTRS.

## Planned Content

### Getting Started
- Development environment setup
- Architecture overview
- Code structure
- Development workflow
- Debugging tips

### Backend Development
- Go project structure
- Service layer architecture
- Database operations
- Authentication & authorization
- API development
- Testing strategies
- Error handling
- Logging

### Frontend Development
- React architecture
- Component structure
- State management (Redux)
- API integration
- Styling and theming
- Testing React components
- Build optimization

### API Development
- REST API design
- GraphQL schema
- WebSocket implementation
- API versioning
- Rate limiting
- Authentication
- Documentation

### Database
- Schema design
- Migrations
- Query optimization
- Transactions
- Connection pooling
- Backup strategies

### Plugin Development
- Plugin architecture
- Creating backend plugins
- Creating frontend plugins
- Hook system
- Event bus
- Plugin testing
- Publishing plugins

### Testing
- Unit testing
- Integration testing
- End-to-end testing
- Performance testing
- Security testing
- Test coverage

### DevOps
- CI/CD pipeline
- Docker builds
- Kubernetes deployment
- Monitoring
- Logging
- Performance profiling

### Contributing
- Code style guide
- Git workflow
- Pull request process
- Code review guidelines
- Documentation standards

## Quick Start

### Setup Development Environment

#### Container-Based Development (Recommended)

```bash
# Clone repository
git clone https://github.com/gotrs-io/gotrs-ce.git
cd gotrs-ce

# Copy environment configuration
cp .env.example .env

# Start all services with hot reload
make up

# View logs
make logs

# Access services:
# - Frontend: http://localhost:3000
# - Backend API: http://localhost:8080
# - Mailhog: http://localhost:8025
```

#### Local Development (Without Containers)

```bash
# Prerequisites: Go 1.21+, Node.js 18+, PostgreSQL 15+, Valkey 7+

# Backend setup
make up

# Frontend development (in separate terminal)  
make frontend-dev
```

#### Development Commands

```bash
# Container management
make up           # Start all services
make down         # Stop all services  
make logs         # View logs
make clean        # Reset everything
make db-shell     # PostgreSQL shell

# Development tools
make lint         # Run linters (when available)
make test         # Run tests (when available)
```

### Project Structure

```
gotrs/
├── cmd/           # Application entrypoints
├── internal/      # Private application code
├── pkg/           # Public libraries
├── web/           # Frontend application
├── api/           # API specifications
├── migrations/    # Database migrations
└── tests/         # Test suites
```

### Key Technologies

- **Backend**: Go, Gin, PostgreSQL, Valkey
- **Frontend**: React, TypeScript, Vite, Material-UI
- **Containers**: Docker/Podman (rootless support)
- **Testing**: Go testing, Jest, Playwright
- **DevOps**: Docker, Kubernetes, GitHub Actions

## Development Troubleshooting

### Common Issues

#### Frontend Permission Errors

```bash
# Error: EACCES: permission denied, open '/app/vite.config.ts.timestamp-*'
# Solution: Fix file ownership (usually not needed)
sudo chown -R $USER:$USER ./web/
make down && make up
```

#### Container Build Issues

```bash
# Clean rebuild
make clean
podman system prune -f  # or docker system prune -f
make up
```

#### Hot Reload Not Working

```bash
# Ensure volume mounts are correct
make down
make up

# Check container logs
podman logs gotrs-frontend
podman logs gotrs-backend
```

#### Port Conflicts

```bash
# Change ports in .env file:
FRONTEND_PORT=3001
BACKEND_PORT=8081
```

### Development Tips

- Use `make logs` to monitor all services
- Frontend changes auto-reload via Vite
- Backend changes auto-reload via Air
- Database persists between container restarts
- Use Mailhog to test email functionality and threading (Message-ID, In-Reply-To, References headers)

## API Reference

- [REST API](../api/README.md)
- [GraphQL Schema](../api/graphql.md)
- [WebSocket Events](../api/websocket.md)

## Resources

- [Architecture](../../ARCHITECTURE.md)
- [Database Schema](../development/DATABASE.md)
- [Security](../SECURITY.md)
- [Contributing](../../CONTRIBUTING.md)

---

*Full developer documentation coming soon. For MVP development, see [MVP Guide](../development/MVP.md)*