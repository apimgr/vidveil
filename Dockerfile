# Vidveil Dockerfile
# Per BASE.md: Alpine-based, tini init, internal port 80

# Build stage
FROM golang:alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git ca-certificates jq

# Copy go mod files
COPY go.mod go.sum* ./
RUN go mod download 2>/dev/null || true

# Copy source
COPY . .

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o vidveil ./src

# Final stage - Alpine per BASE.md
FROM alpine:latest

# Labels
LABEL org.opencontainers.image.title="Vidveil"
LABEL org.opencontainers.image.description="Privacy-respecting adult video meta search engine"
LABEL org.opencontainers.image.source="https://github.com/apimgr/vidveil"
LABEL org.opencontainers.image.vendor="apimgr"
LABEL org.opencontainers.image.licenses="MIT"

# Install required packages per BASE.md (tini, bash, curl for scratch compatibility)
RUN apk add --no-cache tini bash curl ca-certificates jq

# Create directories per BASE.md Docker spec
RUN mkdir -p /config /data/logs/vidveil /data/db

# Copy binary
COPY --from=builder /build/vidveil /usr/local/bin/vidveil

# Set environment per TEMPLATE.md PART 19
# MODE=development allows localhost, .local, .test, etc.
ENV CONFIG_DIR=/config
ENV DATA_DIR=/data
ENV MODE=development

# Expose internal port (per BASE.md: internal port 80)
EXPOSE 80

# Health check using binary --status per BASE.md
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/vidveil", "--status"]

# Use tini as init system per BASE.md
ENTRYPOINT ["/sbin/tini", "--"]
CMD ["/usr/local/bin/vidveil", "--port", "80"]
