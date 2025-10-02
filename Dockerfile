# Optimized Multi-Stage Production Dockerfile
# Syntax directive for BuildKit features
# syntax=docker/dockerfile:1

# ============================================
# Stage 1: Dependencies (cached separately)
# ============================================
FROM docker.io/golang:1.24-alpine AS deps

# Set shell for better error handling
SHELL ["/bin/ash", "-eo", "pipefail", "-c"]

# Install build dependencies once
RUN apk add --no-cache \
    git \
    gcc \
    musl-dev \
    ca-certificates

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
# Force rebuild marker

# Build with optimizations and cache mounts
# -ldflags="-w -s" strips debug info for smaller binary
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -a -installsuffix cgo -o goats ./cmd/goats

# ============================================
# Stage 4: Security scanner (optional, for CI/CD)
# ============================================
FROM builder AS security

# Install security scanning tools
RUN go install github.com/securego/gosec/v2/cmd/gosec@latest && \
    go install honnef.co/go/tools/cmd/staticcheck@latest

# Run security scan and save results
RUN gosec -fmt json -out /tmp/security.json ./... || true && \
    staticcheck -f json ./... > /tmp/staticcheck.json || true

# ============================================
# Stage 5: Final minimal runtime
# ============================================
FROM docker.io/alpine:3.19 AS runtime

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    postgresql15-client \
    tzdata

# Create non-root user
RUN addgroup -g 1000 -S appgroup && \
    adduser -u 1000 -S appuser -G appgroup

# Create app directory
RUN mkdir -p /app /app/tmp && \
    chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser
WORKDIR /app

# Copy binaries from build stages
COPY --from=builder --chown=appuser:appgroup /build/goats ./goats
COPY --from=tools --chown=appuser:appgroup /go/bin/migrate ./migrate

# Copy necessary files
COPY --chown=appuser:appgroup templates ./templates/
COPY --chown=appuser:appgroup static ./static/
COPY --chown=appuser:appgroup routes ./routes/
COPY --chown=appuser:appgroup migrations ./migrations/
COPY --chown=appuser:appgroup config ./config/

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Default command
CMD ["./goats"]