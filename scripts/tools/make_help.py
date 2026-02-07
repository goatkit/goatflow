#!/usr/bin/env python3
"""
Generate colorful grouped Makefile help output.
"""
import os
import pathlib
import re
import subprocess
from typing import Dict, Iterable, List, Set

RESET = "\033[0m"
BOLD = "\033[1m"
DIM = "\033[2m"
MAGENTA = "\033[1;35m"
GREEN = "\033[0;32m"
CYAN = "\033[0;36m"

ROOT = pathlib.Path(__file__).resolve().parents[2]
LOGO_PATH = ROOT / "logo.txt"
MAKEFILE_PATH = ROOT / "Makefile"

# ---------------------------------------------------------------------------
# Human‑curated help metadata. Commands not listed here fall back to the
# automatically generated "Other Targets" section so that every Makefile
# target is represented.
# ---------------------------------------------------------------------------
GroupEntry = Dict[str, object]

GROUPS: List[Dict[str, object]] = [
    {
        "title": "Core Commands",
        "emoji": "🚀",
        "entries": [
            {"name": "up", "description": "Start core services interactively (backend, db, cache)"},
            {"name": "up-d", "description": "Start core services in daemon mode (backend, db, cache, runner)"},
            {"name": "down", "description": "Stop all services and containers"},
            {"name": "logs", "description": "Show the most recent chunk of backend logs"},
            {"name": "logs-follow", "description": "Tail backend logs until Ctrl+C"},
            {"name": "restart", "description": "Rebuild (if needed) and restart core services"},
            {"name": "clean", "description": "Remove containers, volumes, caches, and generated artifacts"},
            {"name": "setup", "description": "Initial project bootstrap with secure secrets"},
            {"name": "build", "description": "Build production images with cached layers"},
            {"name": "debug-env", "description": "Show container runtime / compose diagnostics"},
            {
                "name": "otrs-import",
                "usage": "make otrs-import SQL=/path/to/dump.sql",
                "description": "Import an OTRS SQL dump via the container pipeline",
            },
        ],
    },
    {
        "title": "Cache & Ownership Utilities",
        "emoji": "🗃",
        "entries": [
            {"name": "go-cache-info", "description": "Display Go build/module cache sizes"},
            {"name": "go-cache-clean", "description": "Wipe Go build/module caches"},
            {"name": "lint-cache-info", "description": "Display golangci-lint cache size"},
            {"name": "lint-cache-clean", "description": "Wipe golangci-lint cache"},
            {"name": "cache-audit", "description": "Audit cache ownership for container friendliness"},
            {"name": "cache-fix", "description": "Fix cache volume permissions (recovery for root-owned files)"},
            {
                "name": "toolbox-fix-cache",
                "usage": "make toolbox-fix-cache CONFIRM=YES",
                "description": "Chown caches back to the host user (one‑way)",
            },
            {"name": "cache-clean-all", "description": "Convenience: purge Go + lint caches"},
            {
                "name": None,
                "usage": "CACHE_USE_VOLUMES=1 make toolbox-test",
                "description": "Opt into legacy named volume caches (default is host bind mounts)",
            },
        ],
    },
    {
        "title": "CSS & Frontend Build",
        "emoji": "🎨",
        "entries": [
            {"name": "npm-updates", "description": "Upgrade NPM dependencies (package.json)"},
            {"name": "css-build", "description": "Build production Tailwind CSS bundle"},
            {"name": "css-watch", "description": "Watch and rebuild Tailwind CSS on changes"},
            {"name": "css-deps", "description": "Install CSS build dependencies"},
        ],
    },
    {
        "title": "Secrets Management",
        "emoji": "🔐",
        "entries": [
            {"name": "synthesize", "description": "Generate .env with unique secure secrets"},
            {"name": "rotate-secrets", "description": "Rotate secrets inside an existing .env"},
            {
                "name": "synthesize-force",
                "description": "Force regeneration of .env regardless of current state",
            },
            {"name": "k8s-secrets", "description": "Emit Kubernetes secrets manifest"},
            {"name": "show-dev-creds", "description": "Print the default developer credentials"},
        ],
    },
    {
        "title": "Docker & Container Build",
        "emoji": "🐳",
        "entries": [
            {"name": "build-cached", "description": "Fast build using cached layers"},
            {"name": "build-clean", "description": "Clean build without cache"},
            {"name": "build-secure", "description": "Build images and run security scans"},
            {"name": "build-multi", "description": "Multi‑platform build (amd64 + arm64)"},
            {"name": "build-all-tools", "description": "Build every supporting toolbox image"},
            {"name": "toolbox-build", "description": "Build the rootless development toolbox image"},
            {"name": "analyze-size", "description": "Inspect image size with dive"},
            {"name": "show-sizes", "description": "List image sizes"},
            {"name": "show-cache", "description": "Show Docker build cache usage"},
            {"name": "clear-cache", "description": "Prune Docker build cache"},
        ],
    },
    {
        "title": "Schema Discovery",
        "emoji": "🔮",
        "entries": [
            {"name": "schema-discovery", "description": "Generate YAML schema snapshot from DB"},
            {
                "name": "schema-table",
                "description": "Generate YAML for a specific table (use TABLE=...)",
            },
        ],
    },
    {
        "title": "Toolbox & Dev Utilities",
        "emoji": "🧰",
        "entries": [
            {"name": "toolbox-run", "description": "Spawn interactive shell inside toolbox"},
            {
                "name": "toolbox-exec",
                "usage": "make toolbox-exec ARGS='go version'",
                "description": "Execute a one‑off command in toolbox (with Go env wired)",
            },
            {"name": "verify-container-first", "description": "Check for stray host‑side Go commands"},
            {
                "name": "api-call",
                "usage": "make api-call METHOD=GET ENDPOINT=/api/lookups/statuses",
                "description": "Authenticated JSON API call through toolbox helper",
            },
            {
                "name": "api-call-form",
                "usage": "make api-call-form METHOD=PUT ENDPOINT=/admin/users/1 DATA='login=...'",
                "description": "Authenticated form‑urlencoded API call",
            },
            {
                "name": "http-call",
                "usage": "make http-call ENDPOINT=/login",
                "description": "Public HTTP call (no auth) via toolbox",
            },
            {"name": "toolbox-compile", "description": "Compile entire Go codebase in toolbox"},
            {
                "name": "toolbox-compile-api",
                "description": "Compile API + goats only (faster inner loop)",
            },
            {"name": "compile", "description": "Build goats binary on host using toolbox image"},
            {"name": "compile-safe", "description": "Isolated compile in temporary container"},
            {"name": "toolbox-test", "description": "Run core test suite inside toolbox"},
            {"name": "toolbox-test-api", "description": "Run internal/api tests (containerized DB)"},
            {
                "name": "toolbox-test-api-host",
                "description": "Run internal/api tests with host‑published DB ports",
            },
            {"name": "toolbox-test-all", "description": "Broad Go + generated + service tests"},
            {
                "name": "toolbox-test-pkg",
                "usage": "make toolbox-test-pkg PKG=./internal/api TEST=^TestLogin",
                "description": "Run go test for a specific package (optionally filtered)",
            },
            {
                "name": "toolbox-test-files",
                "usage": "make toolbox-test-files FILES='path/to/a_test.go'",
                "description": "Run go test scoped to explicit test files",
            },
            {
                "name": "toolbox-test-run",
                "usage": "make toolbox-test-run TEST=TestName",
                "description": "Run a single Go test via toolbox runner",
            },
            {
                "name": "toolbox-run-file",
                "usage": "make toolbox-run-file FILE=./cmd/goats/main.go",
                "description": "go run a specific file via toolbox",
            },
            {"name": "toolbox-staticcheck", "description": "Run staticcheck inside toolbox"},
        ],
    },
    {
        "title": "i18n (Babelfish)",
        "emoji": "🐠",
        "entries": [
            {"name": "babelfish", "description": "Build goatflow‑babelfish translation binary"},
            {"name": "babelfish-coverage", "description": "Show translation coverage stats"},
            {
                "name": "babelfish-validate",
                "usage": "make babelfish-validate LANG=de",
                "description": "Validate translations for a specific language",
            },
            {
                "name": "babelfish-missing",
                "usage": "make babelfish-missing LANG=es",
                "description": "List missing translations for a language",
            },
            {
                "name": "babelfish-run",
                "usage": "make babelfish-run ARGS='-help'",
                "description": "Run babelfish CLI with custom arguments",
            },
            {"name": "test-ldap", "description": "Run LDAP integration tests"},
            {"name": "test-ldap-perf", "description": "Run LDAP performance benchmarks"},
        ],
    },
    {
        "title": "Security",
        "emoji": "🔒",
        "entries": [
            {"name": "scan-secrets", "description": "Scan working tree for secrets"},
            {"name": "scan-secrets-history", "description": "Scan git history for secrets"},
            {
                "name": "scan-secrets-precommit",
                "description": "Install pre‑commit hooks for secret scanning",
            },
            {"name": "scan-vulnerabilities", "description": "Run vulnerability scanner"},
            {"name": "security-scan", "description": "Run all security scans"},
            {
                "name": "security-scan-artifacts",
                "description": "Capture security scan outputs (govulncheck, gosec, staticcheck, etc.)",
            },
            {"name": "test-contracts", "description": "Run Pact contract tests"},
            {"name": "test-all", "description": "Run backend + frontend + contracts"},
        ],
    },
    {
        "title": "Service Management",
        "emoji": "📡",
        "entries": [
            {"name": "backend-logs", "description": "Show backend logs once"},
            {"name": "backend-logs-follow", "description": "Tail backend logs"},
            {"name": "runner-logs", "description": "Show Temporal runner logs"},
            {"name": "runner-logs-follow", "description": "Tail Temporal runner logs"},
            {"name": "runner-up", "description": "Start runner service"},
            {"name": "runner-down", "description": "Stop runner service"},
            {"name": "runner-restart", "description": "Restart runner service"},
            {"name": "valkey-cli", "description": "Open Valkey CLI"},
        ],
    },
    {
        "title": "Database Operations",
        "emoji": "🗄️",
        "entries": [
            {"name": "db-shell", "description": "Open primary database shell"},
            {"name": "db-shell-test", "description": "Open test database shell"},
            {"name": "db-query", "description": "Run ad‑hoc SQL against primary DB"},
            {"name": "db-query-test", "description": "Run ad‑hoc SQL against test DB"},
            {"name": "db-migrate", "description": "Apply pending migrations"},
            {"name": "db-migrate-test", "description": "Apply test DB migrations"},
            {"name": "db-rollback", "description": "Rollback last migration"},
            {"name": "db-rollback-test", "description": "Rollback last test migration"},
            {"name": "db-reset", "description": "Reset primary database (and storage)"},
            {"name": "db-reset-test", "description": "Reset test database"},
            {"name": "db-init", "description": "Initialize baseline database schema"},
            {"name": "db-init-test", "description": "Initialize baseline schema for test DB"},
            {"name": "db-apply-test-data", "description": "Seed database with test data"},
            {"name": "db-status", "description": "Show current migration version"},
            {"name": "db-status-test", "description": "Show test DB migration version"},
            {"name": "db-force", "description": "Force migration version (dangerous)"},
            {"name": "db-force-test", "description": "Force migration version on test DB"},
            {"name": "db-migrate-sql", "description": "Replay raw SQL migrations"},
            {
                "name": "db-migrate-sql-test",
                "description": "Replay raw SQL migrations against test DB",
            },
            {"name": "db-fix-sequences", "description": "Fix PostgreSQL sequences"},
            {
                "name": "db-fix-sequences-test",
                "description": "Fix PostgreSQL sequences in test DB",
            },
            {"name": "clean-storage", "description": "Remove orphaned storage files"},
            {"name": "test-db-up", "description": "Start dedicated test database service"},
            {"name": "test-db-down", "description": "Stop dedicated test database service"},
        ],
    },
    {
        "title": "OTRS Migration",
        "emoji": "📦",
        "entries": [
            {
                "name": "migrate-analyze",
                "usage": "make migrate-analyze SQL=dump.sql",
                "description": "Analyze OTRS SQL dump before import",
            },
            {
                "name": "migrate-import",
                "usage": "make migrate-import SQL=dump.sql",
                "description": "Import OTRS data (dry‑run by default)",
                "examples": ["make migrate-import SQL=dump.sql DRY_RUN=false"],
            },
            {"name": "migrate-validate", "description": "Validate imported data"},
            {"name": "import-test-data", "description": "Import canonical test tickets"},
        ],
    },
    {
        "title": "User Management",
        "emoji": "👥",
        "entries": [
            {"name": "reset-password", "description": "Reset a user password"},
            {
                "name": "test-pg-reset-password",
                "description": "Reset password in Postgres test DB",
            },
            {
                "name": "test-mysql-reset-password",
                "description": "Reset password in MySQL test DB",
            },
        ],
    },
    {
        "title": "Linting & Code Quality",
        "emoji": "🔍",
        "entries": [
            {"name": "lint", "description": "Run all linters (Go, YAML, OpenAPI, Helm)"},
            {"name": "toolbox-lint", "description": "Run Go linters (golangci-lint)"},
            {"name": "yaml-lint", "description": "Lint YAML files (routes, config, .github)"},
            {"name": "openapi-lint", "description": "Lint OpenAPI spec with Redocly"},
        ],
    },
    {
        "title": "Helm Chart",
        "emoji": "⎈",
        "entries": [
            {"name": "helm-setup", "description": "Configure Helm repos (valkey)"},
            {"name": "helm-lint", "description": "Lint the GoatFlow Helm chart (included in make lint)"},
            {"name": "helm-template", "description": "Render chart templates (dry-run)"},
            {"name": "helm-template-pg", "description": "Render chart with PostgreSQL values"},
            {"name": "helm-deps", "description": "Update chart dependencies (valkey subchart)"},
            {"name": "helm-package", "description": "Package chart for distribution"},
            {
                "name": "helm",
                "usage": "make helm ARGS='template goatflow charts/goatflow'",
                "description": "Run arbitrary helm command in toolbox",
            },
        ],
    },
]

