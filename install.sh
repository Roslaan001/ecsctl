#!/bin/sh
#
# ecsctl installer
# Downloads the precompiled ecsctl binary from GitHub Releases and installs it.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/Roslaan001/ecsctl/main/install.sh | sh
#   or specify a version:
#   curl -fsSL https://raw.githubusercontent.com/Roslaan001/ecsctl/main/install.sh | sh -s -- v0.1.0

set -e

REPO="Roslaan001/ecsctl"
BINARY="ecsctl"

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
    linux)  OS="linux" ;;
    darwin) OS="darwin" ;;
    *)
        echo "Unsupported OS: $OS. Only Linux and macOS (Darwin) are supported." >&2
        exit 1
        ;;
esac

# Detect Architecture
ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *)
        echo "Unsupported architecture: $ARCH. Only amd64/x86_64 and arm64 are supported." >&2
        exit 1
        ;;
esac

# Parse version argument if provided
VERSION=""
if [ $# -gt 0 ]; then
    VERSION="$1"
fi

# If version is not specified, resolve the latest release version from GitHub API
if [ -z "$VERSION" ]; then
    echo "Resolving the latest release version for $REPO..."
    LATEST_RELEASE_URL="https://api.github.com/repos/$REPO/releases/latest"
    
    # Try fetching using curl or wget with User-Agent header
    if command -v curl >/dev/null 2>&1; then
        VERSION=$(curl -sH "User-Agent: ecsctl-installer" "$LATEST_RELEASE_URL" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    elif command -v wget >/dev/null 2>&1; then
        VERSION=$(wget -qO- --header="User-Agent: ecsctl-installer" "$LATEST_RELEASE_URL" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    fi
    
    # Fallback to latest known version if repo is private or API limit hit
    if [ -z "$VERSION" ] || [ "$VERSION" = "null" ]; then
        VERSION="v0.0.1"
    fi
fi

# Clean version tag (remove leading 'v' for filename construction, keep for download path)
VERSION_CLEAN=${VERSION#v}
echo "Selected version: $VERSION"

# Format download URL
FILENAME="${BINARY}_${VERSION_CLEAN}_${OS}_${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/$REPO/releases/download/$VERSION/$FILENAME"

# Create a temporary directory for extraction
TMP_DIR=$(mktemp -d)
clean_up() {
    rm -rf "$TMP_DIR"
}
trap clean_up EXIT

echo "Downloading ecsctl from: $DOWNLOAD_URL"
TARBALL="$TMP_DIR/$FILENAME"

if command -v curl >/dev/null 2>&1; then
    curl -fsSL -o "$TARBALL" "$DOWNLOAD_URL"
elif command -v wget >/dev/null 2>&1; then
    wget -qO "$TARBALL" "$DOWNLOAD_URL"
else
    echo "Error: curl or wget is required to download the binary." >&2
    exit 1
fi

echo "Extracting binary..."
tar -xzf "$TARBALL" -C "$TMP_DIR"

# Ensure the binary exists
if [ ! -f "$TMP_DIR/$BINARY" ]; then
    echo "Error: ecsctl binary was not found in the downloaded archive." >&2
    exit 1
fi

# Determine target directory
# Default to /usr/local/bin, fallback to $HOME/.local/bin if not root and /usr/local/bin not writable.
TARGET_DIR="/usr/local/bin"
USE_SUDO=""

if [ ! -w "$TARGET_DIR" ]; then
    if [ "$(id -u)" -ne 0 ]; then
        if command -v sudo >/dev/null 2>&1; then
            echo "Installing to $TARGET_DIR (requires sudo)..."
            USE_SUDO="sudo"
        else
            TARGET_DIR="$HOME/.local/bin"
            echo "Installing to $TARGET_DIR (no sudo available)..."
            mkdir -p "$TARGET_DIR"
        fi
    fi
else
    echo "Installing to $TARGET_DIR..."
fi

# Copy binary to target directory
$USE_SUDO cp "$TMP_DIR/$BINARY" "$TARGET_DIR/$BINARY"
$USE_SUDO chmod +x "$TARGET_DIR/$BINARY"

echo "Successfully installed ecsctl to $TARGET_DIR/$BINARY"
echo "Tip: Run 'ecsctl completion [bash|zsh|fish]' to set up shell autocompletions!"

# Verify installation
if command -v "$BINARY" >/dev/null 2>&1 || [ -x "$TARGET_DIR/$BINARY" ]; then
    PATH_EXEC="$BINARY"
    if ! command -v "$BINARY" >/dev/null 2>&1; then
        PATH_EXEC="$TARGET_DIR/$BINARY"
        echo "Warning: $TARGET_DIR is not in your PATH. You may need to add it to your shell configuration."
    fi
    echo "Installed version:"
    "$PATH_EXEC" version
else
    echo "Warning: Installed binary could not be verified."
fi
