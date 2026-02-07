# Optimized Multi-Stage Production Dockerfile
# Syntax directive for BuildKit features
# syntax=docker/dockerfile:1

# Global build arg - must be before any FROM that uses it
# Default provided for docker build without --build-arg; compose/make override from .env
ARG GO_IMAGE=golang:1.24-alpine

# ============================================
# Stage 0: Download third-party assets
# ============================================
FROM docker.io/alpine:3.19 AS assets

RUN apk add --no-cache curl unzip

WORKDIR /assets

# Create directory structure
RUN mkdir -p js css/fontawesome css/webfonts

# Download JavaScript libraries (pinned versions)
RUN curl -sL "https://unpkg.com/htmx.org@1.9.12/dist/htmx.min.js" -o js/htmx.min.js && \
    curl -sL "https://unpkg.com/htmx.org@1.9.12/dist/ext/json-enc.js" -o js/htmx-json-enc.js && \
    curl -sL "https://unpkg.com/alpinejs@3.14.3/dist/cdn.min.js" -o js/alpine.min.js && \
    curl -sL "https://cdn.jsdelivr.net/npm/chart.js@4.4.7/dist/chart.umd.min.js" -o js/chart.min.js

# Download Font Awesome Free (CSS + webfonts)
# Webfonts go in css/webfonts/ because the CSS uses relative path ../webfonts/
RUN curl -sL "https://use.fontawesome.com/releases/v6.7.2/fontawesome-free-6.7.2-web.zip" -o /tmp/fa.zip && \
    unzip -q /tmp/fa.zip -d /tmp && \
    cp /tmp/fontawesome-free-6.7.2-web/css/all.min.css css/fontawesome/all.min.css && \
    cp /tmp/fontawesome-free-6.7.2-web/webfonts/fa-brands-400.woff2 css/webfonts/ && \
    cp /tmp/fontawesome-free-6.7.2-web/webfonts/fa-regular-400.woff2 css/webfonts/ && \
    cp /tmp/fontawesome-free-6.7.2-web/webfonts/fa-solid-900.woff2 css/webfonts/ && \
    rm -rf /tmp/fa.zip /tmp/fontawesome-free-6.7.2-web

# ============================================
# Stage 0b: Build frontend assets (CSS + JS)
# ============================================
FROM oven/bun:1.1-alpine AS frontend

# Add Node.js for tailwindcss compatibility (bunx has mkdir bugs with tailwind)
RUN apk add --no-cache nodejs

WORKDIR /build

# Copy package files and install dependencies
COPY package.json bun.lockb ./
RUN bun install --frozen-lockfile

# Copy source files needed for build
COPY static/css/input.css static/css/
COPY static/js/tiptap-bundle.js static/js/markdown-utils.js static/js/
COPY tailwind.config.js ./
COPY templates ./templates/

# Build CSS and JS (use node_modules/.bin for Node.js-based tools)
RUN ./node_modules/.bin/tailwindcss -i ./static/css/input.css -o ./static/css/output.css --minify && \
    ./node_modules/.bin/esbuild static/js/tiptap-bundle.js --bundle --minify --format=iife --global-name=TiptapBundle --outfile=static/js/tiptap.min.js

# ============================================
# Stage 1: Dependencies (cached separately)
# ============================================
FROM docker.io/${GO_IMAGE} AS deps

ARG name=defaultValue
# Set shell for better error handling
SHELL ["/bin/ash", "-eo", "pipefail", "-c"]

# Install build dependencies once
RUN apk add --no-cache \
    git \
    gcc \
    musl-dev \
    ca-certificates \
    vips-dev \
    vips-heif

# Set up module caching
WORKDIR /build
COPY go.mod go.sum ./

# Download and verify dependencies (cached until go.mod changes)
RUN go mod download && go mod verify

# ============================================
# Stage 2: Build tools in parallel
# ============================================
FROM deps AS tools

# Build migration tool
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go install -tags 'postgres,mysql' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# ============================================
# Stage 3: Build application
# ============================================
FROM deps AS builder

# Copy source code
COPY . ./

# Version information - set at build time via --build-arg or defaults to git info
ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG GIT_BRANCH=unknown
ARG BUILD_DATE=unknown

# Build with optimizations
# -ldflags injects version info and strips debug info for smaller binary
# -p uses all available cores for parallel compilation
RUN --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=1 GOOS=linux \
    go build -p "$(nproc)" -ldflags="-w -s \
        -X github.com/goatkit/goatflow/internal/version.Version=${VERSION} \
        -X github.com/goatkit/goatflow/internal/version.GitCommit=${GIT_COMMIT} \
        -X github.com/goatkit/goatflow/internal/version.GitBranch=${GIT_BRANCH} \
        -X github.com/goatkit/goatflow/internal/version.BuildDate=${BUILD_DATE}" \
    -installsuffix cgo -o goats ./cmd/goats

