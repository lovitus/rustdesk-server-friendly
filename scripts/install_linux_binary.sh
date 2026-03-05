#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-latest}"
OWNER_REPO="lovitus/rustdesk-server-friendly"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
DOWNLOAD_TOOL="${DOWNLOAD_TOOL:-auto}"  # auto|curl|wget

choose_tool() {
  case "$DOWNLOAD_TOOL" in
    curl|wget) echo "$DOWNLOAD_TOOL" ;;
    auto)
      if command -v curl >/dev/null 2>&1; then echo curl; return; fi
      if command -v wget >/dev/null 2>&1; then echo wget; return; fi
      echo ""
      ;;
    *) echo "" ;;
  esac
}

fetch() {
  url="$1"
  out="$2"
  tool="$(choose_tool)"
  [ -n "$tool" ] || { echo "[STOP] neither curl nor wget found"; exit 1; }
  if [ "$tool" = "curl" ]; then
    curl -fL "$url" -o "$out"
  else
    wget -O "$out" "$url"
  fi
}

ARCH=$(uname -m)
case "$ARCH" in
  x86_64) ASSET="rustdesk-friendly-linux-amd64" ;;
  aarch64|arm64) ASSET="rustdesk-friendly-linux-arm64" ;;
  *) echo "[STOP] unsupported arch: $ARCH"; exit 1 ;;
esac

if [ "$VERSION" = "latest" ]; then
  API_URL="https://api.github.com/repos/$OWNER_REPO/releases/latest"
  if command -v curl >/dev/null 2>&1; then
    TAG=$(curl -fsSL "$API_URL" | awk -F '"' '/"tag_name":/{print $4; exit}')
  else
    TAG=$(wget -qO- "$API_URL" | awk -F '"' '/"tag_name":/{print $4; exit}')
  fi
else
  TAG="$VERSION"
fi

URL="https://github.com/$OWNER_REPO/releases/download/$TAG/$ASSET"
TMP="$(mktemp)"
trap 'rm -f "$TMP"' EXIT
fetch "$URL" "$TMP"

sudo install -d "$INSTALL_DIR"
sudo install -m 0755 "$TMP" "$INSTALL_DIR/rustdesk-friendly"

rustdesk-friendly version || true
echo "[OK] installed to $INSTALL_DIR/rustdesk-friendly"
