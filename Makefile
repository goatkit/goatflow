# Explicit default goal
.DEFAULT_GOAL := help

# Load environment from .env (includes GO_IMAGE - single source of truth)
ifneq (,$(wildcard .env))
include .env
export $(shell sed -n 's/^\([A-Za-z_][A-Za-z0-9_]*\)=.*/\1/p' .env)
endif

# Fallback if .env doesn't exist or doesn't define GO_IMAGE
GO_IMAGE ?= golang:1.24.11-alpine
export GO_IMAGE

# Version information from git (used in docker build)
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
GIT_TAG := $(shell git describe --tags --exact-match 2>/dev/null || echo "")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
# Use tag if available, otherwise use branch name
VERSION := $(if $(GIT_TAG),$(GIT_TAG),$(GIT_BRANCH))
# Build args for version injection
VERSION_BUILD_ARGS := --build-arg VERSION=$(VERSION) --build-arg GIT_COMMIT=$(GIT_COMMIT) --build-arg GIT_BRANCH=$(GIT_BRANCH) --build-arg BUILD_DATE=$(BUILD_DATE)

# Route manifest governance
.PHONY: routes-verify routes-baseline-update routes-generate
routes-generate:
	@echo "Generating routes manifest..."
	@mkdir -p generated && chmod 777 generated 2>/dev/null || true
	@chmod +x scripts/generate_routes_manifest.sh
	@$(MAKE) toolbox-exec ARGS='bash scripts/generate_routes_manifest.sh'
	@[ -f generated/routes-manifest.json ] && echo "routes-manifest.json generated." || (echo "Failed to generate routes manifest" && exit 1)

routes-verify:
	@if [ ! -f generated/routes-manifest.json ]; then \
		$(MAKE) routes-generate; \
	fi
	@sh ./scripts/check_routes_manifest.sh

routes-baseline-update:
	@[ -f generated/routes-manifest.json ] || (echo "manifest missing; run server/tests first" && exit 1)
	cp generated/routes-manifest.json generated/routes-manifest.baseline.json
	@echo "Updated route manifest baseline."
# GOTRS Makefile - Docker/Podman compatible development

# Detect container runtime and compose command (single source of truth)
# Allow override via environment (e.g., CI sets CONTAINER_CMD=docker)
# First check for podman, then docker
ifndef CONTAINER_CMD
CONTAINER_CMD := $(shell command -v podman 2> /dev/null || command -v docker 2> /dev/null || echo docker)
endif

# Detect compose command - try multiple variants in order of preference
# Priority based on detected container runtime
ifeq ($(findstring podman,$(CONTAINER_CMD)),podman)
COMPOSE_CMD := $(shell \
	if command -v podman-compose > /dev/null 2>&1; then \
		echo "podman-compose"; \
	else \
		echo "MISSING: podman-compose not found - install with: sudo apt install podman-compose"; \
	fi)
else
COMPOSE_CMD := $(shell \
	if command -v docker > /dev/null 2>&1 && docker compose version > /dev/null 2>&1; then \
		echo "docker compose"; \
	elif command -v docker-compose > /dev/null 2>&1; then \
		echo "docker-compose"; \
	else \
		echo "MISSING: no docker compose found - install docker-compose-plugin"; \
	fi)
endif

# Image naming abstraction
IMAGE_PREFIX := $(if $(findstring podman,$(CONTAINER_CMD)),localhost/,docker.io/)

# Runtime-specific flags
COMPOSE_UP_FLAGS := $(if $(findstring podman-compose,$(COMPOSE_CMD)),--remove-orphans,--remove-orphans)
# Compose build flags (allow opt-in no cache rebuild for toolbox)
TOOLBOX_NO_CACHE ?= 0
COMPOSE_BUILD_FLAGS :=
ifeq ($(TOOLBOX_NO_CACHE),1)
COMPOSE_BUILD_FLAGS += --no-cache
endif
CONTAINER_EXEC_FLAGS := $(if $(findstring podman-compose,$(COMPOSE_CMD)),,--user 1000)

# Validate compose command is available
define check_compose
@if echo "$(COMPOSE_CMD)" | grep -q "^MISSING:"; then \
	echo "ERROR: $(COMPOSE_CMD)"; \
	echo "Please install the required compose tool and try again."; \
	exit 1; \
fi
endef

# Auto-select backend host port if 8080 is busy (only when not explicitly set)
ifeq ($(origin BACKEND_PORT), undefined)
BACKEND_PORT := $(shell if ss -ltn 2>/dev/null | awk '{print $$4}' | grep -qE ':(8080)$$'; then echo 18080; else echo 8080; fi)
endif

# Volume mount SELinux label for Podman
ifeq ($(findstring podman,$(CONTAINER_CMD)),podman)
VZ := :Z
else
VZ :=
endif

# Export compose/container command so all targets and scripts use the same detection
export CONTAINER_CMD
export COMPOSE_CMD

# Convenience macro to route Go commands through toolbox container
TOOLBOX_GO := $(MAKE) toolbox-exec ARGS=

# Cache mount: always use named Docker volume at /cache (not overlaid on workspace)
MOD_CACHE_MOUNT := -v gotrs_cache:/cache

# Helper targets for cache volumes
.PHONY: cache-prune
cache-prune:
	@echo "Pruning shared cache volume (gotrs_cache)..."
	@$(CONTAINER_CMD) volume rm -f gotrs_cache >/dev/null 2>&1 || true
	@$(CONTAINER_CMD) volume rm -f gotrs_go_build_cache gotrs_go_mod_cache gotrs_golangci_cache >/dev/null 2>&1 || true
	@echo "Done."

# Common run flags
VOLUME_PWD := -v "$$(pwd):/workspace"
WORKDIR_FLAGS := -w /workspace
USER_FLAGS := -u "$$(id -u):$$(id -g)"

# Credential validation - NO DEFAULT PASSWORDS
# All credentials MUST be set in .env file
define check_required_env
	@if [ -z "$${$(1)}" ]; then \
		echo "❌ ERROR: $(1) is not set. Add it to your .env file."; \
		exit 1; \
	fi
endef
# All variables below MUST be set in .env - no defaults
# DB_HOST, DB_DRIVER, DB_PORT, DB_NAME, DB_USER, DB_SCOPE
# VALKEY_HOST, VALKEY_PORT  
# TEST_DB_* variables

# Derived test DB variables based on driver
ifeq ($(TEST_DB_DRIVER),postgres)
TEST_DB_HOST := $(TEST_DB_POSTGRES_HOST)
TEST_DB_PORT := $(TEST_DB_POSTGRES_PORT)
TEST_DB_NAME := $(TEST_DB_POSTGRES_NAME)
TEST_DB_USER := $(TEST_DB_POSTGRES_USER)
TEST_DB_PASSWORD := $(TEST_DB_POSTGRES_PASSWORD)
else
TEST_DB_HOST := $(TEST_DB_MYSQL_HOST)
TEST_DB_PORT := $(TEST_DB_MYSQL_PORT)
TEST_DB_NAME := $(TEST_DB_MYSQL_NAME)
TEST_DB_USER := $(TEST_DB_MYSQL_USER)
TEST_DB_PASSWORD := $(TEST_DB_MYSQL_PASSWORD)
endif

# Toolbox uses host network, so point to localhost with mapped port
TOOLBOX_TEST_DB_HOST := 127.0.0.1
TOOLBOX_TEST_DB_PORT := $(TEST_DB_PORT)

TEST_BACKEND_HOST := $(TEST_BACKEND_SERVICE_HOST)
ifndef TEST_BACKEND_PORT
TEST_BACKEND_PORT := 18081
endif
ifeq ($(strip $(TEST_BACKEND_PORT)),)
override TEST_BACKEND_PORT := 18081
endif
# Default TEST_BACKEND_BASE_URL for health checks
ifndef TEST_BACKEND_BASE_URL
TEST_BACKEND_BASE_URL := http://localhost:$(TEST_BACKEND_PORT)
endif
TEST_COMPOSE_FILE := $(CURDIR)/docker-compose.yml:$(CURDIR)/docker-compose.testdb.yml:$(CURDIR)/docker-compose.test.yaml

help:
	@python3 scripts/tools/make_help.py

#########################################
# TEST COMMANDS
#########################################

# Legacy test target - use test-comprehensive via 'make test' instead
# Keeping as test-legacy for backwards compatibility

.PHONY: test-legacy
test-legacy: toolbox-build
	@printf "\n🧪 Running curated Go test suite (make test-legacy) ...\n"
	@$(MAKE) test-stack-up >/dev/null 2>&1 || true
	$(CONTAINER_CMD) run --rm \
		--security-opt label=disable \
		-v "$$PWD:/workspace" \
		--network host \
		$(MOD_CACHE_MOUNT) \
		-w /workspace \
		-e GOCACHE=/cache/go-build \
		-e GOMODCACHE=/cache/go-mod \
		-e APP_ENV=test \
		-e ENABLE_TEST_ADMIN_ROUTES=1 \
		-e STORAGE_PATH=/tmp \
		-e TEMPLATES_DIR=/workspace/templates \
		-e DB_DRIVER=$(TEST_DB_DRIVER) \
		-e DB_HOST=$(TOOLBOX_TEST_DB_HOST) \
		-e DB_PORT=$(TOOLBOX_TEST_DB_PORT) \
		-e DB_NAME=$(TEST_DB_NAME) \
		-e DB_USER=$(TEST_DB_USER) \
		-e DB_PASSWORD=$(TEST_DB_PASSWORD) \
		-e TEST_DB_DRIVER=$(TEST_DB_DRIVER) \
		-e TEST_DB_HOST=$(TOOLBOX_TEST_DB_HOST) \
		-e TEST_DB_PORT=$(TOOLBOX_TEST_DB_PORT) \
		-e TEST_DB_NAME=$(TEST_DB_NAME) \
		-e TEST_DB_USER=$(TEST_DB_USER) \
		-e TEST_DB_PASSWORD=$(TEST_DB_PASSWORD) \
		-e GOTRS_TEST_DB_READY=$(GOTRS_TEST_DB_READY) \
		-e VALKEY_HOST=$(VALKEY_HOST) -e VALKEY_PORT=$(VALKEY_PORT) \
		-e BASE_URL=http://localhost:$(BACKEND_PORT) \
		-e TEST_BACKEND_BASE_URL=http://localhost:$(TEST_BACKEND_PORT) \
		-e TEST_BACKEND_HOST=localhost \
		-e TEST_BACKEND_SERVICE_HOST=localhost \
		-e TEST_BACKEND_PORT=$(TEST_BACKEND_PORT) \
		-e TEST_BACKEND_CONTAINER_PORT=$(TEST_BACKEND_PORT) \
		gotrs-toolbox:latest \
		bash -lc 'export PATH=/usr/local/go/bin:$$PATH; set -e; \
		echo "Running template tests (fail-fast)"; go test -count=1 -timeout=1m -buildvcs=false ./internal/template/...; \
		CORE_PKGS=$$(go list ./... | rg -v "tests/e2e|tests/integration|internal/email/integration|internal/template"); \
		echo "Running core packages"; go test -count=1 -timeout=15m -buildvcs=false $$CORE_PKGS; \
		echo "Running integration packages"; go test -tags=integration -count=1 -timeout=20m -buildvcs=false ./tests/integration ./internal/email/integration'
	@$(MAKE) test-e2e-playwright-go

# Run template tests only (fast fail-fast validation of HTMX attributes and paths)
test-templates:
	@printf "🎨 Running template tests (fail-fast)...\n"
	@$(MAKE) toolbox-exec ARGS="go test -v -count=1 ./internal/template/..."

# Debug environment detection
debug-env:
	@printf "Container Environment Detection:\n"
	@printf "================================\n"
	@printf "Container runtime: $(CONTAINER_CMD)\n"
	@printf "Compose command: $(COMPOSE_CMD)\n"
	@printf "\n"
	@printf "Checking available commands:\n"
	@printf "----------------------------\n"
	@command -v docker > /dev/null 2>&1 && echo "✓ docker found: $$(which docker)" || echo "✗ docker not found"
	@command -v docker-compose > /dev/null 2>&1 && echo "✓ docker-compose found: $$(which docker-compose)" || echo "✗ docker-compose not found"
	@command -v docker > /dev/null 2>&1 && docker compose version > /dev/null 2>&1 && echo "✓ docker compose plugin found" || echo "✗ docker compose plugin not found"
	@command -v podman > /dev/null 2>&1 && echo "✓ podman found: $$(which podman)" || echo "✗ podman not found"
	@command -v podman-compose > /dev/null 2>&1 && echo "✓ podman-compose found: $$(which podman-compose)" || echo "✗ podman-compose not found"
	@command -v podman > /dev/null 2>&1 && podman compose version > /dev/null 2>&1 && echo "✓ podman compose plugin found" || echo "✗ podman compose plugin not found"
	@printf "\n"
	@printf "Selected commands will be used for all make targets.\n"

# Initial setup with secure secret generation
setup:
	@printf "🔬 Synthesizing secure configuration...\n"
	@if [ ! -f .env ]; then \
		$(MAKE) synthesize || echo "⚠️  Failed to synthesize. Using example file as fallback."; \
		if [ ! -f .env ]; then cp -n .env.example .env || true; fi; \
	else \
		echo "✅ .env already exists. Run 'make synthesize' to regenerate."; \
	fi
	@cp -n docker-compose.override.yml.example docker-compose.override.yml || true
	@printf "Setup complete. Run 'make up' to start development environment.\n"
# Generate secure credentials and output CSV to stdout
synthesize-credentials:
	@$(MAKE) toolbox-build >&2
	@$(CONTAINER_CMD) run --rm \
		-v "$$(pwd):/workspace" \
		-w /workspace \
		-u "$$(id -u):$$(id -g)" \
		gotrs-toolbox:latest \
		gotrs synthesize --test-data-only

# Show development credentials from generated SQL file
show-dev-creds:
	@grep "^-- ||" migrations/postgres/000004_generated_test_data.up.sql 2>/dev/null | sed 's/^-- || //' | column -t || echo "No credentials found. Run 'make synthesize' first."

# Apply generated test data to database
db-apply-test-data:
	@printf "📝 Applying generated test data...\n"
	@if echo "$(COMPOSE_CMD)" | grep -q '^MISSING:'; then \
		echo "ERROR: $(COMPOSE_CMD)"; \
		echo "Please install the required compose tool and try again."; \
		exit 1; \
	fi
	@if [ "$(DB_DRIVER)" = "postgres" ]; then \
		$(COMPOSE_CMD) exec -T postgres psql -U $(DB_USER) -d $(DB_NAME) -f - < migrations/postgres/000004_generated_test_data.up.sql; \
		printf "✅ Test data applied. Run 'make show-dev-creds' to see credentials.\n"; \
	else \
		printf "📡 Starting dependencies (mariadb)...\n"; \
		$(COMPOSE_CMD) up -d mariadb >/dev/null 2>&1 || true; \
		if [ -n "$(ADMIN_PASSWORD)" ]; then \
			printf "🔐 Applying admin password from environment (MariaDB)...\n"; \
			$(CONTAINER_CMD) run --rm \
				--network gotrs-ce_gotrs-network \
				-e DB_DRIVER=$(DB_DRIVER) -e DB_HOST=$(DB_HOST) -e DB_PORT=$(DB_PORT) \
				-e DB_NAME=$(DB_NAME) -e DB_USER=$(DB_USER) -e DB_PASSWORD=$(DB_PASSWORD) \
				-e ADMIN_PASSWORD=$(ADMIN_PASSWORD) -e ADMIN_USER=$(ADMIN_USER) \
				gotrs-toolbox:latest \
				sh -c 'gotrs reset-user --username="$${ADMIN_USER:-root@localhost}" --password="$$ADMIN_PASSWORD" --enable'; \
			printf "✅ Root user enabled with configured credentials.\n"; \
		else \
			printf "⚠️  root@localhost remains disabled. Run 'make reset-password' after choosing a password.\n"; \
		fi; \
	fi

