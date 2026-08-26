#!/usr/bin/env bash
set -e

# edio installer script
# Installs the pre-built edio binary for your OS and architecture.

REPO="devxdh/edio"
BINARY_NAME="edio"

main() {
    OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
    ARCH="$(uname -m)"

    case "$ARCH" in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        *)
            echo "Error: Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac

    case "$OS" in
        linux)
            OS="linux"
            ;;
        darwin)
            OS="darwin"
            ;;
        mingw*|msys*|cygwin*)
            OS="windows"
            ;;
        *)
            echo "Error: Unsupported operating system: $OS"
            exit 1
            ;;
    esac

    INSTALL_DIR="/usr/local/bin"
    if [ ! -w "$INSTALL_DIR" ]; then
        INSTALL_DIR="$HOME/.local/bin"
        mkdir -p "$INSTALL_DIR"
    fi

    echo "==> Detected system: ${OS}/${ARCH}"

    # Check if Go is installed for direct source build fallback
    if command -v go >/dev/null 2>&1; then
        echo "==> Building latest edio with Go..."
        if GOBIN="$INSTALL_DIR" go install "github.com/${REPO}/cmd/edio@latest" 2>/dev/null; then
            echo "==> Successfully installed edio to ${INSTALL_DIR}/${BINARY_NAME}"
            verify_install "$INSTALL_DIR"
            exit 0
        fi
    fi

    # Fallback to downloading latest GitHub Release artifact
    echo "==> Fetching latest release from GitHub..."
    LATEST_TAG=$(curl -s "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

    if [ -z "$LATEST_TAG" ]; then
        LATEST_TAG="v1.0.0"
    fi

    ARCHIVE_NAME="edio-${OS}-${ARCH}.tar.gz"
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${LATEST_TAG}/${ARCHIVE_NAME}"

    TMP_DIR=$(mktemp -d)
    trap 'rm -rf "$TMP_DIR"' EXIT

    echo "==> Downloading ${DOWNLOAD_URL}..."
    if curl -fsSL "$DOWNLOAD_URL" -o "${TMP_DIR}/${ARCHIVE_NAME}"; then
        tar -xzf "${TMP_DIR}/${ARCHIVE_NAME}" -C "$TMP_DIR"
        chmod +x "${TMP_DIR}/edio-${OS}-${ARCH}"
        mv "${TMP_DIR}/edio-${OS}-${ARCH}" "${INSTALL_DIR}/${BINARY_NAME}"
        echo "==> Successfully installed edio to ${INSTALL_DIR}/${BINARY_NAME}"
        verify_install "$INSTALL_DIR"
    else
        echo "Error: Failed to download prebuilt binary."
        echo "You can install manually using Go: go install github.com/${REPO}/cmd/edio@latest"
        exit 1
    fi
}

verify_install() {
    local dir="$1"
    if [[ ":$PATH:" != *":$dir:"* ]]; then
        echo ""
        echo "Note: ${dir} is not in your current PATH."
        echo "Add it by running:"
        echo "  export PATH=\"\$PATH:${dir}\""
        echo ""
    fi

    echo "==> Installation complete! Run 'edio' or 'edio ui' to get started."
}

main "$@"
