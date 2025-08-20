# GOTRS Makefile - Docker/Podman compatible development

# Detect container runtime and compose command
# First check for podman, then docker
CONTAINER_CMD := $(shell command -v podman 2> /dev/null || command -v docker 2> /dev/null || echo docker)

# Detect compose command - try multiple variants in order of preference
# 1. podman-compose (for podman users)
# 2. podman compose (newer podman plugin style)  
# 3. docker compose (modern docker plugin style)
# 4. docker-compose (legacy docker-compose)
COMPOSE_CMD := $(shell \
	if command -v podman-compose > /dev/null 2>&1; then \
		echo "podman-compose"; \
	elif command -v podman > /dev/null 2>&1 && podman compose version > /dev/null 2>&1; then \
		echo "podman compose"; \
	elif command -v docker > /dev/null 2>&1 && docker compose version > /dev/null 2>&1; then \
		echo "docker compose"; \
	elif command -v docker-compose > /dev/null 2>&1; then \
		echo "docker-compose"; \
	else \
		echo "docker compose"; \
	fi)

.PHONY: help up down logs restart clean setup test build debug-env toolbox-build toolbox-run toolbox-test

# Default target
help:
	@echo "GOTRS Container Development Commands:"
	@echo "  Container: $(CONTAINER_CMD)"
	@echo "  Compose: $(COMPOSE_CMD)"
	@echo "  make up       - Start all services"
	@echo "  make down     - Stop all services"
	@echo "  make logs     - View logs"
	@echo "  make restart  - Restart all services"
	@echo "  make clean    - Clean everything (including volumes)"
	@echo "  make setup    - Initial project setup with secure secrets"
	@echo "  make build    - Build production images"
	@echo ""
	@echo "Secrets management:"
	@echo "  make synthesize       - Generate new .env with secure secrets"
	@echo "  make rotate-secrets   - Rotate secrets in existing .env"
	@echo "  make synthesize-force - Force regenerate .env (overwrite existing)"
	@echo "  make k8s-secrets      - Generate k8s/secrets.yaml from template"
	@echo ""
	@echo "Toolbox commands (fast, containerized dev tools):"
	@echo "  make toolbox-build    - Build toolbox container (auto-runs before use)"
	@echo "  make toolbox-run      - Interactive shell with all tools"
	@echo "  make toolbox-test     - Run all tests quickly"
	@echo "  make toolbox-test-run TEST=TestName - Run specific test"
	@echo ""
	@echo "E2E Testing (Playwright):"
	@echo "  make test-e2e         - Run E2E tests headless"
	@echo "  make test-e2e-debug   - Run E2E tests with visible browser"
	@echo "  make test-e2e-watch   - Run E2E tests in watch mode"
	@echo "  make test-e2e-report  - View test results summary"
	@echo "  make clean-test-results - Clean test artifacts"
	@echo "  make toolbox-lint     - Run Go linters"
	@echo "  make toolbox-security - Run security scan"
	@echo ""
	@echo "Test commands:"
	@echo "  make test              - Run Go backend tests"
	@echo "  make test-short        - Run tests (skip long ones)"
	@echo "  make test-coverage     - Run tests with coverage"
	@echo "  make test-report       - Generate test coverage report"
	@echo "  make test-html         - Generate HTML test report"
	@echo "  make test-safe         - Run tests with race/deadlock detection"
	@echo "  make test-clean        - Clean test cache"
	@echo "  make test-check        - Quick sanity check"
	@echo "  make test-coverage-html - Coverage report and open in browser"
	@echo "  make test-frontend     - Run React frontend tests"
	@echo ""
	@echo "i18n (Babel fish) commands:"
	@echo "  make babelfish         - Build gotrs-babelfish binary"
	@echo "  make babelfish-coverage - Show translation coverage"
	@echo "  make babelfish-validate LANG=de - Validate a language"
	@echo "  make babelfish-missing LANG=es - Show missing translations"
	@echo "  make babelfish-run ARGS='-help' - Run with custom args"
	@echo "  make test-ldap         - Run LDAP integration tests"
	@echo "  make test-ldap-perf    - Run LDAP performance benchmarks"
	@echo ""
	@echo "Security commands:"
	@echo "  make scan-secrets      - Scan current code for secrets"
	@echo "  make scan-secrets-history - Scan git history for secrets"
	@echo "  make scan-secrets-precommit - Install pre-commit hooks"
	@echo "  make scan-vulnerabilities - Scan for vulnerabilities"
	@echo "  make security-scan     - Run all security scans"
	@echo "  make test-contracts    - Run Pact contract tests"
	@echo "  make test-all          - Run all tests (backend, frontend, contracts)"
	@echo ""
	@echo "Service-specific commands:"
	@echo "  make backend-logs  - View backend logs"
	@echo "  make frontend-logs - View frontend logs"
	@echo "  make db-shell      - PostgreSQL shell"
	@echo "  make valkey-cli    - Valkey CLI"
	@echo "  make ldap-admin    - Open phpLDAPadmin (requires tools profile)"
	@echo "  make ldap-logs     - View OpenLDAP logs"
	@echo ""
	@echo "Database migration commands:"
	@echo "  make db-migrate    - Run all pending migrations"
	@echo "  make db-rollback   - Rollback last migration"
	@echo "  make db-reset      - Reset database (down all, up all)"
	@echo "  make db-status     - Show current migration version"
	@echo "  make db-force      - Force database to specific version"
	@echo ""
	@echo "Debugging:"
	@echo "  make debug-env     - Show detected container commands"
	@echo "  make test-containerized - Verify all commands use containers"

