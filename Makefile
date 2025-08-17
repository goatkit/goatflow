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

.PHONY: help up down logs restart clean setup test build debug-env

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
	@echo "  make setup    - Initial project setup"
	@echo "  make build    - Build production images"
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

# Initial setup
setup:
	@cp -n .env.example .env || true
	@cp -n docker-compose.override.yml.example docker-compose.override.yml || true
	@echo "Setup complete. Edit .env if needed."
	@echo "Run 'make up' to start development environment."

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

# Load environment variables from .env file
include .env
export

# Database operations
db-shell:
	$(COMPOSE_CMD) exec postgres psql -U $(DB_USER) -d $(DB_NAME)

db-migrate:
	@echo "Running database migrations..."
	$(COMPOSE_CMD) exec -e PGPASSWORD=$(DB_PASSWORD) backend psql -h postgres -U $(DB_USER) -d $(DB_NAME) -f /app/migrations/000001_initial_schema.up.sql
	$(COMPOSE_CMD) exec -e PGPASSWORD=$(DB_PASSWORD) backend psql -h postgres -U $(DB_USER) -d $(DB_NAME) -f /app/migrations/000002_initial_data.up.sql
	@echo "Migrations completed successfully!"

db-migrate-schema-only:
	@echo "Running schema migration only..."
	$(COMPOSE_CMD) exec -e PGPASSWORD=$(DB_PASSWORD) backend psql -h postgres -U $(DB_USER) -d $(DB_NAME) -f /app/migrations/000001_initial_schema.up.sql

db-rollback:
	$(COMPOSE_CMD) exec backend migrate -path /app/migrations -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" down 1

db-reset:
	$(COMPOSE_CMD) exec backend migrate -path /app/migrations -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" down -all
	$(COMPOSE_CMD) exec backend migrate -path /app/migrations -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" up

db-status:
	$(COMPOSE_CMD) exec backend migrate -path /app/migrations -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" version

db-force:
	@read -p "Force migration to version: " version; \
	$(COMPOSE_CMD) exec backend migrate -path /app/migrations -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" force $$version

# Valkey CLI
valkey-cli:
	$(COMPOSE_CMD) exec valkey valkey-cli

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

# Run tests with enhanced coverage reporting
test-report:
	@./run_tests.sh

# Generate HTML coverage report
test-html:
	@./run_tests.sh --html

# Run tests with comprehensive safety checks
test-safe:
	@./scripts/test-db-safe.sh test

# Clean test database (with safety checks)
test-clean:
	@./scripts/test-db-safe.sh clean

# Check test environment safety
test-check:
	@./scripts/test-db-safe.sh check

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

test-all: test test-frontend test-contracts
	@echo "All tests completed!"

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

# Check service health
health:
	@echo "Checking service health..."
	@curl -f http://localhost/health || echo "Backend not healthy"
	@curl -f http://localhost/ || echo "Frontend not healthy"

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
	podman generate systemd --new --files --name gotrs-postgres
	podman generate systemd --new --files --name gotrs-valkey
	podman generate systemd --new --files --name gotrs-backend
	podman generate systemd --new --files --name gotrs-frontend
	@echo "Systemd unit files generated. Move to ~/.config/systemd/user/"

# Generate migration file pair
gen-migration:
	@read -p "Migration name: " name; \
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
	@read -p "Username to test: " username; \
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