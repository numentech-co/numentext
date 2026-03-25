#!/bin/sh
set -e

# NumenText installer
# Usage: curl -fsSL https://raw.githubusercontent.com/numentech-co/numentext/main/install.sh | sh

REPO="numentech-co/numentext"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

echo "Installing NumenText..."

# Detect OS
OS="$(uname -s)"
case "$OS" in
    Darwin) OS="darwin" ;;
    Linux)  OS="linux" ;;
    *)
        echo "Error: unsupported operating system: $OS"
        exit 1
        ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
    x86_64)  ARCH="amd64" ;;
    amd64)   ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    arm64)   ARCH="arm64" ;;
    *)
        echo "Error: unsupported architecture: $ARCH"
        exit 1
        ;;
esac

# Get latest release tag from GitHub API
LATEST="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
if [ -z "$LATEST" ]; then
    echo "Error: could not determine latest release version."
    echo "Check your internet connection and try again."
    exit 1
fi

VERSION="${LATEST#v}"
FILENAME="numentext_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${LATEST}/${FILENAME}"

echo "Downloading NumenText ${VERSION} for ${OS}/${ARCH}..."

# Create temp directory
TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

# Download
if ! curl -fsSL -o "${TMPDIR}/${FILENAME}" "$URL"; then
    echo "Error: failed to download ${URL}"
    echo "Check that a release exists for your platform (${OS}/${ARCH})."
    exit 1
fi

# Extract
tar -xzf "${TMPDIR}/${FILENAME}" -C "$TMPDIR"

# Install
if [ ! -w "$INSTALL_DIR" ]; then
    echo "Installing to ${INSTALL_DIR} (requires sudo)..."
    sudo install -m 755 "${TMPDIR}/numentext" "${INSTALL_DIR}/numentext"
else
    install -m 755 "${TMPDIR}/numentext" "${INSTALL_DIR}/numentext"
fi

echo "NumenText ${VERSION} installed to ${INSTALL_DIR}/numentext"
echo ""
echo "Run 'numentext' to start, or 'numentext --version' to verify."