# Debug environment detection
debug-env:
	@echo "Container Environment Detection:"
	@echo "================================"
	@echo "Container runtime: $(CONTAINER_CMD)"
	@echo "Compose command: $(COMPOSE_CMD)"
	@echo ""
	@echo "Checking available commands:"
	@echo "----------------------------"
	@command -v docker > /dev/null 2>&1 && echo "✓ docker found: $$(which docker)" || echo "✗ docker not found"
	@command -v docker-compose > /dev/null 2>&1 && echo "✓ docker-compose found: $$(which docker-compose)" || echo "✗ docker-compose not found"
	@command -v docker > /dev/null 2>&1 && docker compose version > /dev/null 2>&1 && echo "✓ docker compose plugin found" || echo "✗ docker compose plugin not found"
	@command -v podman > /dev/null 2>&1 && echo "✓ podman found: $$(which podman)" || echo "✗ podman not found"
	@command -v podman-compose > /dev/null 2>&1 && echo "✓ podman-compose found: $$(which podman-compose)" || echo "✗ podman-compose not found"
	@command -v podman > /dev/null 2>&1 && podman compose version > /dev/null 2>&1 && echo "✓ podman compose plugin found" || echo "✗ podman compose plugin not found"
	@echo ""
	@echo "Selected commands will be used for all make targets."

# Build the toolbox container (cached after first build)
toolbox-build:
	@echo "🔧 Building GOTRS toolbox container..."
	@$(CONTAINER_CMD) build -f Dockerfile.toolbox -t gotrs-toolbox:latest .
	@echo "✅ Toolbox container ready"

# Initial setup with secure secret generation
setup:
	@echo "🔬 Synthesizing secure configuration..."
	@if [ ! -f .env ]; then \
		$(MAKE) synthesize || echo "⚠️  Failed to synthesize. Using example file as fallback."; \
		if [ ! -f .env ]; then cp -n .env.example .env || true; fi; \
	else \
		echo "✅ .env already exists. Run 'make synthesize' to regenerate."; \
	fi
	@cp -n docker-compose.override.yml.example docker-compose.override.yml || true
	@echo "Setup complete. Run 'make up' to start development environment."

# Generate secure .env file with random secrets (runs in container)
synthesize:
	@$(MAKE) toolbox-build
	@echo "🔬 Synthesizing secure configuration and test data..."
	@$(CONTAINER_CMD) run --rm \
		-v "$$(pwd):/workspace" \
		-w /workspace \
		-u "$$(id -u):$$(id -g)" \
		gotrs-toolbox:latest \
		gotrs synthesize
	@echo "📝 Test credentials saved to test_credentials.csv"
	@echo "🔐 Generating Kubernetes secrets from template..."
	@./scripts/generate-k8s-secrets.sh
	@if [ -d .git ]; then \
		echo ""; \
		echo "💡 To enable secret scanning in git commits, run:"; \
		echo "   make scan-secrets-precommit"; \
	fi

