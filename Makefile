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
	@echo ""
	@echo "    $(shell echo '\033[1;36m')🐐 GOTRS - Go Open Ticketing Resource System$(shell echo '\033[0m')"
	@echo ""
	@echo "    $(shell echo '\033[0;90m')                           ///////                ";
	@echo "    $(shell echo '\033[0;90m')                     ///////     ////////         ";
	@echo "    $(shell echo '\033[0;90m')                 ////                   /////     ";
	@echo "    $(shell echo '\033[0;90m')               ///             ////////////////   ";
	@echo "    $(shell echo '\033[0;90m')             ///          /////              //// ";
	@echo "    $(shell echo '\033[0;90m')           ///         ///                      //";
	@echo "    $(shell echo '\033[0;90m')        ////        //////        /               ";
	@echo "    $(shell echo '\033[0;90m')       //// ///     /    //////////               ";
	@echo "    $(shell echo '\033[0;90m')       //  / / //        //    // ///             ";
	@echo "    $(shell echo '\033[0;90m')       /// ////                /    ////          ";
	@echo "    $(shell echo '\033[0;90m')      //                               ///        ";
	@echo "    $(shell echo '\033[0;90m')     /                                    ////    ";
	@echo "    $(shell echo '\033[0;90m')   ///                /                      //// ";
	@echo "    $(shell echo '\033[0;90m')  //                  /      /                 /  ";
	@echo "    $(shell echo '\033[0;90m')/////     /           /     //               //   ";
	@echo "    $(shell echo '\033[0;90m')//      ///          //     /               //    ";
	@echo "    $(shell echo '\033[0;90m') //   ///         ////     //              //     ";
	@echo "    $(shell echo '\033[0;90m')  /////       //////      //             ///      ";
	@echo "    $(shell echo '\033[0;90m')   //// ///////    //   ///            ///        ";
	@echo "    $(shell echo '\033[0;90m')      ///           /////            ///          ";
	@echo "    $(shell echo '\033[0;90m')                   ///             ///            ";
	@echo "    $(shell echo '\033[0;90m')                  //           ////               ";
	@echo "    $(shell echo '\033[0;90m')                  /         ////                  ";
	@echo "    $(shell echo '\033[0;90m')                    ////////$(shell echo '\033[0m')";
	@echo ""
	@echo "  $(shell echo '\033[1;35m')━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(shell echo '\033[0m')"
	@echo "  $(shell echo '\033[1;33m')🚀 Core Commands$(shell echo '\033[0m')"
	@echo "  $(shell echo '\033[1;35m')━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(shell echo '\033[0m')"
	@echo ""
	@echo "  $(shell echo '\033[0;32m')make up$(shell echo '\033[0m')                           ▶️ Start all services"
	@echo "  $(shell echo '\033[0;32m')make down$(shell echo '\033[0m')                         🛑 Stop all services"
	@echo "  $(shell echo '\033[0;32m')make logs$(shell echo '\033[0m')                         📋 View logs"
	@echo "  $(shell echo '\033[0;32m')make restart$(shell echo '\033[0m')                      🔄 Restart all services"
	@echo "  $(shell echo '\033[0;32m')make clean$(shell echo '\033[0m')                        🧹 Clean everything (including volumes)"
	@echo "  $(shell echo '\033[0;32m')make setup$(shell echo '\033[0m')                        🎯 Initial project setup with secure secrets"
	@echo "  $(shell echo '\033[0;32m')make build$(shell echo '\033[0m')                        🔨 Build production images"
	@echo ""
	@echo "  $(shell echo '\033[1;35m')━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(shell echo '\033[0m')"
	@echo "  $(shell echo '\033[1;33m')🧪 TDD Workflow (Quality Gates Enforced)$(shell echo '\033[0m')"
	@echo "  $(shell echo '\033[1;35m')━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(shell echo '\033[0m')"
	@echo ""
	@echo "  $(shell echo '\033[0;32m')make tdd-init$(shell echo '\033[0m')                     🏁 Initialize TDD workflow"
	@echo "  $(shell echo '\033[0;32m')make tdd-test-first$(shell echo '\033[0m') FEATURE=name  ❌ Start with failing test"
	@echo "  $(shell echo '\033[0;32m')make tdd-implement$(shell echo '\033[0m')                ✅ Implement to pass tests"
	@echo "  $(shell echo '\033[0;32m')make tdd-verify$(shell echo '\033[0m')                   🔍 Run ALL quality gates"
	@echo "  $(shell echo '\033[0;32m')make tdd-refactor$(shell echo '\033[0m')                 ♻️  Safe refactoring"
	@echo "  $(shell echo '\033[0;32m')make tdd-status$(shell echo '\033[0m')                   📊 Show workflow status"
	@echo "  $(shell echo '\033[0;32m')make quality-gates$(shell echo '\033[0m')                🚦 Run quality checks"
	@echo "  $(shell echo '\033[0;32m')make evidence-report$(shell echo '\033[0m')              📄 Generate evidence"
	@echo ""
	@echo "  $(shell echo '\033[1;35m')━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(shell echo '\033[0m')"
	@echo "  $(shell echo '\033[1;33m')🎨 CSS/Frontend Build$(shell echo '\033[0m')"
	@echo "  $(shell echo '\033[1;35m')━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(shell echo '\033[0m')"
	@echo ""
	@echo "  $(shell echo '\033[0;32m')make css-build$(shell echo '\033[0m')                    📦 Build production CSS from Tailwind"
	@echo "  $(shell echo '\033[0;32m')make css-watch$(shell echo '\033[0m')                    👁️ Watch and rebuild CSS on changes"
	@echo "  $(shell echo '\033[0;32m')make css-deps$(shell echo '\033[0m')                     📥 Install CSS build dependencies"
	@echo ""
	@echo "  $(shell echo '\033[1;35m')━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(shell echo '\033[0m')"
	@echo "  $(shell echo '\033[1;33m')🔐 Secrets Management$(shell echo '\033[0m')"
	@echo "  $(shell echo '\033[1;35m')━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(shell echo '\033[0m')"
	@echo ""
	@echo "  $(shell echo '\033[0;32m')make synthesize$(shell echo '\033[0m')                   🔑 Generate new .env with secure secrets"
	@echo "  $(shell echo '\033[0;32m')make rotate-secrets$(shell echo '\033[0m')               🔄 Rotate secrets in existing .env"
	@echo "  $(shell echo '\033[0;32m')make synthesize-force$(shell echo '\033[0m')             ⚡ Force regenerate .env"
	@echo "  $(shell echo '\033[0;32m')make k8s-secrets$(shell echo '\033[0m')                  🙊 Generate k8s/secrets.yaml"
	@echo "  $(shell echo '\033[0;32m')make show-dev-creds$(shell echo '\033[0m')               👤 Show test user credentials"
	@echo ""
	@echo "  $(shell echo '\033[1;35m')━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(shell echo '\033[0m')"
	@echo "  $(shell echo '\033[1;33m')🐳 Docker/Container Build$(shell echo '\033[0m')"
	@echo "  $(shell echo '\033[1;35m')━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(shell echo '\033[0m')"
	@echo ""
	@echo "  $(shell echo '\033[0;32m')make build-cached$(shell echo '\033[0m')                 🚀 Fast build with caching (70% faster)"
	@echo "  $(shell echo '\033[0;32m')make build-clean$(shell echo '\033[0m')                  🧹 Clean build without cache"
	@echo "  $(shell echo '\033[0;32m')make build-secure$(shell echo '\033[0m')                 🔒 Build with security scanning"
	@echo "  $(shell echo '\033[0;32m')make build-multi$(shell echo '\033[0m')                  🌍 Multi-platform build (AMD64/ARM64)"
	@echo "  $(shell echo '\033[0;32m')make build-all-tools$(shell echo '\033[0m')              🛠️ Build all specialized containers"
	@echo "  $(shell echo '\033[0;32m')make toolbox-build$(shell echo '\033[0m')                🔧 Build development toolbox"
	@echo "  $(shell echo '\033[0;32m')make analyze-size$(shell echo '\033[0m')                 📏 Analyze image size with dive"
	@echo "  $(shell echo '\033[0;32m')make show-sizes$(shell echo '\033[0m')                   📊 Show all image sizes"
	@echo "  $(shell echo '\033[0;32m')make show-cache$(shell echo '\033[0m')                   💾 Display build cache usage"
	@echo "  $(shell echo '\033[0;32m')make clear-cache$(shell echo '\033[0m')                  🗑️ Clear Docker build cache"
	@echo ""
	@echo "  $(shell echo '\033[1;35m')━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(shell echo '\033[0m')"
	@echo "  $(shell echo '\033[1;33m')🔮 Schema Discovery$(shell echo '\033[0m')"
	@echo "  $(shell echo '\033[1;35m')━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(shell echo '\033[0m')"
	@echo ""
	@echo "  $(shell echo '\033[0;32m')make schema-discovery$(shell echo '\033[0m')             🔍 Generate YAML from DB schema"
	@echo "  $(shell echo '\033[0;32m')make schema-table$(shell echo '\033[0m')                 📊 Generate YAML for table"
	@echo ""
	@echo "  $(shell echo '\033[1;35m')━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(shell echo '\033[0m')"
	@echo "  $(shell echo '\033[1;33m')🧰 Toolbox (Fast Container Dev)$(shell echo '\033[0m')"
	@echo "  $(shell echo '\033[1;35m')━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(shell echo '\033[0m')"
	@echo ""
	@echo "  $(shell echo '\033[0;32m')make toolbox-build$(shell echo '\033[0m')                🔨 Build toolbox container"
	@echo "  $(shell echo '\033[0;32m')make toolbox-run$(shell echo '\033[0m')                  🐚 Interactive shell"
	@echo "  $(shell echo '\033[0;32m')make toolbox-test$(shell echo '\033[0m')                 🧪 Run all tests quickly"
	@echo "  $(shell echo '\033[0;32m')make toolbox-test-run$(shell echo '\033[0m')             🎯 Run specific test"
	@echo "  $(shell echo '\033[0;32m')make toolbox-run-file$(shell echo '\033[0m')             📄 Run Go file"
	@echo ""
	@echo "  $(shell echo '\033[1;35m')━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(shell echo '\033[0m')"
	@echo "  $(shell echo '\033[1;33m')🎭 E2E Testing (Playwright)$(shell echo '\033[0m')"
	@echo "  $(shell echo '\033[1;35m')━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(shell echo '\033[0m')"
	@echo ""
	@echo "  $(shell echo '\033[0;32m')make test-e2e$(shell echo '\033[0m')                     🤖 Run E2E tests headless"
	@echo "  $(shell echo '\033[0;32m')make test-e2e-debug$(shell echo '\033[0m')               👀 Tests with visible browser"
	@echo "  $(shell echo '\033[0;32m')make test-e2e-watch$(shell echo '\033[0m')               🔁 Tests in watch mode"
	@echo "  $(shell echo '\033[0;32m')make test-e2e-report$(shell echo '\033[0m')              📊 View test results"
	@echo ""
	@echo "  $(shell echo '\033[1;35m')━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(shell echo '\033[0m')"
	@echo "  $(shell echo '\033[1;33m')🧪 Testing Commands$(shell echo '\033[0m')"
	@echo "  $(shell echo '\033[1;35m')━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(shell echo '\033[0m')"
	@echo ""
	@echo "  $(shell echo '\033[0;32m')make test$(shell echo '\033[0m')                         ✅ Run Go backend tests"
	@echo "  $(shell echo '\033[0;32m')make test-short$(shell echo '\033[0m')                   ⚡ Skip long tests"
	@echo "  $(shell echo '\033[0;32m')make test-coverage$(shell echo '\033[0m')                📈 Tests with coverage"
	@echo "  $(shell echo '\033[0;32m')make test-safe$(shell echo '\033[0m')                    🏃 Race/deadlock detection"
	@echo "  $(shell echo '\033[0;32m')make test-html$(shell echo '\033[0m')                    🌐 HTML test report"
	@echo ""
	@echo "  $(shell echo '\033[1;35m')━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(shell echo '\033[0m')"
	@echo "  $(shell echo '\033[1;33m')🐠 i18n (Babel fish) Commands$(shell echo '\033[0m')"
	@echo "  $(shell echo '\033[1;35m')━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(shell echo '\033[0m')"
	@echo ""
	@echo "  $(shell echo '\033[0;32m')make babelfish$(shell echo '\033[0m')                    🏗️ Build gotrs-babelfish binary"
	@echo "  $(shell echo '\033[0;32m')make babelfish-coverage$(shell echo '\033[0m')           📊 Show translation coverage"
	@echo "  $(shell echo '\033[0;32m')make babelfish-validate$(shell echo '\033[0m\n\t') LANG=de   ✅ Validate a language"
	@echo "  $(shell echo '\033[0;32m')make babelfish-missing$(shell echo '\033[0m\n\t') LANG=es    🔍 Show missing translations"
	@echo "  $(shell echo '\033[0;32m')make babelfish-run$(shell echo '\033[0m\n\t') ARGS='-help'   🎯 Run with custom args"
	@echo "  $(shell echo '\033[0;32m')make test-ldap$(shell echo '\033[0m')                    🔐 Run LDAP integration tests"
	@echo "  $(shell echo '\033[0;32m')make test-ldap-perf$(shell echo '\033[0m')               ⚡ Run LDAP performance benchmarks"
	@echo ""
	@echo "  $(shell echo '\033[1;35m')━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(shell echo '\033[0m')"
	@echo "  $(shell echo '\033[1;33m')🔒 Security Commands$(shell echo '\033[0m')"
	@echo "  $(shell echo '\033[1;35m')━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(shell echo '\033[0m')"
	@echo ""
	@echo "  $(shell echo '\033[0;32m')make scan-secrets$(shell echo '\033[0m')\t\t\t🕵️ Scan current code for secrets"
	@echo "  $(shell echo '\033[0;32m')make scan-secrets-history$(shell echo '\033[0m')\t\t📜 Scan git history for secrets"
	@echo "  $(shell echo '\033[0;32m')make scan-secrets-precommit$(shell echo '\033[0m')\t\t🪝 Install pre-commit hooks"
	@echo "  $(shell echo '\033[0;32m')make scan-vulnerabilities$(shell echo '\033[0m')\t\t🐛 Scan for vulnerabilities"
	@echo "  $(shell echo '\033[0;32m')make security-scan$(shell echo '\033[0m')\t\t\t🛡️ Run all security scans"
	@echo "  $(shell echo '\033[0;32m')make test-contracts$(shell echo '\033[0m')\t\t\t📝 Run Pact contract tests"
	@echo "  $(shell echo '\033[0;32m')make test-all$(shell echo '\033[0m')\t\t\t\t🎯 Run all tests (backend, frontend, contracts)"
	@echo ""
	@echo "  $(shell echo '\033[1;35m')━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(shell echo '\033[0m')"
	@echo "  $(shell echo '\033[1;33m')📡 Service Management$(shell echo '\033[0m')"
	@echo "  $(shell echo '\033[1;35m')━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(shell echo '\033[0m')"
	@echo ""
	@echo "  $(shell echo '\033[0;32m')make backend-logs$(shell echo '\033[0m')\t\t\t📋 View backend logs"
	@echo "  $(shell echo '\033[0;32m')make backend-logs-follow$(shell echo '\033[0m')\t\t📺 Follow backend logs"
	@echo "  $(shell echo '\033[0;32m')make valkey-cli$(shell echo '\033[0m')\t\t\t🔑 Valkey CLI"
	@echo ""
	@echo "  $(shell echo '\033[1;35m')━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(shell echo '\033[0m')"
	@echo "  $(shell echo '\033[1;33m')🗄️  Database Operations$(shell echo '\033[0m')"
	@echo "  $(shell echo '\033[1;35m')━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(shell echo '\033[0m')"
	@echo ""
	@echo "  $(shell echo '\033[0;32m')make db-shell$(shell echo '\033[0m')\t\t\t\t🐘 PostgreSQL shell"
	@echo "  $(shell echo '\033[0;32m')make db-migrate$(shell echo '\033[0m')\t\t\t📤 Run pending migrations"
	@echo "  $(shell echo '\033[0;32m')make db-rollback$(shell echo '\033[0m')\t\t\t↩️  Rollback last migration"
	@echo "  $(shell echo '\033[0;32m')make db-reset$(shell echo '\033[0m')\t\t\t\t💥 Reset DB (cleans storage)"
	@echo "  $(shell echo '\033[0;32m')make db-init$(shell echo '\033[0m')\t\t\t\t🚀 Fast baseline init"
	@echo "  $(shell echo '\033[0;32m')make db-apply-test-data$(shell echo '\033[0m')\t\t🧪 Apply test data"
	@echo "  $(shell echo '\033[0;32m')make clean-storage$(shell echo '\033[0m')\t\t\t🗑️ Remove orphaned files"
	@echo ""
	@echo "  $(shell echo '\033[1;35m')━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(shell echo '\033[0m')"
	@echo "  $(shell echo '\033[1;33m')📦 OTRS Migration$(shell echo '\033[0m')"
	@echo "  $(shell echo '\033[1;35m')━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(shell echo '\033[0m')"
	@echo ""
	@echo "  $(shell echo '\033[0;32m')make migrate-analyze$(shell echo '\033[0m\n\t') SQL=dump.sql\t\t\t🔍 Analyze OTRS SQL dump"
	@echo "  $(shell echo '\033[0;32m')make migrate-import$(shell echo '\033[0m\n\t') SQL=dump.sql\t\t\t📥 Import OTRS data (dry-run)"
	@echo "  $(shell echo '\033[0;32m')make migrate-import$(shell echo '\033[0m\n\t') SQL=dump.sql DRY_RUN=false\t💾 Import for real"
	@echo "  $(shell echo '\033[0;32m')make migrate-validate$(shell echo '\033[0m')\t\t\t🔍 Validate imported data"
	@echo "  $(shell echo '\033[0;32m')make import-test-data$(shell echo '\033[0m')\t\t\t🎯 Import test tickets with proper mapping"
	@echo ""
	@echo "  $(shell echo '\033[1;35m')━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(shell echo '\033[0m')"
	@echo "  $(shell echo '\033[1;33m')👥 User Management$(shell echo '\033[0m')"
	@echo "  $(shell echo '\033[1;35m')━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(shell echo '\033[0m')"
	@echo ""
	@echo "  $(shell echo '\033[0;32m')make reset-password$(shell echo '\033[0m')\t\t\t🔓 Reset user password"
	@echo ""
	@echo "  $(shell echo '\033[1;35m')━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(shell echo '\033[0m')"
	@echo ""
	@echo "  $(shell echo '\033[0;90m')🐐 Happy coding with GOTRS!$(shell echo '\033[0m')"
	@echo "  $(shell echo '\033[0;90m')Container: $(CONTAINER_CMD) | Compose: $(COMPOSE_CMD)$(shell echo '\033[0m')"
	@echo ""

