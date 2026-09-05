---
title: Plugin Packaging & Authoring Strategy
description: How homectl packages Agent Plugins, synchronizes canonical skills, and validates bundles in CI.
---

`homectl` adheres to the **Agent Plugins Specification 1.0.0**, distributing each smart home service as a self-contained, versioned plugin bundle that can be loaded into OpenCode, Claude Desktop, or standard agent orchestrators.

---

## 1. Directory Conventions & Single Source of Truth

To prevent split-brain drift between skill definitions, helper scripts, and distribution manifests, `homectl` enforces a strict **Single Source of Truth** convention:

```text
homectl/
├── skills/                            # [CANONICAL SOURCE OF TRUTH]
│   └── sonos-soundscape/
│       ├── SKILL.md                   # Editable canonical skill markdown
│       └── scripts/
│           └── summarize_metadata.py  # Bundled deterministic helper scripts
│
├── plugins/                           # [GENERATED PLUGIN BUNDLES]
│   └── sonos/
│       ├── plugin.json                # Universal Agent Plugins Spec manifest
│       ├── mcp.json                   # Standard MCP server stdio definition
│       ├── opencode.jsonc             # OpenCode-native local server config
│       ├── assets/
│       │   └── architecture.webp      # Bundled architecture diagram
│       └── skills/                    # Generated bundle (DO NOT EDIT DIRECTLY)
│           └── sonos-soundscape/
│               ├── SKILL.md
│               └── scripts/
│                   └── summarize_metadata.py
```

### The Cardinal Rule
* **Edit ONLY in `skills/<name>/`**: Never manually modify files under `plugins/*/skills/`.
* **Sync via automation**: Run `make sync-skills` to mirror canonical skills into their respective plugin bundles.

---

## 2. Manifest Specifications

Each plugin bundle under `plugins/<svc>/` contains three configuration manifests:

### 1. `plugin.json` (Agent Plugins Spec 1.0.0)
Defines metadata, semantic version, license, and keywords:
```json
{
  "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
  "name": "homectl-sonos",
  "version": "0.1.0",
  "description": "Smart home audio plugin for Sonos speakers: discovery, compact track metadata, playback control, and volume management.",
  "author": {
    "name": "ghchinoy",
    "url": "https://github.com/ghchinoy"
  },
  "license": "Apache-2.0",
  "keywords": ["sonos", "smart-home", "audio", "mcp", "upnp", "homectl"]
}
```

### 2. `mcp.json` (Standard MCP Stdio Declaration)
Defines how general MCP clients spawn the server:
```json
{
  "$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
  "mcpServers": {
    "sonos": {
      "type": "stdio",
      "command": "mcp-sonos",
      "args": []
    }
  }
}
```

### 3. `opencode.jsonc` (OpenCode-Native Configuration)
Maps the compiled binary relative to the repository root for immediate zero-config execution:
```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "sonos": {
      "type": "local",
      "command": ["./bin/mcp-sonos"],
      "enabled": true
    }
  }
}
```

---

## 3. Synchronization Tooling (`cmd/sync-skills`)

`homectl` includes a dedicated Go synchronization engine in `cmd/sync-skills/main.go`.

### Synchronizing Skills
```bash
make sync-skills
# or: go run ./cmd/sync-skills
```

What `sync-skills` does:
1. Scans `plugins/` to discover all declared plugin bundles.
2. Identifies associated canonical skills in `skills/`.
3. Clears and re-mirrors the canonical `SKILL.md` and `scripts/**` into `plugins/<svc>/skills/<name>/`.
4. Copies `.webp` architecture diagrams into `plugins/<svc>/assets/`.
5. Generates or updates `opencode.jsonc` with the local executable command.

---

## 4. The CI Consistency Gate (`make check-skills`)

To prevent contributors from modifying a canonical skill without syncing the plugin bundle, `homectl` runs a strict consistency gate in CI:

```bash
make check-skills
# or: go run ./cmd/sync-skills --check
```

### What `make check-skills` asserts:
1. **Byte-for-Byte Sync:** Confirms that `plugins/<svc>/skills/<name>/SKILL.md` exactly matches `skills/<name>/SKILL.md`.
2. **Script Reference Validation:** Scans `SKILL.md` for any backtick references to `scripts/...` (e.g. `` `scripts/summarize_metadata.py` ``). It verifies that:
   * The referenced script exists inside the bundle.
   * The script is marked executable (`chmod +x`).
3. **Manifest Validity:** Asserts that `opencode.jsonc`, `plugin.json`, and `mcp.json` are syntactically valid and reference existing binaries.

If a contributor edits `skills/` and forgets to run `make sync-skills`, `make check-skills` fails loudly with an actionable error message.

---

## 5. Authoring a New Agent Plugin & Skill

To add a new smart home capability (e.g. `plugins/lutron` with `skills/lutron-lighting`):

1. **Create Canonical Skill:**
   Author your skill rules in `skills/<name>/SKILL.md` and add any deterministic helper scripts to `skills/<name>/scripts/`.
2. **Create Plugin Scaffold:**
   Create `plugins/<svc>/plugin.json` declaring the plugin name, version, and license.
3. **Run Synchronization:**
   ```bash
   make sync-skills
   ```
4. **Build Binaries & Verify:**
   ```bash
   make build
   make check-skills
   ```
5. **Commit:**
   Commit the canonical skill, the generated plugin files, and updated manifests together using Conventional Commits (`feat(plugins): add lutron plugin bundle`).