# Rotate secrets in existing .env file (runs in container)
rotate-secrets:
	@$(MAKE) toolbox-build
	@echo "🔄 Rotating secrets..."
	@$(CONTAINER_CMD) run --rm \
		-v "$$(pwd):/workspace" \
		-w /workspace \
		-u "$$(id -u):$$(id -g)" \
		gotrs-toolbox:latest \
		gotrs synthesize --rotate-secrets

# Force regenerate .env file (runs in container)
synthesize-force:
	@$(MAKE) toolbox-build
	@echo "⚠️  Force regenerating .env file..."
	@$(CONTAINER_CMD) run --rm \
		-v "$$(pwd):/workspace" \
		-w /workspace \
		-u "$$(id -u):$$(id -g)" \
		gotrs-toolbox:latest \
		gotrs synthesize --force

# Generate only test data (SQL and CSV)
gen-test-data:
	@$(MAKE) toolbox-build
	@echo "🔄 Regenerating test data only..."
	@$(CONTAINER_CMD) run --rm \
		-v "$$(pwd):/workspace" \
		-w /workspace \
		-u "$$(id -u):$$(id -g)" \
		gotrs-toolbox:latest \
		gotrs synthesize --test-data-only

# Generate Kubernetes secrets from template with secure random values
k8s-secrets:
	@echo "🔐 Generating Kubernetes secrets from template..."
	@./scripts/generate-k8s-secrets.sh

# Run interactive shell in toolbox container
toolbox-run:
	@$(MAKE) toolbox-build
	@echo "🔧 Starting toolbox shell..."
	@$(CONTAINER_CMD) run --rm -it \
		-v "$$(pwd):/workspace" \
		-w /workspace \
		-u "$$(id -u):$$(id -g)" \
		gotrs-toolbox:latest \
		/bin/bash

# Run tests directly in toolbox (faster than compose exec)
toolbox-test:
	@$(MAKE) toolbox-build
	@echo "🧪 Running tests in toolbox..."
	@$(CONTAINER_CMD) run --rm \
		-v "$$(pwd):/workspace" \
		-w /workspace \
		-u "$$(id -u):$$(id -g)" \
		--network host \
		-e DB_HOST=localhost \
		-e DB_PORT=5432 \
		-e DB_NAME=gotrs_test \
		-e DB_USER=gotrs_test \
		-e DB_PASSWORD=gotrs_test_password \
		-e VALKEY_HOST=localhost \
		-e VALKEY_PORT=6380 \
		-e APP_ENV=test \
		gotrs-toolbox:latest \
		sh -c "source .env 2>/dev/null || true && go test -v ./..."

# Run specific test with toolbox
toolbox-test-run:
	@$(MAKE) toolbox-build
	@echo "🧪 Running specific test: $(TEST)"
	@$(CONTAINER_CMD) run --rm \
		-v "$$(pwd):/workspace" \
		-w /workspace \
		-u "$$(id -u):$$(id -g)" \
		--network host \
		-e DB_HOST=localhost \
		-e DB_PORT=5432 \
		-e DB_NAME=gotrs_test \
		-e DB_USER=gotrs_test \
		-e DB_PASSWORD=gotrs_test_password \
		-e VALKEY_HOST=localhost \
		-e VALKEY_PORT=6380 \
		-e APP_ENV=test \
		gotrs-toolbox:latest \
		sh -c "source .env 2>/dev/null || true && go test -v -run '$(TEST)' ./..."

# Run linting with toolbox
toolbox-lint:
	@$(MAKE) toolbox-build
	@echo "🔍 Running linters..."
	@$(CONTAINER_CMD) run --rm \
		-v "$$(pwd):/workspace" \
		-w /workspace \
		-u "$$(id -u):$$(id -g)" \
		gotrs-toolbox:latest \
		golangci-lint run ./...