IGNORE_TARGETS: Set[str] = {".PHONY", "FORCE"}

# ---------------------------------------------------------------------------
# Helper functions
# ---------------------------------------------------------------------------

def read_logo() -> str:
    try:
        return LOGO_PATH.read_text()
    except FileNotFoundError:
        return "  🐐 GoatFlow - Go Open Ticketing Resource System"


def iter_targets_from_makefile() -> Set[str]:
    result = subprocess.run(["make", "-pq"], cwd=ROOT, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, text=True)
    targets: Set[str] = set()
    capture = False
    for line in result.stdout.splitlines():
        if line.startswith("# Files"):
            capture = True
            continue
        if not capture:
            continue
        if not line or line.startswith("#") or line[0].isspace():
            continue
        if ":" not in line:
            continue
        lhs = line.split(":", 1)[0]
        if not lhs or lhs.startswith("."):
            continue
        if "%" in lhs:
            continue
        for candidate in lhs.split():
            if candidate and candidate not in IGNORE_TARGETS:
                targets.add(candidate.strip())
    return targets


def flatten_documented_targets(groups: Iterable[Dict[str, object]]) -> Set[str]:
    documented: Set[str] = set()
    for group in groups:
        for entry in group.get("entries", []):
            name = entry.get("name")
            if isinstance(name, str) and name:
                documented.add(name)
    return documented


