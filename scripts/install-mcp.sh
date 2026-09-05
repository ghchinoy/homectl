#!/usr/bin/env bash

# homectl MCP installer script
# Builds mcp-sonos, installs it to ~/.local/bin/homectl-mcp-sonos,
# and registers it into ~/.config/opencode/opencode.jsonc.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_DEST="${HOME}/.local/bin"
TARGET_NAME="homectl-mcp-sonos"
TARGET_BIN="${BIN_DEST}/${TARGET_NAME}"
OPENCODE_CONFIG="${HOME}/.config/opencode/opencode.jsonc"

echo "==> Building mcp-sonos..."
mkdir -p "${SCRIPT_DIR}/bin"
go build -o "${SCRIPT_DIR}/bin/mcp-sonos" "${SCRIPT_DIR}/cmd/mcp-sonos"

echo "==> Installing binary to ${TARGET_BIN}..."
mkdir -p "${BIN_DEST}"
cp "${SCRIPT_DIR}/bin/mcp-sonos" "${TARGET_BIN}"
chmod +x "${TARGET_BIN}"

if [ -f "${OPENCODE_CONFIG}" ]; then
    echo "==> Registering in OpenCode config: ${OPENCODE_CONFIG}..."
    BACKUP="${OPENCODE_CONFIG}.bak.$(date +%Y%m%d%H%M%S)"
    cp "${OPENCODE_CONFIG}" "${BACKUP}"
    echo "    Backup saved to ${BACKUP}"

    jq --arg bin "${TARGET_BIN}" \
       '.mcp.sonos = { "type": "local", "command": [$bin], "enabled": true }' \
       "${OPENCODE_CONFIG}" > "${OPENCODE_CONFIG}.tmp"
    mv "${OPENCODE_CONFIG}.tmp" "${OPENCODE_CONFIG}"
    echo "==> Successfully registered sonos MCP server in OpenCode!"
else
    echo "==> Note: ${OPENCODE_CONFIG} not found. Configuration skipped."
    echo "    To manually register, add:"
    echo '    "sonos": { "type": "local", "command": ["'"${TARGET_BIN}"'"], "enabled": true }'
fi

echo "==> Verification: testing binary..."
if [ -x "${TARGET_BIN}" ]; then
    echo "    Binary ${TARGET_BIN} is executable and ready."
fi
echo "==> Done! OpenCode can now launch the Sonos MCP server from any working directory."