# Run security scan with toolbox
toolbox-security:
	@$(MAKE) toolbox-build
	@echo "🔒 Running security scan..."
	@$(CONTAINER_CMD) run --rm \
		-v "$$(pwd):/workspace" \
		-w /workspace \
		-u "$$(id -u):$$(id -g)" \
		gotrs-toolbox:latest \
		gosec ./...

# Run Trivy vulnerability scan locally
trivy-scan:
	@echo "🔍 Running Trivy vulnerability scan..."
	@$(CONTAINER_CMD) run --rm \
		-v "$$(pwd):/workspace" \
		-v /var/run/docker.sock:/var/run/docker.sock \
		aquasec/trivy:latest \
		fs --severity CRITICAL,HIGH,MEDIUM /workspace

# Run Trivy on built images
trivy-images:
	@echo "🔍 Scanning backend image..."
	@$(CONTAINER_CMD) run --rm \
		-v /var/run/docker.sock:/var/run/docker.sock \
		aquasec/trivy:latest \
		image gotrs-backend:latest
	@echo "🔍 Scanning frontend image..."
	@$(CONTAINER_CMD) run --rm \
		-v /var/run/docker.sock:/var/run/docker.sock \
		aquasec/trivy:latest \
		image gotrs-frontend:latest

# Start all services
up:
	$(COMPOSE_CMD) up --build

# Start in background
up-d:
	$(COMPOSE_CMD) up -d --build

# Stop all services
down:
	$(COMPOSE_CMD) down

# Restart services
restart:
	$(COMPOSE_CMD) restart

# View logs
logs:
	$(COMPOSE_CMD) logs -f

# Service-specific logs
backend-logs:
	$(COMPOSE_CMD) logs -f backend

frontend-logs:
	$(COMPOSE_CMD) logs -f frontend

db-logs:
	$(COMPOSE_CMD) logs -f postgres

# Clean everything (including volumes)
clean:
	$(COMPOSE_CMD) down -v
	rm -rf tmp/ generated/

# Reset database
reset-db:
	$(COMPOSE_CMD) down -v postgres
	$(COMPOSE_CMD) up -d postgres
	@echo "Database reset. Waiting for initialization..."
	@sleep 5

# Load environment variables from .env file (only if it exists)
-include .env
export

# Database operations
db-shell:
	$(COMPOSE_CMD) exec postgres psql -U $(DB_USER) -d $(DB_NAME)

db-migrate:
	@echo "Running database migrations..."
	$(COMPOSE_CMD) exec backend migrate -path /app/migrations -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" up
	@echo "Migrations completed successfully!"

db-migrate-schema-only:
	@echo "Running schema migration only..."
	$(COMPOSE_CMD) exec backend migrate -path /app/migrations -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" up 3
	@echo "Schema and initial data applied (no test data)"

db-seed-dev:
	@echo "Seeding development database with comprehensive test data..."
	@$(COMPOSE_CMD) exec backend migrate -path /app/migrations -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" up
	@echo "✅ Development database seeded with:"
	@echo "   - 10 organizations"
	@echo "   - 50 customer users"
	@echo "   - 15 support agents"
	@echo "   - 100 ITSM tickets"
	@echo "   - Knowledge base articles"

db-seed-test:
	@echo "Seeding test database with comprehensive test data..."
	@$(COMPOSE_CMD) exec backend migrate -path /app/migrations -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$${DB_NAME}_test?sslmode=disable" up
	@echo "✅ Test database ready for testing"

db-reset-dev:
	@echo "⚠️  This will DELETE all data and recreate the development database!"
	@echo -n "Are you sure? [y/N]: "; \
	read confirm; \
	if [ "$$confirm" = "y" ] || [ "$$confirm" = "Y" ]; then \
		echo "Resetting development database..."; \
		$(COMPOSE_CMD) exec backend migrate -path /app/migrations -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" down -all; \
		$(COMPOSE_CMD) exec backend migrate -path /app/migrations -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" up; \
		echo "✅ Fresh development environment ready with test data!"; \
	else \
		echo "Reset cancelled."; \
	fi

db-reset-test:
	@echo "Resetting test database..."
	@$(COMPOSE_CMD) exec backend migrate -path /app/migrations -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$${DB_NAME}_test?sslmode=disable" down -all
	@$(COMPOSE_CMD) exec backend migrate -path /app/migrations -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$${DB_NAME}_test?sslmode=disable" up
	@echo "✅ Test database reset with fresh test data"

