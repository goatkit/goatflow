# Explicit default goal
.DEFAULT_GOAL := help

# Route manifest governance
.PHONY: routes-verify routes-baseline-update routes-generate
routes-generate:
	@echo "Generating routes manifest..."
	@mkdir -p runtime && chmod 777 runtime 2>/dev/null || true
	@chmod +x scripts/generate_routes_manifest.sh
	@$(MAKE) toolbox-exec ARGS='bash scripts/generate_routes_manifest.sh'
	@[ -f runtime/routes-manifest.json ] && echo "routes-manifest.json generated." || (echo "Failed to generate routes manifest" && exit 1)

routes-verify:
	@if [ ! -f runtime/routes-manifest.json ]; then \
		$(MAKE) routes-generate; \
	fi
	@sh ./scripts/check_routes_manifest.sh

routes-baseline-update:
	@[ -f runtime/routes-manifest.json ] || (echo "manifest missing; run server/tests first" && exit 1)
	cp runtime/routes-manifest.json runtime/routes-manifest.baseline.json
	@echo "Updated route manifest baseline."
# GOTRS Makefile - Docker/Podman compatible development

# Detect container runtime and compose command
# First check for podman, then docker
CONTAINER_CMD := $(shell command -v podman 2> /dev/null || command -v docker 2> /dev/null || echo docker)

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
COMPOSE_BUILD_FLAGS := $(if $(findstring podman-compose,$(COMPOSE_CMD)),,--no-cache)
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

# Ensure Go caches exist for toolbox runs
define ensure_caches
@mkdir -p .cache/go-build .cache/go-mod >/dev/null 2>&1 || true
endef

# Common run flags
VOLUME_PWD := -v "$$(pwd):/workspace"
WORKDIR_FLAGS := -w /workspace
USER_FLAGS := -u "$$(id -u):$$(id -g)"
# Default DB host: prefer host.containers.internal on Podman
ifeq ($(findstring podman,$(CONTAINER_CMD)),podman)
DB_HOST ?= host.containers.internal
else
DB_HOST ?= localhost
endif
DB_PORT ?= 5432
DB_NAME ?= gotrs
DB_USER ?= gotrs_user
DB_PASSWORD ?= gotrs_password
VALKEY_HOST ?= localhost
VALKEY_PORT ?= 6388

# Macro to wrap Go commands inside toolbox container (ensures container-first dev)
TOOLBOX_GO=$(MAKE) toolbox-exec ARGS=

# --- Auto-detect DB driver (mariadb/postgres) if not provided ---
# Priority: existing DB_DRIVER env/.env > running compose services > compose file contents > default mariadb
ifeq ($(origin DB_DRIVER), undefined)
DB_DRIVER := $(shell \
  if $(COMPOSE_CMD) ps --services 2>/dev/null | grep -q "^mariadb$$"; then \
    echo mariadb; \
  elif $(COMPOSE_CMD) ps --services 2>/dev/null | grep -q "^postgres$$"; then \
    echo postgres; \
  elif [ -f docker-compose.yml ] && grep -qE '^[[:space:]]mariadb:' docker-compose.yml; then \
    echo mariadb; \
  elif [ -f docker-compose.yml ] && grep -qE '^[[:space:]]postgres:' docker-compose.yml; then \
    echo postgres; \
  else \
    echo mariadb; \
  fi)
endif

# Align defaults for host/port with detected driver when not explicitly set
ifeq ($(DB_DRIVER),mariadb)
DB_HOST ?= mariadb
DB_PORT ?= 3306
endif
ifeq ($(DB_DRIVER),mysql)
DB_HOST ?= mariadb
DB_PORT ?= 3306
endif
ifeq ($(DB_DRIVER),postgres)
DB_HOST ?= postgres
DB_PORT ?= 5432
endif

.PHONY: help up down logs logs-follow restart clean setup test build debug-env build-cached toolbox-build toolbox-run toolbox-exec api-call toolbox-compile toolbox-compile-api \
	toolbox-test-api toolbox-test toolbox-test-all toolbox-test-run toolbox-run-file toolbox-staticcheck test-actions-dropdown

.PHONY: go-cache-info go-cache-clean
go-cache-info:
	@echo "Go cache directories (inside toolbox):";
	@$(MAKE) toolbox-exec ARGS='bash -lc "echo GOCACHE=$$GOCACHE; echo GOMODCACHE=$$GOMODCACHE; du -sh $$GOCACHE 2>/dev/null || true; du -sh $$GOMODCACHE 2>/dev/null || true"'

go-cache-clean:
	@echo "Cleaning Go build/module caches (named volumes persist but contents cleared)"
	@$(MAKE) toolbox-exec ARGS='bash -lc "rm -rf $$GOCACHE/* $$GOMODCACHE/pkg/mod/cache/download 2>/dev/null || true"'
	@echo "Done"

# GolangCI-Lint cache management
.PHONY: lint-cache-info lint-cache-clean
lint-cache-info:
	@echo "golangci-lint cache (inside toolbox):";
	@$(MAKE) toolbox-exec ARGS='bash -lc "echo $$GOLANGCI_LINT_CACHE; du -sh $$GOLANGCI_LINT_CACHE 2>/dev/null || true"'

lint-cache-clean:
	@echo "Cleaning golangci-lint cache"
	@$(MAKE) toolbox-exec ARGS='bash -lc "rm -rf $$GOLANGCI_LINT_CACHE/* 2>/dev/null || true"'
	@echo "Done"

# Aggregate cache clean (Go + lint + node_modules optional purge)
.PHONY: cache-clean-all
cache-clean-all:
	@echo "🚿 Purging all development caches (Go build/mod, golangci-lint)."
	@$(MAKE) go-cache-clean
	@$(MAKE) lint-cache-clean
	@echo "(node_modules untouched; run 'make node-modules-clean' if you add such a target later)"
	@echo "✅ All caches purged"

