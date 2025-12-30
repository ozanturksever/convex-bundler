#!/bin/bash

# Download convex-local-backend binary
# Usage: ./scripts/download-backend.sh [platform]
#
# Platforms:
#   darwin-arm64  - macOS Apple Silicon (default on M1/M2 Macs)
#   darwin-x64    - macOS Intel
#   linux-x64     - Linux x86_64
#   linux-arm64   - Linux ARM64

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
BIN_DIR="$PROJECT_DIR/bin"

# Detect platform
OS="$(uname -s)"
ARCH="$(uname -m)"

case "$OS" in
    Darwin)
        case "$ARCH" in
            arm64) PLATFORM="aarch64-apple-darwin" ;;
            x86_64) PLATFORM="x86_64-apple-darwin" ;;
            *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
        esac
        ;;
    Linux)
        case "$ARCH" in
            aarch64) PLATFORM="aarch64-unknown-linux-gnu" ;;
            x86_64) PLATFORM="x86_64-unknown-linux-gnu" ;;
            *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
        esac
        ;;
    *)
        echo "Unsupported OS: $OS"
        exit 1
        ;;
esac

# Allow platform override
if [ -n "$1" ]; then
    case "$1" in
        darwin-arm64) PLATFORM="aarch64-apple-darwin" ;;
        darwin-x64) PLATFORM="x86_64-apple-darwin" ;;
        linux-x64) PLATFORM="x86_64-unknown-linux-gnu" ;;
        linux-arm64) PLATFORM="aarch64-unknown-linux-gnu" ;;
        *) PLATFORM="$1" ;;
    esac
fi

RELEASE_TAG="precompiled-2025-12-12-73e805a"
DOWNLOAD_URL="https://github.com/get-convex/convex-backend/releases/download/$RELEASE_TAG/convex-local-backend-$PLATFORM.zip"

echo "Downloading convex-local-backend for $PLATFORM..."
echo "URL: $DOWNLOAD_URL"

mkdir -p "$BIN_DIR"
cd "$BIN_DIR"

# Download and extract
curl -L -o convex-local-backend.zip "$DOWNLOAD_URL"
unzip -o convex-local-backend.zip
rm convex-local-backend.zip
chmod +x convex-local-backend

echo ""
echo "Downloaded to: $BIN_DIR/convex-local-backend"
./convex-local-backend --version