#########################################
# TDD WORKFLOW COMMANDS
#########################################

# Initialize TDD workflow
tdd-init:
	@echo "🧪 Initializing TDD workflow with mandatory quality gates..."
	@./scripts/tdd-enforcer.sh init

# Start TDD cycle with failing test
tdd-test-first:
	@if [ -z "$(FEATURE)" ]; then \
		echo "Error: FEATURE required. Usage: make tdd-test-first FEATURE='Feature Name'"; \
		exit 1; \
	fi
	@echo "🔴 Starting test-first phase for: $(FEATURE)"
	@./scripts/tdd-enforcer.sh test-first "$(FEATURE)"

# Implement code to pass tests
tdd-implement:
	@echo "🔧 Starting implementation phase..."
	@./scripts/tdd-enforcer.sh implement

# Comprehensive verification with all quality gates
tdd-verify:
	@echo "✅ Running comprehensive verification (ALL quality gates must pass)..."
	@./scripts/tdd-enforcer.sh verify

# Safe refactoring with regression checks
tdd-refactor:
	@echo "♻️ Starting refactor phase with regression protection..."
	@./scripts/tdd-enforcer.sh refactor

# Show current TDD workflow status
tdd-status:
	@./scripts/tdd-enforcer.sh status

# Run quality gates independently for debugging
quality-gates:
	@echo "🚪 Running all quality gates independently..."
	@./scripts/tdd-enforcer.sh verify debug