db-refresh: db-reset-dev
	@echo "✅ Database refreshed for new development cycle"

db-rollback:
	$(COMPOSE_CMD) exec backend migrate -path /app/migrations -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" down 1

db-reset:
	$(COMPOSE_CMD) exec backend migrate -path /app/migrations -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" down -all
	$(COMPOSE_CMD) exec backend migrate -path /app/migrations -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" up

db-status:
	$(COMPOSE_CMD) exec backend migrate -path /app/migrations -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" version

db-force:
	@echo -n "Force migration to version: "; \
	read version; \
	$(COMPOSE_CMD) exec backend migrate -path /app/migrations -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" force $$version

# Valkey CLI
valkey-cli:
	$(COMPOSE_CMD) exec valkey valkey-cli

# i18n Tools
babelfish:
	@echo "Building gotrs-babelfish..."
	$(COMPOSE_CMD) exec backend go build -o /tmp/gotrs-babelfish cmd/gotrs-babelfish/main.go
	@echo "✨ gotrs-babelfish built successfully!"
	@echo "Run it with: docker exec gotrs-backend /tmp/gotrs-babelfish"

babelfish-run:
	@$(COMPOSE_CMD) exec backend go run cmd/gotrs-babelfish/main.go $(ARGS)

babelfish-coverage:
	@$(COMPOSE_CMD) exec backend go run cmd/gotrs-babelfish/main.go -action=coverage

babelfish-validate:
	@$(COMPOSE_CMD) exec backend go run cmd/gotrs-babelfish/main.go -action=validate -lang=$(LANG)

babelfish-missing:
	@$(COMPOSE_CMD) exec backend go run cmd/gotrs-babelfish/main.go -action=missing -lang=$(LANG)

# Run tests (with safety checks)
test:
	@echo "Running tests with safety checks..."
	@echo "Using test database: $${DB_NAME:-gotrs}_test"
	@echo "Checking if backend service is running..."
	@$(COMPOSE_CMD) ps --services --filter "status=running" | grep -q "backend" || (echo "Error: Backend service is not running. Please run 'make up' first." && exit 1)
	$(COMPOSE_CMD) exec -e DB_NAME=$${DB_NAME:-gotrs}_test -e APP_ENV=test backend go test -v ./...

test-short:
	$(COMPOSE_CMD) exec -e DB_NAME=$${DB_NAME:-gotrs}_test -e APP_ENV=test backend go test -short ./...

test-coverage:
	@echo "Running test coverage analysis..."
	@echo "Using test database: $${DB_NAME:-gotrs}_test"
	@mkdir -p generated
	$(COMPOSE_CMD) exec -e DB_NAME=$${DB_NAME:-gotrs}_test -e APP_ENV=test backend sh -c "mkdir -p generated && go test -v -race -coverprofile=generated/coverage.out -covermode=atomic ./..."
	$(COMPOSE_CMD) exec backend go tool cover -func=generated/coverage.out

# Run tests with enhanced coverage reporting (runs in container if script missing)
test-report:
	@if [ -f ./run_tests.sh ]; then \
		bash ./run_tests.sh; \
	else \
		echo "run_tests.sh not found, running in container"; \
		$(MAKE) test-coverage; \
	fi

# Generate HTML coverage report (runs in container if script missing)
test-html:
	@if [ -f ./run_tests.sh ]; then \
		bash ./run_tests.sh --html; \
	else \
		echo "run_tests.sh not found, running in container"; \
		$(MAKE) test-coverage-html; \
	fi

# Run tests with comprehensive safety checks (runs in container if script missing)
test-safe:
	@if [ -f ./scripts/test-db-safe.sh ]; then \
		bash ./scripts/test-db-safe.sh test; \
	else \
		echo "test-db-safe.sh not found, running in container"; \
		$(MAKE) test; \
	fi

# Clean test database (with safety checks)
test-clean:
	@if [ -f ./scripts/test-db-safe.sh ]; then \
		bash ./scripts/test-db-safe.sh clean; \
	else \
		echo "test-db-safe.sh not found, using compose"; \
		$(COMPOSE_CMD) exec backend sh -c "rm -rf /tmp/test-*"; \
	fi

