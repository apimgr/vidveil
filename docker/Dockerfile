# =============================================================================
# Build Stage - Compile Go binary
# =============================================================================
FROM casjaysdev/go:latest AS builder

# Install git and bash (git: required for go mod download; bash: for build scripts)
RUN apk add --no-cache git bash

ARG TARGETARCH
ARG VERSION=dev
ARG BUILD_DATE
ARG COMMIT_ID
ARG OFFICIAL_SITE

WORKDIR /build

# Copy go.mod first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build per AI.md PART 7
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build \
    -ldflags "-s -w -X 'main.Version=${VERSION}' -X 'main.CommitID=${COMMIT_ID}' -X 'main.BuildDate=${BUILD_DATE}' -X 'main.OfficialSite=${OFFICIAL_SITE}'" \
    -o /build/binary/vidveil ./src

# =============================================================================
# Runtime Stage - Minimal Alpine image
# =============================================================================
FROM alpine:latest

# ARGs for build-time values (set by docker build --build-arg)
ARG VERSION=dev
ARG BUILD_DATE
ARG COMMIT_ID

# No LABEL blocks — all image metadata is applied as OCI annotations at build time.
# Pass --annotation flags to docker buildx build (Makefile docker target handles this).
# Annotations attach to the manifest index and are visible on multiarch pulls.
# LABEL applies only to per-platform layers and is invisible on multiarch pulls.
# See AI.md PART 26 → "Multi-Arch Image Annotations".

# Install required packages
# NOTE: Tor binary installed but NOT configured here - binary handles all Tor setup (see PART 32)
RUN apk add --no-cache \
    git \
    curl \
    bash \
    tini \
    tor

# NO directory creation - binary handles ALL setup based on env vars
# This allows users to override CONFIG_DIR, DATA_DIR, etc.
# Docker volume mounts auto-create mount points

# Copy binary from builder stage (multi-stage build)
COPY --from=builder /build/binary/vidveil /usr/local/bin/vidveil

# Copy BUILD-TIME overlay (entrypoint.sh) from docker/rootfs/ into image
# Note: This is docker/rootfs/ (build context), NOT runtime ./volumes/ mounts
COPY docker/rootfs/ /

# Copy Dockerfile to image (for reference and backup)
COPY docker/Dockerfile /root/Dockerfile

# Make all binaries/scripts in /usr/local/bin executable (755)
RUN chmod 755 /usr/local/bin/*

# Set environment
# Note: Tor available (binary installed) but server binary controls Tor startup (see PART 32)
ENV MODE=development \
    TZ=America/New_York \
    DATABASE_DIR=/data/db/sqlite

# Expose internal port (always 80)
EXPOSE 80

# Stop signal for graceful shutdown
STOPSIGNAL SIGRTMIN+3

# Health check (long start period for services that need initialization)
# Per AI.md PART 14: Use binary --status command
HEALTHCHECK --start-period=10m --interval=5m --timeout=15s --retries=3 \
    CMD /usr/local/bin/vidveil --status || exit 1

# Use tini as init with signal propagation
# -p SIGTERM: propagate SIGTERM to child processes
ENTRYPOINT [ "tini", "-p", "SIGTERM", "--", "/usr/local/bin/entrypoint.sh" ]