# Default target
help:
	@cat logo.txt
	@printf "\n"
	@printf "  \033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m\n"
	@printf "  \033[1;33m🚀 Core Commands\033[0m\n"
	@printf "  \033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m\n"
	@printf "\n"
	@printf "  \033[0;32mmake up\033[0m                           ▶️ Start all services\n"
	@printf "  \033[0;32mmake down\033[0m                         🛑 Stop all services\n"
	@printf "  \033[0;32mmake logs\033[0m                         📋 View a portion of the most recent logs\n"
	@printf "  \033[0;32mmake logs-follow\033[0m                  📋 View and endlessly follow logs\n"
	@printf "  \033[0;32mmake restart\033[0m                      🔄 Restart all services\n"
	@printf "  \033[0;32mmake clean\033[0m                        🧹 Clean everything (including volumes)\n"
	@printf "  \033[0;32mmake setup\033[0m                        🎯 Initial project setup with secure secrets\n"
	@printf "  \033[0;32mmake build\033[0m                        🔨 Build production images\n"
	@printf "  \033[0;32mmake debug-env\033[0m                    🔍 Show container runtime detection\n"
	@printf "  \033[0;32mmake otrs-import SQL=/path/to/dump.sql\033[0m   📥 Import OTRS database dump\n"
	@printf "\n"
	@printf "  \033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m\n"
	@printf "  \033[1;33m🧪 TDD Workflow (Quality Gates Enforced)\033[0m\n"
	@printf "  \033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m\n"
	@printf "\n"
	@printf "  \033[0;32mmake tdd-init\033[0m                     🏁 Initialize TDD workflow\n"
	@printf "  \033[0;32mmake tdd-test-first\033[0m FEATURE=name  ❌ Start with failing test\n"
	@printf "  \033[0;32mmake tdd-implement\033[0m                ✅ Implement to pass tests\n"
	@printf "  \033[0;32mmake tdd-verify\033[0m                   🔍 Run ALL quality gates\n"
	@printf "  \033[0;32mmake tdd-refactor\033[0m                 ♻️  Safe refactoring\n"
	@printf "  \033[0;32mmake tdd-status\033[0m                   📊 Show workflow status\n"
	@printf "  \033[0;32mmake quality-gates\033[0m                🚦 Run quality checks\n"
	@printf "  \033[0;32mmake evidence-report\033[0m              📄 Generate evidence\n"
	@printf "  \033[0;32mmake tdd-comprehensive-quick\033[0m       ⚡ Quick comprehensive TDD run\n"
	@printf "\n"
	@printf "  \033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m\n"
	@printf "  \033[1;33m🎨 CSS/Frontend Build\033[0m\n"
	@printf "  \033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m\n"
	@printf "\n"
	@printf "  \033[0;32mmake npm-updates\033[0m                  ⬆️ Update NPM dependencies\n"
	@printf "  \033[0;32mmake css-build\033[0m                    📦 Build production CSS from Tailwind\n"
	@printf "  \033[0;32mmake css-watch\033[0m                    👁️ Watch and rebuild CSS on changes\n"
	@printf "  \033[0;32mmake css-deps\033[0m                     📥 Install CSS build dependencies\n"
	@printf "\n"
	@printf "  \033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m\n"
	@printf "  \033[1;33m🔐 Secrets Management\033[0m\n"
	@printf "  \033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m\n"
	@printf "\n"
	@printf "  \033[0;32mmake synthesize\033[0m                   🔑 Generate new .env with secure secrets\n"
	@printf "  \033[0;32mmake rotate-secrets\033[0m               🔄 Rotate secrets in existing .env\n"
	@printf "  \033[0;32mmake synthesize-force\033[0m             ⚡ Force regenerate .env\n"
	@printf "  \033[0;32mmake k8s-secrets\033[0m                  🙊 Generate k8s/secrets.yaml\n"
	@printf "  \033[0;32mmake show-dev-creds\033[0m               👤 Show test user credentials\n"
	@printf "\n"
	@printf "  \033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m\n"
	@printf "  \033[1;33m🐳 Docker/Container Build\033[0m\n"
	@printf "  \033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m\n"
	@printf "\n"
	@printf "  \033[0;32mmake build-cached\033[0m                 🚀 Fast build with caching (70% faster)\n"
	@printf "  \033[0;32mmake build-clean\033[0m                  🧹 Clean build without cache\n"
	@printf "  \033[0;32mmake build-secure\033[0m                 🔒 Build with security scanning\n"
	@printf "  \033[0;32mmake build-multi\033[0m                  🌍 Multi-platform build (AMD64/ARM64)\n"
	@printf "  \033[0;32mmake build-all-tools\033[0m              🛠️ Build all specialized containers\n"
	@printf "  \033[0;32mmake toolbox-build\033[0m                🔧 Build development toolbox\n"
	@printf "  \033[0;32mmake analyze-size\033[0m                 📏 Analyze image size with dive\n"
	@printf "  \033[0;32mmake show-sizes\033[0m                   📊 Show all image sizes\n"
	@printf "  \033[0;32mmake show-cache\033[0m                   💾 Display build cache usage\n"
	@printf "  \033[0;32mmake clear-cache\033[0m                  🗑️ Clear Docker build cache\n"
	@printf "\n"
	@printf "  \033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m\n"
	@printf "  \033[1;33m🔮 Schema Discovery\033[0m\n"
	@printf "  \033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m\n"
	@printf "\n"
	@printf "  \033[0;32mmake schema-discovery\033[0m             🔍 Generate YAML from DB schema\n"
	@printf "  \033[0;32mmake schema-table\033[0m                 📊 Generate YAML for table\n"
	@printf "\n"
	@printf "  \033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m\n"
	@printf "  \033[1;33m🧰 Toolbox (Fast Container Dev)\033[0m\n"
	@printf "  \033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m\n"
	@printf "\n"
	@printf "  \033[0;32mmake toolbox-build\033[0m                🔨 Build toolbox container\n"
	@printf "  \033[0;32mmake toolbox-run\033[0m                  🐚 Interactive shell\n"
	@printf "  \033[0;32mmake toolbox-exec ARGS='go version'\033[0m   ⚡ Execute commands in toolbox\n"
	@printf "  \033[0;32mmake api-call METHOD=GET ENDPOINT=/api/lookups/statuses\033[0m   🌐 Authenticated API calls\n"
	@printf "  \033[0;32mmake toolbox-compile\033[0m               🔨 Compile all Go packages\n"
	@printf "  \033[0;32mmake toolbox-compile-api\033[0m           🚀 Compile API/goats only (faster)\n"
	@printf "  \033[0;32mmake compile\033[0m                       🔨 Compile goats binary\n"
	@printf "  \033[0;32mmake compile-safe\033[0m                 🔒 Compile in isolated container\n"
	@printf "  \033[0;32mmake toolbox-test\033[0m                 🧪 Run core tests quickly\n"
	@printf "  \033[0;32mmake toolbox-test-api\033[0m             🧪 Run API tests only\n"
	@printf "  \033[0;32mmake toolbox-test-all\033[0m             🧪 Run broad test suite\n"
	@printf "  \033[0;32mmake toolbox-staticcheck\033[0m           🔎 Run static analysis\n"
	@printf "  \033[0;32mmake openapi-lint\033[0m                 📜 Lint OpenAPI spec (Node 22)\n"
	@printf "  \033[0;32mmake yaml-lint\033[0m                     📄 Lint YAML files\n"
	@printf "  \033[0;32mmake openapi-bundle\033[0m               📦 Bundle OpenAPI spec\n"
	@printf "  \033[0;32mmake tdd-comprehensive\033[0m            📋 Run comprehensive TDD gates\n"
	@printf "  \033[0;32mmake toolbox-test-run\033[0m             🎯 Run specific test\n"
	@printf "  \033[0;32mmake toolbox-run-file\033[0m             📄 Run Go file\n"
	@printf "  \033[0;32mmake test-unit\033[0m                    🧪 Run unit tests only (stable set)\n"
	@printf "  \033[0;32mmake test-e2e TEST=...\033[0m            🎯 Run targeted E2E tests (pattern)\n"
	@printf "\n"
	@printf "  \033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m\n"
	@printf "  \033[1;33m🎭 E2E Testing (Playwright)\033[0m\n"
	@printf "  \033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m\n"
	@printf "\n"
	@printf "  \033[0;32mmake test-e2e-playwright\033[0m                 🤖 Run Playwright E2E tests headless\n"
	@printf "  \033[0;32mmake test-e2e-playwright-debug\033[0m           👀 Playwright tests with visible browser\n"
	@printf "  \033[0;32mmake test-e2e-playwright-watch\033[0m           🔁 Playwright tests in watch mode\n"
	@printf "  \033[0;32mmake test-e2e-playwright-report\033[0m          📊 View Playwright test results\n"
	@printf "\n"
	@printf "  \033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m\n"
	@printf "  \033[1;33m🧪 Testing Commands\033[0m\n"
	@printf "  \033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m\n"
	@printf "\n"
	@printf "  \033[0;32mmake test\033[0m                         ✅ Run Go backend tests\n"
	@printf "  \033[0;32mmake test-short\033[0m                   ⚡ Skip long tests\n"
	@printf "  \033[0;32mmake test-coverage\033[0m                📈 Tests with coverage\n"
	@printf "  \033[0;32mmake test-safe\033[0m                    🏃 Race/deadlock detection\n"
	@printf "  \033[0;32mmake test-html\033[0m                    🌐 HTML test report\n"
	@printf "  \033[0;32mmake test-actions-dropdown\033[0m         🎯 Test Actions dropdown components\n"
	@printf "  \033[0;32mmake test-containerized\033[0m            🐳 Run tests in containers\n"
	@printf "\n"
	@printf "  \033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m\n"
	@printf "  \033[1;33m🐠 i18n (Babel fish) Commands\033[0m\n"
	@printf "  \033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m\n"
	@printf "\n"
	@printf "  \033[0;32mmake babelfish\033[0m                    🏗️ Build gotrs-babelfish binary\n"
	@printf "  \033[0;32mmake babelfish-coverage\033[0m           📊 Show translation coverage\n"
	@printf "  \033[0;32mmake babelfish-validate\033[0m\n\t LANG=de   ✅ Validate a language\n"
	@printf "  \033[0;32mmake babelfish-missing\033[0m\n\t LANG=es    🔍 Show missing translations\n"
	@printf "  \033[0;32mmake babelfish-run\033[0m\n\t ARGS='-help'   🎯 Run with custom args\n"
	@printf "  \033[0;32mmake test-ldap\033[0m                    🔐 Run LDAP integration tests\n"
	@printf "  \033[0;32mmake test-ldap-perf\033[0m               ⚡ Run LDAP performance benchmarks\n"
	@printf "\n"
	@printf "  \033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m\n"
	@printf "  \033[1;33m🔒 Security Commands\033[0m\n"
	@printf "  \033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m\n"
	@printf "\n"
	@printf "  \033[0;32mmake scan-secrets\033[0m\t\t\t🕵️ Scan current code for secrets\n"
	@printf "  \033[0;32mmake scan-secrets-history\033[0m\t\t📜 Scan git history for secrets\n"
	@printf "  \033[0;32mmake scan-secrets-precommit\033[0m\t\t🪝 Install pre-commit hooks\n"
	@printf "  \033[0;32mmake scan-vulnerabilities\033[0m\t\t🐛 Scan for vulnerabilities\n"
	@printf "  \033[0;32mmake security-scan\033[0m\t\t\t🛡️ Run all security scans\n"
	@printf "  \033[0;32mmake test-contracts\033[0m\t\t\t📝 Run Pact contract tests\n"
	@printf "  \033[0;32mmake test-all\033[0m\t\t\t\t🎯 Run all tests (backend, frontend, contracts)\n"
	@printf "\n"
	@printf "  \033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m\n"
	@printf "  \033[1;33m📡 Service Management\033[0m\n"
	@printf "  \033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m\n"
	@printf "\n"
	@printf "  \033[0;32mmake backend-logs\033[0m\t\t\t📋 View backend logs\n"
	@printf "  \033[0;32mmake backend-logs-follow\033[0m\t\t📺 Follow backend logs\n"
	@printf "  \033[0;32mmake valkey-cli\033[0m\t\t\t🔑 Valkey CLI\n"
	@printf "\n"
	@printf "  \033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m\n"
	@printf "  \033[1;33m🗄️  Database Operations\033[0m\n"
	@printf "  \033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m\n"
	@printf "\n"
	@printf "  \033[0;32mmake db-shell\033[0m\t\t\t\t🗄️  Database shell (driver-aware)\n"
	@printf "  \033[0;32mmake db-migrate\033[0m\t\t\t📤 Run pending migrations\n"
	@printf "  \033[0;32mmake db-rollback\033[0m\t\t\t↩️  Rollback last migration\n"
	@printf "  \033[0;32mmake db-reset\033[0m\t\t\t\t💥 Reset DB (cleans storage)\n"
	@printf "  \033[0;32mmake db-init\033[0m\t\t\t\t🚀 Fast baseline init\n"
	@printf "  \033[0;32mmake db-apply-test-data\033[0m\t\t🧪 Apply test data\n"
	@printf "  \033[0;32mmake clean-storage\033[0m\t\t\t🗑️ Remove orphaned files\n"
	@printf "\n"
	@printf "  \033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m\n"
	@printf "  \033[1;33m📦 OTRS Migration\033[0m\n"
	@printf "  \033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m\n"
	@printf "\n"
	@printf "  \033[0;32mmake migrate-analyze\033[0m\n\t SQL=dump.sql\t\t\t🔍 Analyze OTRS SQL dump\n"
	@printf "  \033[0;32mmake migrate-import\033[0m\n\t SQL=dump.sql\t\t\t📥 Import OTRS data (dry-run)\n"
	@printf "  \033[0;32mmake migrate-import\033[0m\n\t SQL=dump.sql DRY_RUN=false\t💾 Import for real\n"
	@printf "  \033[0;32mmake migrate-validate\033[0m\t\t\t🔍 Validate imported data\n"
	@printf "  \033[0;32mmake import-test-data\033[0m\t\t\t🎯 Import test tickets with proper mapping\n"
	@printf "\n"
	@printf "  \033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m\n"
	@printf "  \033[1;33m👥 User Management\033[0m\n"
	@printf "  \033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m\n"
	@printf "\n"
	@printf "  \033[0;32mmake reset-password\033[0m\t\t\t🔓 Reset user password\n"
	@printf "\n"
	@printf "  \033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m\n"
	@printf "\n"
	@printf "  \033[0;90m🐐 Happy coding with GOTRS!\033[0m\n"
	@printf "  \033[0;90mContainer Runtime: $(CONTAINER_CMD) | Compose Tool: $(COMPOSE_CMD) | Image Prefix: $(IMAGE_PREFIX)\033[0m\n"
	@printf "\n"