# Generate evidence report from latest verification
evidence-report:
	@echo "📊 Latest evidence reports:"
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
toolbox-build: build-cached
	@echo "🔧 Building GOTRS toolbox container (using backend as base)..."
	@$(CONTAINER_CMD) build \
		--cache-from gotrs-toolbox:latest \
		--build-arg BUILDKIT_INLINE_CACHE=1 \
		-f Dockerfile.toolbox -t gotrs-toolbox:latest .
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
	@echo "📝 Applying generated test data..."
	@$(COMPOSE_CMD) exec -T postgres psql -U $(DB_USER) -d $(DB_NAME) -f - < migrations/000004_generated_test_data.up.sql
	@echo "✅ Test data applied. Run 'make show-dev-creds' to see credentials."

# Clean up storage directory (orphaned files after DB reset)
clean-storage:
	@echo "🧹 Cleaning orphaned storage files..."
	@rm -rf internal/api/storage/* 2>/dev/null || true
	@rm -rf storage/* 2>/dev/null || true
	@echo "✅ Storage directories cleaned"

# Generate secure .env file with random secrets (runs in container)
synthesize:
	@$(MAKE) toolbox-build
	@echo "🔬 Synthesizing secure configuration and test data..." >&2
	@$(CONTAINER_CMD) run --rm \
		-v "$$(pwd):/workspace" \
		-w /workspace \
		-u "$$(id -u):$$(id -g)" \
		gotrs-toolbox:latest \
		gotrs synthesize $(SYNTH_ARGS)
	@if [ -z "$(SYNTH_ARGS)" ]; then \
		echo "📝 Test credentials saved to test_credentials.csv" >&2; \
	fi
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
		$(if $(ARGS),$(ARGS),/bin/bash)

# Execute command in toolbox container (non-interactive, no TTY)
toolbox-exec:
	@$(MAKE) toolbox-build
	@$(CONTAINER_CMD) run --rm \
		-v "$$(pwd):/workspace" \
		-w /workspace \
		-u "$$(id -u):$$(id -g)" \
		--network host \
		gotrs-toolbox:latest \
		$(ARGS)

# Check compilation of all packages
toolbox-compile:
	@$(MAKE) toolbox-build
	@echo "🔨 Checking compilation..."
	@$(CONTAINER_CMD) run --rm \
		-v "$$(pwd):/workspace" \
		-w /workspace \
		-u "$$(id -u):$$(id -g)" \
		gotrs-toolbox:latest \
		go build ./...

# Compile the main goats binary in container
.PHONY: compile
compile: toolbox-build
	@echo "🔨 Compiling goats binary..."
	@$(CONTAINER_CMD) run --rm \
		-v "$$(pwd):/workspace" \
		-w /workspace \
		-u "$$(id -u):$$(id -g)" \
		-e CGO_ENABLED=1 \
		-e GOOS=linux \
		-e GOARCH=amd64 \
		gotrs-toolbox:latest \
		go build -ldflags="-w -s" -o bin/goats ./cmd/goats
	@echo "✅ Binary compiled to bin/goats"

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
		-e VALKEY_PORT=6388 \
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
		-e VALKEY_PORT=6388 \
		-e APP_ENV=test \
		gotrs-toolbox:latest \
		sh -c "source .env 2>/dev/null || true && go test -v -run '$(TEST)' ./..."

# Run specific Go file with toolbox
toolbox-run-file:
	@$(MAKE) toolbox-build
	@echo "🚀 Running Go file: $(FILE)"
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
		sh -c "source .env 2>/dev/null || true && go run $(FILE)"

# Run anti-gaslighting detector in toolbox container
toolbox-antigaslight:
	@$(MAKE) toolbox-build
	@echo "🔍 Running anti-gaslighting detector in container..."
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

# Schema discovery - generate YAML modules from database
schema-discovery:
	@echo "🔍 Discovering database schema and generating YAML modules..."
	@./scripts/schema-discovery.sh --verbose

# Schema discovery for specific table
schema-table:
	@if [ -z "$(TABLE)" ]; then \
		echo "Error: TABLE not specified. Usage: make schema-table TABLE=tablename"; \
		exit 1; \
	fi
	@echo "🔍 Generating YAML module for table: $(TABLE)..."
	@./scripts/schema-discovery.sh --table $(TABLE) --verbose

# Start all services (and clean host binaries after build)
up:
	$(COMPOSE_CMD) up --build
	@echo "🧹 Cleaning host binaries after container build..."
	@rm -f bin/goats bin/gotrs bin/server bin/migrate bin/generator bin/gotrs-migrate bin/schema-discovery 2>/dev/null || true
	@echo "✅ Host binaries cleaned - containers have the only copies"

# Start in background (and clean host binaries after build)
up-d:
	$(COMPOSE_CMD) up -d --build
	@echo "🧹 Cleaning host binaries after container build..."
	@rm -f bin/goats bin/gotrs bin/server bin/migrate bin/generator bin/gotrs-migrate bin/schema-discovery 2>/dev/null || true
	@echo "✅ Host binaries cleaned - containers have the only copies"

# Stop all services
down:
	$(COMPOSE_CMD) down

# Restart services
restart: down up-d
	@echo "🔄 Restarted all services"

# View logs
logs:
	$(COMPOSE_CMD) logs -f

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
# Set this from the environment or override on the command line
#    e.g.   echo "select * from users;"| make db-shell
#           echo "select * from users;"| make DB_DRIVER=mysql   db-shell
db-shell:
	@if [ "$(DB_DRIVER)" = "postgres" ]; then \
		$(COMPOSE_CMD) exec -T postgres psql -U $(DB_USER) -d $(DB_NAME); \
	elif [ "$(DB_DRIVER)" = "mysql" ]; then \
		$(CONTAINER_CMD) run --rm -i \
			--network gotrs-ce_default \
			-v "$$(pwd):/workspace" \
			-w /workspace \
			gotrs-toolbox:latest \
			mysql -h 172.18.0.3 -u $(MYSQL_ROOT_USER) -p$(MYSQL_ROOT_PASSWORD) -D $(DB_NAME); \
    fi

# Fix PostgreSQL sequences after data import
db-fix-sequences:
	@echo "🔧 Fixing database sequences..."
	@./scripts/fix-sequences.sh
	@echo "✅ Sequences fixed - duplicate key errors should be resolved"

# Run a database query (use QUERY="SELECT ..." make db-query)
db-query:
	@if [ -z "$(QUERY)" ]; then \
		echo "Usage: make db-query QUERY=\"SELECT * FROM table\""; \
		exit 1; \
	fi
	@$(COMPOSE_CMD) exec -T postgres psql -U $(DB_USER) -d $(DB_NAME) -t -c "$(QUERY)"

db-migrate:
	@echo "Running database migrations..."
	$(COMPOSE_CMD) exec backend ./migrate -path /app/migrations -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" up
	@echo "Migrations completed successfully!"
	@echo "🔧 Fixing database sequences to prevent duplicate key errors..."
	@./scripts/fix-sequences.sh > /dev/null 2>&1 || true
	@echo "✅ Database ready with sequences properly synchronized!"

db-migrate-schema-only:
	@echo "Running schema migration only..."
	$(COMPOSE_CMD) exec backend ./migrate -path /app/migrations -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" up 3
	@echo "Schema and initial data applied (no test data)"
	@echo "🔧 Fixing database sequences..."
	@./scripts/fix-sequences.sh > /dev/null 2>&1 || true
	@echo "✅ Sequences synchronized!"

db-seed-dev:
	@echo "Seeding development database with comprehensive test data..."
	@$(COMPOSE_CMD) exec backend ./migrate -path /app/migrations -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" up
	@echo "🔧 Fixing sequences after seeding..."
	@./scripts/fix-sequences.sh > /dev/null 2>&1 || true
	@echo "✅ Development database seeded with:"
	@echo "   - 10 organizations"
	@echo "   - 50 customer users"
	@echo "   - 15 support agents"
	@echo "   - 100 ITSM tickets"
	@echo "   - Knowledge base articles"

db-seed-test:
	@echo "Seeding test database with comprehensive test data..."
	@$(COMPOSE_CMD) exec backend ./migrate -path /app/migrations -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$${DB_NAME}_test?sslmode=disable" up
	@echo "✅ Test database ready for testing"

db-reset-dev:
	@echo "⚠️  This will DELETE all data and recreate the development database!"
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
	@echo "Resetting test database..."
	@$(COMPOSE_CMD) exec backend migrate -path /app/migrations -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$${DB_NAME}_test?sslmode=disable" down -all
	@$(COMPOSE_CMD) exec backend migrate -path /app/migrations -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$${DB_NAME}_test?sslmode=disable" up
	@$(MAKE) clean-storage
	@echo "✅ Test database reset with fresh test data"

db-refresh: db-reset-dev
	@echo "✅ Database refreshed for new development cycle"

db-rollback:
	$(COMPOSE_CMD) exec backend migrate -path /app/migrations -database "postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" down 1

# Fast database initialization from baseline (new approach)
db-init:
	@echo "🚀 Initializing database from baseline (fast path)..."
	@$(COMPOSE_CMD) exec -T postgres psql -U $(DB_USER) -d $(DB_NAME) -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"
	@$(COMPOSE_CMD) exec -T postgres psql -U $(DB_USER) -d $(DB_NAME) -f - < schema/baseline/otrs_complete.sql
	@$(COMPOSE_CMD) exec -T postgres psql -U $(DB_USER) -d $(DB_NAME) -f - < schema/baseline/required_lookups.sql
	@$(MAKE) clean-storage
	@echo "🔧 Fixing sequences after baseline initialization..."
	@./scripts/fix-sequences.sh > /dev/null 2>&1 || true
	@echo "✅ Database initialized from baseline"

# Initialize for OTRS import (structure only, no data)
db-init-import:
	@echo "🚀 Initializing database structure for OTRS import..."
	@$(COMPOSE_CMD) exec -T postgres psql -U $(DB_USER) -d $(DB_NAME) -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"
	@$(COMPOSE_CMD) exec -T postgres psql -U $(DB_USER) -d $(DB_NAME) -f - < schema/baseline/otrs_complete.sql
	@echo "✅ Database structure ready for OTRS import"

# Development environment with minimal seed data
db-init-dev:
	@echo "🚀 Initializing development database..."
	@$(MAKE) db-init
	@$(COMPOSE_CMD) exec -T postgres psql -U $(DB_USER) -d $(DB_NAME) -f - < schema/seed/minimal.sql
	@echo "🔧 Fixing sequences after initialization..."
	@./scripts/fix-sequences.sh > /dev/null 2>&1 || true
	@echo "✅ Development database ready (admin/admin)"

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
	@echo "📄 Applying SQL migrations directly..."
	@for f in migrations/*.up.sql; do \
		echo "  Running $$(basename $$f)..."; \
		$(COMPOSE_CMD) exec -T postgres psql -U $(DB_USER) -d $(DB_NAME) -f - < "$$f" 2>&1 | grep -E "(CREATE|ALTER|INSERT|ERROR)" | head -3 || true; \
	done
	@echo "✅ SQL migrations applied"

# OTRS Migration Tools
# Analyze OTRS SQL dump file
migrate-analyze:
	@$(MAKE) toolbox-build
	@if [ -z "$(SQL)" ]; then \
		echo "❌ SQL file required. Usage: make migrate-analyze SQL=/path/to/dump.sql"; \
		exit 1; \
	fi
	@echo "🔍 Analyzing OTRS SQL dump: $(SQL)"
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
	@echo "📥 Importing OTRS data from: $(SQL)"
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
	@echo "⚠️  WARNING: Force import will CLEAR ALL EXISTING DATA!"
	@echo "📥 Importing OTRS data from: $(SQL)"
	@$(CONTAINER_CMD) run --rm \
		-v "$$(dirname $(SQL)):/data:ro" \
		-u "$$(id -u):$$(id -g)" \
		--network gotrs-ce_gotrs-network \
		gotrs-toolbox:latest \
		gotrs-migrate -cmd=import -sql="/data/$$(basename $(SQL))" \
			-db="postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" \
			-force -v || true
	@echo "✅ Force import completed successfully!"

# Validate imported OTRS data
migrate-validate:
	@$(MAKE) toolbox-build
	@echo "🔍 Validating imported OTRS data"
	@$(CONTAINER_CMD) run --rm \
		-u "$$(id -u):$$(id -g)" \
		--network gotrs-ce_gotrs-network \
		gotrs-toolbox:latest \
		gotrs-migrate -cmd=validate \
			-db="postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable" -v

# Import test data with proper ID mapping
import-test-data:
	@echo "📥 Building and importing test tickets with proper ID mapping..."
	@echo "🔨 Building import tool..."
	@mkdir -p bin
	@$(CONTAINER_CMD) run --rm \
		-v "$$(pwd):/workspace" \
		-w /workspace \
		-e GOCACHE=/tmp/.cache/go-build \
		-e GOMODCACHE=/tmp/.cache/go-mod \
		-u "$$(id -u):$$(id -g)" \
		golang:1.23-alpine \
		go build -o /workspace/bin/import-otrs ./cmd/import-otrs/main.go
	@echo "🗑️ Clearing existing data..."
	@$(COMPOSE_CMD) exec -T postgres psql -U $(DB_USER) -d $(DB_NAME) -c "TRUNCATE ticket CASCADE;" > /dev/null 2>&1
	@$(COMPOSE_CMD) exec -T postgres psql -U $(DB_USER) -d $(DB_NAME) -c "TRUNCATE article CASCADE;" > /dev/null 2>&1
	@echo "📦 Running import..."
	@$(CONTAINER_CMD) run --rm \
		-v "$$(pwd)/bin:/bin:ro" \
		--network gotrs-ce_gotrs-network \
		alpine:3.19 \
		/bin/import-otrs -db="postgres://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=disable"
	@echo "✅ Test data imported successfully with correct article mappings!"

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
		-e DB_HOST=postgres \
		-e DB_PORT=5432 \
		-e DB_NAME=${DB_NAME} \
		-e DB_USER=${DB_USER} \
		-e PGPASSWORD=${DB_PASSWORD} \
		gotrs-toolbox:latest \
		gotrs reset-user --username="$$username" --password="$$password" --enable


# Valkey CLI
valkey-cli:
	$(COMPOSE_CMD) exec valkey valkey-cli

# i18n Tools
babelfish:
	@echo "Building gotrs-babelfish..."
	$(COMPOSE_CMD) exec backend go build -o /tmp/bin/gotrs-babelfish cmd/gotrs-babelfish/main.go
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

# Build for production (includes CSS and container build)
build: css-build
	@echo "🔨 Building backend container..."
	$(CONTAINER_CMD) build -f Dockerfile -t gotrs:latest .
	@echo "🧹 Cleaning host binaries..."
	@rm -f goats gotrs gotrs-* generator migrate server  # Clean root directory
	@rm -f bin/* 2>/dev/null || true  # Clean bin directory
	@echo "✅ Build complete - CSS compiled and containers ready"

# ============================================
# Enhanced Build Targets with BuildKit
# ============================================

# Enable BuildKit for better caching
export DOCKER_BUILDKIT=1
export COMPOSE_DOCKER_CLI_BUILD=1

# Build with caching (70% faster rebuilds)
build-cached:
	@echo "🚀 Building with BuildKit cache mounts..."
	$(CONTAINER_CMD) build \
		--cache-from gotrs:latest \
		--build-arg BUILDKIT_INLINE_CACHE=1 \
		-t gotrs:latest .
	@echo "✅ Build complete with caching"

# Security scan build (CI/CD)
build-secure:
	@echo "🔒 Building with security scanning..."
	$(CONTAINER_CMD) build \
		--target security \
		--output type=local,dest=./security-reports \
		.
	@echo "📊 Security reports saved to ./security-reports/"

# Multi-platform build (AMD64 and ARM64)
build-multi:
	@echo "🌍 Building for multiple platforms..."
	$(CONTAINER_CMD) buildx build \
		--platform linux/amd64,linux/arm64 \
		-t gotrs:latest .
	@echo "✅ Multi-platform build complete"

# Analyze image size with dive
analyze-size:
	@echo "📏 Analyzing Docker image size..."
	@if command -v dive > /dev/null 2>&1; then \
		dive gotrs:latest; \
	else \
		$(CONTAINER_CMD) run --rm -it \
			-v /var/run/docker.sock:/var/run/docker.sock \
			wagoodman/dive:latest gotrs:latest; \
	fi

# Build without cache (clean build)
build-clean:
	@echo "🧹 Clean build without cache..."
	$(CONTAINER_CMD) build --no-cache -t gotrs:latest .
	@echo "✅ Clean build complete"

# Show build cache usage
show-cache:
	@echo "💾 Docker build cache usage:"
	@$(CONTAINER_CMD) system df --verbose | grep -A 10 "Build Cache" || \
		$(CONTAINER_CMD) buildx du --verbose 2>/dev/null || \
		echo "Build cache info not available"

# Clear build cache
clear-cache:
	@echo "🗑️ Clearing Docker build cache..."
	@$(CONTAINER_CMD) builder prune -f
	@echo "✅ Build cache cleared"

# Build specialized containers
build-all-tools: build-cached toolbox-build
	@echo "🛠️ Building all specialized tool containers..."
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
	@echo "✅ All tool containers built successfully"

# Show image sizes
show-sizes:
	@echo "📏 Docker image sizes:"
	@$(CONTAINER_CMD) images --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}" | grep -E "(REPOSITORY|gotrs)" | column -t

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

# Include task coordination system
include task-coordination.mk

# CSS Build Commands
.PHONY: css-deps css-build css-watch

# Install CSS build dependencies (in container with user permissions)
css-deps:
	@echo "📦 Installing CSS build dependencies..."
	@$(CONTAINER_CMD) run --rm -u $(shell id -u):$(shell id -g) -v $(PWD):/app -w /app node:20-alpine npm install
	@echo "✅ CSS dependencies installed"

# Build production CSS (in container with user permissions)
css-build:
	@echo "🎨 Building production CSS..."
	@if [ ! -d "node_modules" ]; then \
		echo "📦 Installing CSS dependencies first..."; \
		$(MAKE) css-deps; \
	fi
	@$(CONTAINER_CMD) run --rm -u $(shell id -u):$(shell id -g) -v $(PWD):/app -w /app node:20-alpine npm run build-css
	@echo "✅ CSS built to static/css/output.css"

# Build JavaScript assets (Tiptap editor bundle)
js-deps:
	@echo "📦 Installing JavaScript dependencies..."
	@$(CONTAINER_CMD) run --rm -u $(shell id -u):$(shell id -g) -v $(PWD):/app -w /app node:20-alpine npm install
	@echo "✅ JavaScript dependencies installed"

js-build: js-deps
	@echo "🔨 Building JavaScript bundles..."
	@$(CONTAINER_CMD) run --rm -u $(shell id -u):$(shell id -g) -v $(PWD):/app -w /app node:20-alpine npm run build-js
	@echo "✅ JavaScript built to static/js/tiptap.min.js"

# Build all frontend assets
frontend-build: css-build js-build
	@echo "✅ All frontend assets built"

# Watch and rebuild CSS on changes (in container with user permissions)
css-watch: css-deps
	@echo "👁️  Watching for CSS changes..."
	@$(CONTAINER_CMD) run --rm -it -u $(shell id -u):$(shell id -g) -v $(PWD):/app -w /app node:20-alpine npm run watch-css# Add these lines to the help section around line 40:
	@echo "Advanced TDD Commands (Zero Tolerance for False Claims):"
	@echo "  make tdd-comprehensive           - Run ALL quality gates with evidence"
	@echo "  make anti-gaslighting            - Detect false success claims"
	@echo "  make tdd-test-first-init FEATURE=name - Initialize test-first TDD cycle"
	@echo "  make tdd-full-cycle FEATURE=name - Complete guided TDD cycle"
	@echo "  make tdd-quick                   - Quick verification for development"
	@echo "  make tdd-dashboard              - Show TDD status and metrics"
	@echo ""

# Add these commands after the existing TDD section around line 178:

#########################################
# COMPREHENSIVE TDD AUTOMATION
#########################################

# Initialize comprehensive TDD environment
tdd-comprehensive-init:
	@echo "🚀 Initializing comprehensive TDD environment..."
	@./scripts/comprehensive-tdd-integration.sh init

# Run comprehensive TDD verification with ALL quality gates
tdd-comprehensive:
	@echo "🧪 Running COMPREHENSIVE TDD verification..."
	@echo "Zero tolerance for false positives and premature success claims"
	@./scripts/tdd-comprehensive.sh comprehensive

# Anti-gaslighting detection - prevents false success claims
anti-gaslighting:
	@echo "🚨 Running anti-gaslighting detection..."
	@echo "Detecting premature success claims and hidden failures..."
	@./scripts/anti-gaslighting-detector.sh detect

# Initialize test-first TDD cycle with proper enforcement
tdd-test-first-init:
	@if [ -z "$(FEATURE)" ]; then \
		echo "Error: FEATURE required. Usage: make tdd-test-first-init FEATURE='Feature Name'"; \
		exit 1; \
	fi
	@echo "🔴 Initializing test-first TDD cycle for: $(FEATURE)"
	@./scripts/tdd-test-first-enforcer.sh init "$(FEATURE)"

# Generate failing test for TDD cycle
tdd-generate-test:
	@if [ ! -f .tdd-state ]; then \
		echo "Error: TDD not initialized. Run 'make tdd-test-first-init FEATURE=name' first"; \
		exit 1; \
	fi
	@echo "📝 Generating failing test..."
	@echo "Test types: unit, integration, api, browser"
	@read -p "Enter test type (default: unit): " test_type; \
	test_type=$${test_type:-unit}; \
	./scripts/tdd-test-first-enforcer.sh generate-test $$test_type

# Verify test is actually failing (TDD enforcement)
tdd-verify-failing:
	@if [ -z "$(TEST_FILE)" ]; then \
		echo "Error: TEST_FILE required. Usage: make tdd-verify-failing TEST_FILE=path/to/test.go"; \
		exit 1; \
	fi
	@echo "🔍 Verifying test actually fails..."
	@./scripts/tdd-test-first-enforcer.sh verify-failing "$(TEST_FILE)"

# Verify tests now pass after implementation
tdd-verify-passing:
	@if [ -z "$(TEST_FILE)" ]; then \
		echo "Error: TEST_FILE required. Usage: make tdd-verify-passing TEST_FILE=path/to/test.go"; \
		exit 1; \
	fi
	@echo "✅ Verifying tests now pass..."
	@./scripts/tdd-test-first-enforcer.sh verify-passing "$(TEST_FILE)"

# Complete guided TDD cycle with comprehensive verification
tdd-full-cycle:
	@if [ -z "$(FEATURE)" ]; then \
		echo "Error: FEATURE required. Usage: make tdd-full-cycle FEATURE='Feature Name'"; \
		exit 1; \
	fi
	@echo "🔄 Starting full TDD cycle for: $(FEATURE)"
	@./scripts/comprehensive-tdd-integration.sh full-cycle "$(FEATURE)"

# Quick verification for development (fast feedback)
tdd-quick:
	@echo "⚡ Running quick TDD verification..."
	@./scripts/comprehensive-tdd-integration.sh quick

# Run specific test in toolbox container
test-specific:
	@if [ -z "$(TEST)" ]; then \
		echo "Error: TEST required. Usage: make test-specific TEST=TestRequiredQueueExists"; \
		exit 1; \
	fi
	@echo "🧪 Running specific test: $(TEST)"
	@$(CONTAINER_CMD) run --rm \
		--network gotrs-ce_gotrs-network \
		-e DB_HOST=postgres \
		-e DB_USER=$(DB_USER) \
		-e DB_PASSWORD=$(DB_PASSWORD) \
		-e DB_NAME=$(DB_NAME) \
		-v "$$(pwd):/workspace" \
		-w /workspace \
		gotrs-toolbox:latest \
		sh -c 'echo "Testing with DB_HOST=$$DB_HOST"; go test -v ./internal/repository -run $(TEST)'

# Show TDD dashboard with current status and metrics
tdd-dashboard:
	@./scripts/comprehensive-tdd-integration.sh dashboard

# Enhanced test command that integrates with comprehensive TDD
test-comprehensive:
	@echo "🧪 Running tests with comprehensive TDD integration..."
	@if [ -f .tdd-state ]; then \
		echo "TDD cycle active - running comprehensive verification..."; \
		$(MAKE) tdd-comprehensive; \
	else \
		echo "No TDD cycle - running comprehensive test suite..."; \
		./scripts/tdd-comprehensive.sh comprehensive; \
	fi

# Test-first enforcement (prevents implementation without failing test)
test-enforce-first:
	@echo "🚫 Enforcing test-first development..."
	@if [ ! -f .tdd-state ]; then \
		echo "Error: No TDD cycle active. Start with 'make tdd-test-first-init FEATURE=name'"; \
		exit 1; \
	fi
	@./scripts/tdd-test-first-enforcer.sh check-implementation

# Generate comprehensive TDD report
tdd-report:
	@echo "📊 Generating comprehensive TDD report..."
	@./scripts/tdd-test-first-enforcer.sh report

# Clean TDD state (reset cycle)
tdd-clean:
	@echo "🧹 Cleaning TDD state..."
	@rm -f .tdd-state
	@echo "TDD cycle reset. Start new cycle with 'make tdd-test-first-init FEATURE=name'"

# Verify system integrity (prevents gaslighting)
verify-integrity:
	@echo "🔍 Verifying system integrity..."
	@echo "Checking for false success claims and hidden failures..."
	@./scripts/anti-gaslighting-detector.sh detect
	@echo "Running comprehensive verification..."
	@./scripts/tdd-comprehensive.sh comprehensive

# TDD pre-commit hook (runs before commits)
tdd-pre-commit:
	@echo "🔒 Running TDD pre-commit verification..."
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
