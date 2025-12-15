#!/bin/bash
# Vidveil Install Script
# OS/distro agnostic installer

set -e

BINARY_NAME="vidveil"
REPO="apimgr/vidveil"
INSTALL_DIR="/usr/local/bin"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info() { echo -e "${GREEN}[INFO]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$ARCH" in
        x86_64|amd64) ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        *) error "Unsupported architecture: $ARCH" ;;
    esac

    case "$OS" in
        linux) OS="linux" ;;
        darwin) OS="darwin" ;;
        freebsd) OS="freebsd" ;;
        openbsd) OS="openbsd" ;;
        *) error "Unsupported OS: $OS" ;;
    esac

    info "Detected platform: $OS/$ARCH"
}

# Check for required tools
check_deps() {
    for cmd in curl tar; do
        if ! command -v $cmd &>/dev/null; then
            error "Required command not found: $cmd"
        fi
    done
}

# Get latest release version
get_latest_version() {
    VERSION=$(curl -sL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"v([^"]+)".*/\1/')
    if [ -z "$VERSION" ]; then
        error "Could not determine latest version"
    fi
    info "Latest version: $VERSION"
}

# Download and install binary
install_binary() {
    DOWNLOAD_URL="https://github.com/$REPO/releases/download/v$VERSION/${BINARY_NAME}-${OS}-${ARCH}"

    info "Downloading from: $DOWNLOAD_URL"

    TMP_FILE=$(mktemp)
    if ! curl -sL "$DOWNLOAD_URL" -o "$TMP_FILE"; then
        error "Download failed"
    fi

    chmod +x "$TMP_FILE"

    # Install
    if [ -w "$INSTALL_DIR" ]; then
        mv "$TMP_FILE" "$INSTALL_DIR/$BINARY_NAME"
    else
        info "Elevated permissions required for installation"
        sudo mv "$TMP_FILE" "$INSTALL_DIR/$BINARY_NAME"
    fi

    info "Installed to $INSTALL_DIR/$BINARY_NAME"
}

# Verify installation
verify() {
    if command -v $BINARY_NAME &>/dev/null; then
        info "Installation successful!"
        $BINARY_NAME --version
    else
        warn "Binary installed but not in PATH"
        info "You may need to add $INSTALL_DIR to your PATH"
    fi
}

# Main
main() {
    info "Installing $BINARY_NAME..."

    check_deps
    detect_platform
    get_latest_version
    install_binary
    verify

    echo ""
    info "Run '$BINARY_NAME --help' for usage information"
}

main "$@"