#########################################
# TDD WORKFLOW COMMANDS
#########################################

# Initialize TDD workflow
tdd-init:
	@printf "🧪 Initializing TDD workflow with mandatory quality gates...\n"
	@./scripts/tdd-enforcer.sh init

# Start TDD cycle with failing test
tdd-test-first:
	@if [ -z "$(FEATURE)" ]; then \
		echo "Error: FEATURE required. Usage: make tdd-test-first FEATURE='Feature Name'"; \
		exit 1; \
	fi
	@printf "🔴 Starting test-first phase for: $(FEATURE)\n"
	@./scripts/tdd-enforcer.sh test-first "$(FEATURE)"

# Implement code to pass tests
tdd-implement:
	@printf "🔧 Starting implementation phase...\n"
	@./scripts/tdd-enforcer.sh implement

# Comprehensive verification with all quality gates
tdd-verify:
	@printf "✅ Running comprehensive verification (ALL quality gates must pass)...\n"
	@./scripts/tdd-enforcer.sh verify

# Safe refactoring with regression checks
tdd-refactor:
	@printf "♻️ Starting refactor phase with regression protection...\n"
	@./scripts/tdd-enforcer.sh refactor

# Show current TDD workflow status
tdd-status:
	@./scripts/tdd-enforcer.sh status

# Run quality gates independently for debugging
quality-gates:
	@printf "🚪 Running all quality gates independently...\n"
	@./scripts/tdd-enforcer.sh verify debug

# Generate evidence report from latest verification
evidence-report:
	@printf "📊 Latest evidence reports:\n"
	@find generated/evidence -name "*_report_*.html" -type f -exec ls -la {} \; | head -5 || echo "No evidence reports found"

#########################################
# ENHANCED TEST COMMANDS WITH TDD INTEGRATION
#########################################

# Override test command to use TDD if in TDD cycle
test:
	@if [ -f .tdd-state ]; then \
		echo "🧪 TDD workflow active - using TDD test verification..."; \
		$(MAKE) tdd-verify; \
	else \
		echo "Running tests with safety checks..."; \
		echo "Using test database: $${DB_NAME:-gotrs}_test"; \
		echo "Checking if backend service is running..."; \
		$(COMPOSE_CMD) ps --services --filter "status=running" | grep -q "backend" || (echo "Error: Backend service is not running. Please run 'make up' first." && exit 1); \
		$(COMPOSE_CMD) exec -e DB_NAME=$${DB_NAME:-gotrs}_test -e APP_ENV=test backend go test -v ./...; \
	fi

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
	@grep "^-- ||" migrations/000004_generated_test_data.up.sql 2>/dev/null | sed 's/^-- || //' | column -t || echo "No credentials found. Run 'make synthesize' first."

# Apply generated test data to database
db-apply-test-data:
	@printf "📝 Applying generated test data...\n"
	@if [ "$(DB_DRIVER)" = "postgres" ]; then \
		$(COMPOSE_CMD) exec -T postgres psql -U $(DB_USER) -d $(DB_NAME) -f - < migrations/000004_generated_test_data.up.sql; \
		printf "✅ Test data applied. Run 'make show-dev-creds' to see credentials.\n"; \
	else \
		printf "📡 Starting dependencies (mariadb)...\n"; \
		$(COMPOSE_CMD) up -d mariadb >/dev/null 2>&1 || true; \
		printf "👤 Ensuring root user exists (MariaDB)...\n"; \
		$(CONTAINER_CMD) run --rm \
			--network gotrs-ce_gotrs-network \
			-e DB_DRIVER=$(DB_DRIVER) -e DB_HOST=$(DB_HOST) -e DB_PORT=$(DB_PORT) \
			-e DB_NAME=$(DB_NAME) -e DB_USER=$(DB_USER) -e DB_PASSWORD=$(DB_PASSWORD) \
			gotrs-toolbox:latest \
			gotrs reset-user --username="root@localhost" --password="Admin123!1" --enable; \
		printf "✅ Root user ready. Use 'make reset-password' to change.\n"; \
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

# Build toolbox image
toolbox-build:
	@printf "\n🔧 Building GOTRS toolbox container...\n"
	@if echo "$(COMPOSE_CMD)" | grep -q "podman-compose"; then \
		COMPOSE_PROFILES=toolbox $(COMPOSE_CMD) build $(COMPOSE_BUILD_FLAGS) toolbox; \
	else \
		$(COMPOSE_CMD) --profile toolbox build $(COMPOSE_BUILD_FLAGS) toolbox; \
	fi
	@printf "✅ Toolbox container ready\n"

# Interactive toolbox shell (non-root, with SELinux-friendly mounts)
toolbox-run:
	@printf "\n🔧 Starting toolbox shell...\n"
	@printf "💡 Type 'exit' or Ctrl+D to exit the shell\n"
	@$(TOOLBOX_GO)"golangci-lint run ./..."

# Non-interactive toolbox exec
toolbox-exec:
	@$(call ensure_caches)
	@if echo "$(COMPOSE_CMD)" | grep -q "podman-compose"; then \
		COMPOSE_PROFILES=toolbox $(COMPOSE_CMD) run --rm toolbox bash -c 'export GOFLAGS="-buildvcs=false $$GOFLAGS"; export PATH="/usr/local/go/bin:$$PATH"; $(ARGS)'; \
	else \
		$(COMPOSE_CMD) --profile toolbox run --rm toolbox bash -c 'export GOFLAGS="-buildvcs=false $$GOFLAGS"; export PATH="/usr/local/go/bin:$$PATH"; $(ARGS)'; \
	fi

# API testing with automatic authentication
api-call:
	@if [ -z "$(METHOD)" ]; then echo "❌ METHOD required. Usage: make api-call METHOD=GET ENDPOINT=/api/v1/tickets [BODY='{}']"; exit 1; fi
	@if [ -z "$(ENDPOINT)" ]; then echo "❌ ENDPOINT required. Usage: make api-call METHOD=GET ENDPOINT=/api/v1/tickets [BODY='{}']"; exit 1; fi
	@printf "\n🔧 Making API call: $(METHOD) $(ENDPOINT)\n"
	@$(call ensure_caches)
	@if echo "$(COMPOSE_CMD)" | grep -q "podman-compose"; then \
		COMPOSE_PROFILES=toolbox $(COMPOSE_CMD) run --rm toolbox bash scripts/api-test.sh "$(METHOD)" "$(ENDPOINT)" "$(BODY)"; \
	else \
		$(COMPOSE_CMD) --profile toolbox run --rm toolbox bash scripts/api-test.sh "$(METHOD)" "$(ENDPOINT)" "$(BODY)"; \
	fi

# Compile everything (bind mounts + caches)
toolbox-compile:
	@$(MAKE) toolbox-build
	@printf "\n🔨 Checking compilation...\n"
	@$(call ensure_caches)
	$(CONTAINER_CMD) run --rm \
        --security-opt label=disable \
        -v "$$PWD:/workspace" \
		-w /workspace \
		-u "$$UID:$$GID" \
		-e GOCACHE=/workspace/.cache/go-build \
		-e GOMODCACHE=/workspace/.cache/go-mod \
		gotrs-toolbox:latest \
		bash -lc 'export PATH=/usr/local/go/bin:$$PATH && go version && go build -buildvcs=false ./...'

