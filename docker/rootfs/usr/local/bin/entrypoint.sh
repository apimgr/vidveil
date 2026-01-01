#!/usr/bin/env bash
set -e

# =============================================================================
# Container Entrypoint Script
# Handles service startup, signal handling, and graceful shutdown
# =============================================================================

APP_NAME="vidveil"
APP_BIN="/usr/local/bin/${APP_NAME}"

# Container defaults (exported for app to use)
# Timezone - default to America/New_York
export TZ="${TZ:-America/New_York}"

# Configurable paths (exported for server to use)
# Organized by component: /config/vidveil/, /data/vidveil/
export CONFIG_DIR="/config/${APP_NAME}"
export DATA_DIR="/data/${APP_NAME}"
export LOG_DIR="/data/log/${APP_NAME}"
export DATABASE_DIR="/data/db"
export BACKUP_DIR="/data/backups/${APP_NAME}"

# NOTE: Server handles ALL directory setup including:
# - Creating directories (config, data, tor, security, etc.)
# - Setting permissions
# - Managing ownership
# - Tor configuration and process management

# Array to track background PIDs
declare -a PIDS=()

# -----------------------------------------------------------------------------
# Logging
# -----------------------------------------------------------------------------
log() {
    echo "[entrypoint] $(date '+%Y-%m-%d %H:%M:%S') $*"
}

log_error() {
    echo "[entrypoint] $(date '+%Y-%m-%d %H:%M:%S') ERROR: $*" >&2
}

# Check if value is truthy (case-insensitive)
# Usage: if is_truthy "$DEBUG"; then ...
is_truthy() {
    local val="${1:-false}"
    val="${val,,}"  # lowercase
    [[ "$val" =~ ^(1|y|t|yes|true|on|ok|enable|enabled|sure|yep|yup|yeah|aye|si|oui|da|hai|affirmative|accept|allow|totally)$ ]]
}

# -----------------------------------------------------------------------------
# Signal handling
# -----------------------------------------------------------------------------
cleanup() {
    log "Received shutdown signal, stopping services..."

    # Stop services in reverse order
    for ((i=${#PIDS[@]}-1; i>=0; i--)); do
        pid="${PIDS[i]}"
        if kill -0 "$pid" 2>/dev/null; then
            log "Stopping PID $pid..."
            kill -TERM "$pid" 2>/dev/null || true
        fi
    done

    # Wait for processes to exit (max 30 seconds)
    local timeout=30
    while [ $timeout -gt 0 ]; do
        local running=0
        for pid in "${PIDS[@]}"; do
            if kill -0 "$pid" 2>/dev/null; then
                running=1
                break
            fi
        done
        [ $running -eq 0 ] && break
        sleep 1
        ((timeout--))
    done

    # Force kill any remaining
    for pid in "${PIDS[@]}"; do
        if kill -0 "$pid" 2>/dev/null; then
            log "Force killing PID $pid..."
            kill -9 "$pid" 2>/dev/null || true
        fi
    done

    log "Shutdown complete"
    exit 0
}

# Trap signals for graceful shutdown
# SIGRTMIN+3 (37) is the Docker STOPSIGNAL
# SIGTERM is propagated by tini -p SIGTERM
trap cleanup SIGTERM SIGINT SIGQUIT
trap cleanup SIGRTMIN+3 2>/dev/null || trap cleanup 37

# -----------------------------------------------------------------------------
# Directory setup
# -----------------------------------------------------------------------------
# Container directory structure (for reference - SERVER handles all setup):
#   $CONFIG_DIR          - configuration files (mounted: ./rootfs/config)
#   $CONFIG_DIR/security - TLS certs, keys
#   $CONFIG_DIR/tor      - Tor config files (torrc)
#   $DATA_DIR            - all persistent data (mounted: ./rootfs/data)
#   $DATA_DIR/db         - SQLite databases
#   $DATA_DIR/log        - application and service logs
#   $DATA_DIR/tor        - Tor hidden service data
#   $DATA_DIR/backup     - backup files
#
# NOTE: Server binary handles ALL directory creation, permissions, and ownership.
# Entrypoint does NOT create directories or set permissions - server does this.
# -----------------------------------------------------------------------------

# -----------------------------------------------------------------------------
# Start main application
# -----------------------------------------------------------------------------
start_app() {
    log "Starting ${APP_NAME}..."

    # Container defaults: 0.0.0.0:80 (override with ADDRESS/PORT env vars)
    local listen_addr="${ADDRESS:-0.0.0.0}"
    local listen_port="${PORT:-80}"
    local debug_flag=""

    # Enable debug mode if DEBUG is truthy (see Boolean Values table)
    if is_truthy "$DEBUG"; then
        debug_flag="--debug"
        log "Debug mode enabled"
    fi

    # Run the main application with container directory paths
    # Uses exported env vars that match volume mounts in docker-compose.yml
    # App can also read DATABASE_DIR, BACKUP_DIR env vars directly
    "$APP_BIN" \
        --address "$listen_addr" \
        --port "$listen_port" \
        --config "$CONFIG_DIR" \
        --data "$DATA_DIR" \
        --log "$LOG_DIR" \
        --pid "$DATA_DIR/${APP_NAME}.pid" \
        $debug_flag \
        "$@" &
    PIDS+=($!)
    log "${APP_NAME} started on ${listen_addr}:${listen_port} (PID: ${PIDS[-1]})"
}

# -----------------------------------------------------------------------------
# Wait for services
# -----------------------------------------------------------------------------
wait_for_services() {
    log "All services started, waiting..."

    # Wait for any process to exit
    while true; do
        for pid in "${PIDS[@]}"; do
            if ! kill -0 "$pid" 2>/dev/null; then
                log_error "Process $pid exited unexpectedly"
                cleanup
            fi
        done
        sleep 5
    done
}

# -----------------------------------------------------------------------------
# Main
# -----------------------------------------------------------------------------
main() {
    log "Container starting..."
    log "MODE: ${MODE:-development}"
    log "DEBUG: ${DEBUG:-false}"
    log "TZ: ${TZ:-America/New_York}"
    log "ADDRESS: ${ADDRESS:-0.0.0.0}"
    log "PORT: ${PORT:-80}"

    # Server handles all directory setup, permissions, and Tor management
    start_app "$@"
    wait_for_services
}

main "$@"