# ============================================
# Stage 4: Export build artifacts
# ============================================
FROM scratch AS artifacts

COPY --from=builder /build/goats /artifacts/goats
COPY --from=tools /go/bin/migrate /artifacts/migrate

# ============================================
# Stage 5: Security scanner (optional, for CI/CD)
# ============================================
FROM builder AS security

# Install security scanning tools
RUN go install github.com/securego/gosec/v2/cmd/gosec@latest && \
    go install honnef.co/go/tools/cmd/staticcheck@latest

# Run security scan and save results
RUN gosec -fmt json -out /tmp/security.json ./... || true && \
    staticcheck -f json ./... > /tmp/staticcheck.json || true

# ============================================
# Stage 7: Base runtime image with utilities
# ============================================
FROM docker.io/alpine:3.19 AS runtime-base

# Allow customizing runtime UID/GID so host bind mounts/caches are not root-owned
ARG UID=1000
ARG GID=1000

# Install runtime dependencies and diagnostics helpers
RUN apk add --no-cache \
    ca-certificates \
    curl \
    postgresql15-client \
    tzdata \
    vips \
    vips-heif

# Create non-root user and cache directories
RUN addgroup -g ${GID} -S appgroup && \
    adduser -u ${UID} -S appuser -G appgroup && \
    mkdir -p /app /app/tmp /home/appuser/.cache && \
    chown -R appuser:appgroup /app /home/appuser/.cache

# Set cache-related envs (Go build cache mostly relevant in toolbox, but harmless here)
ENV XDG_CACHE_HOME=/home/appuser/.cache

# Switch to non-root user
USER appuser
WORKDIR /app

# ============================================
# Stage 7a: Runner runtime (shares base image)
# ============================================
FROM runtime-base AS runner-runtime

COPY --from=artifacts --chown=appuser:appgroup /artifacts/goats ./goats
COPY --from=artifacts --chown=appuser:appgroup /artifacts/migrate ./migrate
COPY --chown=appuser:appgroup templates ./templates/
COPY --chown=appuser:appgroup static ./static/
COPY --chown=appuser:appgroup routes ./routes/
COPY --chown=appuser:appgroup modules ./modules/
COPY --chown=appuser:appgroup migrations ./migrations/
COPY --chown=appuser:appgroup config ./config/
COPY --chown=appuser:appgroup plugins ./config/plugins/

# Overlay downloaded third-party assets
COPY --from=assets --chown=appuser:appgroup /assets/js/*.js ./static/js/
COPY --from=assets --chown=appuser:appgroup /assets/css/fontawesome/ ./static/css/fontawesome/
COPY --from=assets --chown=appuser:appgroup /assets/css/webfonts/ ./static/css/webfonts/

# Overlay built frontend assets (CSS + TipTap bundle)
COPY --from=frontend --chown=appuser:appgroup /build/static/css/output.css ./static/css/
COPY --from=frontend --chown=appuser:appgroup /build/static/js/tiptap.min.js ./static/js/

# ============================================
# Stage 7b: Final minimal backend runtime
# ============================================
FROM runtime-base AS runtime

# Copy binaries from build artifacts
COPY --from=artifacts --chown=appuser:appgroup /artifacts/goats ./goats
COPY --from=artifacts --chown=appuser:appgroup /artifacts/migrate ./migrate

# Copy necessary files (our code)
COPY --chown=appuser:appgroup templates ./templates/
COPY --chown=appuser:appgroup static ./static/
COPY --chown=appuser:appgroup routes ./routes/
COPY --chown=appuser:appgroup modules ./modules/
COPY --chown=appuser:appgroup migrations ./migrations/
COPY --chown=appuser:appgroup config ./config/
COPY --chown=appuser:appgroup plugins ./config/plugins/

# Overlay downloaded third-party assets
COPY --from=assets --chown=appuser:appgroup /assets/js/*.js ./static/js/
COPY --from=assets --chown=appuser:appgroup /assets/css/fontawesome/ ./static/css/fontawesome/
COPY --from=assets --chown=appuser:appgroup /assets/css/webfonts/ ./static/css/webfonts/

# Overlay built frontend assets (CSS + TipTap bundle)
COPY --from=frontend --chown=appuser:appgroup /build/static/css/output.css ./static/css/
COPY --from=frontend --chown=appuser:appgroup /build/static/js/tiptap.min.js ./static/js/

# Plugin system defaults (can be overridden in compose.yml)
ENV GOATFLOW_PLUGIN_LAZY_LOAD=true
ENV GOATFLOW_PLUGIN_HOT_RELOAD=false

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Default command
CMD ["./goats"]