# Compile only API and goats (faster)
toolbox-compile-api:
	@$(MAKE) toolbox-build
	@printf "\n🔨 Compiling API and goats packages only...\n"
	@$(call ensure_caches)
	@$(CONTAINER_CMD) run --rm \
        --security-opt label=disable \
        -v "$$PWD:/workspace" \
		-w /workspace \
		-u "$$UID:$$GID" \
		-e GOCACHE=/workspace/.cache/go-build \
		-e GOMODCACHE=/workspace/.cache/go-mod \
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
	@$(call ensure_caches)
	@printf "📡 Starting dependencies (mariadb, valkey)...\n"
	@$(COMPOSE_CMD) up -d mariadb valkey >/dev/null 2>&1 || true
	@$(CONTAINER_CMD) run --rm \
        --security-opt label=disable \
        -v "$$PWD:/workspace" \
		--network host \
		-v gotrs_go_mod_cache:/go/pkg/mod \
		-v gotrs_go_build_cache:/home/appuser/.cache/go-build \
		-w /workspace \
		-e GOCACHE=/home/appuser/.cache/go-build \
		-e GOMODCACHE=/go/pkg/mod \
		-e APP_ENV=test \
		-e STORAGE_PATH=/tmp \
		-e TEMPLATES_DIR=/workspace/templates \
		-e DB_HOST=$(DB_HOST) -e DB_PORT=3306 \
        -e DB_DRIVER=mariadb \
        -e DB_NAME=otrs -e DB_USER=otrs -e DB_PASSWORD=LetClaude.1n \
		gotrs-toolbox:latest \
		bash -lc 'export PATH=/usr/local/go/bin:$$PATH; $(TOOLBOX_GO)"go test -v ./internal/api -run ^Test(BuildRoutesManifest|Queue|Article|Search|Priority|User)"'

# Run core tests (cmd/goats + internal/api + generated/tdd-comprehensive)
toolbox-test:
	@$(MAKE) toolbox-build
	@printf "\n🧪 Running core test suite in toolbox...\n"
	@$(call ensure_caches)
	@printf "📡 Starting dependencies (mariadb, valkey)...\n"
	@$(COMPOSE_CMD) up -d mariadb valkey >/dev/null 2>&1 || true
	@$(CONTAINER_CMD) run --rm \
        --security-opt label=disable \
        -v "$$PWD:/workspace" \
		--network host \
		-v gotrs_go_mod_cache:/go/pkg/mod \
		-v gotrs_go_build_cache:/home/appuser/.cache/go-build \
		-w /workspace \
		-e GOCACHE=/home/appuser/.cache/go-build \
		-e GOMODCACHE=/go/pkg/mod \
		-e APP_ENV=test \
		-e STORAGE_PATH=/tmp \
		-e TEMPLATES_DIR=/workspace/templates \
		-e DB_HOST=$(DB_HOST) -e DB_PORT=3306 \
        -e DB_DRIVER=mariadb \
        -e DB_NAME=otrs -e DB_USER=otrs -e DB_PASSWORD=LetClaude.1n \
		-e VALKEY_HOST=$(VALKEY_HOST) -e VALKEY_PORT=$(VALKEY_PORT) \
		gotrs-toolbox:latest \
		bash -lc 'export PATH=/usr/local/go/bin:$$PATH; set -e; \
		echo Running: ./cmd/goats; $(TOOLBOX_GO)"go test -v ./cmd/goats"; \
		echo Running: ./internal/api focused; $(TOOLBOX_GO)"go test -v ./internal/api -run ^Test(AdminType|Queue|Article|Search|Priority|User|TicketZoom|AdminService|AdminStates|AdminGroupManagement|HandleGetQueues|HandleGetPriorities|DatabaseIntegrity)"; \
		echo Running: ./internal/service; $(TOOLBOX_GO)"go test -v ./internal/service"'

.PHONY: tdd-comprehensive-quick
tdd-comprehensive-quick:
	@printf "\n📋 Running comprehensive TDD gates...\n"
	@mkdir -p generated/tdd-comprehensive generated/evidence generated/test-results || true
	@$(CONTAINER_CMD) run --rm \
		--security-opt label=disable \
		-v "$$PWD:/workspace" \
		-v "$$PWD/generated:/workspace/generated" \
		-w /workspace \
		--network host \
		-u 0 \
		gotrs-toolbox:latest \
		bash -lc 'bash scripts/tdd-comprehensive.sh quick || true; echo "See generated/evidence for report"'

.PHONY: openapi-lint
openapi-lint:
	@echo "📜 Linting OpenAPI spec with Node 22 (Redocly)..."
	@$(CONTAINER_CMD) run --rm \
		--security-opt label=disable \
		-v "$$PWD/api:/spec"$(VZ) \
		node:22-alpine \
		sh -lc 'npm -g i @redocly/cli >/dev/null 2>&1 && redocly lint /spec/openapi.yaml'

.PHONY: openapi-bundle
openapi-bundle:
	@echo "📦 Bundling OpenAPI spec with Redocly..."
	@$(CONTAINER_CMD) run --rm \
		--security-opt label=disable \
		-v "$$PWD/api:/spec"$(VZ) \
		node:22-alpine \
		sh -lc 'npm -g i @redocly/cli >/dev/null 2>&1 && redocly bundle /spec/openapi.yaml -o /spec/openapi.bundle.yaml'

# Run almost-all tests (excludes heavyweight e2e/integration and unstable lambda tests)
toolbox-test-all:
	@$(MAKE) toolbox-build
	@printf "\n🧪 Running broad test suite (excluding e2e/integration) in toolbox...\n"
	@$(call ensure_caches)
	@printf "📡 Starting dependencies (mariadb, valkey)...\n"
	@$(COMPOSE_CMD) up -d mariadb valkey >/dev/null 2>&1 || true
	@$(CONTAINER_CMD) run --rm \
		--security-opt label=disable \
		-v "$$PWD:/workspace" \
		--network host \
		-v gotrs_go_mod_cache:/go/pkg/mod \
		-v gotrs_go_build_cache:/home/appuser/.cache/go-build \
		-w /workspace \
		-e GOCACHE=/home/appuser/.cache/go-build \
		-e GOMODCACHE=/go/pkg/mod \
		-e APP_ENV=test \
		-e STORAGE_PATH=/tmp \
		-e TEMPLATES_DIR=/workspace/templates \
		-e DB_HOST=$(DB_HOST) -e DB_PORT=3306 \
		-e DB_DRIVER=mariadb \
		-e DB_NAME=otrs -e DB_USER=otrs -e DB_PASSWORD=LetClaude.1n \
		gotrs-toolbox:latest \
		bash -lc 'export PATH=/usr/local/go/bin:$$PATH; set -e; \
		echo Running curated set: cmd/goats internal/api internal/service generated/tdd-comprehensive; \
		$(TOOLBOX_GO)"go test -buildvcs=false -v ./cmd/goats"; \
		$(TOOLBOX_GO)"go test -buildvcs=false -v ./internal/api -run ^Test(AdminType|Queue|Article|Search|Priority|User|TicketZoom|AdminService|AdminStates|AdminGroupManagement|HandleGetQueues|HandleGetPriorities|DatabaseIntegrity)"; \
		$(TOOLBOX_GO)"go test -buildvcs=false -v ./internal/service"; \
		$(TOOLBOX_GO)"go test -buildvcs=false -v ./generated/tdd-comprehensive"'

.PHONY: test-unit
test-unit:
	@echo "🧪 Running stable unit test set (excluding examples and e2e)..."
	@$(MAKE) toolbox-build
	@$(call ensure_caches)
	@$(CONTAINER_CMD) run --rm \
		--security-opt label=disable \
		-v "$$PWD:/workspace" \
		-w /workspace \
		-u "$$(id -u):$$(id -g)" \
		-e GOCACHE=/workspace/.cache/go-build \
		-e GOMODCACHE=/workspace/.cache/go-mod \
		-e GOFLAGS=-buildvcs=false \
		gotrs-toolbox:latest \
		bash -lc 'export PATH=/usr/local/go/bin:$$PATH; \
		$(TOOLBOX_GO)"go test -count=1 -buildvcs=false -v ./cmd/goats ./internal/... ./generated/..." | tee generated/test-results/unit_stable.log'

