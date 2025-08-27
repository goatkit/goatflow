# Production Dockerfile for Go backend
FROM golang:1.23-alpine AS builder

# Set shell options for better error handling
SHELL ["/bin/ash", "-eo", "pipefail", "-c"]

# Install build dependencies
RUN apk add --no-cache \
    git \
    make \
    gcc \
    musl-dev

# Create non-root user for building
RUN addgroup -g 1000 -S appgroup && \
    adduser -u 1000 -S appuser -G appgroup

# Set up build environment
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . ./

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o goats ./cmd/goats

# Production stage
FROM alpine:latest

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

# Copy binary from builder stage
COPY --from=builder --chown=appuser:appgroup /build/goats ./goats

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