def render_group(title: str, emoji: str, entries: List[GroupEntry], width: int) -> str:
    lines: List[str] = []
    header = f"  {MAGENTA}{'━' * 40}{RESET}"
    lines.append(header)
    lines.append(f"  {MAGENTA}{emoji} {title}{RESET}")
    lines.append(header)
    lines.append("")
    for entry in entries:
        usage = entry.get("usage")
        name = entry.get("name")
        if usage:
            label = usage
        elif name:
            label = f"make {name}"
        else:
            label = str(entry.get("label", ""))
        description = entry.get("description", "")
        label_fmt = f"  {GREEN}{label:<{width}}{RESET}"
        lines.append(label_fmt + f" {description}")
    lines.append("")
    return "\n".join(lines)

# ---------------------------------------------------------------------------
# Main logic
# ---------------------------------------------------------------------------

def main() -> None:
    logo = read_logo()
    print("\n" + logo + "\n")

    documented = flatten_documented_targets(GROUPS)
    all_targets = iter_targets_from_makefile()
    missing = sorted(all_targets - documented - IGNORE_TARGETS)

    # Build size for alignment (consider documented labels + missing "make target").
    labels: List[str] = []
    for group in GROUPS:
        for entry in group.get("entries", []):
            if entry.get("usage"):
                labels.append(entry["usage"])
            elif entry.get("name"):
                labels.append(f"make {entry['name']}")
            elif entry.get("label"):
                labels.append(str(entry.get("label")))
    labels.extend([f"make {t}" for t in missing])
    width = max((len(label) for label in labels), default=0) + 2

    for group in GROUPS:
        print(render_group(group["title"], group["emoji"], group.get("entries", []), width))

    if missing:
        # Generate human‑readable description for undocumented targets.
        def describe_target(name: str) -> str:
            words = re.sub(r"([a-z])([A-Z])", r"\1 \2", name).replace("-", " ").replace("_", " ")
            words = words.split()
            if not words:
                return f"Run {name}"
            prefix_map = {
                "test": "Test",
                "db": "Database",
                "schema": "Schema",
                "migrate": "Migrate",
                "reset": "Reset",
                "setup": "Setup",
                "build": "Build",
                "clean": "Clean",
            }
            first = words[0]
            if first in prefix_map:
                verb = prefix_map[first]
                rest = " ".join(words[1:])
                return f"{verb} {rest}" if rest else verb
            return f"Run {name.replace('-', ' ')}"

        other_entries = [
            {
                "name": target,
                "description": describe_target(target),
            }
            for target in missing
        ]
        print(render_group("Other Targets", "🧩", other_entries, width))

    print(f"  {CYAN}🐐 Happy coding with GoatFlow!{RESET}")
    container_cmd = os.environ.get("CONTAINER_CMD") or "configure CONTAINER_CMD"
    compose_cmd = os.environ.get("COMPOSE_CMD") or "configure COMPOSE_CMD"
    print(
        f"  {DIM}Container Runtime: {container_cmd} | Compose Tool: {compose_cmd} | Toolbox: make toolbox-build{RESET}"
    )

if __name__ == "__main__":
    main()