.PHONY: test-e2e
test-e2e:
	@echo "🎯 Running targeted E2E tests (set TEST=pattern, e.g., TEST=Login|Groups)"
	@[ -n "$(TEST)" ] || (echo "Usage: make test-e2e TEST=Login|Groups|Queues" && exit 2)
	@$(MAKE) toolbox-build
	@$(call ensure_caches)
	@HEADLESS=${HEADLESS:-true} \
	 BASE_URL=${BASE_URL:-http://localhost:$(BACKEND_PORT)} \
	 DEMO_ADMIN_EMAIL=${DEMO_ADMIN_EMAIL:-} \
	 DEMO_ADMIN_PASSWORD=${DEMO_ADMIN_PASSWORD:-} \
	 $(CONTAINER_CMD) run --rm \
		--security-opt label=disable \
		-v "$$PWD:/workspace" \
		-w /workspace \
		-u "$$(id -u):$$(id -g)" \
		-e GOCACHE=/workspace/.cache/go-build \
		-e GOMODCACHE=/workspace/.cache/go-mod \
		-e GOFLAGS=-buildvcs=false \
		-e HEADLESS \
		-e BASE_URL \
		-e DEMO_ADMIN_EMAIL \
		-e DEMO_ADMIN_PASSWORD \
		gotrs-toolbox:latest \
		bash -lc 'export PATH=/usr/local/go/bin:$$PATH; \
		go test -count=1 -buildvcs=false -v ./tests/e2e -run "$(TEST)" | tee generated/test-results/e2e_$(shell echo $(TEST) | tr ' ' '_').log'

# Run integration tests (requires running Postgres and proper creds)
toolbox-test-integration:
	@$(MAKE) toolbox-build
	@printf "\n🧪 Running integration tests (requires DB) in toolbox...\n"
	@$(call ensure_caches)
	@printf "📡 Starting dependencies (postgres, valkey)...\n"
	@$(COMPOSE_CMD) up -d postgres valkey >/dev/null 2>&1 || true
	$(CONTAINER_CMD) run --rm \
		--security-opt label=disable \
		-v "$$PWD:/workspace" \
		--network host \
		-w /workspace \
		-u "$$UID:$$GID" \
		-e GOCACHE=/workspace/.cache/go-build \
		-e GOMODCACHE=/workspace/.cache/go-mod \
		-e GOFLAGS=-buildvcs=false \
		-e APP_ENV=test \
		gotrs-toolbox:latest \
		bash -lc 'export PATH=/usr/local/go/bin:$$PATH; export GOFLAGS="-buildvcs=false"; set -e; \
		PKGS="$${INT_PKGS:-./internal/middleware}"; \
		echo "Running integration-tagged tests for packages: $$PKGS"; \
		go test -tags=integration -buildvcs=false -count=1 $$PKGS'

# Run a specific test pattern across all packages
toolbox-test-run:
	@$(MAKE) toolbox-build
	@printf "\n🧪 Running specific test: $(TEST)\n"
	@$(call ensure_caches)
	@$(CONTAINER_CMD) run --rm \
        --security-opt label=disable \
        -v "$$PWD:/workspace" \
		-v gotrs_go_mod_cache:/go/pkg/mod \
		-v gotrs_go_build_cache:/home/appuser/.cache/go-build \
		-w /workspace \
		--network host \
		-e GOCACHE=/home/appuser/.cache/go-build \
		-e GOMODCACHE=/go/pkg/mod \
		-e DB_HOST=$(DB_HOST) -e DB_PORT=$(DB_PORT) \
		-e DB_NAME=gotrs_test -e DB_USER=gotrs_test -e DB_PASSWORD=gotrs_test_password \
		-e VALKEY_HOST=$(VALKEY_HOST) -e VALKEY_PORT=$(VALKEY_PORT) \
		-e APP_ENV=test \
		gotrs-toolbox:latest \
		bash -lc 'export PATH=/usr/local/go/bin:$$PATH; go test -v -run "$(TEST)" ./...'

# Run static analysis using staticcheck inside toolbox
toolbox-staticcheck:
	@$(MAKE) toolbox-build
	@printf "\n🔎 Running staticcheck in toolbox...\n"
	@$(call ensure_caches)
	$(CONTAINER_CMD) run --rm \
		--security-opt label=disable \
		-v "$$PWD:/workspace" \
		-w /workspace \
		-u "$$UID:$$GID" \
		-e GOCACHE=/workspace/.cache/go-build \
		-e GOMODCACHE=/workspace/.cache/go-mod \
		-e GOFLAGS=-buildvcs=false \
		gotrs-toolbox:latest \
		bash -lc 'set -e; export PATH=/usr/local/go/bin:/usr/local/bin:$$PATH; export GOFLAGS="-buildvcs=false"; go version; staticcheck -version; \
		PKGS=$$(go list ./... | rg -v "^(github.com/gotrs-io/gotrs-ce/(tests/e2e))"); \
		echo "Staticchecking packages:"; echo "$$PKGS" | tr "\n" " "; echo; \
		GOTOOLCHAIN=local staticcheck -f=stylish -checks=all,-U1000,-ST1000,-ST1003,-SA9003,-ST1020,-ST1021,-ST1022,-ST1023 $$PKGS'

# Run a specific Go file
toolbox-run-file:
	@$(MAKE) toolbox-build
	@printf "\n🚀 Running Go file: $(FILE)\n"
	@$(call ensure_caches)
	@$(CONTAINER_CMD) run --rm \
        --security-opt label=disable \
        -v "$$PWD:/workspace" \
		-w /workspace \
		-u "$$UID:$$GID" \
		--network host \
		-e GOCACHE=/workspace/.cache/go-build \
		-e GOMODCACHE=/workspace/.cache/go-mod \
		-e DB_HOST=postgres -e DB_PORT=$(DB_PORT) \
		-e DB_NAME=$(DB_NAME) -e DB_USER=$(DB_USER) \
		-e PGPASSWORD=$(DB_PASSWORD) \
		-e VALKEY_HOST=$(VALKEY_HOST) -e VALKEY_PORT=$(VALKEY_PORT) \
		-e APP_ENV=development \
		gotrs-toolbox:latest \
		bash -lc 'export PATH=/usr/local/go/bin:$$PATH && go run $(FILE)'

# Run anti-gaslighting detector in toolbox container
toolbox-antigaslight:
	@$(MAKE) toolbox-build
	@printf "🔍 Running anti-gaslighting detector in container...\n"
	@$(CONTAINER_CMD) run --rm \
		-v "$$(pwd):/workspace" \
		-w /workspace \
		-u "$$(id -u):$$(id -g)" \
		--network host \
		-e DB_HOST=localhost \
		-e DB_PORT=5432 \
		-e DB_NAME=gotrs \
		-e DB_USER=gotrs_user \
		-e PGPASSWORD=$${DB_PASSWORD:-gotrs_password} \
		-e VALKEY_HOST=localhost \
		-e VALKEY_PORT=6388 \
		-e APP_ENV=development \
		gotrs-toolbox:latest \
		sh -c "source .env 2>/dev/null || true && ./scripts/anti-gaslighting-detector.sh detect"

# Run linting with toolbox
toolbox-lint:
	@$(MAKE) toolbox-build
	@printf "🔍 Running linters...\n"
	@$(CONTAINER_CMD) run --rm \
		-v "$$(pwd):/workspace" \
		-w /workspace \
		-u "$$(id -u):$$(id -g)" \
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
		yamllint routes/*.yaml config/*.yaml docker-compose*.yml .github/**/*.yaml 2>/dev/null || echo "⚠️  yamllint found issues or no YAML files found"

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
		aquasec/trivy:latest \
		fs --severity CRITICAL,HIGH,MEDIUM /workspace

# Run Trivy on built images
trivy-images:
	@printf "🔍 Scanning backend image...\n"
	@$(CONTAINER_CMD) run --rm \
		-v /var/run/docker.sock:/var/run/docker.sock \
		aquasec/trivy:latest \
		image gotrs-backend:latest
	@printf "🔍 Scanning frontend image...\n"
	@$(CONTAINER_CMD) run --rm \
		-v /var/run/docker.sock:/var/run/docker.sock \
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

# Start all services (and clean host binaries after build)
up:
	$(call check_compose)
	@if [ "$(DB_DRIVER)" = "postgres" ]; then \
		$(COMPOSE_CMD) up $(COMPOSE_UP_FLAGS) --build postgres valkey backend; \
	else \
		$(COMPOSE_CMD) up $(COMPOSE_UP_FLAGS) --build mariadb valkey backend; \
	fi
	@printf "🧹 Cleaning host binaries after container build...\n"
	@rm -f bin/goats bin/gotrs bin/server bin/migrate bin/generator bin/gotrs-migrate bin/schema-discovery 2>/dev/null || true
	@printf "✅ Host binaries cleaned - containers have the only copies\n"
# Start in background (and clean host binaries after build)
up-d:
	@if [ "$(DB_DRIVER)" = "postgres" ]; then \
		$(COMPOSE_CMD) up -d --build postgres valkey backend; \
	else \
		$(COMPOSE_CMD) up -d --build mariadb valkey backend; \
	fi
	@printf "🧹 Cleaning host binaries after container build...\n"
	@rm -f bin/goats bin/gotrs bin/server bin/migrate bin/generator bin/gotrs-migrate bin/schema-discovery 2>/dev/null || true
	@printf "✅ Host binaries cleaned - containers have the only copies\n"
# Stop all services
down:
	$(call check_compose)
	$(COMPOSE_CMD) down

# Restart services
restart: down up-d
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

# Load environment variables from .env file (only if it exists)
-include .env
export

# Sensible DB defaults for current compose (MariaDB)
DB_DRIVER ?= mariadb
DB_HOST   ?= mariadb
DB_PORT   ?= 3306
DB_NAME   ?= otrs
DB_USER   ?= otrs
DB_PASSWORD ?= LetClaude.1n

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

# Fix PostgreSQL sequences after data import (PostgreSQL only)
db-fix-sequences:
	@if [ "$(DB_DRIVER)" = "postgres" ]; then \
		@printf "🔧 Fixing database sequences...\n"; \
		@./scripts/fix-sequences.sh; \
		@printf "✅ Sequences fixed - duplicate key errors should be resolved\n"; \
	else \
		@printf "ℹ️  Sequence fixing is only needed for PostgreSQL databases\n"; \
	fi
# Run a database query (use QUERY="SELECT ..." make db-query)
db-query:
	@if [ -z "$(QUERY)" ]; then \
		echo "Usage: make db-query QUERY=\"SELECT * FROM table\""; \
		exit 1; \
	fi; \
	if [ "$(DB_DRIVER)" = "postgres" ]; then \
		$(COMPOSE_CMD) --profile toolbox run --rm -T toolbox psql -h $(DB_HOST) -U $(DB_USER) -d $(DB_NAME) -t -c "$(QUERY)"; \
	else \
		$(COMPOSE_CMD) --profile toolbox run --rm -T toolbox mysql -h $(DB_HOST) -u $(DB_USER) -p$(DB_PASSWORD) -D $(DB_NAME) -e "$(QUERY)"; \
	fi

db-migrate:
	@printf "Running database migrations...\n"
	@if [ "$(DB_DRIVER)" = "postgres" ]; then \
		$(COMPOSE_CMD) exec backend ./migrate -path /app/migrations -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" up; \
	else \
		$(COMPOSE_CMD) exec backend ./migrate -path /app/migrations -database "mysql://$(DB_USER):$(DB_PASSWORD)@tcp(mariadb:3306)/$(DB_NAME)" up; \
	fi
	@printf "Migrations completed successfully!\n"
	@if [ "$(DB_DRIVER)" = "postgres" ]; then \
		@printf "🔧 Fixing database sequences to prevent duplicate key errors...\n"; \
		@./scripts/fix-sequences.sh > /dev/null 2>&1 || true; \
		@printf "✅ Database ready with sequences properly synchronized!\n"; \
	fi
db-migrate-schema-only:
	@printf "Running schema migration only...\n"
	@if [ "$(DB_DRIVER)" = "postgres" ]; then \
		$(COMPOSE_CMD) exec backend ./migrate -path /app/migrations -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" up 3; \
	else \
		$(COMPOSE_CMD) exec backend ./migrate -path /app/migrations -database "mysql://$(DB_USER):$(DB_PASSWORD)@tcp(mariadb:3306)/$(DB_NAME)" up 3; \
	fi
	@printf "Schema and initial data applied (no test data)\n"
	@if [ "$(DB_DRIVER)" = "postgres" ]; then \
		@printf "🔧 Fixing database sequences...\n"; \
		@./scripts/fix-sequences.sh > /dev/null 2>&1 || true; \
		@printf "✅ Sequences synchronized!\n"; \
	fi
db-seed-dev:
	@printf "Seeding development database with comprehensive test data...\n"
	@$(COMPOSE_CMD) exec backend ./migrate -path /app/migrations -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" up
	@printf "🔧 Fixing sequences after seeding...\n"
	@./scripts/fix-sequences.sh > /dev/null 2>&1 || true
	@printf "✅ Development database seeded with:\n"
	@printf "   - 10 organizations\n"
	@printf "   - 50 customer users\n"
	@printf "   - 15 support agents\n"
	@printf "   - 100 ITSM tickets\n"
	@printf "   - Knowledge base articles\n"
db-seed-test:
	@printf "Seeding test database with comprehensive test data...\n"
	@$(COMPOSE_CMD) exec backend ./migrate -path /app/migrations -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$${DB_NAME}_test?sslmode=disable" up
	@printf "✅ Test database ready for testing\n"
db-reset-dev:
	@printf "⚠️  This will DELETE all data and recreate the development database!\n"
	@echo -n "Are you sure? [y/N]: "; \
	read confirm; \
	if [ "$$confirm" = "y" ] || [ "$$confirm" = "Y" ]; then \
		echo "Resetting development database..."; \
		$(COMPOSE_CMD) exec backend migrate -path /app/migrations -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" down -all; \
		$(COMPOSE_CMD) exec backend migrate -path /app/migrations -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" up; \
		$(MAKE) clean-storage; \
		echo "✅ Fresh development environment ready with test data!"; \
	else \
		echo "Reset cancelled."; \
	fi

db-reset-test:
	@printf "Resetting test database...\n"
	@$(COMPOSE_CMD) exec backend migrate -path /app/migrations -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$${DB_NAME}_test?sslmode=disable" down -all
	@$(COMPOSE_CMD) exec backend migrate -path /app/migrations -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$${DB_NAME}_test?sslmode=disable" up
	@$(MAKE) clean-storage
	@printf "✅ Test database reset with fresh test data\n"
db-refresh: db-reset-dev
	@printf "✅ Database refreshed for new development cycle\n"
db-rollback:
	$(COMPOSE_CMD) exec backend migrate -path /app/migrations -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" down 1

# Fast database initialization from baseline (new approach)
db-init:
	@printf "🚀 Initializing database (fast path)...\n"
	@if [ "$(DB_DRIVER)" = "postgres" ]; then \
		$(COMPOSE_CMD) exec -T postgres psql -U $(DB_USER) -d $(DB_NAME) -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"; \
		$(COMPOSE_CMD) exec -T postgres psql -U $(DB_USER) -d $(DB_NAME) -f - < schema/baseline/otrs_complete.sql; \
		$(COMPOSE_CMD) exec -T postgres psql -U $(DB_USER) -d $(DB_NAME) -f - < schema/baseline/required_lookups.sql; \
		$(MAKE) clean-storage; \
		printf "🔧 Fixing sequences after baseline initialization...\n"; \
		./scripts/fix-sequences.sh > /dev/null 2>&1 || true; \
		printf "✅ Database initialized from baseline (Postgres)\n"; \
	else \
		printf "📡 Starting dependencies (mariadb)...\n"; \
		$(COMPOSE_CMD) up -d mariadb >/dev/null 2>&1 || true; \
		printf "🧰 Ensuring minimal users table exists (MariaDB)...\n"; \
		$(CONTAINER_CMD) run --rm --network gotrs-ce_gotrs-network \
			-e DB_DRIVER=$(DB_DRIVER) -e DB_HOST=$(DB_HOST) -e DB_PORT=$(DB_PORT) \
			-e DB_NAME=$(DB_NAME) -e DB_USER=$(DB_USER) -e DB_PASSWORD=$(DB_PASSWORD) \
			gotrs-toolbox:latest \
			gotrs reset-user --username="root@localhost" --password="Admin123!1" --enable; \
		printf "✅ Database initialized (MariaDB minimal schema; root user created).\n"; \
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
	@./scripts/fix-sequences.sh > /dev/null 2>&1 || true
	@printf "✅ Development database ready (admin/admin)\n"
# Legacy reset using old migrations (kept for compatibility)
db-reset-legacy:
	$(COMPOSE_CMD) exec backend migrate -path /app/migrations/legacy -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" down -all
	$(COMPOSE_CMD) exec backend migrate -path /app/migrations/legacy -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" up

# New reset using baseline
db-reset: db-init-dev

db-status:
	$(COMPOSE_CMD) exec backend ./migrate -path /app/migrations -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" version

db-force:
	@echo -n "Force migration to version: "; \
	read version; \
	$(COMPOSE_CMD) exec backend ./migrate -path /app/migrations -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" force $$version

# Apply SQL migrations directly via psql
db-migrate-sql:
	@printf "📄 Applying SQL migrations directly...\n"
	@for f in migrations/*.up.sql; do \
		echo "  Running $$(basename $$f)..."; \
		$(COMPOSE_CMD) exec -T postgres psql -U $(DB_USER) -d $(DB_NAME) -f - < "$$f" 2>&1 | grep -E "(CREATE|ALTER|INSERT|ERROR)" | head -3 || true; \
	done
	@printf "✅ SQL migrations applied\n"
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
		golang:1.23-alpine \
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
	@echo -n "Username: "; \
	read username; \
	echo -n "New password: "; \
	stty -echo; read password; stty echo; \
	echo ""; \
	echo "🔑 Resetting password for user: $$username"; \
	$(CONTAINER_CMD) run --rm \
		-v "$$(pwd):/workspace" \
		-w /workspace \
		-u "$$(id -u):$$(id -g)" \
		--network gotrs-ce_gotrs-network \
		-e DB_DRIVER=$${DB_DRIVER:-mariadb} \
		-e DB_HOST=$${DB_HOST:-mariadb} \
		-e DB_PORT=$${DB_PORT:-3306} \
		-e DB_NAME=$${DB_NAME:-otrs} \
		-e DB_USER=$${DB_USER:-otrs} \
		-e DB_PASSWORD=$${DB_PASSWORD:-LetClaude.1n} \
		gotrs-toolbox:latest \
		gotrs reset-user --username="$$username" --password="$$password" --enable


# Valkey CLI
valkey-cli:
	$(COMPOSE_CMD) exec valkey valkey-cli

# i18n Tools
babelfish:
	@printf "Building gotrs-babelfish...\n"	$(COMPOSE_CMD) exec backend go build -o /tmp/bin/gotrs-babelfish cmd/gotrs-babelfish/main.go
	@printf "✨ gotrs-babelfish built successfully!\n"
	@printf "Run it with: docker exec gotrs-backend /tmp/gotrs-babelfish\n"
babelfish-run:
	@$(COMPOSE_CMD) exec backend go run cmd/gotrs-babelfish/main.go $(ARGS)

babelfish-coverage:
	@$(COMPOSE_CMD) exec backend go run cmd/gotrs-babelfish/main.go -action=coverage

babelfish-validate:
	@$(COMPOSE_CMD) exec backend go run cmd/gotrs-babelfish/main.go -action=validate -lang=$(LANG)

babelfish-missing:
	@$(COMPOSE_CMD) exec backend go run cmd/gotrs-babelfish/main.go -action=missing -lang=$(LANG)

test-short:
	$(COMPOSE_CMD) exec -e DB_NAME=$${DB_NAME:-gotrs}_test -e APP_ENV=test backend go test -short ./...

test-coverage:
	@printf "Running test coverage analysis...\n"
	@printf "Using test database: $${DB_NAME:-gotrs}_test\n"
	@mkdir -p generated
	$(COMPOSE_CMD) exec -e DB_NAME=$${DB_NAME:-gotrs}_test -e APP_ENV=test backend sh -c "mkdir -p generated && go test -v -race -coverprofile=generated/coverage.out -covermode=atomic ./..."
	$(COMPOSE_CMD) exec backend go tool cover -func=generated/coverage.out

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

test-coverage-html:
	@mkdir -p generated
	$(COMPOSE_CMD) exec -e DB_NAME=$${DB_NAME:-gotrs}_test -e APP_ENV=test backend sh -c "mkdir -p generated && go test -v -race -coverprofile=generated/coverage.out -covermode=atomic ./..."
	$(COMPOSE_CMD) exec backend sh -c "go tool cover -html=generated/coverage.out -o generated/coverage.html"
	$(COMPOSE_CMD) cp backend:/app/generated/coverage.html ./generated/coverage.html
	@printf "Coverage report generated: generated/coverage.html\n"
# Frontend test commands
test-frontend:
	@printf "Running frontend tests...\n"	$(COMPOSE_CMD) exec frontend npm test

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
	# Prefer explicit BASE_URL provided on invocation; ignore .env for this target
	@if [ -n "$(BASE_URL)" ]; then echo "[playwright-go] (explicit) BASE_URL=$(BASE_URL)"; else echo "[playwright-go] (default) BASE_URL=$${BASE_URL:-http://localhost:8080}"; fi
	# Allow overriding network (e.g. PLAYWRIGHT_NETWORK=gotrs-ce_default) to access compose service DNS
	@if [ -n "$(PLAYWRIGHT_NETWORK)" ]; then echo "[playwright-go] Using network '$(PLAYWRIGHT_NETWORK)'"; else echo "[playwright-go] Using host network (override with PLAYWRIGHT_NETWORK=...)"; fi
	$(CONTAINER_CMD) run --rm \
		--security-opt label=disable \
		-v "$$PWD:/workspace" \
		-w /workspace \
		$$( [ -n "$(PLAYWRIGHT_NETWORK)" ] && printf -- "--network $(PLAYWRIGHT_NETWORK)" || printf -- "--network host" ) \
		-e BASE_URL=$(BASE_URL) \
		-e RAW_BASE_URL=$(BASE_URL) \
		-e DEMO_ADMIN_EMAIL=$(DEMO_ADMIN_EMAIL) \
		-e DEMO_ADMIN_PASSWORD=$(DEMO_ADMIN_PASSWORD) \
		 gotrs-playwright-go:latest bash -lc "go test -v ./tests/e2e/playwright $${ARGS}"

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
		playwright go test ./tests/e2e/... -v -watch

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
		playwright go test ./tests/e2e/... -v

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

# Scan for vulnerabilities with Trivy
scan-vulnerabilities:
	@printf "Scanning for vulnerabilities...\n"
	@$(CONTAINER_CMD) run --rm \
		-v "$$(pwd):/workspace" \
		-w /workspace \
		aquasec/trivy:latest \
		fs --scanners vuln,secret,misconfig . \
		--severity HIGH,CRITICAL

# Run all security scans
security-scan: scan-secrets scan-vulnerabilities
	@printf "Security scanning completed!\n"
# Build for production (includes CSS, JS and container build)
build: pre-build frontend-build
	@printf "🔨 Building backend container...\n" \
		&& $(CONTAINER_CMD) build -f Dockerfile -t gotrs:latest .
	@printf "🧹 Cleaning host binaries...\n"
	@rm -f goats gotrs gotrs-* generator migrate server  # Clean root directory
	@rm -f bin/* 2>/dev/null || true  # Clean bin directory
	@printf "✅ Build complete - CSS and JS compiled, containers ready\n"

.PHONY: pre-build generate-route-map validate-routes

# Pre-build chain: ensure API map + static route audit executed every build
pre-build: generate-route-map validate-routes

generate-route-map:
	@printf "📡 Generating API map artifacts...\n"
	@$(CONTAINER_CMD) run --rm -v "$$PWD:/workspace" -w /workspace --user 1000 alpine:3.19 \
		sh -c 'apk add --no-cache jq graphviz >/dev/null 2>&1 || true; sh scripts/api_map.sh >/dev/null 2>&1 || true'
	@printf "   API map complete (runtime/api-map.*)\n"

validate-routes:
	@printf "🔍 Validating no new static routes...\n"
	@$(CONTAINER_CMD) run --rm -v "$$PWD:/workspace" -w /workspace --user 1000 alpine:3.19 \
		sh -c 'sh scripts/validate_routes.sh' || { echo "Route validation failed"; exit 1; }
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
build-cached:
	@printf "🚀 Building backend image (cache flags disabled for podman compatibility)...\n"	$(CONTAINER_CMD) build -t gotrs:latest .
	@$(CONTAINER_CMD) build -t gotrs:latest .
	@printf "✅ Build complete\n"
# Security scan build (CI/CD)
build-secure:
	@printf "🔒 Building with security scanning...\n"	$(CONTAINER_CMD) build \
		--target security \
		--output type=local,dest=./security-reports \
		.
	@printf "📊 Security reports saved to ./security-reports/\n"
# Multi-platform build (AMD64 and ARM64)
build-multi:
	@printf "🌍 Building for multiple platforms...\n"	$(CONTAINER_CMD) buildx build \
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
build-clean:
	@printf "🧹 Clean build without cache...\n"	$(CONTAINER_CMD) build --no-cache -t gotrs:latest .
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
		--cache-from gotrs-tests:latest \
		--build-arg BUILDKIT_INLINE_CACHE=1 \
		-f Dockerfile.tests -t gotrs-tests:latest .
	@$(CONTAINER_CMD) build \
		--cache-from gotrs-route-tools:latest \
		--build-arg BUILDKIT_INLINE_CACHE=1 \
		-f Dockerfile.route-tools -t gotrs-route-tools:latest .
	@$(CONTAINER_CMD) build \
		--cache-from gotrs-goatkit:latest \
		--build-arg BUILDKIT_INLINE_CACHE=1 \
		-f Dockerfile.goatkit -t gotrs-goatkit:latest .
	@$(CONTAINER_CMD) build \
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
	@printf "Starting phpLDAPadmin...\n"	$(COMPOSE_CMD) --profile tools up -d phpldapadmin
	@printf "Opening phpLDAPadmin at http://localhost:8091\n"
	@printf "Login with:\n"
	@printf "  Login DN: cn=admin,dc=gotrs,dc=local\n"
	@printf "  Password: admin123\n"
	@open http://localhost:8091 || xdg-open http://localhost:8091 || echo "Open http://localhost:8091"

# View OpenLDAP logs
ldap-logs:
	$(COMPOSE_CMD) logs -f openldap

# Setup LDAP for development (start services and wait)
ldap-setup:
	@printf "Setting up LDAP development environment...\n"	$(COMPOSE_CMD) up -d openldap
	@printf "Waiting for LDAP server to initialize (this may take up to 60 seconds)...\n"
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
	@printf "\n"
	@printf "LDAP Server Configuration:\n"
	@printf "=========================\n"
	@printf "Host: localhost:389\n"
	@printf "Base DN: dc=gotrs,dc=local\n"
	@printf "Admin DN: cn=admin,dc=gotrs,dc=local\n"
	@printf "Admin Password: admin123\n"
	@printf "Readonly DN: cn=readonly,dc=gotrs,dc=local\n"
	@printf "Readonly Password: readonly123\n"
	@printf "\n"
	@printf "Test Users (password: password123):\n"
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
	@printf "Testing LDAP connectivity...\n"	$(COMPOSE_CMD) exec openldap ldapsearch -x -H ldap://localhost \
		-D "cn=admin,dc=gotrs,dc=local" -w "admin123" \
		-b "dc=gotrs,dc=local" \
		"(objectclass=*)" dn | head -20

# Test that all Makefile commands are properly containerized
.PHONY: test-containerized
test-containerized:
	@bash scripts/test-containerized.sh

# Include task coordination system
include task-coordination.mk

# CSS Build Commands
.PHONY: npm-updates css-deps css-build css-watch

npm-updates:
	@printf "📦 Updating NPM dependencies...\n"
	@if echo "$(COMPOSE_CMD)" | grep -q "podman-compose"; then \
		COMPOSE_PROFILES=toolbox $(COMPOSE_CMD) run --rm toolbox sh -c 'cd /workspace && npx npm-check-updates -u && npm install'; \
	else \
		$(COMPOSE_CMD) --profile toolbox run --rm toolbox sh -c 'cd /workspace && npx npm-check-updates -u && npm install'; \
	fi
	@printf "✅ NPM dependencies updated\n"
# Install CSS build dependencies (in container with user permissions)
css-deps:
	@printf "📦 Installing CSS build dependencies...\n"
	@if echo "$(COMPOSE_CMD)" | grep -q "podman-compose"; then \
		COMPOSE_PROFILES=toolbox $(COMPOSE_CMD) run --rm toolbox sh -c 'cd /workspace && export NPM_CONFIG_CACHE=/tmp/npm-cache && mkdir -p $$NPM_CONFIG_CACHE && npm install && touch .frontend_deps_installed || true'; \
	else \
		$(COMPOSE_CMD) --profile toolbox run --rm toolbox sh -c 'cd /workspace && export NPM_CONFIG_CACHE=/tmp/npm-cache && mkdir -p $$NPM_CONFIG_CACHE && npm install && touch .frontend_deps_installed || true'; \
	fi
	@printf "✅ CSS dependencies installed\n"
# Install CSS dependencies without upgrading (preserves pinned versions)
css-deps-stable:
	@printf "📦 Installing CSS build dependencies (stable versions)...\n"
	@if echo "$(COMPOSE_CMD)" | grep -q "podman-compose"; then \
		COMPOSE_PROFILES=toolbox $(COMPOSE_CMD) run --rm toolbox sh -c 'cd /workspace && if [ ! -d node_modules/tailwindcss ]; then echo "🧹 Cleaning existing node_modules (fresh install)"; rm -rf node_modules 2>/dev/null || true; fi; export NPM_CONFIG_CACHE=/tmp/npm-cache && mkdir -p $$NPM_CONFIG_CACHE && cp package-lock.json /tmp/lock.json 2>/dev/null || true && npm install --no-audit --no-fund --no-save && touch .frontend_deps_installed || true'; \
	else \
		$(COMPOSE_CMD) --profile toolbox run --rm toolbox sh -c 'cd /workspace && if [ ! -d node_modules/tailwindcss ]; then echo "🧹 Cleaning existing node_modules (fresh install)"; rm -rf node_modules 2>/dev/null || true; fi; export NPM_CONFIG_CACHE=/tmp/npm-cache && mkdir -p $$NPM_CONFIG_CACHE && cp package-lock.json /tmp/lock.json 2>/dev/null || true && npm install --no-audit --no-fund --no-save && touch .frontend_deps_installed || true'; \
	fi
	@printf "✅ CSS dependencies installed\n"
# Build production CSS (in container with user permissions)
css-build:
	@printf "🎨 Building production CSS...\n"
	@if [ ! -d "node_modules/tailwindcss" ]; then \
		echo "📦 Installing CSS dependencies first (tailwindcss not present)..."; \
		$(MAKE) css-deps-stable; \
	fi
	@if echo "$(COMPOSE_CMD)" | grep -q "podman-compose"; then \
		COMPOSE_PROFILES=toolbox $(COMPOSE_CMD) run --rm toolbox sh -c 'cd /workspace && npm run build-css'; \
	else \
		$(COMPOSE_CMD) --profile toolbox run --rm toolbox sh -c 'cd /workspace && npm run build-css'; \
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
		COMPOSE_PROFILES=toolbox $(COMPOSE_CMD) run --rm toolbox sh -c 'cd /workspace && npm run build-tiptap'; \
	else \
		$(COMPOSE_CMD) --profile toolbox run --rm toolbox sh -c 'cd /workspace && npm run build-tiptap'; \
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
	@printf "🧹 Removing node_modules named volume (gotrs_node_modules)...\n"
	@$(CONTAINER_CMD) volume rm -f gotrs_node_modules >/dev/null 2>&1 || true
	@rm -f .frontend_deps_installed 2>/dev/null || true
	@printf "✅ Volume removed. Next build will reinstall dependencies.\n"

.PHONY: frontend-reset-deps
frontend-reset-deps: frontend-clean-cache
	@printf "🔄 Forcing fresh dependency install next build (volume + sentinel cleared)\\n"

.PHONY: frontend-perms-fix
frontend-perms-fix:
	@printf "🔧 Normalizing frontend file permissions...\n"
	@find static/js -maxdepth 1 -type f -name 'tiptap.min.js' -exec $(CONTAINER_CMD) run --rm -v "$$PWD:/workspace" -w /workspace --user 0 alpine:3.19 sh -c 'chown 1000:1000 /workspace/{} || true' \;
	@chmod ug+rw static/js 2>/dev/null || true
	@printf "✅ Permissions normalized (owner=uid 1000 for built assets where possible)\n"
# Watch and rebuild CSS on changes (in container with user permissions)
css-watch: css-deps
	@printf "👁️  Watching for CSS changes...\n"
	@$(CONTAINER_CMD) run --rm -it --security-opt label=disable -u $(shell id -u):$(shell id -g) -v $(PWD):/app -w /app node:20-alpine npm run watch-css

# Add these commands after the existing TDD section around line 178:

#########################################
# COMPREHENSIVE TDD AUTOMATION
#########################################

# Initialize comprehensive TDD environment
tdd-comprehensive-init:
	@printf "🚀 Initializing comprehensive TDD environment...\n"
	@./scripts/comprehensive-tdd-integration.sh init

# Run comprehensive TDD verification with ALL quality gates (containerized)
tdd-comprehensive:
	@printf "🧪 Running COMPREHENSIVE TDD verification (containerized)...\n"
	@mkdir -p generated/tdd-comprehensive generated/evidence generated/test-results || true
	@$(CONTAINER_CMD) run --rm \
		--security-opt label=disable \
		-v "$$PWD:/workspace" \
		-v "$$PWD/generated:/workspace/generated" \
		-w /workspace \
		--network host \
		-u 0 \
		gotrs-toolbox:latest \
		bash -lc 'bash scripts/tdd-comprehensive.sh comprehensive || true; echo "See generated/evidence for report"'

# Anti-gaslighting detection - prevents false success claims
anti-gaslighting:
	@printf "🚨 Running anti-gaslighting detection...\n"
	@printf "Detecting premature success claims and hidden failures...\n"
	@./scripts/anti-gaslighting-detector.sh detect

# Initialize test-first TDD cycle with proper enforcement
tdd-test-first-init:
	@if [ -z "$(FEATURE)" ]; then \
		echo "Error: FEATURE required. Usage: make tdd-test-first-init FEATURE='Feature Name'"; \
		exit 1; \
	fi
	@printf "🔴 Initializing test-first TDD cycle for: $(FEATURE)\n"
	@./scripts/tdd-test-first-enforcer.sh init "$(FEATURE)"

# Generate failing test for TDD cycle
tdd-generate-test:
	@if [ ! -f .tdd-state ]; then \
		echo "Error: TDD not initialized. Run 'make tdd-test-first-init FEATURE=name' first"; \
		exit 1; \
	fi
	@printf "📝 Generating failing test...\n"
	@printf "Test types: unit, integration, api, browser\n"
	@read -p "Enter test type (default: unit): " test_type; \
	test_type=$${test_type:-unit}; \
	./scripts/tdd-test-first-enforcer.sh generate-test $$test_type

# Verify test is actually failing (TDD enforcement)
tdd-verify-failing:
	@if [ -z "$(TEST_FILE)" ]; then \
		echo "Error: TEST_FILE required. Usage: make tdd-verify-failing TEST_FILE=path/to/test.go"; \
		exit 1; \
	fi
	@printf "🔍 Verifying test actually fails...\n"
	@./scripts/tdd-test-first-enforcer.sh verify-failing "$(TEST_FILE)"

# Verify tests now pass after implementation
tdd-verify-passing:
	@if [ -z "$(TEST_FILE)" ]; then \
		echo "Error: TEST_FILE required. Usage: make tdd-verify-passing TEST_FILE=path/to/test.go"; \
		exit 1; \
	fi
	@printf "✅ Verifying tests now pass...\n"
	@./scripts/tdd-test-first-enforcer.sh verify-passing "$(TEST_FILE)"

# Complete guided TDD cycle with comprehensive verification
tdd-full-cycle:
	@if [ -z "$(FEATURE)" ]; then \
		echo "Error: FEATURE required. Usage: make tdd-full-cycle FEATURE='Feature Name'"; \
		exit 1; \
	fi
	@printf "🔄 Starting full TDD cycle for: $(FEATURE)\n"
	@./scripts/comprehensive-tdd-integration.sh full-cycle "$(FEATURE)"

# Quick verification for development (fast feedback)
tdd-quick:
	@printf "⚡ Running quick TDD verification...\n"
	@./scripts/comprehensive-tdd-integration.sh quick

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

# Show TDD dashboard with current status and metrics
tdd-dashboard:
	@./scripts/comprehensive-tdd-integration.sh dashboard

# Enhanced test command that integrates with comprehensive TDD
test-comprehensive:
	@printf "🧪 Running tests with comprehensive TDD integration...\n"
	@if [ -f .tdd-state ]; then \
		echo "TDD cycle active - running comprehensive verification..."; \
		$(MAKE) tdd-comprehensive; \
	else \
		echo "No TDD cycle - running comprehensive test suite..."; \
		./scripts/tdd-comprehensive.sh comprehensive; \
	fi

# Test-first enforcement (prevents implementation without failing test)
test-enforce-first:
	@printf "🚫 Enforcing test-first development...\n"
	@if [ ! -f .tdd-state ]; then \
		echo "Error: No TDD cycle active. Start with 'make tdd-test-first-init FEATURE=name'"; \
		exit 1; \
	fi
	@./scripts/tdd-test-first-enforcer.sh check-implementation

# Generate comprehensive TDD report
tdd-report:
	@printf "📊 Generating comprehensive TDD report...\n"
	@./scripts/tdd-test-first-enforcer.sh report

# Clean TDD state (reset cycle)
tdd-clean:
	@printf "🧹 Cleaning TDD state...\n"
	@rm -f .tdd-state
	@printf "TDD cycle reset. Start new cycle with 'make tdd-test-first-init FEATURE=name'\n"
# Verify system integrity (prevents gaslighting)
verify-integrity:
	@printf "🔍 Verifying system integrity...\n"
	@printf "Checking for false success claims and hidden failures...\n"
	@./scripts/anti-gaslighting-detector.sh detect
	@printf "Running comprehensive verification...\n"
	@./scripts/tdd-comprehensive.sh comprehensive

# TDD pre-commit hook (runs before commits)
tdd-pre-commit:
	@printf "🔒 Running TDD pre-commit verification...\n"
	@./scripts/anti-gaslighting-detector.sh quick
	@if [ -f .tdd-state ]; then \
		echo "TDD cycle active - verifying cycle state..."; \
		./scripts/tdd-test-first-enforcer.sh status; \
	fi

#########################################
# EVIDENCE-BASED VERIFICATION OVERRIDES
#########################################

# Override existing test command to be more robust
test: test-comprehensive

# Override existing tdd-verify to use comprehensive verification
tdd-verify: tdd-comprehensive