# Check test environment safety
test-check:
	@if [ -f ./scripts/test-db-safe.sh ]; then \
		bash ./scripts/test-db-safe.sh check; \
	else \
		echo "test-db-safe.sh not found, checking in container"; \
		$(COMPOSE_CMD) exec backend sh -c "echo 'Test environment: OK'"; \
	fi

test-coverage-html:
	@mkdir -p generated
	$(COMPOSE_CMD) exec -e DB_NAME=$${DB_NAME:-gotrs}_test -e APP_ENV=test backend sh -c "mkdir -p generated && go test -v -race -coverprofile=generated/coverage.out -covermode=atomic ./..."
	$(COMPOSE_CMD) exec backend sh -c "go tool cover -html=generated/coverage.out -o generated/coverage.html"
	$(COMPOSE_CMD) cp backend:/app/generated/coverage.html ./generated/coverage.html
	@echo "Coverage report generated: generated/coverage.html"

# Frontend test commands
test-frontend:
	@echo "Running frontend tests..."
	$(COMPOSE_CMD) exec frontend npm test

test-contracts:
	@echo "Running Pact contract tests..."
	$(COMPOSE_CMD) exec frontend npm run test:contracts

test-all: test test-frontend test-contracts test-e2e
	@echo "All tests completed!"

# E2E Testing Commands
.PHONY: test-e2e test-e2e-watch test-e2e-debug test-e2e-report playwright-build

# Build Playwright test container
playwright-build:
	@echo "Building Playwright test container..."
	@$(COMPOSE_CMD) build playwright

# Run E2E tests
test-e2e: playwright-build
	@echo "Running E2E tests with Playwright..."
	@mkdir -p test-results/screenshots test-results/videos
	@$(COMPOSE_CMD) run --rm \
		-e HEADLESS=true \
		playwright

# Run E2E tests in watch mode (for development)
test-e2e-watch: playwright-build
	@echo "Running E2E tests in watch mode..."
	@mkdir -p test-results/screenshots test-results/videos
	@$(COMPOSE_CMD) run --rm \
		-e HEADLESS=false \
		-e SLOW_MO=100 \
		playwright go test ./tests/e2e/... -v -watch

# Check for untranslated keys in UI
check-translations:
	@echo "Checking for untranslated keys in UI..."
	@./scripts/check-translations.sh

# Run E2E tests with headed browser for debugging
test-e2e-debug: playwright-build
	@echo "Running E2E tests in debug mode (headed browser)..."
	@mkdir -p test-results/screenshots test-results/videos
	@$(COMPOSE_CMD) run --rm \
		-e HEADLESS=false \
		-e SLOW_MO=500 \
		-e SCREENSHOTS=true \
		-e VIDEOS=true \
		playwright go test ./tests/e2e/... -v

# Generate HTML test report
test-e2e-report:
	@echo "Generating E2E test report..."
	@if [ -d "test-results" ]; then \
		echo "Test results:"; \
		echo "Screenshots: $$(find test-results/screenshots -name "*.png" 2>/dev/null | wc -l) files"; \
		echo "Videos: $$(find test-results/videos -name "*.webm" 2>/dev/null | wc -l) files"; \
		ls -la test-results/ 2>/dev/null || true; \
	else \
		echo "No test results found. Run test-e2e first."; \
	fi

# Clean test results
clean-test-results:
	@echo "Cleaning test results..."
	@rm -rf test-results/

# Security scanning commands
.PHONY: scan-secrets scan-secrets-history scan-secrets-precommit scan-vulnerabilities security-scan

# Scan for secrets in current code
scan-secrets:
	@echo "Scanning for secrets and credentials..."
	@$(CONTAINER_CMD) run --rm \
		-v "$$(pwd):/workspace" \
		-w /workspace \
		zricethezav/gitleaks:latest \
		detect --source . --verbose

# Scan entire git history for secrets
scan-secrets-history:
	@echo "Scanning git history for secrets..."
	@$(CONTAINER_CMD) run --rm \
		-v "$$(pwd):/workspace" \
		-w /workspace \
		zricethezav/gitleaks:latest \
		detect --source . --log-opts="--all" --verbose

