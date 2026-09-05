#!/usr/bin/env bash
# scripts/build-diagrams.sh
# Compiles all Graphviz .dot architecture files in docs/src/assets/architecture to .webp

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
DIAGRAMS_DIR="${ROOT_DIR}/docs/src/assets/architecture"

if ! command -v dot >/dev/null 2>&1; then
    echo "Error: Graphviz 'dot' executable not found on PATH." >&2
    echo "Install via Homebrew: brew install graphviz" >&2
    exit 1
fi

echo "Compiling Graphviz architecture diagrams (.dot -> .webp)..."
mkdir -p "${DIAGRAMS_DIR}"

count=0
for dotfile in "${DIAGRAMS_DIR}"/*.dot; do
    if [[ ! -f "$dotfile" ]]; then
        continue
    fi
    base_name="$(basename "$dotfile" .dot)"
    webp_out="${DIAGRAMS_DIR}/${base_name}.webp"
    
    dot -Twebp "$dotfile" -o "$webp_out"
    size=$(du -h "$webp_out" | awk '{print $1}')
    echo "  ✓ Generated: ${base_name}.webp (${size})"
    count=$((count + 1))
done

# Mirror service diagrams into plugin bundles if directories exist
if [[ -d "${ROOT_DIR}/plugins/sonos" ]]; then
    mkdir -p "${ROOT_DIR}/plugins/sonos/assets"
    cp "${DIAGRAMS_DIR}/sonos.webp" "${ROOT_DIR}/plugins/sonos/assets/architecture.webp" 2>/dev/null || true
fi

echo "Successfully built ${count} architecture diagrams in ${DIAGRAMS_DIR}."
