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
make -C "${SCRIPT_DIR}" bin-mcp-sonos

echo "==> Installing binary to ${TARGET_BIN}..."
mkdir -p "${BIN_DEST}"
cp "${SCRIPT_DIR}/bin/mcp-sonos" "${TARGET_BIN}.tmp"
chmod +x "${TARGET_BIN}.tmp"
mv -f "${TARGET_BIN}.tmp" "${TARGET_BIN}"

if [ -f "${OPENCODE_CONFIG}" ]; then
    echo "==> Registering in OpenCode config: ${OPENCODE_CONFIG}..."
    BACKUP="${OPENCODE_CONFIG}.bak.$(date +%Y%m%d%H%M%S)"
    cp "${OPENCODE_CONFIG}" "${BACKUP}"
    echo "    Backup saved to ${BACKUP}"

    python3 - "${OPENCODE_CONFIG}" "${TARGET_BIN}" <<'PY'
import json
import sys

cfg_path, target_bin = sys.argv[1], sys.argv[2]
with open(cfg_path, "r", encoding="utf-8") as f:
    raw = f.read()

# Strip JSONC // and /* */ comments and trailing commas while preserving quoted strings
out = []
i, n = 0, len(raw)
in_str = False
while i < n:
    ch = raw[i]
    if in_str:
        out.append(ch)
        if ch == "\\" and i + 1 < n:
            out.append(raw[i + 1])
            i += 2
            continue
        if ch == '"':
            in_str = False
        i += 1
        continue
    if ch == '"':
        in_str = True
        out.append(ch)
        i += 1
        continue
    if ch == "/" and i + 1 < n and raw[i + 1] == "/":
        i += 2
        while i < n and raw[i] not in "\r\n":
            i += 1
        continue
    if ch == "/" and i + 1 < n and raw[i + 1] == "*":
        i += 2
        while i + 1 < n and not (raw[i] == "*" and raw[i + 1] == "/"):
            i += 1
        i += 2
        continue
    out.append(ch)
    i += 1

cleaned = "".join(out)
# Remove trailing commas before } or ] outside strings
final_chars = []
i, m = 0, len(cleaned)
in_str = False
while i < m:
    ch = cleaned[i]
    if in_str:
        final_chars.append(ch)
        if ch == "\\" and i + 1 < m:
            final_chars.append(cleaned[i + 1])
            i += 2
            continue
        if ch == '"':
            in_str = False
        i += 1
        continue
    if ch == '"':
        in_str = True
        final_chars.append(ch)
        i += 1
        continue
    if ch == ",":
        j = i + 1
        while j < m and cleaned[j] in " \t\r\n":
            j += 1
        if j < m and cleaned[j] in "}]":
            i += 1
            continue
    final_chars.append(ch)
    i += 1

doc = json.loads("".join(final_chars) or "{}")
if not isinstance(doc.get("mcp"), dict):
    doc["mcp"] = {}
doc["mcp"]["sonos"] = {
    "type": "local",
    "command": [target_bin],
    "enabled": True,
}

with open(cfg_path + ".tmp", "w", encoding="utf-8") as f:
    json.dump(doc, f, indent=2)
    f.write("\n")
PY
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