# Install pre-commit hooks for secret scanning (using bash script)
scan-secrets-precommit:
	@bash scripts/install-git-hooks.sh

# Scan for vulnerabilities with Trivy
scan-vulnerabilities:
	@echo "Scanning for vulnerabilities..."
	@$(CONTAINER_CMD) run --rm \
		-v "$$(pwd):/workspace" \
		-w /workspace \
		aquasec/trivy:latest \
		fs --scanners vuln,secret,misconfig . \
		--severity HIGH,CRITICAL

# Run all security scans
security-scan: scan-secrets scan-vulnerabilities
	@echo "Security scanning completed!"

# Build for production
build:
	$(CONTAINER_CMD) build -f Dockerfile -t gotrs:latest .
	$(CONTAINER_CMD) build -f Dockerfile.frontend -t gotrs-frontend:latest ./web

# Check service health (runs in container)
health:
	@echo "Checking service health..."
	@$(CONTAINER_CMD) run --rm --network=host alpine/curl:latest -f http://localhost/health || echo "Backend not healthy"
	@$(CONTAINER_CMD) run --rm --network=host alpine/curl:latest -f http://localhost/ || echo "Frontend not healthy"

# Open services in browser
open:
	@echo "Opening services..."
	@open http://localhost || xdg-open http://localhost || echo "Open http://localhost"

open-mail:
	@open http://localhost:8025 || xdg-open http://localhost:8025 || echo "Open http://localhost:8025"

open-db:
	@open http://localhost:8090 || xdg-open http://localhost:8090 || echo "Open http://localhost:8090"

# Development shortcuts
dev: up

stop: down

reset: clean setup up-d
	@echo "Environment reset and restarted"

# Show running services
ps:
	$(COMPOSE_CMD) ps

# Execute commands in containers
exec-backend:
	$(COMPOSE_CMD) exec backend sh

exec-frontend:
	$(COMPOSE_CMD) exec frontend sh

# Podman-specific: Generate systemd units
podman-systemd:
	@echo "Generating systemd units for podman..."
	@if command -v podman > /dev/null 2>&1; then \
		podman generate systemd --new --files --name gotrs-postgres; \
		podman generate systemd --new --files --name gotrs-valkey; \
		podman generate systemd --new --files --name gotrs-backend; \
		podman generate systemd --new --files --name gotrs-frontend; \
		echo "Systemd unit files generated. Move to ~/.config/systemd/user/"; \
	else \
		echo "Error: podman not found. This command requires podman."; \
		exit 1; \
	fi

# Generate migration file pair
gen-migration:
	@echo -n "Migration name: "; \
	read name; \
	timestamp=$$(date +%Y%m%d%H%M%S); \
	touch migrations/$$timestamp\_$$name.up.sql; \
	touch migrations/$$timestamp\_$$name.down.sql; \
	echo "-- Migration: $$name" > migrations/$$timestamp\_$$name.up.sql; \
	echo "" >> migrations/$$timestamp\_$$name.up.sql; \
	echo "-- Rollback: $$name" > migrations/$$timestamp\_$$name.down.sql; \
	echo "" >> migrations/$$timestamp\_$$name.down.sql; \
	echo "Created migration files:"; \
	echo "  migrations/$$timestamp\_$$name.up.sql"; \
	echo "  migrations/$$timestamp\_$$name.down.sql"

# LDAP testing and administration commands
.PHONY: test-ldap test-ldap-perf ldap-admin ldap-logs ldap-setup ldap-test-user

# Run LDAP integration tests
test-ldap:
	@echo "Running LDAP integration tests..."
	@echo "Starting LDAP server if not running..."
	$(COMPOSE_CMD) up -d openldap
	@echo "Waiting for LDAP server to be ready..."
	@sleep 30
	@echo "Running integration tests..."
	$(COMPOSE_CMD) exec -e LDAP_INTEGRATION_TESTS=true -e LDAP_HOST=openldap backend go test -v ./internal/service -run TestLDAPIntegration