# Clean up storage directory (orphaned files after DB reset)
clean-storage:
	@printf "🧹 Cleaning orphaned storage files...\n"
	@rm -rf internal/api/storage/* 2>/dev/null || true
	@rm -rf storage/* 2>/dev/null || true
	@printf "✅ Storage directories cleaned\n"
# Generate secure .env file with random secrets (runs in container)
synthesize:
	@$(MAKE) toolbox-build
	@printf "🔬 Synthesizing secure configuration and test data..." >&2
	@$(CONTAINER_CMD) run --rm \
		-v "$$(pwd):/workspace" \
		-w /workspace \
		-u "$$(id -u):$$(id -g)" \
		gotrs-toolbox:latest \
		gotrs synthesize $(SYNTH_ARGS)
	@if [ -z "$(SYNTH_ARGS)" ]; then \
		echo "📝 Test credentials saved to test_credentials.csv" >&2; \
	fi
	@printf "🔐 Generating Kubernetes secrets from template...\n"
	@./scripts/generate-k8s-secrets.sh
	@if [ -d .git ]; then \
		echo ""; \
		echo "💡 To enable secret scanning in git commits, run:"; \
		echo "   make scan-secrets-precommit"; \
	fi

# Rotate secrets in existing .env file (runs in container)
rotate-secrets:
	@$(MAKE) toolbox-build
	@printf "🔄 Rotating secrets...\n"
	@$(CONTAINER_CMD) run --rm \
		-v "$$(pwd):/workspace" \
		-w /workspace \
		-u "$$(id -u):$$(id -g)" \
		gotrs-toolbox:latest \
		gotrs synthesize --rotate-secrets

# Force regenerate .env file (runs in container)
synthesize-force:
	@$(MAKE) toolbox-build
	@printf "⚠️  Force regenerating .env file...\n"
	@$(CONTAINER_CMD) run --rm \
		-v "$$(pwd):/workspace" \
		-w /workspace \
		-u "$$(id -u):$$(id -g)" \
		gotrs-toolbox:latest \
		gotrs synthesize --force

# Generate only test data (SQL and CSV)
gen-test-data:
	@$(MAKE) toolbox-build
	@printf "🔄 Regenerating test data only...\n"
	@$(CONTAINER_CMD) run --rm \
		-v "$$(pwd):/workspace" \
		-w /workspace \
		-u "$$(id -u):$$(id -g)" \
		gotrs-toolbox:latest \
		gotrs synthesize --test-data-only

# Generate Kubernetes secrets from template with secure random values
k8s-secrets:
	@printf "🔐 Generating Kubernetes secrets from template...\n"
	@./scripts/generate-k8s-secrets.sh

# =============================================================================
# Helm Chart Targets
# =============================================================================
# Uses the gotrs_cache Docker volume mounted at /cache
# XDG_CACHE_HOME is already set to /cache/xdg in docker-compose.yml

# Fix cache permissions if any dir is root-owned (runs as root in container to fix ownership)
# This is a recovery target for volumes that have incorrect permissions from previous runs
.PHONY: cache-fix
cache-fix:
	@printf "\n🔧 Fixing cache permissions in Docker volume...\n"
	@$(CONTAINER_CMD) run --rm -v gotrs-ce_gotrs_cache:/cache alpine:3.19 \
		sh -c 'chown -R 1000:1000 /cache && mkdir -p /cache/go-build /cache/go-mod /cache/xdg/helm/repository /cache/xdg/helm/cache /cache/xdg/helm/config /cache/bun /cache/golangci-lint && chown -R 1000:1000 /cache'
	@printf "✅ Cache permissions fixed\n"

# Setup helm repos (run once, idempotent)
.PHONY: helm-setup
helm-setup: toolbox-build
	@printf "\n⚙️  Setting up Helm repositories...\n"
	@$(MAKE) toolbox-exec ARGS="helm repo add valkey https://valkey.io/valkey-helm/ 2>/dev/null || true && helm repo update"
	@printf "✅ Helm repos configured\n"

# Run helm command in toolbox (usage: make helm ARGS="lint charts/gotrs")
.PHONY: helm
helm: toolbox-build
	@[ -n "$(ARGS)" ] || (echo "Usage: make helm ARGS=\"<helm command>\"" && echo "Examples:" && echo "  make helm ARGS=\"lint charts/gotrs\"" && echo "  make helm ARGS=\"template gotrs charts/gotrs\"" && echo "  make helm ARGS=\"dependency update charts/gotrs\"" && exit 2)
	@$(MAKE) toolbox-exec ARGS="helm $(ARGS)"

# Lint the GOTRS Helm chart
.PHONY: helm-lint
helm-lint: helm-setup
	@printf "\n🔍 Linting Helm chart...\n"
	@$(MAKE) toolbox-exec ARGS="helm lint charts/gotrs"

# Render Helm chart templates (dry-run)
.PHONY: helm-template
helm-template: helm-deps
	@printf "\n📄 Rendering Helm chart templates...\n"
	@$(MAKE) toolbox-exec ARGS="helm template gotrs charts/gotrs"

# Render Helm chart with PostgreSQL values
.PHONY: helm-template-pg
helm-template-pg: helm-deps
	@printf "\n📄 Rendering Helm chart with PostgreSQL...\n"
	@$(MAKE) toolbox-exec ARGS="helm template gotrs charts/gotrs -f charts/gotrs/values-postgresql.yaml"

# Update Helm chart dependencies (valkey subchart)
.PHONY: helm-deps
helm-deps: helm-setup
	@printf "\n📦 Updating Helm chart dependencies...\n"
	@$(MAKE) toolbox-exec ARGS="helm dependency update charts/gotrs"

# Package the Helm chart for distribution
.PHONY: helm-package
helm-package: helm-deps
	@printf "\n📦 Packaging Helm chart...\n"
	@$(MAKE) toolbox-exec ARGS="helm package charts/gotrs -d dist"

# =============================================================================

# Build toolbox image
toolbox-build: build-artifacts
	@printf "\n🔧 Building GOTRS toolbox container...\n"
	@if echo "$(COMPOSE_CMD)" | grep -q '^MISSING:'; then \
		echo "⚠️  compose not available; falling back to direct docker build"; \
		command -v docker >/dev/null 2>&1 || (echo "docker not installed" && exit 1); \
		docker build --build-arg GO_IMAGE=$(GO_IMAGE) -f Dockerfile.toolbox -t gotrs-toolbox:latest .; \
	else \
		if echo "$(COMPOSE_CMD)" | grep -q "podman-compose"; then \
			COMPOSE_PROFILES=toolbox $(COMPOSE_CMD) build $(COMPOSE_BUILD_FLAGS) toolbox; \
		else \
			$(COMPOSE_CMD) --profile toolbox build $(COMPOSE_BUILD_FLAGS) toolbox; \
		fi; \
	fi
	@printf "✅ Toolbox container ready\n"

# Interactive toolbox shell (non-root, with SELinux-friendly mounts)
toolbox-run:
	@printf "\n🔧 Starting toolbox shell...\n"
	@printf "💡 Type 'exit' or Ctrl+D to exit the shell\n"
	@$(TOOLBOX_GO)"golangci-lint run ./..."

	fi

# API testing with automatic authentication
api-call:
	@if [ -z "$(ENDPOINT)" ]; then echo "❌ ENDPOINT required. Usage: make api-call [METHOD=GET] ENDPOINT=/api/v1/tickets [BODY='{}']"; exit 1; fi
	@if [ -z "$(METHOD)" ]; then METHOD=GET; fi;
	@printf "\n🔧 Making API call: $$METHOD $(ENDPOINT)\n";
	@if echo "$(COMPOSE_CMD)" | grep -q '^MISSING:'; then \
		echo "ERROR: $(COMPOSE_CMD)"; \
		echo "Please install the required compose tool and try again."; \
		exit 1; \
	fi
	@if echo "$(COMPOSE_CMD)" | grep -q "podman-compose"; then \
		COMPOSE_PROFILES=toolbox $(COMPOSE_CMD) run --rm toolbox bash scripts/api-test.sh "$$METHOD" "$(ENDPOINT)" "$(BODY)"; \
	else \
		$(COMPOSE_CMD) --profile toolbox run --rm toolbox bash scripts/api-test.sh "$$METHOD" "$(ENDPOINT)" "$(BODY)"; \
	fi

# API testing for form-urlencoded bodies with automatic authentication
.PHONY: api-call-form
api-call-form:
	@if [ -z "$(ENDPOINT)" ]; then echo "❌ ENDPOINT required. Usage: make api-call-form [METHOD=PUT] ENDPOINT=/admin/users/1 [DATA='a=b&c=d']"; exit 1; fi
	@if [ -z "$(METHOD)" ]; then METHOD=PUT; fi;
	@printf "\n🔧 Making form API call: $$METHOD $(ENDPOINT)\n";
	@if echo "$(COMPOSE_CMD)" | grep -q '^MISSING:'; then \
		echo "ERROR: $(COMPOSE_CMD)"; \
		echo "Please install the required compose tool and try again."; \
		exit 1; \
	fi
	@if echo "$(COMPOSE_CMD)" | grep -q "podman-compose"; then \
		COMPOSE_PROFILES=toolbox $(COMPOSE_CMD) run --rm toolbox bash scripts/api-form.sh "$$METHOD" "$(ENDPOINT)" "$(DATA)"; \
	else \
		$(COMPOSE_CMD) --profile toolbox run --rm toolbox bash scripts/api-form.sh "$$METHOD" "$(ENDPOINT)" "$(DATA)"; \
	fi

# Authenticated HTTP call (logs in with ADMIN_USER/ADMIN_PASSWORD, or use AUTH_TOKEN)
.PHONY: http-call
http-call:
	@if [ -z "$(ENDPOINT)" ]; then echo "❌ ENDPOINT required. Usage: make http-call [METHOD=GET] ENDPOINT=/ [BODY='...'] [CONTENT_TYPE='text/html']"; exit 1; fi
	@printf "\n🔧 Making authenticated HTTP call: $(or $(METHOD),GET) $(ENDPOINT)\n";
	@$(CONTAINER_CMD) run --rm \
		--security-opt label=disable \
		-v "$$PWD:/workspace" \
		-w /workspace \
		-u "$$UID:$$GID" \
		--network gotrs-ce_gotrs-network \
		-e METHOD="$(METHOD)" \
		-e ENDPOINT="$(ENDPOINT)" \
		-e BODY="$(BODY)" \
		-e CONTENT_TYPE="$(CONTENT_TYPE)" \
		-e BACKEND_URL="$(BACKEND_URL)" \
		-e AUTH_TOKEN="$(AUTH_TOKEN)" \
		-e LOGIN="$(or $(LOGIN),$(ADMIN_USER))" \
		-e PASSWORD="$(or $(PASSWORD),$(ADMIN_PASSWORD))" \
		gotrs-toolbox:latest \
		bash -lc 'chmod +x scripts/http-call.sh 2>/dev/null || true; scripts/http-call.sh'

# File upload with JWT auth
.PHONY: api-upload
api-upload:
	@if [ -z "$(ENDPOINT)" ]; then echo "❌ ENDPOINT required. Usage: make api-upload ENDPOINT=/api/tickets/<tn>/attachments FILE=/path/to/file"; exit 1; fi
	@if [ -z "$(FILE)" ]; then echo "❌ FILE required. Usage: make api-upload ENDPOINT=/api/tickets/<tn>/attachments FILE=/path/to/file"; exit 1; fi
	@if echo "$(COMPOSE_CMD)" | grep -q "podman-compose"; then \
		COMPOSE_PROFILES=toolbox $(COMPOSE_CMD) run --rm toolbox bash -lc 'chmod +x scripts/api-upload.sh; BACKEND_URL=$${BACKEND_URL:-http://backend:8080} scripts/api-upload.sh '"$(ENDPOINT)"' '"$(FILE)"''; \
	else \
		$(CONTAINER_CMD) run --rm \
		--security-opt label=disable \
		-v "$$PWD:/workspace" \
		-w /workspace \
		-u "$$UID:$$GID" \
		--network gotrs-ce_gotrs-network \
		gotrs-toolbox:latest \
		bash -lc 'chmod +x scripts/api-upload.sh; BACKEND_URL=$${BACKEND_URL:-http://backend:8080} scripts/api-upload.sh '"$(ENDPOINT)"' '"$(FILE)"''; \
	fi



# Compile everything (bind mounts + caches)
toolbox-compile:
	@$(MAKE) toolbox-build
	@printf "\n🔨 Checking compilation...\n"
	$(CONTAINER_CMD) run --rm \
        --security-opt label=disable \
        -v "$$PWD:/workspace" \
		-w /workspace \
		-u "$$UID:$$GID" \
		-e GOCACHE=/cache/go-build \
		-e GOMODCACHE=/cache/go-mod \
		gotrs-toolbox:latest \
		bash -lc 'export PATH=/usr/local/go/bin:$$PATH && go version && go build -buildvcs=false ./...'

# Compile only API and goats (faster)
toolbox-compile-api:
	@$(MAKE) toolbox-build
	@printf "\n🔨 Compiling API and goats packages only...\n"
	@$(CONTAINER_CMD) run --rm \
        --security-opt label=disable \
        -v "$$PWD:/workspace" \
		-w /workspace \
		-u "$$UID:$$GID" \
		-e GOCACHE=/cache/go-build \
		-e GOMODCACHE=/cache/go-mod \
		gotrs-toolbox:latest \
		bash -lc 'export PATH=/usr/local/go/bin:$$PATH && go version && go build -buildvcs=false ./internal/api ./cmd/goats'

# Compile the main goats binary in container
.PHONY: compile
compile: toolbox-build
	@printf "🔨 Compiling goats binary...\n"
	@mkdir -p bin
	@$(CONTAINER_CMD) run --rm \
		--security-opt label=disable \
        -v "$$(pwd):/workspace" \
		-v "$$(pwd)/bin:/workspace/bin$(VZ)" \
		-w /workspace \
		-u "$$(id -u):$$(id -g)" \
		gotrs-toolbox:latest \
		bash -lc 'export PATH=/usr/local/go/bin:$$PATH && mkdir -p bin && go build -buildvcs=false -ldflags="-w -s" -o bin/goats ./cmd/goats'
	@printf "✅ Binary compiled to bin/goats\n"
# Safe compile without bind-mounts (avoids SELinux relabel issues)
.PHONY: compile-safe
compile-safe: toolbox-build
	@printf "🔒 Compiling goats binary in isolated toolbox container...\n"
	-@$(CONTAINER_CMD) rm -f gotrs-compile >/dev/null 2>&1 || true
	@$(CONTAINER_CMD) create --name gotrs-compile gotrs-toolbox:latest sleep infinity >/dev/null
	@$(CONTAINER_CMD) cp . gotrs-compile:/workspace
	@$(CONTAINER_CMD) start gotrs-compile >/dev/null
	@$(CONTAINER_CMD) exec gotrs-compile bash -lc 'export PATH=/usr/local/go/bin:$$PATH && mkdir -p /workspace/bin && go build -buildvcs=false -ldflags="-w -s" -o /workspace/bin/goats ./cmd/goats'
	@mkdir -p bin
	@$(CONTAINER_CMD) cp gotrs-compile:/workspace/bin/goats ./bin/goats
	@$(CONTAINER_CMD) rm -f gotrs-compile >/dev/null
	@printf "✅ Binary compiled to bin/goats (compile-safe)\n"

# Run internal/api tests (bind mounts + caches; DB-less-safe)
toolbox-test-api: toolbox-build
	@printf "\n🧪 Running internal/api tests in toolbox...\n"
	@# Enforce static route policy during tests
	@$(MAKE) generate-route-map validate-routes
	@printf "📡 Starting dependencies (test database, valkey)...\n"
	@$(MAKE) test-db-up >/dev/null 2>&1 || true
	@$(COMPOSE_CMD) up -d valkey >/dev/null 2>&1 || true
	@$(CONTAINER_CMD) run --rm \
        --security-opt label=disable \
        -v "$$PWD:/workspace" \
		--network host \
		$(MOD_CACHE_MOUNT) \
		-w /workspace \
		-e GOCACHE=/cache/go-build \
		-e GOMODCACHE=/cache/go-mod \
		-e APP_ENV=test \
		-e ENABLE_TEST_ADMIN_ROUTES=1 \
		-e STORAGE_PATH=/tmp \
		-e TEMPLATES_DIR=/workspace/templates \
		-e DB_DRIVER=$(TEST_DB_DRIVER) \
		-e DB_HOST=$(TOOLBOX_TEST_DB_HOST) \
		-e DB_PORT=$(TOOLBOX_TEST_DB_PORT) \
		-e DB_NAME=$(TEST_DB_NAME) \
		-e DB_USER=$(TEST_DB_USER) \
		-e DB_PASSWORD=$(TEST_DB_PASSWORD) \
		-e TEST_DB_DRIVER=$(TEST_DB_DRIVER) \
		-e TEST_DB_HOST=$(TOOLBOX_TEST_DB_HOST) \
		-e TEST_DB_PORT=$(TOOLBOX_TEST_DB_PORT) \
		-e TEST_DB_NAME=$(TEST_DB_NAME) \
		-e TEST_DB_USER=$(TEST_DB_USER) \
		-e TEST_DB_PASSWORD=$(TEST_DB_PASSWORD) \
		-e GOTRS_TEST_DB_READY=$(GOTRS_TEST_DB_READY) \
		gotrs-toolbox:latest \
		bash -lc 'export PATH=/usr/local/go/bin:$$PATH; go test -buildvcs=false -v ./internal/api -run ^Test\(BuildRoutesManifest\|Queue\|Article\|Search\|Priority\|User\)'

# Run internal/api tests binding to host-published test DB
toolbox-test-api-host: toolbox-build
	@printf "\n🧪 Running internal/api tests (host-network DB) in toolbox...\n"
	@$(MAKE) generate-route-map validate-routes
	@printf "📡 Starting dependencies (test database, valkey)...\n"
	@$(MAKE) test-db-up >/dev/null 2>&1 || true
	@$(COMPOSE_CMD) up -d valkey >/dev/null 2>&1 || true
	@$(CONTAINER_CMD) run --rm \
        --security-opt label=disable \
        -v "$$PWD:/workspace" \
		--network host \
		$(MOD_CACHE_MOUNT) \
		-w /workspace \
		-e GOCACHE=/cache/go-build \
		-e GOMODCACHE=/cache/go-mod \
		-e APP_ENV=test \
		-e ENABLE_TEST_ADMIN_ROUTES=1 \
		-e STORAGE_PATH=/tmp \
		-e TEMPLATES_DIR=/workspace/templates \
		-e DB_DRIVER=$(TEST_DB_DRIVER) \
		-e DB_HOST=$(TOOLBOX_TEST_DB_HOST) \
		-e DB_PORT=$(TOOLBOX_TEST_DB_PORT) \
		-e DB_NAME=$(TEST_DB_NAME) \
		-e DB_USER=$(TEST_DB_USER) \
		-e DB_PASSWORD=$(TEST_DB_PASSWORD) \
		-e TEST_DB_DRIVER=$(TEST_DB_DRIVER) \
		-e TEST_DB_HOST=$(TOOLBOX_TEST_DB_HOST) \
		-e TEST_DB_PORT=$(TOOLBOX_TEST_DB_PORT) \
		-e TEST_DB_NAME=$(TEST_DB_NAME) \
		-e TEST_DB_USER=$(TEST_DB_USER) \
		-e TEST_DB_PASSWORD=$(TEST_DB_PASSWORD) \
		-e GOTRS_TEST_DB_READY=$(GOTRS_TEST_DB_READY) \
		gotrs-toolbox:latest \
		bash -lc 'export PATH=/usr/local/go/bin:$$PATH; go test -buildvcs=false -v ./internal/api -run ^Test\(BuildRoutesManifest\|Queue\|Article\|Search\|Priority\|User\|AdminCustomerCompan\)'

# Run core tests (cmd/goats + internal/api + internal/service)
toolbox-test:
	@$(MAKE) toolbox-build
	@printf "\n🧪 Running core test suite in toolbox...\n"
	@# Enforce static route policy during tests
	@$(MAKE) generate-route-map validate-routes
	@printf "📡 Starting dependencies (test database, valkey)...\n"
	@$(MAKE) test-db-up >/dev/null 2>&1 || true
	@$(COMPOSE_CMD) up -d valkey >/dev/null 2>&1 || true
	@$(CONTAINER_CMD) run --rm \
        --security-opt label=disable \
        -v "$$PWD:/workspace" \
		--network host \
		$(MOD_CACHE_MOUNT) \
		-w /workspace \
		-e GOCACHE=/cache/go-build \
		-e GOMODCACHE=/cache/go-mod \
		-e APP_ENV=test \
		-e ENABLE_TEST_ADMIN_ROUTES=1 \
		-e STORAGE_PATH=/tmp \
		-e TEMPLATES_DIR=/workspace/templates \
		-e DB_DRIVER=$(TEST_DB_DRIVER) \
		-e DB_HOST=$(TOOLBOX_TEST_DB_HOST) \
		-e DB_PORT=$(TOOLBOX_TEST_DB_PORT) \
		-e DB_NAME=$(TEST_DB_NAME) \
		-e DB_USER=$(TEST_DB_USER) \
		-e DB_PASSWORD=$(TEST_DB_PASSWORD) \
		-e TEST_DB_DRIVER=$(TEST_DB_DRIVER) \
		-e TEST_DB_HOST=$(TOOLBOX_TEST_DB_HOST) \
		-e TEST_DB_PORT=$(TOOLBOX_TEST_DB_PORT) \
		-e TEST_DB_NAME=$(TEST_DB_NAME) \
		-e TEST_DB_USER=$(TEST_DB_USER) \
		-e TEST_DB_PASSWORD=$(TEST_DB_PASSWORD) \
		-e GOTRS_TEST_DB_READY=$(GOTRS_TEST_DB_READY) \
		-e VALKEY_HOST=$(VALKEY_HOST) -e VALKEY_PORT=$(VALKEY_PORT) \
		gotrs-toolbox:latest \
		bash -lc 'export PATH=/usr/local/go/bin:$$PATH; set -e; \
		echo Running: ./cmd/goats; go test -buildvcs=false -v ./cmd/goats; \
		echo Running: ./internal/i18n; go test -buildvcs=false -v ./internal/i18n; \
		echo Running: ./internal/api focused; go test -buildvcs=false -v ./internal/api -run ^Test\(AdminType\|Queue\|Article\|Search\|Priority\|User\|TicketZoom\|AdminService\|AdminStates\|AdminGroupManagement\|HandleGetQueues\|HandleGetPriorities\|DatabaseIntegrity\); \
		echo Running: ./internal/service; go test -buildvcs=false -v ./internal/service'



.PHONY: openapi-lint
openapi-lint:
	@echo "📜 Linting OpenAPI spec with Bun (Redocly)..."
	@$(CONTAINER_CMD) run --rm \
		--security-opt label=disable \
		-v "$$PWD/api:/spec"$(VZ) \
		-v gotrs_cache:/cache \
		-e BUN_INSTALL_CACHE_DIR=/cache/bun \
		oven/bun:1.1-alpine \
		sh -lc 'bun add -g @redocly/cli >/dev/null 2>&1 && redocly lint /spec/openapi.yaml'

.PHONY: openapi-bundle
openapi-bundle:
	@echo "📦 Bundling OpenAPI spec with Redocly..."
	@$(CONTAINER_CMD) run --rm \
		--security-opt label=disable \
		-v "$$PWD/api:/spec"$(VZ) \
		-v gotrs_cache:/cache \
		-e BUN_INSTALL_CACHE_DIR=/cache/bun \
		oven/bun:1.1-alpine \
		sh -lc 'bun add -g @redocly/cli >/dev/null 2>&1 && redocly bundle /spec/openapi.yaml -o /spec/openapi.bundle.yaml'

# Run almost-all tests (excludes heavyweight e2e/integration and unstable lambda tests)
toolbox-test-all:
	@$(MAKE) toolbox-build
	@printf "\n🧪 Running broad test suite (excluding e2e/integration) in toolbox...\n"
	@printf "📡 Starting dependencies (mariadb, valkey)...\n"
	@$(COMPOSE_CMD) up -d mariadb valkey >/dev/null 2>&1 || true
	@$(CONTAINER_CMD) run --rm \
		--security-opt label=disable \
		-v "$$PWD:/workspace" \
		--network host \
		$(MOD_CACHE_MOUNT) \
		-w /workspace \
		-e GOCACHE=/cache/go-build \
		-e GOMODCACHE=/cache/go-mod \
		-e APP_ENV=test \
		-e STORAGE_PATH=/tmp \
		-e TEMPLATES_DIR=/workspace/templates \
		-e DB_HOST=$(DB_HOST) -e DB_PORT=$(DB_PORT) \
		-e DB_DRIVER=$(DB_DRIVER) \
		-e DB_NAME=$(DB_NAME) -e DB_USER=$(DB_USER) -e DB_PASSWORD=$(DB_PASSWORD) \
		-e GOTRS_TEST_DB_READY=$(GOTRS_TEST_DB_READY) \
		gotrs-toolbox:latest \
		bash -lc 'export PATH=/usr/local/go/bin:$$PATH; set -e; \
		echo Running curated set: cmd/goats internal/api internal/service; \
		$(TOOLBOX_GO)"go test -buildvcs=false -v ./cmd/goats"; \
		$(TOOLBOX_GO)"go test -buildvcs=false -v ./internal/api -run ^Test(AdminType|Queue|Article|Search|Priority|User|TicketZoom|AdminService|AdminStates|AdminGroupManagement|HandleGetQueues|HandleGetPriorities|DatabaseIntegrity)"; \
		$(TOOLBOX_GO)"go test -buildvcs=false -v ./internal/service"'

.PHONY: test-unit
test-unit:
	@echo "🧪 Running unit tests (with test database)..."
	@$(MAKE) toolbox-build
	@# Ensure test DB is running
	@$(MAKE) test-db-up >/dev/null 2>&1 || true
	@$(COMPOSE_CMD) -f docker-compose.yml -f docker-compose.testdb.yml --profile toolbox --profile testdb run --rm -T \
		-e GOCACHE=/cache/go-build \
		-e GOMODCACHE=/cache/go-mod \
		-e GOFLAGS=-buildvcs=false \
		toolbox \
		bash -lc 'export PATH=/usr/local/go/bin:$$PATH; go test -count=1 -buildvcs=false -v ./cmd/goats ./internal/... ./generated/... | tee generated/test-results/unit_stable.log'

.PHONY: test-e2e
test-e2e:
	@echo "🎯 Running targeted E2E tests (set TEST=pattern, e.g., TEST=Login|Groups)"
	@[ -n "$(TEST)" ] || (echo "Usage: make test-e2e TEST=Login|Groups|Queues" && exit 2)
	@$(MAKE) toolbox-build
	@HEADLESS=${HEADLESS:-true} \
	 BASE_URL=${BASE_URL:-http://localhost:$(BACKEND_PORT)} \
	 DEMO_ADMIN_EMAIL=${DEMO_ADMIN_EMAIL:-} \
	 DEMO_ADMIN_PASSWORD=${DEMO_ADMIN_PASSWORD:-} \
	 $(CONTAINER_CMD) run --rm \
		--security-opt label=disable \
		-v "$$PWD:/workspace" \
		-w /workspace \
		-u "$$(id -u):$$(id -g)" \
		-e GOCACHE=/cache/go-build \
		-e GOMODCACHE=/cache/go-mod \
		-e GOFLAGS=-buildvcs=false \
		-e HEADLESS \
		-e BASE_URL \
		-e DEMO_ADMIN_EMAIL \
		-e DEMO_ADMIN_PASSWORD \
		gotrs-toolbox:latest \
		bash -lc 'export PATH=/usr/local/go/bin:$$PATH; \
		go test -tags e2e -count=1 -buildvcs=false -v ./tests/e2e -run "$(TEST)" | tee generated/test-results/e2e_$(shell echo $(TEST) | tr ' ' '_').log'

# Run integration tests (requires running test DB stack)
toolbox-test-integration:
	@$(MAKE) toolbox-build
	@printf "\n🧪 Running integration tests (requires DB) in toolbox...\n"
	@printf "📡 Starting test stack (mariadb-test, valkey-test)...\n"
	@$(MAKE) test-stack-up >/dev/null 2>&1 || true
	$(CONTAINER_CMD) run --rm \
		--security-opt label=disable \
		-v "$$PWD:/workspace" \
		--network host \
		-w /workspace \
		-u "$$UID:$$GID" \
		-e GOCACHE=/cache/go-build \
		-e GOMODCACHE=/cache/go-mod \
		-e GOFLAGS=-buildvcs=false \
		-e APP_ENV=test \
		-e ENABLE_TEST_ADMIN_ROUTES=1 \
		-e STORAGE_PATH=/tmp \
		-e TEMPLATES_DIR=/workspace/templates \
		-e DB_DRIVER=$(TEST_DB_DRIVER) \
		-e DB_HOST=$(TOOLBOX_TEST_DB_HOST) \
		-e DB_PORT=$(TOOLBOX_TEST_DB_PORT) \
		-e DB_NAME=$(TEST_DB_NAME) \
		-e DB_USER=$(TEST_DB_USER) \
		-e DB_PASSWORD=$(TEST_DB_PASSWORD) \
		-e TEST_DB_DRIVER=$(TEST_DB_DRIVER) \
		-e TEST_DB_HOST=$(TOOLBOX_TEST_DB_HOST) \
		-e TEST_DB_PORT=$(TOOLBOX_TEST_DB_PORT) \
		-e TEST_DB_NAME=$(TEST_DB_NAME) \
		-e TEST_DB_USER=$(TEST_DB_USER) \
		-e TEST_DB_PASSWORD=$(TEST_DB_PASSWORD) \
		-e GOTRS_TEST_DB_READY=$(GOTRS_TEST_DB_READY) \
		-e VALKEY_HOST=$(VALKEY_HOST) -e VALKEY_PORT=$(VALKEY_PORT) \
		-e INT_PKGS \
		gotrs-toolbox:latest \
		bash -lc 'export PATH=/usr/local/go/bin:$$PATH; export GOFLAGS="-buildvcs=false"; set -e; \
		PKGS="$${INT_PKGS:-./internal/middleware}"; \
		echo "Running integration-tagged tests for packages: $$PKGS"; \
		go test -tags=integration -buildvcs=false -count=1 -v $$PKGS'

# Run smtp4dev + POP/DB email integrations end-to-end
toolbox-test-email-integration:
	@$(MAKE) toolbox-build
	@printf "\n📧 Running smtp4dev email integrations (requires DB + smtp4dev) in toolbox...\n"
	@printf "📡 Starting dependencies (postgres, valkey, smtp4dev)...\n"
	@$(COMPOSE_CMD) up -d postgres valkey smtp4dev >/dev/null 2>&1 || true
	$(CONTAINER_CMD) run --rm \
		--security-opt label=disable \
		-v "$$PWD:/workspace" \
		--network host \
		-w /workspace \
		-u "$$UID:$$GID" \
		-e GOCACHE=/cache/go-build \
		-e GOMODCACHE=/cache/go-mod \
		-e GOFLAGS=-buildvcs=false \
		-e APP_ENV=test \
		-e GOTRS_TEST_DB_READY=$(GOTRS_TEST_DB_READY) \
		-e SMTP4DEV_API_BASE \
		-e SMTP4DEV_SMTP_ADDR \
		-e SMTP4DEV_POP_HOST \
		-e SMTP4DEV_POP_PORT \
		-e SMTP4DEV_USER \
		-e SMTP4DEV_PASS \
		-e SMTP4DEV_FROM \
		-e SMTP4DEV_SYSTEM_ADDRESS \
		gotrs-toolbox:latest \
		bash -lc 'export PATH=/usr/local/go/bin:$$PATH; export GOFLAGS="-buildvcs=false"; set -e; \
		go test -tags=integration -buildvcs=false -count=1 ./internal/email/integration'

# Run a specific test pattern across all packages
toolbox-test-run:
	@$(MAKE) toolbox-build
	@printf "\n🧪 Running specific test: $(TEST)\n"
	@$(CONTAINER_CMD) run --rm \
        --security-opt label=disable \
        -v "$$PWD:/workspace" \
		$(MOD_CACHE_MOUNT) \
		-w /workspace \
		--network host \
		-e GOCACHE=/cache/go-build \
		-e GOMODCACHE=/cache/go-mod \
		-e DB_HOST=$(DB_HOST) -e DB_PORT=$(DB_PORT) \
		-e DB_NAME=gotrs_test -e DB_USER=gotrs_test -e DB_PASSWORD=gotrs_test_password \
		-e VALKEY_HOST=$(VALKEY_HOST) -e VALKEY_PORT=$(VALKEY_PORT) \
		-e APP_ENV=test \
		-e GOTRS_TEST_DB_READY=$(GOTRS_TEST_DB_READY) \
		gotrs-toolbox:latest \
		bash -lc 'export PATH=/usr/local/go/bin:$$PATH; go test -v -run "$(TEST)" ./...'

# Tidy Go modules inside toolbox (fetches missing deps and updates go.sum)
.PHONY: toolbox-mod-tidy
toolbox-mod-tidy:
	@$(MAKE) toolbox-build
	@printf "\n🧹 Running go mod tidy in toolbox...\n"
	@$(CONTAINER_CMD) run --rm \
		--security-opt label=disable \
		-v "$$PWD:/workspace" \
		-w /workspace \
		-u "$$UID:$$GID" \
		-e GOCACHE=/cache/go-build \
		-e GOMODCACHE=/cache/go-mod \
		gotrs-toolbox:latest \
		bash -lc 'export PATH=/usr/local/go/bin:$$PATH; go mod tidy && go mod download'

.PHONY: toolbox-gofmt
toolbox-gofmt:
	@$(MAKE) toolbox-build
	@printf "\n🧹 Running gofmt in toolbox...\n"
	@$(CONTAINER_CMD) run --rm \
		--security-opt label=disable \
		-v "$$PWD:/workspace" \
		-w /workspace \
		-u "$$UID:$$GID" \
		gotrs-toolbox:latest \
		bash -lc 'export PATH=/usr/local/go/bin:$$PATH; if [ -n "$(FILES)" ]; then gofmt -w $(FILES); else find . -path "./vendor" -prune -o -name "*.go" -print | xargs gofmt -w; fi'

# Run tests for a specific package (PKG=./internal/api) with optional TEST pattern
.PHONY: toolbox-test-pkg
toolbox-test-pkg:
	@[ -n "$(PKG)" ] || (echo "Usage: make toolbox-test-pkg PKG=./internal/api [TEST=^TestName]" && exit 2)
	@$(MAKE) toolbox-build
	@printf "\n🧪 Running package tests in toolbox: PKG=$(PKG) TEST=$(TEST)\n"
	@$(CONTAINER_CMD) run --rm \
		--security-opt label=disable \
		-v "$$PWD:/workspace" \
		$(MOD_CACHE_MOUNT) \
		-w /workspace \
		-u "$$UID:$$GID" \
		--network host \
		-e GOCACHE=/cache/go-build \
		-e GOMODCACHE=/cache/go-mod \
		-e APP_ENV=test \
		-e ENABLE_TEST_ADMIN_ROUTES=1 \
		-e STORAGE_PATH=/tmp \
		-e TEMPLATES_DIR=/workspace/templates \
		-e DB_DRIVER=$(TEST_DB_DRIVER) \
		-e DB_HOST=$(TOOLBOX_TEST_DB_HOST) \
		-e DB_PORT=$(TOOLBOX_TEST_DB_PORT) \
		-e DB_NAME=$(TEST_DB_NAME) \
		-e DB_USER=$(TEST_DB_USER) \
		-e DB_PASSWORD=$(TEST_DB_PASSWORD) \
		-e TEST_DB_DRIVER=$(TEST_DB_DRIVER) \
		-e TEST_DB_HOST=$(TOOLBOX_TEST_DB_HOST) \
		-e TEST_DB_PORT=$(TOOLBOX_TEST_DB_PORT) \
		-e TEST_DB_NAME=$(TEST_DB_NAME) \
		-e TEST_DB_USER=$(TEST_DB_USER) \
		-e TEST_DB_PASSWORD=$(TEST_DB_PASSWORD) \
		-e GOTRS_TEST_DB_READY=$(GOTRS_TEST_DB_READY) \
		gotrs-toolbox:latest \
		bash -lc 'export PATH=/usr/local/go/bin:$$PATH; if [ -n "$(TEST)" ]; then go test -v -run "$(TEST)" $(PKG); else go test -v $(PKG); fi'

# Run tests for explicit files (FILES="./internal/api/attachment_validation_webp_svg_test.go ./internal/api/attachment_validation_jpeg_test.go")
.PHONY: toolbox-test-files
toolbox-test-files:
	@[ -n "$(FILES)" ] || (echo "Usage: make toolbox-test-files FILES=\"path/to/a_test.go path/to/b_test.go\" [TEST=^Pattern]" && exit 2)
	@$(MAKE) toolbox-build
	@printf "\n🧪 Running test files in toolbox: FILES=$(FILES) TEST=$(TEST)\n"
	@$(CONTAINER_CMD) run --rm \
		--security-opt label=disable \
		-v "$$PWD:/workspace" \
		$(MOD_CACHE_MOUNT) \
		-w /workspace \
		-u "$$UID:$$GID" \
		--network host \
		-e GOCACHE=/cache/go-build \
		-e GOMODCACHE=/cache/go-mod \
		-e APP_ENV=test \
		-e ENABLE_TEST_ADMIN_ROUTES=1 \
		-e STORAGE_PATH=/tmp \
		-e TEMPLATES_DIR=/workspace/templates \
		-e DB_DRIVER=$(TEST_DB_DRIVER) \
		-e DB_HOST=$(TOOLBOX_TEST_DB_HOST) \
		-e DB_PORT=$(TOOLBOX_TEST_DB_PORT) \
		-e DB_NAME=$(TEST_DB_NAME) \
		-e DB_USER=$(TEST_DB_USER) \
		-e DB_PASSWORD=$(TEST_DB_PASSWORD) \
		-e TEST_DB_DRIVER=$(TEST_DB_DRIVER) \
		-e TEST_DB_HOST=$(TOOLBOX_TEST_DB_HOST) \
		-e TEST_DB_PORT=$(TOOLBOX_TEST_DB_PORT) \
		-e TEST_DB_NAME=$(TEST_DB_NAME) \
		-e TEST_DB_USER=$(TEST_DB_USER) \
		-e TEST_DB_PASSWORD=$(TEST_DB_PASSWORD) \
		-e GOTRS_TEST_DB_READY=$(GOTRS_TEST_DB_READY) \
		gotrs-toolbox:latest \
		bash -lc 'export PATH=/usr/local/go/bin:$$PATH; if [ -n "$(TEST)" ]; then go test -v -run "$(TEST)" $(FILES); else go test -v $(FILES); fi'

# Run static analysis using staticcheck inside toolbox
toolbox-staticcheck:
	@$(MAKE) toolbox-build
	@printf "\n🔎 Running staticcheck in toolbox...\n"
	$(CONTAINER_CMD) run --rm \
		--security-opt label=disable \
		-v "$$PWD:/workspace" \
		-w /workspace \
		-u "$$UID:$$GID" \
		-e GOCACHE=/cache/go-build \
		-e GOMODCACHE=/cache/go-mod \
		-e GOFLAGS=-buildvcs=false \
		gotrs-toolbox:latest \
		bash -lc 'set -e; export PATH=/usr/local/go/bin:/usr/local/bin:$$PATH; export GOFLAGS="-buildvcs=false"; go version; \
		go install honnef.co/go/tools/cmd/staticcheck@master >/dev/null 2>&1 || true; \
		staticcheck -version; \
		PKGS=$$(go list ./... | rg -v "^(github.com/gotrs-io/gotrs-ce/(tests/e2e))"); \
		echo "Staticchecking packages:"; echo "$$PKGS" | tr "\n" " "\; echo; \
		set +e; OUT=$$(GOTOOLCHAIN=local staticcheck -f=stylish -checks=all,-U1000,-ST1000,-ST1003,-SA9003,-ST1020,-ST1021,-ST1022,-ST1023 $$PKGS 2>&1); RC=$$?; set -e; \
		if [ $$RC -ne 0 ]; then \
		  echo "staticcheck failed:"; echo "$$OUT"; \
		  if echo "$$OUT" | grep -qi "unsupported version: 2"; then \
		    echo "⚠  Detected staticcheck vs Go 1.24 export format mismatch. Skipping staticcheck until upstream supports Go 1.24."; \
		    exit 0; \
		  fi; \
		  exit $$RC; \
		fi; echo "staticcheck: PASS";'

# Run a specific Go file
toolbox-run-file:
	@$(MAKE) toolbox-build
	@printf "\n🚀 Running Go file: $(FILE)\n"
	@$(CONTAINER_CMD) run --rm \
        --security-opt label=disable \
        -v "$$PWD:/workspace" \
		-w /workspace \
		-u "$$UID:$$GID" \
		--network host \
		-e GOCACHE=/cache/go-build \
		-e GOMODCACHE=/cache/go-mod \
		-e DB_HOST=postgres -e DB_PORT=$(DB_PORT) \
		-e DB_NAME=$(DB_NAME) -e DB_USER=$(DB_USER) \
		-e PGPASSWORD=$(DB_PASSWORD) \
		-e VALKEY_HOST=$(VALKEY_HOST) -e VALKEY_PORT=$(VALKEY_PORT) \
		-e APP_ENV=development \
		gotrs-toolbox:latest \
		bash -lc 'export PATH=/usr/local/go/bin:$$PATH && go run $(FILE)'

# Run all linters (Go, YAML, OpenAPI, Helm)
.PHONY: lint
lint: toolbox-lint yaml-lint openapi-lint helm-lint
	@printf "✅ All linting complete\n"

# Run linting with toolbox
toolbox-lint:
	@$(MAKE) toolbox-build
	@printf "🔍 Running linters...\n"
	@$(CONTAINER_CMD) run --rm \
		-v "$$(pwd):/workspace" \
		-v gotrs_cache:/cache \
		-w /workspace \
		-u "$$(id -u):$$(id -g)" \
		-e GOCACHE=/cache/go-build \
		-e GOMODCACHE=/cache/go-mod \
		-e GOPATH=/cache/gopath \
		-e GOLANGCI_LINT_CACHE=/cache/golangci-lint \
		gotrs-toolbox:latest \
		golangci-lint run ./...

# Run YAML linting with toolbox
yaml-lint:
	@$(MAKE) toolbox-build
	@printf "📄 Linting YAML files...\n"
	@$(CONTAINER_CMD) run --rm \
		-v "$$(pwd):/workspace" \
		-w /workspace \
		-u "$$(id -u):$$(id -g)" \
		gotrs-toolbox:latest \
		bash -lc 'tmpr=$$(mktemp -u); tmpr=$$(mktemp "$$tmpr.XXXXXX"); (find routes -type f -name "*.yaml" -print0; find config -type f -name "*.yaml" -print0; find .github -type f -name "*.yaml" -print0) > "$$tmpr"; if [ ! -s "$$tmpr" ]; then echo "⚠️  no YAML files found"; rm -f "$$tmpr"; exit 0; fi; echo "🔧 Linting YAML files with .yamllint"; rc=0; xargs -0 yamllint -c .yamllint < "$$tmpr" || rc=$$?; rm -f "$$tmpr"; if [ $$rc -ne 0 ]; then echo "⚠️  yamllint found issues"; exit $$rc; fi'
# Run security scan with toolbox
toolbox-security:
	@$(MAKE) toolbox-build
	@printf "🔒 Running security scan...\n"
	@$(CONTAINER_CMD) run --rm \
		-v "$$(pwd):/workspace" \
		-w /workspace \
		-u "$$(id -u):$$(id -g)" \
		gotrs-toolbox:latest \
		gosec ./...

# Run Trivy vulnerability scan locally
trivy-scan:
	@printf "🔍 Running Trivy vulnerability scan...\n"
	@$(CONTAINER_CMD) run --rm \
		-v "$$(pwd):/workspace" \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v gotrs_cache:/cache \
		-e TRIVY_CACHE_DIR=/cache/trivy \
		aquasec/trivy:latest \
		fs --severity CRITICAL,HIGH,MEDIUM /workspace

# Run Trivy on built images
trivy-images:
	@printf "🔍 Scanning backend image...\n"
	@$(CONTAINER_CMD) run --rm \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v gotrs_cache:/cache \
		-e TRIVY_CACHE_DIR=/cache/trivy \
		aquasec/trivy:latest \
		image gotrs-backend:latest
	@printf "🔍 Scanning frontend image...\n"
	@$(CONTAINER_CMD) run --rm \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v gotrs_cache:/cache \
		-e TRIVY_CACHE_DIR=/cache/trivy \
		aquasec/trivy:latest \
		image gotrs-frontend:latest

# Schema discovery - generate YAML modules from database
schema-discovery:
	@printf "🔍 Discovering database schema and generating YAML modules...\n"
	@./scripts/schema-discovery.sh --verbose

# Schema discovery for specific table
schema-table:
	@if [ -z "$(TABLE)" ]; then \
		echo "Error: TABLE not specified. Usage: make schema-table TABLE=tablename"; \
		exit 1; \
	fi
	@printf "🔍 Generating YAML module for table: $(TABLE)...\n"
	@./scripts/schema-discovery.sh --table $(TABLE) --verbose

# Centralized host binary cleanup
.PHONY: clean-host-binaries
clean-host-binaries:
	@printf "🧹 Cleaning host binaries after container build...\n"
	@rm -f bin/goats bin/gotrs bin/server bin/migrate bin/generator bin/gotrs-migrate bin/schema-discovery 2>/dev/null || true
	@rm -f goats gotrs gotrs-* generator migrate server 2>/dev/null || true
	@printf "✅ Host binaries cleaned - containers have the only copies\n"

# Start core services interactively (and clean host binaries after build)
up:
	$(call check_compose)
	@if [ "$(DB_DRIVER)" = "postgres" ]; then \
		$(COMPOSE_CMD) up $(COMPOSE_UP_FLAGS) --build postgres valkey backend customer-fe; \
	else \
		$(COMPOSE_CMD) up $(COMPOSE_UP_FLAGS) --build mariadb valkey backend customer-fe; \
	fi
	@$(MAKE) clean-host-binaries
# Start in background (and clean host binaries after build)
up-d:
	@if [ "$(DB_DRIVER)" = "postgres" ]; then \
		$(COMPOSE_CMD) up -d --build postgres valkey backend customer-fe runner; \
	else \
		$(COMPOSE_CMD) up -d --build mariadb valkey backend customer-fe runner; \
	fi
	@$(MAKE) clean-host-binaries
# Stop all services
down:
	$(call check_compose)
	$(COMPOSE_CMD) down

# Restart services
restart: down up-d
	@$(MAKE) clean-host-binaries
	@printf "🔄 Restarted all services\n"
# View logs
logs:
	$(call check_compose)
	@if [ "$(DB_DRIVER)" = "postgres" ]; then \
		$(COMPOSE_CMD) logs postgres valkey backend; \
	else \
		$(COMPOSE_CMD) logs mariadb valkey backend; \
	fi

logs-follow:
	@if [ "$(DB_DRIVER)" = "postgres" ]; then \
		$(COMPOSE_CMD) logs -f postgres valkey backend; \
	else \
		$(COMPOSE_CMD) logs -f mariadb valkey backend; \
	fi

# Service-specific logs
backend-logs:
	$(COMPOSE_CMD) logs backend

backend-logs-follow:
	$(COMPOSE_CMD) logs -f backend

runner-logs:
	$(COMPOSE_CMD) logs runner

runner-logs-follow:
	$(COMPOSE_CMD) logs -f runner

runner-up:
	$(COMPOSE_CMD) up -d runner

runner-down:
	$(COMPOSE_CMD) down runner

runner-restart: runner-down runner-up

frontend-logs:
	$(COMPOSE_CMD) logs frontend

frontend-logs-follow:
	$(COMPOSE_CMD) logs -f frontend

db-logs:
	@if [ "$(DB_DRIVER)" = "postgres" ]; then \
		$(COMPOSE_CMD) logs -f postgres; \
	else \
		$(COMPOSE_CMD) logs -f mariadb; \
	fi

# Clean everything (including volumes)
clean:
	$(COMPOSE_CMD) down -v
	rm -rf tmp/ generated/

# Reset database
reset-db:
	$(COMPOSE_CMD) down -v postgres
	$(COMPOSE_CMD) up -d postgres
	@printf "Database reset. Waiting for initialization...\n"
	@sleep 5

# Test environment management
.PHONY: test-stack-up test-stack-teardown test-stack-wait

test-stack-up:
	@$(MAKE) test-stack-teardown >/dev/null 2>&1 || true
	@$(MAKE) test-up
	@$(MAKE) test-stack-wait

test-stack-wait:
	@printf "⏳ Waiting for test backend health at %s/health...\n" "$(TEST_BACKEND_BASE_URL)"
	@if echo "$(COMPOSE_CMD)" | grep -q '^MISSING:'; then \
		echo "ERROR: $(COMPOSE_CMD)"; \
		echo "Please install the required compose tool and try again."; \
		exit 1; \
	fi
	@# Use host curl if available (CI), otherwise use toolbox container
	@if command -v curl > /dev/null 2>&1; then \
		for i in $$(seq 1 60); do \
			if curl -fsS -o /dev/null "$(TEST_BACKEND_BASE_URL)/health" 2>/dev/null; then \
				exit 0; \
			fi; \
			sleep 1; \
		done; \
		echo "Timed out waiting for test backend at $(TEST_BACKEND_BASE_URL)"; \
		exit 1; \
	else \
		COMPOSE_PROFILES="toolbox,testdb" $(COMPOSE_CMD) -f docker-compose.yml -f docker-compose.testdb.yml -f docker-compose.test.yaml run --rm -T \
			-e TEST_BACKEND_BASE_URL=$(TEST_BACKEND_BASE_URL) \
			toolbox \
			bash -lc 'set -e; for i in $$(seq 1 60); do if curl -fsS -o /dev/null "$${TEST_BACKEND_BASE_URL%/}/health"; then exit 0; fi; sleep 1; done; echo "Timed out waiting for test backend at $$TEST_BACKEND_BASE_URL"; exit 1'; \
	fi

test-stack-teardown:
	@if echo "$(COMPOSE_CMD)" | grep -q '^MISSING:'; then \
		exit 0; \
	fi
	# Stop only test-specific containers - do NOT use 'down' with docker-compose.yml
	# as that would bring down the dev/prod stack too
	@docker stop gotrs-backend-test gotrs-runner-test gotrs-customer-fe-test 2>/dev/null || true
	@docker rm gotrs-backend-test gotrs-runner-test gotrs-customer-fe-test 2>/dev/null || true

test-up:
	@printf "🚀 Starting test environment...\n"
	@if echo "$(COMPOSE_CMD)" | grep -q '^MISSING:'; then \
		echo "ERROR: $(COMPOSE_CMD)"; \
		echo "Please install the required compose tool and try again."; \
		exit 1; \
	fi
	$(COMPOSE_CMD) -f docker-compose.yml -f docker-compose.testdb.yml --profile testdb up -d mariadb-test valkey-test smtp4dev
	$(COMPOSE_CMD) -f docker-compose.yml -f docker-compose.testdb.yml --profile testdb -f docker-compose.test.yaml build backend-test runner-test
	$(COMPOSE_CMD) -f docker-compose.yml -f docker-compose.testdb.yml --profile testdb -f docker-compose.test.yaml up -d backend-test runner-test
	@printf "✅ Test environment ready!\n"
	@printf "   - Test backend: http://localhost:%s\n" "$(TEST_BACKEND_PORT)"
	@printf "   - Test DB MySQL: localhost:$(TEST_DB_MYSQL_PORT:-3308)\n"
	@printf "   - Test DB Postgres: localhost:$(TEST_DB_POSTGRES_PORT:-5433)\n"

test-down:
	@printf "🛑 Stopping test environment...\n"
	@if echo "$(COMPOSE_CMD)" | grep -q '^MISSING:'; then \
		echo "ERROR: $(COMPOSE_CMD)"; \
		echo "Please install the required compose tool and try again."; \
		exit 1; \
	fi
	$(COMPOSE_CMD) -f docker-compose.yml -f docker-compose.testdb.yml --profile testdb -f docker-compose.test.yaml down
	$(COMPOSE_CMD) -f docker-compose.yml -f docker-compose.testdb.yml --profile testdb down smtp4dev >/dev/null 2>&1 || true
	@printf "✅ Test environment stopped\n"

test-restart: test-down test-up
	@printf "🔄 Test environment restarted\n"

test-status:
	@printf "📊 Test environment status:\n"
	@if echo "$(COMPOSE_CMD)" | grep -q '^MISSING:'; then \
		echo "ERROR: $(COMPOSE_CMD)"; \
		echo "Please install the required compose tool and try again."; \
		exit 1; \
	fi
	@$(COMPOSE_CMD) -f docker-compose.yml -f docker-compose.testdb.yml --profile testdb -f docker-compose.test.yaml ps

test-logs:
	@if echo "$(COMPOSE_CMD)" | grep -q '^MISSING:'; then \
		echo "ERROR: $(COMPOSE_CMD)"; \
		echo "Please install the required compose tool and try again."; \
		exit 1; \
	fi
	@$(COMPOSE_CMD) -f docker-compose.yml -f docker-compose.testdb.yml --profile testdb -f docker-compose.test.yaml logs -f

# Load environment variables from .env file (only if it exists)
-include .env
export

# Migration paths (inside container)
PG_MIGRATE_PATH     ?= /app/migrations/postgres
MYSQL_MIGRATE_PATH  ?= /app/migrations/mysql
PG_MIGRATIONS_DIR   ?= migrations/postgres
MYSQL_MIGRATIONS_DIR ?= migrations/mysql
# Active migrations dir depends on DB_DRIVER (for gen-migration target)
ifeq ($(DB_DRIVER),postgres)
ACTIVE_MIGRATIONS_DIR ?= $(PG_MIGRATIONS_DIR)
else
ACTIVE_MIGRATIONS_DIR ?= $(MYSQL_MIGRATIONS_DIR)
endif

# Database operations
# Set this from the environment or override on the command line
#    e.g.   echo "select * from users;"| make db-shell
#           echo "select * from users;"| make DB_DRIVER=mysql   db-shell
db-shell:
	@if [ -t 0 ]; then \
		TTY_FLAGS="-it"; \
	else \
		TTY_FLAGS="-T"; \
	fi; \
	if [ "$(DB_DRIVER)" = "postgres" ]; then \
		$(COMPOSE_CMD) --profile toolbox run --rm $$TTY_FLAGS toolbox psql -h $(DB_HOST) -U $(DB_USER) -d $(DB_NAME); \
	else \
		$(COMPOSE_CMD) --profile toolbox run --rm $$TTY_FLAGS toolbox mysql -h $(DB_HOST) -u $(DB_USER) -p$(DB_PASSWORD) -D $(DB_NAME); \
	fi

db-shell-test: test-db-up
	@if [ -t 0 ]; then \
		TTY_FLAGS="-it"; \
	else \
		TTY_FLAGS="-T"; \
	fi; \
	if [ "$(TEST_DB_DRIVER)" = "postgres" ]; then \
		$(COMPOSE_CMD) --profile toolbox run --rm $$TTY_FLAGS toolbox psql -h $(TEST_DB_POSTGRES_HOST) -U $(TEST_DB_POSTGRES_USER) -d $(TEST_DB_POSTGRES_NAME); \
	else \
		$(COMPOSE_CMD) --profile toolbox run --rm $$TTY_FLAGS toolbox mysql -h $(TEST_DB_MYSQL_HOST) -u $(TEST_DB_MYSQL_USER) -p$(TEST_DB_MYSQL_PASSWORD) -D $(TEST_DB_MYSQL_NAME); \
	fi

# Fix PostgreSQL sequences after data import (PostgreSQL only)
db-fix-sequences:
	@if [ "$(DB_DRIVER)" = "postgres" ]; then \
		printf "🔧 Fixing database sequences...\n"; \
		$(COMPOSE_CMD) --profile toolbox run --rm -T toolbox psql -h $(DB_HOST) -U $(DB_USER) -d $(DB_NAME) -v ON_ERROR_STOP=1 -f /workspace/scripts/db/postgres/fix_sequences.sql; \
		printf "✅ Sequences fixed - duplicate key errors should be resolved\n"; \
	else \
		printf "ℹ️  Sequence fixing is only needed for PostgreSQL databases\n"; \
	fi

db-fix-sequences-test:
	@$(MAKE) test-db-up >/dev/null 2>&1 || true
	@if [ "$(TEST_DB_DRIVER)" = "postgres" ]; then \
		printf "🔧 Fixing test database sequences...\n"; \
		$(COMPOSE_CMD) --profile toolbox run --rm -T toolbox psql -h $(TEST_DB_POSTGRES_HOST) -p $(TEST_DB_POSTGRES_CONTAINER_PORT) -U $(TEST_DB_POSTGRES_USER) -d $(TEST_DB_POSTGRES_NAME) -v ON_ERROR_STOP=1 -f /workspace/scripts/db/postgres/fix_sequences.sql; \
		printf "✅ Test database sequences synchronized\n"; \
	else \
		printf "ℹ️  Sequence fixing is only needed for PostgreSQL test databases\n"; \
	fi
# Run a database query (use QUERY="SELECT ..." make db-query)
db-query:
	# Robust query execution: supports STDIN, QUERY_FILE, or QUERY var
	# Priority: (1) read SQL from STDIN if not a TTY, (2) read from QUERY_FILE, (3) use QUERY variable
	@if [ -t 0 ] && [ -z "$(QUERY)" ] && [ -z "$(QUERY_FILE)" ]; then \
		echo "Usage:"; \
		echo "  echo 'SELECT 1;' | make db-query"; \
		echo "  make db-query QUERY=\"SELECT * FROM table WHERE name = 'foo';\""; \
		echo "  make db-query QUERY_FILE=path/to/query.sql"; \
		exit 1; \
	fi; \
	if [ "$(DB_DRIVER)" = "postgres" ]; then \
		if [ ! -t 0 ]; then \
			$(COMPOSE_CMD) --profile toolbox run --rm -T toolbox psql -h $(DB_HOST) -U $(DB_USER) -d $(DB_NAME) -t; \
		elif [ -n "$(QUERY_FILE)" ]; then \
			$(COMPOSE_CMD) --profile toolbox run --rm -T toolbox bash -lc "psql -h $(DB_HOST) -U $(DB_USER) -d $(DB_NAME) -t < '$(QUERY_FILE)'"; \
		else \
			$(COMPOSE_CMD) --profile toolbox run --rm -T toolbox psql -h $(DB_HOST) -U $(DB_USER) -d $(DB_NAME) -t -c "$(QUERY)"; \
		fi; \
	else \
		if [ ! -t 0 ]; then \
			$(COMPOSE_CMD) --profile toolbox run --rm -T toolbox mysql -h $(DB_HOST) -u $(DB_USER) -p$(DB_PASSWORD) -D $(DB_NAME); \
		elif [ -n "$(QUERY_FILE)" ]; then \
			$(COMPOSE_CMD) --profile toolbox run --rm -T toolbox bash -lc "mysql -h $(DB_HOST) -u $(DB_USER) -p$(DB_PASSWORD) -D $(DB_NAME) < '$(QUERY_FILE)'"; \
		else \
			$(COMPOSE_CMD) --profile toolbox run --rm -T toolbox mysql -h $(DB_HOST) -u $(DB_USER) -p$(DB_PASSWORD) -D $(DB_NAME) -e "$(QUERY)"; \
		fi; \
	fi

# Test database operations (automatically use TEST_DB_* variables)
db-query-test:
	# Test database query execution: supports STDIN, QUERY_FILE, or QUERY var
	@$(MAKE) test-db-up >/dev/null 2>&1 || true
	@if [ -t 0 ] && [ -z "$(QUERY)" ] && [ -z "$(QUERY_FILE)" ]; then \
		echo "Usage:"; \
		echo "  echo 'SELECT 1;' | make db-query-test"; \
		echo "  make db-query-test QUERY=\"SELECT * FROM table WHERE name = 'foo';\""; \
		echo "  make db-query-test QUERY_FILE=path/to/query.sql"; \
		exit 1; \
	fi
	@if echo "$(COMPOSE_CMD)" | grep -q '^MISSING:'; then \
		echo "ERROR: $(COMPOSE_CMD)"; \
		echo "Please install the required compose tool and try again."; \
		exit 1; \
	fi
	@if [ "$(TEST_DB_DRIVER)" = "postgres" ]; then \
		if [ ! -t 0 ]; then \
			$(COMPOSE_CMD) --profile toolbox run --rm -T toolbox psql -h $(TEST_DB_POSTGRES_HOST) -U $(TEST_DB_POSTGRES_USER) -d $(TEST_DB_POSTGRES_NAME) -t; \
		elif [ -n "$(QUERY_FILE)" ]; then \
			$(COMPOSE_CMD) --profile toolbox run --rm -T toolbox bash -lc "psql -h $(TEST_DB_POSTGRES_HOST) -U $(TEST_DB_POSTGRES_USER) -d $(TEST_DB_POSTGRES_NAME) -t < '$(QUERY_FILE)'"; \
		else \
			$(COMPOSE_CMD) --profile toolbox run --rm -T toolbox psql -h $(TEST_DB_POSTGRES_HOST) -U $(TEST_DB_POSTGRES_USER) -d $(TEST_DB_POSTGRES_NAME) -t -c "$(QUERY)"; \
		fi; \
	else \
		if [ ! -t 0 ]; then \
			$(COMPOSE_CMD) --profile toolbox run --rm -T toolbox mysql -h $(TEST_DB_MYSQL_HOST) -u $(TEST_DB_MYSQL_USER) -p$(TEST_DB_MYSQL_PASSWORD) -D $(TEST_DB_MYSQL_NAME); \
		elif [ -n "$(QUERY_FILE)" ]; then \
			$(COMPOSE_CMD) --profile toolbox run --rm -T toolbox bash -lc "mysql -h $(TEST_DB_MYSQL_HOST) -u $(TEST_DB_MYSQL_USER) -p$(TEST_DB_MYSQL_PASSWORD) -D $(TEST_DB_MYSQL_NAME) < '$(QUERY_FILE)'"; \
		else \
			$(COMPOSE_CMD) --profile toolbox run --rm -T toolbox mysql -h $(TEST_DB_MYSQL_HOST) -u $(TEST_DB_MYSQL_USER) -p$(TEST_DB_MYSQL_PASSWORD) -D $(TEST_DB_MYSQL_NAME) -e "$(QUERY)"; \
		fi; \
	fi

db-seed-test:
	@$(MAKE) test-db-up >/dev/null 2>&1 || true
	@printf "Seeding test database with comprehensive test data...\n"
	@if [ "$(TEST_DB_DRIVER)" = "postgres" ]; then \
		DB_URI="postgres://$(TEST_DB_POSTGRES_USER):$(TEST_DB_POSTGRES_PASSWORD)@$(TEST_DB_POSTGRES_HOST):$(TEST_DB_POSTGRES_CONTAINER_PORT)/$(TEST_DB_POSTGRES_NAME)?sslmode=$(TEST_DB_SSLMODE)"; \
		$(COMPOSE_CMD) exec backend ./migrate -path $(PG_MIGRATE_PATH) -database "$$DB_URI" up; \
		$(MAKE) db-fix-sequences-test > /dev/null 2>&1 || true; \
	else \
		DB_URI="mysql://$(TEST_DB_MYSQL_USER):$(TEST_DB_MYSQL_PASSWORD)@tcp($(TEST_DB_MYSQL_HOST):$(TEST_DB_MYSQL_CONTAINER_PORT))/$(TEST_DB_MYSQL_NAME)?multiStatements=true"; \
		$(COMPOSE_CMD) exec backend ./migrate -path $(MYSQL_MIGRATE_PATH) -database "$$DB_URI" up; \
	fi
	@printf "✅ Test database ready for testing\n"
db-reset-test:
	@$(MAKE) test-db-up >/dev/null 2>&1 || true
	@printf "Resetting test database...\n"
	@if [ "$(TEST_DB_DRIVER)" = "postgres" ]; then \
		DB_URI="postgres://$(TEST_DB_POSTGRES_USER):$(TEST_DB_POSTGRES_PASSWORD)@$(TEST_DB_POSTGRES_HOST):$(TEST_DB_POSTGRES_CONTAINER_PORT)/$(TEST_DB_POSTGRES_NAME)?sslmode=$(TEST_DB_SSLMODE)"; \
		$(COMPOSE_CMD) exec backend migrate -path $(PG_MIGRATE_PATH) -database "$$DB_URI" down -all; \
		$(COMPOSE_CMD) exec backend migrate -path $(PG_MIGRATE_PATH) -database "$$DB_URI" up; \
		$(MAKE) db-fix-sequences-test > /dev/null 2>&1 || true; \
	else \
		DB_URI="mysql://$(TEST_DB_MYSQL_USER):$(TEST_DB_MYSQL_PASSWORD)@tcp($(TEST_DB_MYSQL_HOST):$(TEST_DB_MYSQL_CONTAINER_PORT))/$(TEST_DB_MYSQL_NAME)?multiStatements=true"; \
		$(COMPOSE_CMD) exec backend migrate -path $(MYSQL_MIGRATE_PATH) -database "$$DB_URI" down -all; \
		$(COMPOSE_CMD) exec backend migrate -path $(MYSQL_MIGRATE_PATH) -database "$$DB_URI" up; \
	fi
	@$(MAKE) clean-storage
	@printf "✅ Test database reset with fresh test data\n"

# Quick reseed test data without full migration (faster than db-reset-test)
db-reseed-test:
	@$(MAKE) test-db-up >/dev/null 2>&1 || true
	@printf "Quick reseed test database...\n"
	@if [ "$(TEST_DB_DRIVER)" = "postgres" ]; then \
		echo "PostgreSQL quick reseed not implemented yet - using full reset"; \
		$(MAKE) db-reset-test; \
	else \
		$(COMPOSE_CMD) -f docker-compose.yml -f docker-compose.testdb.yml exec -T mariadb-test \
			mariadb -u$(TEST_DB_MYSQL_USER) -p$(TEST_DB_MYSQL_PASSWORD) $(TEST_DB_MYSQL_NAME) < scripts/reset-test-data.sql; \
	fi
	@printf "✅ Test database reseeded\n"

 .PHONY: toolbox-exec
toolbox-exec:
	@sh -c 'if [ -z "$$1" ]; then echo "Usage: make toolbox-exec ARGS=\"<command>\""; exit 2; fi' -- "$(ARGS)"
	@if echo "$(COMPOSE_CMD)" | grep -q '^MISSING:'; then \
		echo "ERROR: $(COMPOSE_CMD)"; \
		echo "Please install the required compose tool and try again."; \
		exit 1; \
	fi
	@printf "\n🔧 toolbox -> %s\n" "$(ARGS)"
	@# Always include testdb profile so tests can reach the test database
	@if echo "$(COMPOSE_CMD)" | grep -q "podman-compose"; then \
		COMPOSE_PROFILES="toolbox,testdb" $(COMPOSE_CMD) -f docker-compose.yml -f docker-compose.testdb.yml run --rm -T toolbox bash -lc 'set -o pipefail; bash -lc "$$1"; rc=$$?; if [ $$rc -ne 0 ]; then echo "❌ toolbox command failed with exit $$rc"; exit $$rc; fi' -- "$(ARGS)"; \
	else \
		$(COMPOSE_CMD) -f docker-compose.yml -f docker-compose.testdb.yml --profile toolbox --profile testdb run --rm -T toolbox bash -lc 'set -o pipefail; bash -lc "$$1"; rc=$$?; if [ $$rc -ne 0 ]; then echo "❌ toolbox command failed with exit $$rc"; exit $$rc; fi' -- "$(ARGS)"; \
	fi

toolbox-exec-test:
	@if echo "$(COMPOSE_CMD)" | grep -q '^MISSING:'; then \
		echo "ERROR: $(COMPOSE_CMD)"; \
		echo "Please install the required compose tool and try again."; \
		exit 1; \
	fi
	@$(COMPOSE_CMD) --profile toolbox run --rm -T toolbox bash -lc 'set -o pipefail; bash -lc "$$1"; rc=$$?; if [ $$rc -ne 0 ]; then echo "❌ toolbox command failed with exit $$rc"; exit $$rc; fi' -- "$(ARGS)"

api-call-test:
	@if [ -z "$(ENDPOINT)" ]; then echo "❌ ENDPOINT required. Usage: make api-call-test [METHOD=GET] ENDPOINT=/api/v1/tickets [BODY='{}']"; exit 1; fi
	@if [ -z "$(METHOD)" ]; then METHOD=GET; fi; \
	printf "\n🔧 Making test API call: $$METHOD $(ENDPOINT)\n"; \
	@if echo "$(COMPOSE_CMD)" | grep -q '^MISSING:'; then \
		echo "ERROR: $(COMPOSE_CMD)"; \
		echo "Please install the required compose tool and try again."; \
		exit 1; \
	fi
	@if echo "$(COMPOSE_CMD)" | grep -q "podman-compose"; then \
		COMPOSE_PROFILES=toolbox BACKEND_URL="http://backend-test:8080" $(COMPOSE_CMD) run --rm toolbox bash scripts/api-test.sh "$$METHOD" "$(ENDPOINT)" "$(BODY)"; \
	else \
		BACKEND_URL="http://backend-test:8080" $(COMPOSE_CMD) --profile toolbox run --rm toolbox bash scripts/api-test.sh "$$METHOD" "$(ENDPOINT)" "$(BODY)"; \
	fi

# API testing for form-urlencoded bodies with automatic authentication (test environment)
.PHONY: api-call-form-test
api-call-form-test:
	@if [ -z "$(ENDPOINT)" ]; then echo "❌ ENDPOINT required. Usage: make api-call-form-test [METHOD=PUT] ENDPOINT=/admin/users/1 [DATA='a=b&c=d']"; exit 1; fi
	@if [ -z "$(METHOD)" ]; then METHOD=PUT; fi; \
	printf "\n🔧 Making test form API call: $$METHOD $(ENDPOINT)\n"; \
	@if echo "$(COMPOSE_CMD)" | grep -q "podman-compose"; then \
		COMPOSE_PROFILES=toolbox $(COMPOSE_CMD) run --rm -u "$$

db-migrate:
	@printf "Running database migrations...\n"
	@if [ "$(DB_DRIVER)" = "postgres" ]; then \
		$(COMPOSE_CMD) exec backend ./migrate -path $(PG_MIGRATE_PATH) -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" up; \
	else \
		$(COMPOSE_CMD) exec backend ./migrate -path $(MYSQL_MIGRATE_PATH) -database "mysql://$(DB_USER):$(DB_PASSWORD)@tcp(mariadb:3306)/$(DB_NAME)?multiStatements=true" up; \
	fi
	@printf "Migrations completed successfully!\n"
	@if [ "$(DB_DRIVER)" = "postgres" ]; then \
		printf "🔧 Fixing database sequences to prevent duplicate key errors...\n"; \
		$(MAKE) db-fix-sequences > /dev/null 2>&1 || true; \
		printf "✅ Database ready with sequences properly synchronized!\n"; \
	fi

db-migrate-test:
	@$(MAKE) test-db-up >/dev/null 2>&1 || true
	@printf "Running test database migrations...\n"
	@if [ "$(TEST_DB_DRIVER)" = "postgres" ]; then \
		DB_URI="postgres://$(TEST_DB_POSTGRES_USER):$(TEST_DB_POSTGRES_PASSWORD)@$(TEST_DB_POSTGRES_HOST):$(TEST_DB_POSTGRES_CONTAINER_PORT)/$(TEST_DB_POSTGRES_NAME)?sslmode=$(TEST_DB_SSLMODE)"; \
		$(COMPOSE_CMD) exec backend ./migrate -path $(PG_MIGRATE_PATH) -database "$$DB_URI" up; \
	else \
		DB_URI="mysql://$(TEST_DB_MYSQL_USER):$(TEST_DB_MYSQL_PASSWORD)@tcp($(TEST_DB_MYSQL_HOST):$(TEST_DB_MYSQL_CONTAINER_PORT))/$(TEST_DB_MYSQL_NAME)?multiStatements=true"; \
		$(COMPOSE_CMD) exec backend ./migrate -path $(MYSQL_MIGRATE_PATH) -database "$$DB_URI" up; \
	fi
	@printf "Test migrations completed successfully!\n"
	@if [ "$(TEST_DB_DRIVER)" = "postgres" ]; then \
		$(MAKE) db-fix-sequences-test > /dev/null 2>&1 || true; \
	fi
db-migrate-schema-only:
	@printf "Running schema migration only...\n"
	@if [ "$(DB_DRIVER)" = "postgres" ]; then \
		$(COMPOSE_CMD) exec backend ./migrate -path $(PG_MIGRATE_PATH) -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" up 3; \
	else \
		$(COMPOSE_CMD) exec backend ./migrate -path $(MYSQL_MIGRATE_PATH) -database "mysql://$(DB_USER):$(DB_PASSWORD)@tcp(mariadb:3306)/$(DB_NAME)?multiStatements=true" up 3; \
	fi
	@printf "Schema and initial data applied (no test data)\n"
	@if [ "$(DB_DRIVER)" = "postgres" ]; then \
		printf "🔧 Fixing database sequences...\n"; \
		$(MAKE) db-fix-sequences > /dev/null 2>&1 || true; \
		printf "✅ Sequences synchronized!\n"; \
	fi

db-migrate-schema-only-test:
	@$(MAKE) test-db-up >/dev/null 2>&1 || true
	@printf "Running test schema migration only...\n"
	@if [ "$(TEST_DB_DRIVER)" = "postgres" ]; then \
		DB_URI="postgres://$(TEST_DB_POSTGRES_USER):$(TEST_DB_POSTGRES_PASSWORD)@$(TEST_DB_POSTGRES_HOST):$(TEST_DB_POSTGRES_CONTAINER_PORT)/$(TEST_DB_POSTGRES_NAME)?sslmode=$(TEST_DB_SSLMODE)"; \
		$(COMPOSE_CMD) exec backend ./migrate -path $(PG_MIGRATE_PATH) -database "$$DB_URI" up 3; \
	else \
		DB_URI="mysql://$(TEST_DB_MYSQL_USER):$(TEST_DB_MYSQL_PASSWORD)@tcp($(TEST_DB_MYSQL_HOST):$(TEST_DB_MYSQL_CONTAINER_PORT))/$(TEST_DB_MYSQL_NAME)?multiStatements=true"; \
		$(COMPOSE_CMD) exec backend ./migrate -path $(MYSQL_MIGRATE_PATH) -database "$$DB_URI" up 3; \
	fi
	@printf "Test schema and initial data applied\n"
	@if [ "$(TEST_DB_DRIVER)" = "postgres" ]; then \
		$(MAKE) db-fix-sequences-test > /dev/null 2>&1 || true; \
	fi
db-seed-dev:
	@printf "Seeding development database with comprehensive test data...\n"
	@$(COMPOSE_CMD) exec backend ./migrate -path $(PG_MIGRATE_PATH) -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" up
	@printf "🔧 Fixing sequences after seeding...\n"
	@$(MAKE) db-fix-sequences > /dev/null 2>&1 || true
	@printf "✅ Development database seeded with:\n"
	@printf "   - 10 organizations\n"
	@printf "   - 50 customer users\n"
	@printf "   - 15 support agents\n"
	@printf "   - 100 ITSM tickets\n"
	@printf "   - Knowledge base articles\n"
db-reset-dev:
	@printf "⚠️  This will DELETE all data and recreate the development database!\n"
	@echo -n "Are you sure? [y/N]: "; \
	read confirm; \
	if [ "$$confirm" = "y" ] || [ "$$confirm" = "Y" ]; then \
		echo "Resetting development database..."; \
		$(COMPOSE_CMD) exec backend migrate -path $(PG_MIGRATE_PATH) -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" down -all; \
		$(COMPOSE_CMD) exec backend migrate -path $(PG_MIGRATE_PATH) -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" up; \
		$(MAKE) clean-storage; \
		echo "✅ Fresh development environment ready with test data!"; \
	else \
		echo "Reset cancelled."; \
	fi
db-refresh: db-reset-dev
	@printf "✅ Database refreshed for new development cycle\n"
db-rollback:
	$(COMPOSE_CMD) exec backend migrate -path $(PG_MIGRATE_PATH) -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" down 1

db-rollback-test:
	@$(MAKE) test-db-up >/dev/null 2>&1 || true
	@if [ "$(TEST_DB_DRIVER)" = "postgres" ]; then \
		DB_URI="postgres://$(TEST_DB_POSTGRES_USER):$(TEST_DB_POSTGRES_PASSWORD)@$(TEST_DB_POSTGRES_HOST):$(TEST_DB_POSTGRES_CONTAINER_PORT)/$(TEST_DB_POSTGRES_NAME)?sslmode=$(TEST_DB_SSLMODE)"; \
		$(COMPOSE_CMD) exec backend migrate -path $(PG_MIGRATE_PATH) -database "$$DB_URI" down 1; \
	else \
		DB_URI="mysql://$(TEST_DB_MYSQL_USER):$(TEST_DB_MYSQL_PASSWORD)@tcp($(TEST_DB_MYSQL_HOST):$(TEST_DB_MYSQL_CONTAINER_PORT))/$(TEST_DB_MYSQL_NAME)?multiStatements=true"; \
		$(COMPOSE_CMD) exec backend migrate -path $(MYSQL_MIGRATE_PATH) -database "$$DB_URI" down 1; \
	fi

# Fast database initialization from baseline (new approach)
db-init:
	@printf "🚀 Initializing database (fast path)...\n"
	@if [ "$(DB_DRIVER)" = "postgres" ]; then \
		$(COMPOSE_CMD) exec -T postgres psql -U $(DB_USER) -d $(DB_NAME) -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"; \
		$(COMPOSE_CMD) exec -T postgres psql -U $(DB_USER) -d $(DB_NAME) -f - < schema/baseline/otrs_complete.sql; \
		$(COMPOSE_CMD) exec -T postgres psql -U $(DB_USER) -d $(DB_NAME) -f - < schema/baseline/required_lookups.sql; \
		$(MAKE) clean-storage; \
		printf "🔧 Fixing sequences after baseline initialization...\n"; \
		$(MAKE) db-fix-sequences > /dev/null 2>&1 || true; \
		printf "✅ Database initialized from baseline (Postgres)\n"; \
	else \
		printf "📡 Starting dependencies (mariadb)...\n"; \
		$(COMPOSE_CMD) up -d mariadb >/dev/null 2>&1 || true; \
		printf "🧰 Ensuring minimal users table exists (MariaDB)...\n"; \
		if [ -z "$(GOTRS_ADMIN_PASSWORD)" ]; then \
			printf "❌ Error: GOTRS_ADMIN_PASSWORD not set. Add it to .env\n"; exit 1; \
		fi; \
		$(CONTAINER_CMD) run --rm --network gotrs-ce_gotrs-network \
			-e DB_DRIVER=$(DB_DRIVER) -e DB_HOST=$(DB_HOST) -e DB_PORT=$(DB_PORT) \
			-e DB_NAME=$(DB_NAME) -e DB_USER=$(DB_USER) -e DB_PASSWORD=$(DB_PASSWORD) \
			gotrs-toolbox:latest \
			gotrs reset-user --username="root@localhost" --password="$(GOTRS_ADMIN_PASSWORD)" --enable; \
		printf "✅ Database initialized (MariaDB minimal schema; root user created).\n"; \
	fi

db-init-test:
	@$(MAKE) test-db-up >/dev/null 2>&1 || true
	@printf "🚀 Initializing test database...\n"
	@if [ "$(TEST_DB_DRIVER)" = "postgres" ]; then \
		APP_ENV=test $(COMPOSE_CMD) -f docker-compose.yml -f docker-compose.testdb.yml exec -T postgres-test psql -U $(TEST_DB_POSTGRES_USER) -d $(TEST_DB_POSTGRES_NAME) -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"; \
		APP_ENV=test $(COMPOSE_CMD) -f docker-compose.yml -f docker-compose.testdb.yml exec -T postgres-test psql -U $(TEST_DB_POSTGRES_USER) -d $(TEST_DB_POSTGRES_NAME) -f - < schema/baseline/otrs_complete.sql; \
		APP_ENV=test $(COMPOSE_CMD) -f docker-compose.yml -f docker-compose.testdb.yml exec -T postgres-test psql -U $(TEST_DB_POSTGRES_USER) -d $(TEST_DB_POSTGRES_NAME) -f - < schema/baseline/required_lookups.sql; \
		$(MAKE) db-fix-sequences-test > /dev/null 2>&1 || true; \
		printf "✅ Test database initialized from baseline (Postgres)\n"; \
	else \
		printf "📡 Starting dependencies (mariadb-test)...\n"; \
		APP_ENV=test $(COMPOSE_CMD) -f docker-compose.yml -f docker-compose.testdb.yml up -d mariadb-test >/dev/null 2>&1 || true; \
		printf "🧰 Ensuring minimal users table exists (MariaDB test)...\n"; \
		if [ -z "$(GOTRS_ADMIN_PASSWORD)" ]; then \
			printf "❌ Error: GOTRS_ADMIN_PASSWORD not set. Add it to .env\n"; exit 1; \
		fi; \
		$(CONTAINER_CMD) run --rm \
			--security-opt label=disable \
			--network gotrs-ce_gotrs-network \
			-e DB_DRIVER=$(TEST_DB_DRIVER) -e DB_HOST=$(TEST_DB_MYSQL_HOST) -e DB_PORT=$(TEST_DB_MYSQL_CONTAINER_PORT) \
			-e DB_NAME=$(TEST_DB_MYSQL_NAME) -e DB_USER=$(TEST_DB_MYSQL_USER) -e DB_PASSWORD=$(TEST_DB_MYSQL_PASSWORD) \
			gotrs-toolbox:latest \
			gotrs reset-user --username="root@localhost" --password="$(GOTRS_ADMIN_PASSWORD)" --enable; \
		printf "✅ Test database initialized (MariaDB)\n"; \
	fi
# Initialize for OTRS import (structure only, no data)
db-init-import:
	@printf "🚀 Initializing database structure for OTRS import...\n"
	@$(COMPOSE_CMD) exec -T postgres psql -U $(DB_USER) -d $(DB_NAME) -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"
	@$(COMPOSE_CMD) exec -T postgres psql -U $(DB_USER) -d $(DB_NAME) -f - < schema/baseline/otrs_complete.sql
	@printf "✅ Database structure ready for OTRS import\n"
# Development environment with minimal seed data
db-init-dev:
	@printf "🚀 Initializing development database...\n"
	@$(MAKE) db-init
	@$(COMPOSE_CMD) exec -T postgres psql -U $(DB_USER) -d $(DB_NAME) -f - < schema/seed/minimal.sql
	@printf "🔧 Fixing sequences after initialization...\n"
	@$(MAKE) db-fix-sequences > /dev/null 2>&1 || true
	@printf "✅ Development database ready (admin/admin)\n"
# New reset using baseline
db-reset: db-init-dev

db-status:
	$(COMPOSE_CMD) exec backend ./migrate -path $(PG_MIGRATE_PATH) -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" version

db-status-test:
	@$(MAKE) test-db-up >/dev/null 2>&1 || true
	@if [ "$(TEST_DB_DRIVER)" = "postgres" ]; then \
		DB_URI="postgres://$(TEST_DB_POSTGRES_USER):$(TEST_DB_POSTGRES_PASSWORD)@$(TEST_DB_POSTGRES_HOST):$(TEST_DB_POSTGRES_CONTAINER_PORT)/$(TEST_DB_POSTGRES_NAME)?sslmode=$(TEST_DB_SSLMODE)"; \
		$(COMPOSE_CMD) exec backend ./migrate -path $(PG_MIGRATE_PATH) -database "$$DB_URI" version; \
	else \
		DB_URI="mysql://$(TEST_DB_MYSQL_USER):$(TEST_DB_MYSQL_PASSWORD)@tcp($(TEST_DB_MYSQL_HOST):$(TEST_DB_MYSQL_CONTAINER_PORT))/$(TEST_DB_MYSQL_NAME)?multiStatements=true"; \
		$(COMPOSE_CMD) exec backend ./migrate -path $(MYSQL_MIGRATE_PATH) -database "$$DB_URI" version; \
	fi

db-force:
	@echo -n "Force migration to version: "; \
	read version; \
	$(COMPOSE_CMD) exec backend ./migrate -path $(PG_MIGRATE_PATH) -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" force $$version

db-force-test:
	@$(MAKE) test-db-up >/dev/null 2>&1 || true
	@echo -n "Force migration to version: "; \
	read version; \
	if [ "$(TEST_DB_DRIVER)" = "postgres" ]; then \
		DB_URI="postgres://$(TEST_DB_POSTGRES_USER):$(TEST_DB_POSTGRES_PASSWORD)@$(TEST_DB_POSTGRES_HOST):$(TEST_DB_POSTGRES_CONTAINER_PORT)/$(TEST_DB_POSTGRES_NAME)?sslmode=$(TEST_DB_SSLMODE)"; \
		$(COMPOSE_CMD) exec backend ./migrate -path $(PG_MIGRATE_PATH) -database "$$DB_URI" force $$version; \
	else \
		DB_URI="mysql://$(TEST_DB_MYSQL_USER):$(TEST_DB_MYSQL_PASSWORD)@tcp($(TEST_DB_MYSQL_HOST):$(TEST_DB_MYSQL_CONTAINER_PORT))/$(TEST_DB_MYSQL_NAME)?multiStatements=true"; \
		$(COMPOSE_CMD) exec backend ./migrate -path $(MYSQL_MIGRATE_PATH) -database "$$DB_URI" force $$version; \
	fi

# Apply SQL migrations directly via psql
db-migrate-sql:
	@printf "📄 Applying SQL migrations directly...\n"
	@for f in $(PG_MIGRATIONS_DIR)/*.up.sql; do \
		echo "  Running $$(basename $$f)..."; \
		$(COMPOSE_CMD) exec -T postgres psql -U $(DB_USER) -d $(DB_NAME) -f - < "$$f" 2>&1 | grep -E "(CREATE|ALTER|INSERT|ERROR)" | head -3 || true; \
	done
	@printf "✅ SQL migrations applied\n"

db-migrate-sql-test:
	@$(MAKE) test-db-up >/dev/null 2>&1 || true
	@if [ "$(TEST_DB_DRIVER)" = "postgres" ]; then \
		printf "📄 Applying SQL migrations directly to test database...\n"; \
		for f in $(PG_MIGRATIONS_DIR)/*.up.sql; do \
			echo "  Running $$(basename $$f)..."; \
			$(COMPOSE_CMD) -f docker-compose.yml -f docker-compose.testdb.yml exec -T postgres-test psql -U $(TEST_DB_POSTGRES_USER) -d $(TEST_DB_POSTGRES_NAME) -f - < "$$f" 2>&1 | grep -E "(CREATE|ALTER|INSERT|ERROR)" | head -3 || true; \
		done; \
		printf "✅ Test SQL migrations applied\n"; \
	else \
		printf "ℹ️  SQL migration replay only applies to PostgreSQL test databases\n"; \
	fi
# OTRS Migration Tools
# Analyze OTRS SQL dump file
migrate-analyze:
	@$(MAKE) toolbox-build
	@if [ -z "$(SQL)" ]; then \
		echo "❌ SQL file required. Usage: make migrate-analyze SQL=/path/to/dump.sql"; \
		exit 1; \
	fi
	@printf "🔍 Analyzing OTRS SQL dump: $(SQL)\n"
	@$(CONTAINER_CMD) run --rm \
		-v "$$(dirname $(SQL)):/data:ro" \
		-u "$$(id -u):$$(id -g)" \
		gotrs-toolbox:latest \
		gotrs-migrate -cmd=analyze -sql="/data/$$(basename $(SQL))"

# Import OTRS data (dry run by default)
migrate-import:
	@$(MAKE) toolbox-build
	@if [ -z "$(SQL)" ]; then \
		echo "❌ SQL file required. Usage: make migrate-import SQL=/path/to/dump.sql [DRY_RUN=false]"; \
		exit 1; \
	fi
	@printf "📥 Importing OTRS data from: $(SQL)\n"
	@DRY_RUN_FLAG=""; \
	if [ "$${DRY_RUN:-true}" = "true" ]; then \
		DRY_RUN_FLAG="-dry-run"; \
		echo "🧪 Running in DRY RUN mode (no data will be imported)"; \
	fi; \
	$(CONTAINER_CMD) run --rm \
		-v "$$(dirname $(SQL)):/data:ro" \
		-u "$$(id -u):$$(id -g)" \
		--network gotrs-ce_gotrs-network \
		gotrs-toolbox:latest \
		gotrs-migrate -cmd=import -sql="/data/$$(basename $(SQL))" \
			-db="postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" \
			$$DRY_RUN_FLAG -v

# Force import - clears existing data before importing (DESTRUCTIVE!)
migrate-import-force:
	@$(MAKE) toolbox-build
	@if [ -z "$(SQL)" ]; then \
		echo "❌ SQL file required. Usage: make migrate-import-force SQL=/path/to/dump.sql"; \
		exit 1; \
	fi
	@printf "⚠️  WARNING: Force import will CLEAR ALL EXISTING DATA!\n"
	@printf "📥 Importing OTRS data from: $(SQL)\n"
	@$(CONTAINER_CMD) run --rm \
		-v "$$(dirname $(SQL)):/data:ro" \
		-u "$$(id -u):$$(id -g)" \
		--network gotrs-ce_gotrs-network \
		gotrs-toolbox:latest \
		gotrs-migrate -cmd=import -sql="/data/$$(basename $(SQL))" \
			-db="postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" \
			-force -v || true
	@printf "✅ Force import completed successfully!\n"
# Validate imported OTRS data
migrate-validate:
	@$(MAKE) toolbox-build
	@printf "🔍 Validating imported OTRS data\n"
	@$(CONTAINER_CMD) run --rm \
		-u "$$(id -u):$$(id -g)" \
		--network gotrs-ce_gotrs-network \
		gotrs-toolbox:latest \
		gotrs-migrate -cmd=validate \
			-db="postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" -v

.PHONY: otrs-import
otrs-import:
	@$(MAKE) toolbox-build
	@if [ -z "$(SQL)" ]; then \
		echo "❌ SQL file required. Usage: make otrs-import SQL=/path/to/otrs_dump.sql"; \
		exit 1; \
	fi
	@printf "📥 Importing OTRS dump (driver: $(DB_DRIVER)) from: $(SQL)\n"
	@if [ "$(DB_DRIVER)" = "postgres" ]; then \
		$(CONTAINER_CMD) run --rm \
			--security-opt label=disable \
			-v "$(dir $(abspath $(SQL))):/data:ro" \
			-u "$(shell id -u):$(shell id -g)" \
			--network gotrs-ce_gotrs-network \
			gotrs-toolbox:latest \
			gotrs-migrate -cmd=import -sql="/data/$(notdir $(SQL))" \
				-db="postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" \
				$${DRY_RUN:+-dry-run} $${FORCE:+-force} -v; \
	else \
		printf "🧹 Preparing MariaDB schema (dropping all tables in $(DB_NAME))...\n"; \
		$(CONTAINER_CMD) run --rm \
			--security-opt label=disable \
			--network gotrs-ce_gotrs-network \
			gotrs-toolbox:latest \
			bash -lc 'mysql -h"$(DB_HOST)" -u"$(DB_USER)" -p"$(DB_PASSWORD)" -D"$(DB_NAME)" -e '\''\
SET SESSION group_concat_max_len = 1000000;\
SET FOREIGN_KEY_CHECKS=0;\
SELECT CONCAT("DROP TABLE IF EXISTS ", GROUP_CONCAT(CONCAT("`", table_name, "`") SEPARATOR ", ")) INTO @sql\
  FROM information_schema.tables WHERE table_schema = "$(DB_NAME)";\
SET @sql = IFNULL(@sql, "SELECT 1");\
PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;\
SET FOREIGN_KEY_CHECKS=1;\
'\'''; \
		printf "📦 Loading dump into MariaDB...\n"; \
		$(CONTAINER_CMD) run --rm \
			--security-opt label=disable \
			-v "$(dir $(abspath $(SQL))):/data:ro" \
			--network gotrs-ce_gotrs-network \
			gotrs-toolbox:latest \
			bash -lc 'mysql -h"$(DB_HOST)" -u"$(DB_USER)" -p"$(DB_PASSWORD)" "$(DB_NAME)" < "/data/$(notdir $(SQL))"'; \
	fi
	@printf "✅ OTRS dump import completed\n"

# Import test data with proper ID mapping
import-test-data:
	@printf "📥 Building and importing test tickets with proper ID mapping...\n"
	@if [ "$(DB_DRIVER)" != "postgres" ]; then \
		echo "❌ import-test-data currently supports Postgres only."; \
		echo "   Tip: DB_DRIVER=postgres make up && DB_DRIVER=postgres make import-test-data"; \
		exit 1; \
	fi
	@printf "🔨 Building import tool...\n"
	@mkdir -p bin
	@$(CONTAINER_CMD) run --rm \
		--security-opt label=disable \
		-v "$(pwd):/workspace" \
		-w /workspace \
		-e GOCACHE=/tmp/.cache/go-build \
		-e GOMODCACHE=/tmp/.cache/go-mod \
		-u "$(id -u):$(id -g)" \
		$(GO_IMAGE) \
		go build -o /workspace/bin/import-otrs ./cmd/import-otrs/main.go
	@printf "🗑️ Clearing existing data...\n"
	@$(COMPOSE_CMD) exec -T postgres psql -U $(DB_USER) -d $(DB_NAME) -c "TRUNCATE ticket CASCADE;" > /dev/null 2>&1
	@$(COMPOSE_CMD) exec -T postgres psql -U $(DB_USER) -d $(DB_NAME) -c "TRUNCATE article CASCADE;" > /dev/null 2>&1
	@printf "📦 Running import...\n"
	@$(CONTAINER_CMD) run --rm \
		--security-opt label=disable \
		-v "$(pwd)/bin:/bin:ro" \
		--network gotrs-ce_gotrs-network \
		alpine:3.19 \
		/bin/import-otrs -db="postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable"
	@printf "✅ Test data imported successfully with correct article mappings!\n"
# Reset user password and enable account (using toolbox)
reset-password:
	@$(MAKE) toolbox-build
	@mkdir -p tmp
	@tmp_env=$$(mktemp tmp/db-scope.XXXXXX); \
	trap 'rm -f "$$tmp_env"' EXIT; \
	DB_SCOPE=$${DB_SCOPE:-primary} ./scripts/db-scope-env.sh describe; \
	DB_SCOPE=$${DB_SCOPE:-primary} ./scripts/db-scope-env.sh print > "$$tmp_env"; \
	. "$$tmp_env"; \
	rm -f "$$tmp_env"; \
	trap - EXIT; \
	if [ "$$DB_CONN_SCOPE" = "pg-test" ]; then \
		APP_ENV=test \
		TEST_DB_NAME="$$DB_CONN_NAME" \
		TEST_DB_USER="$$DB_CONN_USER" \
		TEST_DB_PASSWORD="$$DB_CONN_PASSWORD" \
		TEST_DB_PORT="$$DB_CONN_HOST_PORT" \
		$(COMPOSE_CMD) -f docker-compose.yml -f docker-compose.testdb.yml up -d postgres-test >/dev/null 2>&1 || true; \
		echo "⏳ Waiting for postgres-test to accept connections..."; \
		for attempt in $$(seq 1 60); do \
			if TEST_DB_USER="$$DB_CONN_USER" TEST_DB_PASSWORD="$$DB_CONN_PASSWORD" $(COMPOSE_CMD) -f docker-compose.yml -f docker-compose.testdb.yml exec -T postgres-test env PGPASSWORD="$$DB_CONN_PASSWORD" pg_isready -h localhost -p 5432 -U "$$DB_CONN_USER" >/dev/null 2>&1; then \
				echo "✅ postgres-test is ready"; \
				break; \
			fi; \
			sleep 1; \
			if [ $$attempt -eq 60 ]; then \
				echo "❌ postgres-test did not become ready in time"; \
				exit 1; \
			fi; \
		done; \
	elif [ "$$DB_CONN_SCOPE" = "mysql-test" ]; then \
		APP_ENV=test \
		TEST_DB_MYSQL_NAME="$$DB_CONN_NAME" \
		TEST_DB_MYSQL_USER="$$DB_CONN_USER" \
		TEST_DB_MYSQL_PASSWORD="$$DB_CONN_PASSWORD" \
		TEST_DB_MYSQL_PORT="$$DB_CONN_HOST_PORT" \
		$(COMPOSE_CMD) -f docker-compose.yml -f docker-compose.testdb.yml up -d mariadb-test >/dev/null 2>&1 || true; \
		echo "⏳ Waiting for mariadb-test to accept connections..."; \
		for attempt in $$(seq 1 60); do \
			if $(COMPOSE_CMD) -f docker-compose.yml -f docker-compose.testdb.yml exec -T mariadb-test mariadb-admin --ssl=0 ping -h 127.0.0.1 -P 3306 -u "$$DB_CONN_USER" -p"$$DB_CONN_PASSWORD" >/dev/null 2>&1; then \
				echo "✅ mariadb-test is ready"; \
				break; \
			fi; \
			sleep 1; \
			if [ $$attempt -eq 60 ]; then \
				echo "❌ mariadb-test did not become ready in time"; \
				exit 1; \
			fi; \
		done; \
	fi; \
	echo -n "Username: "; \
	read username; \
	echo -n "New password: "; \
	stty -echo; read password; stty echo; \
	echo ""; \
	echo "🔑 Resetting password for user: $$username"; \
	DB_DRIVER="$$DB_CONN_DRIVER" \
	DB_CONN_DRIVER="$$DB_CONN_DRIVER" \
	DB_HOST="$$DB_CONN_HOST" \
	DB_PORT="$$DB_CONN_PORT" \
	DB_NAME="$$DB_CONN_NAME" \
	DB_USER="$$DB_CONN_USER" \
	DB_PASSWORD="$$DB_CONN_PASSWORD" \
	DB_CONN_HOST="$$DB_CONN_HOST" \
	DB_CONN_PORT="$$DB_CONN_PORT" \
	DB_CONN_NAME="$$DB_CONN_NAME" \
	DB_CONN_USER="$$DB_CONN_USER" \
	DB_CONN_PASSWORD="$$DB_CONN_PASSWORD" \
	DB_CONTAINER_NETWORK="gotrs-ce_gotrs-network" \
	TOOLBOX_IMAGE="gotrs-toolbox:latest" \
	./scripts/reset-user-password.sh "$$username" "$$password"; \
	status=$$?; \
	if [ $$status -ne 0 ]; then \
		echo "❌ Password reset failed (scope $$DB_CONN_SCOPE)"; \
		exit $$status; \
	fi; \
	if [ -n "$$DB_CONN_ADMIN_PASSWORD_VAR" ] && [ "$$username" = "$$DB_CONN_ADMIN_USER" ]; then \
		./scripts/env-set.sh "$$DB_CONN_ADMIN_PASSWORD_VAR" "$$password"; \
		echo "📝 Updated .env ($$DB_CONN_ADMIN_PASSWORD_VAR)"; \
		if [ "$$DB_CONN_SCOPE" = "pg-test" ] && [ "$$username" = "$$TEST_PG_ADMIN_USER" ]; then \
			./scripts/env-set.sh TEST_PASSWORD "$$password"; \
			echo "📝 Updated .env (TEST_PASSWORD)"; \
		fi; \
		if [ "$$DB_CONN_SCOPE" = "mysql-test" ] && [ "$$username" = "$$TEST_MYSQL_ADMIN_USER" ]; then \
			./scripts/env-set.sh TEST_PASSWORD "$$password"; \
			echo "📝 Updated .env (TEST_PASSWORD)"; \
		fi; \
	fi

.PHONY: test-pg-reset-password test-mysql-reset-password
test-pg-reset-password:
	@$(MAKE) DB_SCOPE=pg-test reset-password

test-mysql-reset-password:
	@$(MAKE) DB_SCOPE=mysql-test reset-password


# Valkey CLI

valkey-cli:
	$(COMPOSE_CMD) exec valkey valkey-cli

# i18n Tools (run via toolbox to ensure Go toolchain is available)
BF_FLAGS ?= -v
babelfish: toolbox-build
	@printf "Building gotrs-babelfish (toolbox)...\n"
	@$(CONTAINER_CMD) run --rm \
		--security-opt label=disable \
		-v "$$PWD:/workspace" \
		-w /workspace \
		-u "$$UID:$$GID" \
		-e GOCACHE=/cache/go-build \
		-e GOMODCACHE=/cache/go-mod \
		gotrs-toolbox:latest \
		bash -lc 'export PATH=/usr/local/go/bin:$$PATH && mkdir -p /workspace/tmp/bin && go build -buildvcs=false -o /workspace/tmp/bin/gotrs-babelfish cmd/gotrs-babelfish/main.go'
	@printf "✨ gotrs-babelfish built at tmp/bin/gotrs-babelfish\n"
	@printf "Run with: make babelfish-run ARGS='-help'\n"
babelfish-run: toolbox-build
	@$(CONTAINER_CMD) run --rm \
		--security-opt label=disable \
		-v "$$PWD:/workspace" \
		-w /workspace \
		-u "$$UID:$$GID" \
		-e GOCACHE=/cache/go-build \
		-e GOMODCACHE=/cache/go-mod \
		gotrs-toolbox:latest \
		bash -lc 'export PATH=/usr/local/go/bin:$$PATH && go run -buildvcs=false cmd/gotrs-babelfish/main.go $(ARGS)'

babelfish-coverage: toolbox-build
	@$(CONTAINER_CMD) run --rm \
		--security-opt label=disable \
		-v "$$PWD:/workspace" \
		-w /workspace \
		-u "$$UID:$$GID" \
		-e GOCACHE=/cache/go-build \
		-e GOMODCACHE=/cache/go-mod \
		gotrs-toolbox:latest \
		bash -lc 'export PATH=/usr/local/go/bin:$$PATH && go run -buildvcs=false cmd/gotrs-babelfish/main.go -action=coverage'

babelfish-validate: toolbox-build
	@$(CONTAINER_CMD) run --rm \
		--security-opt label=disable \
		-v "$$PWD:/workspace" \
		-w /workspace \
		-u "$$UID:$$GID" \
		-e GOCACHE=/cache/go-build \
		-e GOMODCACHE=/cache/go-mod \
		gotrs-toolbox:latest \
		bash -lc 'export PATH=/usr/local/go/bin:$$PATH && go run -buildvcs=false cmd/gotrs-babelfish/main.go -action=validate -lang=$(LANG) $(BF_FLAGS)'

babelfish-missing: toolbox-build
	@$(CONTAINER_CMD) run --rm \
		--security-opt label=disable \
		-v "$$PWD:/workspace" \
		-w /workspace \
		-u "$$UID:$$GID" \
		-e GOCACHE=/cache/go-build \
		-e GOMODCACHE=/cache/go-mod \
		gotrs-toolbox:latest \
		bash -lc 'export PATH=/usr/local/go/bin:$$PATH && go run -buildvcs=false cmd/gotrs-babelfish/main.go -action=missing -lang=$(LANG) $(BF_FLAGS)'

test-short:
	$(COMPOSE_CMD) exec -e DB_NAME=$${DB_NAME:-gotrs}_test -e APP_ENV=test backend go test -short ./...

test-coverage: toolbox-build
	@printf "Running test coverage analysis...\n"
	@printf "Using test database: $${DB_NAME:-gotrs}_test\n"
	@printf "📡 Ensuring database/cache services are available...\n"
	@$(COMPOSE_CMD) up -d mariadb valkey >/dev/null 2>&1 || true
	@mkdir -p generated
	@DB_NAME=$${DB_NAME:-gotrs}_test \
	APP_ENV=test \
	STORAGE_PATH=/tmp \
	TEMPLATES_DIR=/workspace/templates \
	DB_DRIVER=$(DB_DRIVER) \
	DB_HOST=$(DB_HOST) \
	DB_PORT=$(DB_PORT) \
	DB_USER=$(DB_USER) \
	DB_PASSWORD=$(DB_PASSWORD) \
	VALKEY_HOST=$(VALKEY_HOST) \
	VALKEY_PORT=$(VALKEY_PORT) \
	$(MAKE) toolbox-exec ARGS='bash scripts/run_coverage.sh'
	@$(MAKE) toolbox-exec ARGS='go tool cover -func=generated/coverage.out'

# Run tests with enhanced coverage reporting (runs in container if script missing)
test-report:
	@if [ -f ./scripts/run_tests.sh ]; then \
		bash ./scripts/run_tests.sh; \
	else \
		echo "run_tests.sh not found, running in container"; \
		$(MAKE) test-coverage; \
	fi

# Generate HTML coverage report (runs in container if script missing)
test-html:
	@if [ -f ./scripts/run_tests.sh ]; then \
		bash ./scripts/run_tests.sh --html; \
	else \
		echo "run_tests.sh not found, running in container"; \
		$(MAKE) test-coverage-html; \
	fi

# Test Actions dropdown functionality
test-actions-dropdown:
	@echo "🔍 Testing Actions dropdown components..."
	@./test_actions_dropdown.sh

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

test-db-up:
	@if [ -f ./scripts/test-db-safe.sh ]; then \
		SKIP_BACKEND_CHECK=1 DB_SAFE_ASSUME_YES=1 APP_ENV=test DB_DRIVER=$(TEST_DB_DRIVER) bash ./scripts/test-db-safe.sh up; \
	else \
		driver=$(TEST_DB_DRIVER); \
		if [ "$$driver" = "postgres" ]; then \
			echo "test-db-safe.sh not found, starting postgres-test via compose"; \
			APP_ENV=test $(COMPOSE_CMD) -f docker-compose.yml -f docker-compose.testdb.yml up -d postgres-test; \
		else \
			echo "test-db-safe.sh not found, starting mariadb-test via compose"; \
			APP_ENV=test $(COMPOSE_CMD) -f docker-compose.yml -f docker-compose.testdb.yml up -d mariadb-test; \
		fi; \
	fi

test-db-down:
	@if [ -f ./scripts/test-db-safe.sh ]; then \
		SKIP_BACKEND_CHECK=1 DB_SAFE_ASSUME_YES=1 APP_ENV=test DB_DRIVER=$(TEST_DB_DRIVER) bash ./scripts/test-db-safe.sh down; \
	else \
		driver=$(TEST_DB_DRIVER); \
		if [ "$$driver" = "postgres" ]; then \
			echo "test-db-safe.sh not found, stopping postgres-test via compose"; \
			$(COMPOSE_CMD) -f docker-compose.yml -f docker-compose.testdb.yml stop postgres-test >/dev/null 2>&1 || true; \
			$(COMPOSE_CMD) -f docker-compose.yml -f docker-compose.testdb.yml rm -f postgres-test >/dev/null 2>&1 || true; \
		else \
			echo "test-db-safe.sh not found, stopping mariadb-test via compose"; \
			$(COMPOSE_CMD) -f docker-compose.yml -f docker-compose.testdb.yml stop mariadb-test >/dev/null 2>&1 || true; \
			$(COMPOSE_CMD) -f docker-compose.yml -f docker-compose.testdb.yml rm -f mariadb-test >/dev/null 2>&1 || true; \
		fi; \
	fi

test-coverage-html: toolbox-build
	@printf "Running test coverage (HTML) analysis...\n"
	@printf "Using test database: $${DB_NAME:-gotrs}_test\n"
	@printf "📡 Ensuring database/cache services are available...\n"
	@$(COMPOSE_CMD) up -d mariadb valkey >/dev/null 2>&1 || true
	@mkdir -p generated
	@DB_NAME=$${DB_NAME:-gotrs}_test \
	APP_ENV=test \
	STORAGE_PATH=/tmp \
	TEMPLATES_DIR=/workspace/templates \
	DB_DRIVER=$(DB_DRIVER) \
	DB_HOST=$(DB_HOST) \
	DB_PORT=$(DB_PORT) \
	DB_USER=$(DB_USER) \
	DB_PASSWORD=$(DB_PASSWORD) \
	VALKEY_HOST=$(VALKEY_HOST) \
	VALKEY_PORT=$(VALKEY_PORT) \
	$(MAKE) toolbox-exec ARGS='bash scripts/run_coverage.sh'
	@$(MAKE) toolbox-exec ARGS='go tool cover -html=generated/coverage.out -o generated/coverage.html'
	@printf "Coverage report generated: generated/coverage.html\n"
# Frontend test commands
test-frontend:
	@printf "Running frontend tests...\n"	$(COMPOSE_CMD) exec frontend bun test

test-contracts: toolbox-build
	@printf "🔍 Running API contract tests...\n"
	@$(CONTAINER_CMD) run --rm \
		-v "$$(pwd):/workspace" \
		-w /workspace \
		-u "$$(id -u):$$(id -g)" \
		--network host \
		gotrs-toolbox:latest \
		go test -v ./internal/testing/contracts/...

test-all: test test-frontend test-contracts test-e2e-playwright
	@printf "All tests completed!\n"
# E2E Testing Commands
.PHONY: test-e2e-playwright test-e2e-playwright-watch test-e2e-playwright-debug test-e2e-playwright-report playwright-build

# Build Playwright test container
playwright-build:
	@printf "Building Playwright test container...\n"
	@$(COMPOSE_CMD) -f docker-compose.playwright.yml build playwright

.PHONY: test-e2e-playwright-go
test-e2e-playwright-go:
	@printf "\n🎭 Running Go Playwright-tagged e2e tests in dedicated container...\n"
	$(CONTAINER_CMD) build -f Dockerfile.playwright-go -t gotrs-playwright-go:latest . >/dev/null
	@# Ensure cache volume directories exist with correct ownership
	@HOST_UID=$$(id -u); HOST_GID=$$(id -g); \
	$(CONTAINER_CMD) run --rm -u 0:0 -e HOST_UID=$$HOST_UID -e HOST_GID=$$HOST_GID -v gotrs_cache:/cache alpine sh -c "mkdir -p /cache/xdg /cache/go-build /cache/go-mod /cache/ms-playwright && chown -R $$HOST_UID:$$HOST_GID /cache"
	# Prefer explicit BASE_URL provided on invocation; ignore .env for this target
	@if [ -n "$(BASE_URL)" ]; then echo "[playwright-go] (explicit) BASE_URL=$(BASE_URL)"; else echo "[playwright-go] (default) BASE_URL=$${BASE_URL:-http://localhost:8080}"; fi
	# Allow overriding network (e.g. PLAYWRIGHT_NETWORK=gotrs-ce_default) to access compose service DNS
	@if [ -n "$(PLAYWRIGHT_NETWORK)" ]; then echo "[playwright-go] Using network '$(PLAYWRIGHT_NETWORK)'"; else echo "[playwright-go] Using host network (override with PLAYWRIGHT_NETWORK=...)"; fi
	$(CONTAINER_CMD) run --rm \
		--security-opt label=disable \
		-u "$$(id -u):$$(id -g)" \
		-v "$$PWD:/workspace" \
		-v gotrs_cache:/cache \
		-w /workspace \
		$$( [ -n "$(PLAYWRIGHT_NETWORK)" ] && printf -- "--network $(PLAYWRIGHT_NETWORK)" || printf -- "--network host" ) \
		-e HOME=/workspace \
		-e BASE_URL=$(BASE_URL) \
		-e RAW_BASE_URL=$(BASE_URL) \
		-e TEST_USERNAME=$(TEST_USERNAME) \
		-e TEST_PASSWORD=$(TEST_PASSWORD) \
		-e DEMO_ADMIN_EMAIL=$(DEMO_ADMIN_EMAIL) \
		-e DEMO_ADMIN_PASSWORD=$(DEMO_ADMIN_PASSWORD) \
		-e PLAYWRIGHT_BROWSERS_PATH=/cache/ms-playwright \
		-e XDG_CACHE_HOME=/cache/xdg \
		-e GOCACHE=/cache/go-build \
		-e GOMODCACHE=/cache/go-mod \
		 gotrs-playwright-go:latest bash -lc "go test -tags e2e -v ./tests/e2e/playwright $${ARGS}"

.PHONY: test-e2e-go
test-e2e-go:
	@printf "\n🎭 Running Go E2E tests (Playwright) via dedicated container...\n"
	$(CONTAINER_CMD) build -f Dockerfile.playwright-go -t gotrs-playwright-go:latest . >/dev/null
	@if [ -n "$(BASE_URL)" ]; then echo "[e2e-go] (explicit) BASE_URL=$(BASE_URL)"; else echo "[e2e-go] (default) BASE_URL=$${BASE_URL:-http://localhost:8080}"; fi
	@TEST_PATTERN=$${TEST:-CustomerTicket}; echo "[e2e-go] Running pattern: $$TEST_PATTERN";
	$(CONTAINER_CMD) run --rm \
		--security-opt label=disable \
		-u "$$UID:$$GID" \
		-v "$$PWD:/workspace" \
		-v gotrs_cache:/cache \
		-w /workspace \
		--network host \
		-e HOME=/workspace \
		-e BASE_URL=$(BASE_URL) \
		-e RAW_BASE_URL=$(BASE_URL) \
		-e TEST_USERNAME=$(TEST_USERNAME) \
		-e TEST_PASSWORD=$(TEST_PASSWORD) \
		-e DEMO_ADMIN_EMAIL=$(DEMO_ADMIN_EMAIL) \
		-e DEMO_ADMIN_PASSWORD=$(DEMO_ADMIN_PASSWORD) \
		-e PLAYWRIGHT_BROWSERS_PATH=/cache/ms-playwright \
		-e XDG_CACHE_HOME=/cache/xdg \
		-e GOCACHE=/cache/go-build \
		-e GOMODCACHE=/cache/go-mod \
		 gotrs-playwright-go:latest bash -lc "go test -tags e2e -v ./tests/e2e -run \"$${TEST:-CustomerTicket}\""

PLAYWRIGHT_RESULTS_DIR ?= /tmp/playwright-results
PLAYWRIGHT_OUTPUT_DIR ?= /tmp/playwright-artifacts
PLAYWRIGHT_HTML_REPORT_DIR ?= /tmp/playwright-report

.PHONY: test-acceptance-playwright
test-acceptance-playwright: css-deps-stable playwright-build
	@$(MAKE) test-stack-up
	@printf "Running Playwright acceptance tests...\n"
	@$(COMPOSE_CMD) -f docker-compose.playwright.yml run --rm \
		-e HEADLESS=$${HEADLESS:-true} \
		-e BASE_URL=http://backend-test:8080 \
		-e PLAYWRIGHT_FALLBACK_BASE_URL=http://backend-test:8080 \
		-e PLAYWRIGHT_SKIP_WEBSERVER=1 \
		-e PWTEST_HTML_REPORT_OPEN=$${PWTEST_HTML_REPORT_OPEN:-never} \
		-e PLAYWRIGHT_RESULTS_DIR=$(PLAYWRIGHT_RESULTS_DIR) \
		-e PLAYWRIGHT_OUTPUT_DIR=$(PLAYWRIGHT_OUTPUT_DIR) \
		-e PLAYWRIGHT_HTML_REPORT_DIR=$(PLAYWRIGHT_HTML_REPORT_DIR) \
		playwright bash -lc "mkdir -p \"$${PLAYWRIGHT_RESULTS_DIR}\" \"$${PLAYWRIGHT_OUTPUT_DIR}\" \"$${PLAYWRIGHT_HTML_REPORT_DIR}\" && bunx playwright test $$([ -n "$(TEST)" ] && printf %s "$(TEST)" || printf %s "tests/acceptance/ticket-new-queue.spec.js") --project=$${PLAYWRIGHT_PROJECT:-chromium} --reporter=list"

# Run E2E tests
test-e2e-playwright: playwright-build
	@printf "Running E2E tests with Playwright...\n"
	@mkdir -p test-results/screenshots test-results/videos
	@$(COMPOSE_CMD) -f docker-compose.playwright.yml run --rm \
		-e HEADLESS=true \
		playwright

# Run E2E tests in watch mode (for development)
test-e2e-playwright-watch: playwright-build
	@printf "Running E2E tests in watch mode...\n"
	@mkdir -p test-results/screenshots test-results/videos
	@$(COMPOSE_CMD) -f docker-compose.playwright.yml run --rm \
		-e HEADLESS=false \
		-e SLOW_MO=100 \
		playwright go test -tags e2e ./tests/e2e/... -v -watch

# Check for untranslated keys in UI
check-translations:
	@printf "Checking for untranslated keys in UI...\n"
	@./scripts/check-translations.sh

# Run E2E tests with headed browser for debugging
test-e2e-playwright-debug: playwright-build
	@printf "Running E2E tests in debug mode (headed browser)...\n"
	@mkdir -p test-results/screenshots test-results/videos
	@$(COMPOSE_CMD) -f docker-compose.playwright.yml run --rm \
		-e HEADLESS=false \
		-e SLOW_MO=500 \
		-e SCREENSHOTS=true \
		-e VIDEOS=true \
		playwright go test -tags e2e ./tests/e2e/... -v

# Generate HTML test report
test-e2e-playwright-report:
	@printf "Generating E2E test report...\n"
	@if [ -d "test-results" ]; then \
		echo "Test results:"; \
		echo "Screenshots: $$(find test-results/screenshots -name "*.png" 2>/dev/null | wc -l) files"; \
		echo "Videos: $$(find test-results/videos -name "*.webm" 2>/dev/null | wc -l) files"; \
		ls -la test-results/ 2>/dev/null || true; \
	else \
		echo "No test results found. Run test-e2e-playwright first."; \
	fi

# Clean test results
clean-test-results:
	@printf "Cleaning test results...\n"
	@rm -rf test-results/

# Security scanning commands
.PHONY: scan-secrets scan-secrets-history scan-secrets-precommit scan-vulnerabilities security-scan

# Scan for secrets in current code
scan-secrets:
	@printf "Scanning for secrets and credentials...\n"
	@$(CONTAINER_CMD) run --rm \
		-v "$$(pwd):/workspace" \
		-w /workspace \
		zricethezav/gitleaks:latest \
		detect --source . --verbose

# Scan entire git history for secrets
scan-secrets-history:
	@printf "Scanning git history for secrets...\n"
	@$(CONTAINER_CMD) run --rm \
		-v "$$(pwd):/workspace" \
		-w /workspace \
		zricethezav/gitleaks:latest \
		detect --source . --log-opts="--all" --verbose

# Install pre-commit hooks for secret scanning (using bash script)
scan-secrets-precommit:
	@bash scripts/install-git-hooks.sh

# Scan for vulnerabilities (Go dependencies + container/config)
scan-vulnerabilities: toolbox-build
	@printf "🔍 Scanning Go dependencies for vulnerabilities...\n"
	@$(MAKE) toolbox-exec ARGS="govulncheck -show verbose ./..."
	@printf "🔍 Scanning container/config for vulnerabilities...\n"
	@$(CONTAINER_CMD) run --rm \
		-v "$$(pwd):/workspace" \
		-v gotrs_cache:/cache \
		-e TRIVY_CACHE_DIR=/cache/trivy \
		-w /workspace \
		aquasec/trivy:latest \
		fs --scanners vuln,secret,misconfig . \
		--skip-dirs .cache \
		--ignorefile .trivyignore \
		--severity HIGH,CRITICAL

# Run all security scans
security-scan: scan-secrets scan-vulnerabilities
	@printf "Security scanning completed!\n"

.PHONY: build-artifacts
build-artifacts:
	@printf "🎯 Building backend artifacts image...\n"
	@$(CONTAINER_CMD) build --build-arg GO_IMAGE=$(GO_IMAGE) $(VERSION_BUILD_ARGS) --target artifacts -t gotrs-artifacts:latest .
# Build for production (Dockerfile handles CSS/JS via frontend stage)
build: build-artifacts pre-build helm-package
	@printf "🔨 Building backend container ($(VERSION) @ $(GIT_COMMIT))...\n" \
		&& $(CONTAINER_CMD) build --build-arg GO_IMAGE=$(GO_IMAGE) $(VERSION_BUILD_ARGS) -f Dockerfile -t gotrs:latest .
	@printf "🧹 Cleaning host binaries...\n"
	@rm -f goats gotrs gotrs-* generator migrate server  # Clean root directory
	@rm -f bin/* 2>/dev/null || true  # Clean bin directory
	@printf "✅ Build complete - CSS, JS, Helm chart compiled, containers ready\n"

.PHONY: pre-build generate-route-map validate-routes

# Pre-build chain: ensure API map + static route audit executed every build
pre-build: generate-route-map validate-routes

generate-route-map:
	@printf "📡 Generating API map artifacts...\n"
	@mkdir -p generated/api-map
	@$(MAKE) toolbox-exec ARGS='sh scripts/api_map.sh'
	@printf "   API map complete (generated/api-map/api-map.*)\n"

validate-routes:
	@printf "🔍 Validating no new static routes...\n"
	@$(MAKE) toolbox-exec ARGS='sh scripts/validate_routes.sh' || { echo "Route validation failed"; exit 1; }
	@printf "   Route validation passed\n"
# ============================================
# Enhanced Build Targets with BuildKit
# ============================================

# Enable BuildKit for better caching
# Enable BuildKit for Docker only
ifeq ($(findstring docker,$(CONTAINER_CMD)),docker)
export DOCKER_BUILDKIT=1
export COMPOSE_DOCKER_CLI_BUILD=1
endif

# Build with caching (70% faster rebuilds)
build-cached: build-artifacts
	@printf "🚀 Building backend image ($(VERSION) @ $(GIT_COMMIT))...\n"
	@$(CONTAINER_CMD) build --build-arg GO_IMAGE=$(GO_IMAGE) $(VERSION_BUILD_ARGS) -t gotrs:latest .
	@printf "✅ Build complete\n"
# Security scan build (CI/CD)
build-secure: build-artifacts
	@printf "🔒 Building with security scanning...\n"
	@$(CONTAINER_CMD) build --build-arg GO_IMAGE=$(GO_IMAGE) $(VERSION_BUILD_ARGS) \
		--target security \
		--output type=local,dest=./security-reports \
		.
	@printf "📊 Security reports saved to ./security-reports/\n"
# Multi-platform build (AMD64 and ARM64)
build-multi: build-artifacts
	@printf "🌍 Building for multiple platforms...\n"
	@$(CONTAINER_CMD) buildx build --build-arg GO_IMAGE=$(GO_IMAGE) $(VERSION_BUILD_ARGS) \
		--platform linux/amd64,linux/arm64 \
		-t gotrs:latest .
	@printf "✅ Multi-platform build complete\n"
# Analyze image size with dive
analyze-size:
	@printf "📏 Analyzing Docker image size...\n"
	@if command -v dive > /dev/null 2>&1; then \
		dive gotrs:latest; \
	else \
		$(CONTAINER_CMD) run --rm -it \
			-v /var/run/docker.sock:/var/run/docker.sock \
			wagoodman/dive:latest gotrs:latest; \
	fi

# Build without cache (clean build)
build-clean: build-artifacts
	@printf "🧹 Clean build without cache ($(VERSION) @ $(GIT_COMMIT))...\n"
	@$(CONTAINER_CMD) build --build-arg GO_IMAGE=$(GO_IMAGE) $(VERSION_BUILD_ARGS) --no-cache -t gotrs:latest .
	@printf "✅ Clean build complete\n"
# Show build cache usage
show-cache:
	@printf "💾 Docker build cache usage:\n"
	@$(CONTAINER_CMD) system df --verbose | grep -A 10 "Build Cache" || \
		$(CONTAINER_CMD) buildx du --verbose 2>/dev/null || \
		echo "Build cache info not available"

# Clear build cache
clear-cache:
	@printf "🗑️ Clearing Docker build cache...\n"
	@$(CONTAINER_CMD) builder prune -f
	@printf "✅ Build cache cleared\n"
# Build specialized containers
build-all-tools: build-cached toolbox-build
	@printf "🛠️ Building all specialized tool containers...\n"
	@$(CONTAINER_CMD) build \
		--build-arg GO_IMAGE=$(GO_IMAGE) \
		--cache-from gotrs-tests:latest \
		--build-arg BUILDKIT_INLINE_CACHE=1 \
		-f Dockerfile.tests -t gotrs-tests:latest .
	@$(CONTAINER_CMD) build \
		--build-arg GO_IMAGE=$(GO_IMAGE) \
		--cache-from gotrs-route-tools:latest \
		--build-arg BUILDKIT_INLINE_CACHE=1 \
		-f Dockerfile.route-tools -t gotrs-route-tools:latest .
	@$(CONTAINER_CMD) build \
		--build-arg GO_IMAGE=$(GO_IMAGE) \
		--cache-from gotrs-goatkit:latest \
		--build-arg BUILDKIT_INLINE_CACHE=1 \
		-f Dockerfile.goatkit -t gotrs-goatkit:latest .
	@$(CONTAINER_CMD) build \
		--build-arg GO_IMAGE=$(GO_IMAGE) \
		--cache-from gotrs-config-manager:latest \
		--build-arg BUILDKIT_INLINE_CACHE=1 \
		-f Dockerfile.config-manager -t gotrs-config-manager:latest .
	@printf "✅ All tool containers built successfully\n"
# Show image sizes
show-sizes:
	@printf "📏 Docker image sizes:\n"
	@$(CONTAINER_CMD) images --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}" | grep -E "(REPOSITORY|gotrs)" | column -t

# Check service health (runs in container)
health:
	@printf "Checking service health...\n"
	@$(CONTAINER_CMD) run --rm --network=host alpine/curl:latest -f http://localhost/health || echo "Backend not healthy"
	@$(CONTAINER_CMD) run --rm --network=host alpine/curl:latest -f http://localhost/ || echo "Frontend not healthy"

# Open services in browser
open:
	@printf "Opening services...\n"
	@open http://localhost || xdg-open http://localhost || echo "Open http://localhost"

open-mail:
	@open http://localhost:8025 || xdg-open http://localhost:8025 || echo "Open http://localhost:8025"

open-db:
	@open http://localhost:8090 || xdg-open http://localhost:8090 || echo "Open http://localhost:8090"

# Development shortcuts
dev: up

stop: down

reset: clean setup up-d
	@printf "Environment reset and restarted\n"
# Show running services
ps:
	$(COMPOSE_CMD) ps

# Execute commands in containers
exec-backend:
	$(COMPOSE_CMD) exec backend sh

exec-backend-run:
	$(COMPOSE_CMD) exec backend $(if $(ARGS),$(ARGS),echo "No command specified - use ARGS='command'")

exec-frontend:
	$(COMPOSE_CMD) exec frontend sh

# Podman-specific: Generate systemd units
podman-systemd:
	@printf "Generating systemd units for podman...\n"
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
	touch $(ACTIVE_MIGRATIONS_DIR)/$$timestamp\_$$name.up.sql; \
	touch $(ACTIVE_MIGRATIONS_DIR)/$$timestamp\_$$name.down.sql; \
	echo "-- Migration: $$name" > $(ACTIVE_MIGRATIONS_DIR)/$$timestamp\_$$name.up.sql; \
	echo "" >> $(ACTIVE_MIGRATIONS_DIR)/$$timestamp\_$$name.up.sql; \
	echo "-- Rollback: $$name" > $(ACTIVE_MIGRATIONS_DIR)/$$timestamp\_$$name.down.sql; \
	echo "" >> $(ACTIVE_MIGRATIONS_DIR)/$$timestamp\_$$name.down.sql; \
	echo "Created migration files:"; \
	echo "  $(ACTIVE_MIGRATIONS_DIR)/$$timestamp\_$$name.up.sql"; \
	echo "  $(ACTIVE_MIGRATIONS_DIR)/$$timestamp\_$$name.down.sql"

# LDAP testing and administration commands
.PHONY: test-ldap test-ldap-perf ldap-admin ldap-logs ldap-setup ldap-test-user

# Run LDAP integration tests
test-ldap:
	@printf "Running LDAP integration tests...\n"
	@printf "Starting LDAP server if not running...\n"	$(COMPOSE_CMD) up -d openldap
	@printf "Waiting for LDAP server to be ready...\n"
	@sleep 30
	@printf "Running integration tests...\n"	$(COMPOSE_CMD) exec -e LDAP_INTEGRATION_TESTS=true -e LDAP_HOST=openldap backend go test -v ./internal/service -run TestLDAPIntegration

# Run LDAP performance benchmarks
test-ldap-perf:
	@printf "Running LDAP performance benchmarks...\n"	$(COMPOSE_CMD) up -d openldap
	@printf "Waiting for LDAP server...\n"
	@sleep 30
	$(COMPOSE_CMD) exec -e LDAP_INTEGRATION_TESTS=true -e LDAP_HOST=openldap backend go test -v ./internal/service -bench=BenchmarkLDAP -run=^$$

# Open phpLDAPadmin in browser
ldap-admin:
	@printf "Starting phpLDAPadmin...\n"
	$(COMPOSE_CMD) --profile tools up -d phpldapadmin
	@printf "Opening phpLDAPadmin at http://localhost:8091\n"
	@printf "Login with:\n"
	@printf "  Login DN: cn=admin,dc=gotrs,dc=local\n"
	@printf "  Password: (LDAP_ADMIN_PASSWORD from .env)\n"
	@open http://localhost:8091 || xdg-open http://localhost:8091 || echo "Open http://localhost:8091"

# View OpenLDAP logs
ldap-logs:
	$(COMPOSE_CMD) logs -f openldap

# Setup LDAP for development (start services and wait)
ldap-setup:
	@if [ -z "$${LDAP_ADMIN_PASSWORD}" ]; then echo "ERROR: LDAP_ADMIN_PASSWORD must be set in .env"; exit 1; fi
	@printf "Setting up LDAP development environment...\n"
	$(COMPOSE_CMD) up -d openldap
	@printf "Waiting for LDAP server to initialize (this may take up to 60 seconds)...\n"
	@timeout=60; \
	while [ $$timeout -gt 0 ]; do \
		if $(COMPOSE_CMD) exec openldap ldapsearch -x -H ldap://localhost -b "dc=gotrs,dc=local" -D "cn=admin,dc=gotrs,dc=local" -w "$${LDAP_ADMIN_PASSWORD}" "(objectclass=*)" dn > /dev/null 2>&1; then \
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
	@printf "\n"
	@printf "LDAP Server Configuration:\n"
	@printf "=========================\n"
	@printf "Host: localhost:389\n"
	@printf "Base DN: dc=gotrs,dc=local\n"
	@printf "Admin DN: cn=admin,dc=gotrs,dc=local\n"
	@printf "Admin Password: (LDAP_ADMIN_PASSWORD from .env)\n"
	@printf "Readonly DN: cn=readonly,dc=gotrs,dc=local\n"
	@printf "Readonly Password: (LDAP_READONLY_PASSWORD from .env)\n"
	@printf "\n"
	@printf "Test Users (password from LDAP_TEST_USER_PASSWORD):\n"
	@printf "===================================\n"
	@printf "jadmin     - john.admin@gotrs.local (System Administrator)\n"
	@printf "smitchell  - sarah.mitchell@gotrs.local (IT Manager)\n"
	@printf "mwilson    - mike.wilson@gotrs.local (Senior Support Agent)\n"
	@printf "lchen      - lisa.chen@gotrs.local (Support Agent)\n"
	@printf "djohnson   - david.johnson@gotrs.local (Junior Support Agent)\n"
	@printf "\n"
	@printf "Web Interface:\n"
	@printf "==============\n"
	@printf "phpLDAPadmin: http://localhost:8091 (run 'make ldap-admin')\n"
# Test LDAP authentication with a specific user
ldap-test-user:
	@if [ -z "$${LDAP_READONLY_PASSWORD}" ]; then echo "ERROR: LDAP_READONLY_PASSWORD must be set in .env"; exit 1; fi
	@echo -n "Username to test: "; \
	read username; \
	echo "Testing LDAP authentication for user: $$username"; \
	$(COMPOSE_CMD) exec openldap ldapsearch -x -H ldap://localhost \
		-D "cn=readonly,dc=gotrs,dc=local" -w "$${LDAP_READONLY_PASSWORD}" \
		-b "ou=Users,dc=gotrs,dc=local" \
		"(&(objectClass=inetOrgPerson)(uid=$$username))" \
		uid mail displayName telephoneNumber departmentNumber title

# Quick LDAP connectivity test
ldap-test:
	@if [ -z "$${LDAP_ADMIN_PASSWORD}" ]; then echo "ERROR: LDAP_ADMIN_PASSWORD must be set in .env"; exit 1; fi
	@printf "Testing LDAP connectivity...\n"
	$(COMPOSE_CMD) exec openldap ldapsearch -x -H ldap://localhost \
		-D "cn=admin,dc=gotrs,dc=local" -w "$${LDAP_ADMIN_PASSWORD}" \
		-b "dc=gotrs,dc=local" \
		"(objectclass=*)" dn | head -20

# Test that all Makefile commands are properly containerized
.PHONY: test-containerized
test-containerized:
	@bash scripts/test-containerized.sh

# CSS Build Commands
.PHONY: bun-updates css-deps css-build css-watch browserslist-update browserslist-update-one

BROWSERSLIST_DIRS ?= . web sdk/typescript
BROWSERSLIST_LOCKFILES ?= package-lock.json bun.lockb

browserslist-update-one:
	@if [ -z "$(DIR)" ]; then \
		echo "DIR is required"; \
		exit 1; \
	fi
	@if ! [ -f "$(DIR)/package-lock.json" ] && ! [ -f "$(DIR)/bun.lockb" ]; then \
		printf "ℹ️  Skipping %s (no lockfile)\n" "$(DIR)"; \
	else \
		printf "🌐 Updating Browserslist data (%s)…\n" "$(DIR)"; \
		if echo "$(COMPOSE_CMD)" | grep -q "podman-compose"; then \
			COMPOSE_PROFILES=toolbox $(COMPOSE_CMD) run --rm toolbox sh -c 'cd /workspace/$(DIR) && bunx update-browserslist-db@latest'; \
		else \
			$(COMPOSE_CMD) --profile toolbox run --rm toolbox sh -c 'cd /workspace/$(DIR) && bunx update-browserslist-db@latest'; \
		fi; \
		printf "✅ Browserslist data refreshed (%s)\n" "$(DIR)"; \
	fi

browserslist-update:
	@if [ -n "$(DIR)" ]; then \
		$(MAKE) browserslist-update-one DIR=$(DIR); \
	else \
		for dir in $(BROWSERSLIST_DIRS); do \
			if [ -d "$$dir" ] && [ -f "$$dir/package.json" ]; then \
				$(MAKE) browserslist-update-one DIR=$$dir; \
			else \
				printf "ℹ️  Skipping %s (no package.json)\n" "$$dir"; \
			fi; \
		done; \
	fi

bun-updates:
	@printf "📦 Updating Bun dependencies...\n"
	@if echo "$(COMPOSE_CMD)" | grep -q "podman-compose"; then \
		COMPOSE_PROFILES=toolbox $(COMPOSE_CMD) run --rm toolbox sh -c 'cd /workspace && bunx npm-check-updates -u && bun install'; \
	else \
		$(COMPOSE_CMD) --profile toolbox run --rm toolbox sh -c 'cd /workspace && bunx npm-check-updates -u && bun install'; \
	fi
	@printf "✅ Bun dependencies updated\n"
# Install CSS build dependencies (in container with user permissions)
css-deps:
	@printf "📦 Installing CSS build dependencies...\n"
	@if echo "$(COMPOSE_CMD)" | grep -q "podman-compose"; then \
		COMPOSE_PROFILES=toolbox $(COMPOSE_CMD) run --rm toolbox sh -c 'cd /workspace && bun install && touch /cache/.frontend_deps_installed || true'; \
	else \
		$(COMPOSE_CMD) --profile toolbox run --rm toolbox sh -c 'cd /workspace && bun install && touch /cache/.frontend_deps_installed || true'; \
	fi
	@printf "✅ CSS dependencies installed\n"
# Install CSS dependencies without upgrading (preserves pinned versions)
css-deps-stable:
	@printf "📦 Installing CSS build dependencies (stable versions)...\n"
	@# Pre-create node_modules with correct ownership before Docker mounts volume
	@[ -d node_modules ] || mkdir -p node_modules
	@if echo "$(COMPOSE_CMD)" | grep -q "podman-compose"; then \
		COMPOSE_PROFILES=toolbox $(COMPOSE_CMD) run --rm toolbox sh -c 'cd /workspace && if [ ! -d node_modules/tailwindcss ]; then echo "🧹 Cleaning existing node_modules (fresh install)"; rm -rf node_modules 2>/dev/null || true; fi; bun install && touch /cache/.frontend_deps_installed || true'; \
	else \
		$(COMPOSE_CMD) --profile toolbox run --rm toolbox sh -c 'cd /workspace && if [ ! -d node_modules/tailwindcss ]; then echo "🧹 Cleaning existing node_modules (fresh install)"; rm -rf node_modules 2>/dev/null || true; fi; bun install && touch /cache/.frontend_deps_installed || true'; \
	fi
	@printf "✅ CSS dependencies installed\n"
# Build production CSS (in container with user permissions) - ensure deps first
css-build: css-deps-stable
	@printf "🎨 Building production CSS...\n"
	@if echo "$(COMPOSE_CMD)" | grep -q "podman-compose"; then \
		COMPOSE_PROFILES=toolbox $(COMPOSE_CMD) run --rm toolbox sh -c 'cd /workspace && bun run build-css'; \
	else \
		$(COMPOSE_CMD) --profile toolbox run --rm toolbox sh -c 'cd /workspace && bun run build-css'; \
	fi
	@printf "✅ CSS built to static/css/output.css\n"
js-build: css-deps-stable
	@printf "🔨 Building JavaScript bundles (rootless)...\n"
	@[ -d static/js ] || mkdir -p static/js
	@# Probe write as container user (touch). If fails, repair ownership.
	@if echo "$(COMPOSE_CMD)" | grep -q "podman-compose"; then \
		COMPOSE_PROFILES=toolbox $(COMPOSE_CMD) run --rm toolbox sh -c 'cd /workspace && touch static/js/.permcheck 2>/dev/null || exit 23'; status=$$?; \
	else \
		$(COMPOSE_CMD) --profile toolbox run --rm toolbox sh -c 'cd /workspace && touch static/js/.permcheck 2>/dev/null || exit 23'; status=$$?; \
	fi; \
	if [ $$status -eq 23 ]; then \
		printf "⚠️  static/js not writable by uid 1000 – fixing...\n"; \
		$(MAKE) frontend-fix-js-dir; \
	else \
		rm -f static/js/.permcheck || true; \
	fi
	@# Run build (produces static/js/tiptap.min.js via esbuild --outfile)
	@if echo "$(COMPOSE_CMD)" | grep -q "podman-compose"; then \
		COMPOSE_PROFILES=toolbox $(COMPOSE_CMD) run --rm toolbox sh -c 'cd /workspace && bun run build-tiptap'; \
	else \
		$(COMPOSE_CMD) --profile toolbox run --rm toolbox sh -c 'cd /workspace && bun run build-tiptap'; \
	fi
	@# Validate artifact exists and non-empty; show size
	@if [ ! -s static/js/tiptap.min.js ]; then \
		echo "❌ Build failed: tiptap.min.js missing or empty"; exit 1; \
	fi
	@ls -lh static/js/tiptap.min.js | awk '{print "✅ JavaScript built:" $$9 " (" $$5 ")"}'

.PHONY: frontend-fix-js-dir
frontend-fix-js-dir:
	@printf "🩹 Fixing static/js ownership inside container (one-time)...\n"
	@$(CONTAINER_CMD) run --rm -v "$$PWD:/workspace:Z" -w /workspace --user 0 alpine:3.19 sh -c 'chown -R 1000:1000 static/js'
	@printf "✅ static/js now owned by uid 1000 (container view)\n"
# Build all frontend assets
frontend-build: css-build js-build
	@printf "✅ All frontend assets built\n"

.PHONY: frontend-clean-cache
frontend-clean-cache:
	@printf "🧹 Removing node_modules and frontend marker...\n"
	@rm -rf node_modules 2>/dev/null || true
	@if echo "$(COMPOSE_CMD)" | grep -q "podman-compose"; then \
		COMPOSE_PROFILES=toolbox $(COMPOSE_CMD) run --rm toolbox sh -c 'rm -f /cache/.frontend_deps_installed 2>/dev/null || true'; \
	else \
		$(COMPOSE_CMD) --profile toolbox run --rm toolbox sh -c 'rm -f /cache/.frontend_deps_installed 2>/dev/null || true'; \
	fi
	@printf "✅ node_modules removed. Next build will reinstall dependencies.\n"

.PHONY: frontend-reset-deps
frontend-reset-deps: frontend-clean-cache
	@printf "🔄 Forcing fresh dependency install next build\\n"

.PHONY: frontend-perms-fix
frontend-perms-fix:
	@printf "🔧 Normalizing frontend file permissions...\n"
	@find static/js -maxdepth 1 -type f -name 'tiptap.min.js' -exec $(CONTAINER_CMD) run --rm -v "$$PWD:/workspace" -w /workspace --user 0 alpine:3.19 sh -c 'chown 1000:1000 /workspace/{} || true' \;
	@chmod ug+rw static/js 2>/dev/null || true
	@printf "✅ Permissions normalized (owner=uid 1000 for built assets where possible)\n"
# Watch and rebuild CSS on changes (in container with user permissions)
css-watch: css-deps
	@printf "👁️  Watching for CSS changes...\n"
	@$(CONTAINER_CMD) run --rm -it --security-opt label=disable -u $(shell id -u):$(shell id -g) \
		-v $(PWD):/app -v gotrs_cache:/cache -e BUN_INSTALL_CACHE_DIR=/cache/bun \
		-w /app oven/bun:1.1-alpine bun run watch-css

#########################################
# TEST TARGETS
#########################################

# Run specific test in toolbox container
test-specific:
	@if [ -z "$(TEST)" ]; then \
		echo "Error: TEST required. Usage: make test-specific TEST=TestRequiredQueueExists"; \
		exit 1; \
	fi
	@printf "🧪 Running specific test: $(TEST)\n"
	@$(CONTAINER_CMD) run --rm \
		--network gotrs-ce_gotrs-network \
		-e DB_HOST=postgres \
		-e DB_USER=$(DB_USER) \
		-e DB_PASSWORD=$(DB_PASSWORD) \
		-e DB_NAME=$(DB_NAME) \
		-v "$$(pwd):/workspace" \
		-w /workspace \
		gotrs-toolbox:latest \
			bash -lc 'export PATH=/usr/local/go/bin:$$PATH; echo "Testing with DB_HOST=$$DB_HOST"; go test -buildvcs=false -v ./internal/repository -run $(TEST)'

.PHONY: verify-container-first
verify-container-first:
	@chmod +x scripts/tools/check-container-go.sh 2>/dev/null || true
	@./scripts/tools/check-container-go.sh

# Enhanced test command - runs test stack and executes full test suite
test-comprehensive:
	@./scripts/test-runner.sh

# Run integration tests (curl-based API tests against test backend only)
.PHONY: test-integration
test-integration: test-stack-up
	@printf "🔗 Running integration tests against test backend (port $(TEST_BACKEND_PORT))...\n"
	@TEST_BACKEND_PORT=$(TEST_BACKEND_PORT) \
	 DEMO_ADMIN_EMAIL=$(DEMO_ADMIN_EMAIL) \
	 DEMO_ADMIN_PASSWORD=$(DEMO_ADMIN_PASSWORD) \
	 ./scripts/integration-test.sh

#########################################
# TEST OVERRIDES
#########################################

# Main test target
test: test-comprehensive