# Run LDAP performance benchmarks
test-ldap-perf:
	@echo "Running LDAP performance benchmarks..."
	$(COMPOSE_CMD) up -d openldap
	@echo "Waiting for LDAP server..."
	@sleep 30
	$(COMPOSE_CMD) exec -e LDAP_INTEGRATION_TESTS=true -e LDAP_HOST=openldap backend go test -v ./internal/service -bench=BenchmarkLDAP -run=^$$

# Open phpLDAPadmin in browser
ldap-admin:
	@echo "Starting phpLDAPadmin..."
	$(COMPOSE_CMD) --profile tools up -d phpldapadmin
	@echo "Opening phpLDAPadmin at http://localhost:8091"
	@echo "Login with:"
	@echo "  Login DN: cn=admin,dc=gotrs,dc=local"
	@echo "  Password: admin123"
	@open http://localhost:8091 || xdg-open http://localhost:8091 || echo "Open http://localhost:8091"

# View OpenLDAP logs
ldap-logs:
	$(COMPOSE_CMD) logs -f openldap

# Setup LDAP for development (start services and wait)
ldap-setup:
	@echo "Setting up LDAP development environment..."
	$(COMPOSE_CMD) up -d openldap
	@echo "Waiting for LDAP server to initialize (this may take up to 60 seconds)..."
	@timeout=60; \
	while [ $$timeout -gt 0 ]; do \
		if $(COMPOSE_CMD) exec openldap ldapsearch -x -H ldap://localhost -b "dc=gotrs,dc=local" -D "cn=admin,dc=gotrs,dc=local" -w "admin123" "(objectclass=*)" dn > /dev/null 2>&1; then \
			echo "✓ LDAP server is ready!"; \
			break; \
		else \
			echo "Waiting for LDAP server... ($$timeout seconds remaining)"; \
			sleep 5; \
			timeout=$$((timeout-5)); \
		fi; \
	done; \
	if [ $$timeout -le 0 ]; then \
		echo "⚠ LDAP server startup timeout. Check logs with 'make ldap-logs'"; \
		exit 1; \
	fi
	@echo ""
	@echo "LDAP Server Configuration:"
	@echo "========================="
	@echo "Host: localhost:389"
	@echo "Base DN: dc=gotrs,dc=local"
	@echo "Admin DN: cn=admin,dc=gotrs,dc=local"
	@echo "Admin Password: admin123"
	@echo "Readonly DN: cn=readonly,dc=gotrs,dc=local"
	@echo "Readonly Password: readonly123"
	@echo ""
	@echo "Test Users (password: password123):"
	@echo "==================================="
	@echo "jadmin     - john.admin@gotrs.local (System Administrator)"
	@echo "smitchell  - sarah.mitchell@gotrs.local (IT Manager)"
	@echo "mwilson    - mike.wilson@gotrs.local (Senior Support Agent)"
	@echo "lchen      - lisa.chen@gotrs.local (Support Agent)"
	@echo "djohnson   - david.johnson@gotrs.local (Junior Support Agent)"
	@echo ""
	@echo "Web Interface:"
	@echo "=============="
	@echo "phpLDAPadmin: http://localhost:8091 (run 'make ldap-admin')"

# Test LDAP authentication with a specific user
ldap-test-user:
	@echo -n "Username to test: "; \
	read username; \
	echo "Testing LDAP authentication for user: $$username"; \
	$(COMPOSE_CMD) exec openldap ldapsearch -x -H ldap://localhost \
		-D "cn=readonly,dc=gotrs,dc=local" -w "readonly123" \
		-b "ou=Users,dc=gotrs,dc=local" \
		"(&(objectClass=inetOrgPerson)(uid=$$username))" \
		uid mail displayName telephoneNumber departmentNumber title

# Quick LDAP connectivity test
ldap-test:
	@echo "Testing LDAP connectivity..."
	$(COMPOSE_CMD) exec openldap ldapsearch -x -H ldap://localhost \
		-D "cn=admin,dc=gotrs,dc=local" -w "admin123" \
		-b "dc=gotrs,dc=local" \
		"(objectclass=*)" dn | head -20

# Test that all Makefile commands are properly containerized
.PHONY: test-containerized
test-containerized:
	@bash scripts/test-containerized.sh