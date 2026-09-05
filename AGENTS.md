# Agent Instructions

This project uses **bd** (beads) for issue tracking. Run `bd onboard` to get started.

## Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --status in_progress  # Claim work
bd close <id>         # Complete work
bd sync               # Sync with git
```

## Developer Guidelines

### 1. IoT & Discovery
- **Efficient Polling:** Never poll multiple IoT resources sequentially if a collective/batch endpoint exists (e.g., use `/zone/status` instead of querying each zone individually).
- **Resilient Connections:** Assume all IoT connections will eventually reset; implement auto-reconnection logic.
- **Library Safety:** Avoid raw struct instantiation for external library clients (e.g., `go-chromecast`). Always use provided constructors (e.g., `NewApplication`) to ensure internal fields like storage/cache are initialized and avoid nil-pointer panics.
- **Port Probing & Concurrent Discovery:** Camera discovery runs mDNS browsing and throttled TCP port 554 subnet scanning concurrently under bounded timeouts to prevent mDNS from starving RTSP probes.
- **SSDP M-SEARCH Fallback:** When mDNS yields zero Sonos speakers (common on macOS or segmented LANs), fallback to UPnP SSDP M-SEARCH (`urn:schemas-upnp-org:device:ZonePlayer:1`) ensures device discovery.
- **IPv4 Preference & Link-Local Rejection:** Always prioritize usable IPv4 addresses over IPv6 during mDNS discovery, and explicitly reject link-local IPv6 addresses (`fe80::`) which fail without interface zone indices.
- **Sonos Stereo-Pair/Group Followers:** A stereo-pair or group *follower* reports its transport as `PLAYING` with a `TrackURI` of `x-rincon:<coordinator-rincon>` and empty track metadata — a false positive. Never report a speaker's playback state directly; if `TrackURI` starts with `x-rincon:`, resolve the group coordinator via `Client.GetCoordinatorIP()` (or inspect `Client.GetZoneGroupState()`) and re-query the coordinator for authoritative state. The `sonos_get_now_playing` MCP tool does this automatically and returns `is_follower`/`coordinator_ip`; `sonos_get_topology` exposes the full group/pair structure.

### 2. Web UI (Lit + Vite)
- **Component Isolation:** Keep Lit components atomic and isolated (e.g., `lutron-card`, `sonos-card`). Use properties for data-in and custom events for data-out.
- **Centralized API:** All HTTP calls must go through a centralized service utility (e.g., `ui/src/api.ts`) to maintain consistency and ease of maintenance.
- **Type Safety:** Use `import type` when importing interfaces in TypeScript to avoid `verbatimModuleSyntax` build errors.
- **Proxy Pattern:** Use the Go backend as a proxy/transcoder for protocols browsers cannot handle (RTSP, UPnP Art). 
- **Metadata Handling:** Always use `html.UnescapeString` on IoT-provided metadata before using it in the UI or proxy requests to avoid issues with escaped characters (e.g., `&amp;`).

### 3. Deployment & Standards
- **XDG Compliance:** Always use `pkg/config.GetPath(filename)` for all persistent configuration, certificates, logs, and state files. Never hardcode home directory paths.
- **Service Deployment:** Maintain `scripts/install.sh` as the primary installation and update method for Linux. The service runs the `serve` command and points to `/usr/local/share/homectl/ui` by default.
- **State Persistence:** Use `pkg/config` utilities like `LoadNicknames` and `SaveNicknames` to manage user-defined metadata consistently across CLI, TUI, and Web UI.

### 4. Coding Conventions & Common Pitfalls
- **Commits:** Use [Conventional Commits](https://www.conventionalcommits.org/) (e.g. `feat:`, `fix:`, `refactor:`, `docs:`).
- **RTSP 401 Unauthorized:** Security cameras require `camera_auth` in `config.json` (format: `user:pass`).
- **Art Proxy 404:** Check for double-escaped `&amp;` in the `path` query parameter.
- **Omnivision / OV Ready:** ADC cameras often identify as `Server: OV Ready` on ports 6080/6443. These use a proprietary protocol; RTSP on port 554 is the standard local stream.

### 5. Changelog Management
To generate or update `CHANGELOG.md` from completed tasks in `bd`:
```bash
bd list --status closed --json | jq -r 'sort_by(.closed_at) | reverse | map(select(.closed_at != null)) | group_by(.closed_at[0:10]) | reverse | .[] | "## " + (.[0].closed_at[0:10]) + "\n" + (map("- " + .title + " (" + .id + ")") | join("\n")) + "\n"' > CHANGELOG.md
```

### 6. Skills & Plugin Packaging Strategy
- **Canonical Skill Source:** Top-level `skills/<name>/` is the single source of truth for authoring skills. Edit `SKILL.md` and helper scripts (`skills/<name>/scripts/**`) ONLY in this directory.
- **Self-Contained Plugin Bundles:** `plugins/<svc>/skills/<name>/` is a generated, self-contained bundle adhering to the Agent Plugins Specification. Never hand-edit files under `plugins/*/skills/`.
- **Synchronization:** Run `make sync-skills` (or `go run ./cmd/sync-skills`) to mirror canonical skills into their respective plugin bundles (including scripts). Plugins declare their bundled skills via `plugins/<svc>/bundle.json` or existing subdirectories.
- **Dual Manifests:** Each plugin provides both the standard Agent Plugins Spec `mcp.json` (`mcpServers.<svc>`) and an OpenCode-native `opencode.jsonc` (`mcp.<svc>.type="local"`, `command: ["./bin/mcp-<svc>"]`). Both are generated and asserted by `make check-skills`.
- **CI Consistency Gate:** Run `make check-skills` (or `go run ./cmd/sync-skills --check`). CI verifies that all plugin skills match canonical sources and asserts that all `scripts/...` paths referenced in `SKILL.md` exist inside the bundle.
- **MCP Tool Output Schemas:** Per MCP SEP-2106 and OpenCode schema validation, tool `Out` types MUST be Go structs/records (JSON objects), never bare slices/arrays. For list operations, always return a wrapping struct (e.g. `type ListResult struct { Count int; Items []T }`).
- **Binary Artifacts:** All compiled binaries (`homectl`, `mcp-sonos`, `sync-skills`) build into `./bin/` via `make build`. The `./bin` directory is strictly gitignored.
- **Global MCP Installation:** Run `make install-mcp` (or `./scripts/install-mcp.sh`) to install MCP binaries to `~/.local/bin/homectl-mcp-<svc>` and register them into `~/.config/opencode/opencode.jsonc` with absolute executable paths, decoupling agent invocations from the repository working directory.

### 7. Privacy, PII & Hardware Redaction
- **Local State Isolation (`local/`):** Real physical hardware MAC addresses, static LAN IP allocations, physical camera entryway placements, and device serial numbers belong **ONLY in the gitignored `local/` directory** (e.g., `local/NETWORK_DISCOVERY.md`, `local/config.json`). Never commit or stage files under `local/`.
- **Public Documentation Sanitization:** All tracked repository files (`NETWORK_DISCOVERY.md`, `README.md`, `docs/**`, test fixtures) must strictly use sanitized placeholders:
  - **MAC Addresses:** Mask the lower 24 bits to `XX:XX:XX` while preserving the vendor OUI prefix for technical value (e.g., `CC:33:31:XX:XX:XX` for Lutron, `00:0E:58:XX:XX:XX` / `38:42:0B:XX:XX:XX` for Sonos, `B8:3A:9D:XX:XX:XX` for Alarm.com, `3C:31:78:XX:XX:XX` for Qolsys).
  - **IP Addresses & Subnets:** Use standard documentation subnets (`192.168.1.0/24`) or RFC 5737 ranges; never hardcode personal LAN subnets or personal static IP maps.
  - **Device & Room Names:** Use generic functional roles ("Entryway Doorbell", "Perimeter Camera", "Living Room Speaker", "Alarm Panel") rather than private floorplan descriptions ("Cat Room TV", "Front Door ADC-VDB770").
  - **Device Serials & UDNs:** Mask hardware serial numbers and Sonos Rincon IDs (`RINCON_XXXXXXXXXXXXXXXXX`).
- **Pre-Commit Verification:** Before staging or committing, assert no real MAC addresses or physical residence details are included (`git diff`). If a leak occurs, purge it via `git-filter-repo` before pushing to public remotes.

### 8. Architecture Documentation & Diagrams (Graphviz DOT ➔ WebP)
- **Canonical Diagram Source:** All architecture diagrams are authored as Graphviz DOT files in `docs/src/assets/architecture/<name>.dot`.
- **Compilation Toolchain (`make diagrams`):** Run `make diagrams` (or `./scripts/build-diagrams.sh`) to recompile all `.dot` files into high-efficiency `.webp` images via Graphviz (`dot -Twebp`). The script automatically mirrors service diagrams into `plugins/<svc>/assets/architecture.webp`.
- **Starlight Documentation Pages:** Deep-dive documentation lives in `docs/src/content/docs/architecture/<service>.md` and is registered in the sidebar via `docs/astro.config.mjs`.
- **Diagram Aesthetic Conventions:**
  - **Typography:** `fontname="Inter, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif"`.
  - **Node Style:** `shape=box, style="filled,rounded"`, HTML-like table labels with subtitle rows.
  - **Color Palette:** Blue (`#0284c7`) for clients/Lutron, Purple (`#7c3aed`) for plugins/skills, Green (`#059669`) for gateways/Cast, Amber/Orange (`#d97706`/`#ea580c`) for modules/Sonos, Red (`#dc2626`) for physical hardware/Qolsys.
- **Git Hygiene:** Always commit `.dot` source files and compiled `.webp` images together in the same commit. Intermediate `*.png` files are strictly gitignored.
- **Documentation Verification Gate:** Before completing an architecture change, assert that `npm --prefix docs run build` succeeds with zero errors and zero broken links.

## Landing the Plane (Session Completion)

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   bd sync
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds

<!-- BEGIN BEADS INTEGRATION v:1 profile:minimal hash:46cd31e7 -->
## Beads Issue Tracker

This project uses **bd (beads)** for issue tracking. Run `bd prime` to see full workflow context and commands.

### Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --claim  # Claim work
bd close <id>         # Complete work
```

### Rules

- Use `bd` for ALL task tracking — do NOT use TodoWrite, TaskCreate, or markdown TODO lists
- Run `bd prime` for detailed command reference and session close protocol
- Use `bd remember` for persistent knowledge — do NOT use MEMORY.md files

**Architecture in one line:** issues live in a local Dolt DB; sync uses `refs/dolt/data` on your git remote; `.beads/issues.jsonl` is a passive export. See https://github.com/gastownhall/beads/blob/main/docs/core-concepts/sync-concepts.md for details and anti-patterns.

## Agent Context Profiles

The managed Beads block is task-tracking guidance, not permission to override repository, user, or orchestrator instructions.

- **Conservative (default)**: Use `bd` for task tracking. Do not run git commits, git pushes, or Dolt remote sync unless explicitly asked. At handoff, report changed files, validation, and suggested next commands.
- **Minimal**: Keep tool instruction files as pointers to `bd prime`; use the same conservative git policy unless active instructions say otherwise.
- **Team-maintainer**: Only when the repository explicitly opts in, agents may close beads, run quality gates, commit, and push as part of session close. A current "do not commit" or "do not push" instruction still wins.

## Session Completion

This protocol applies when ending a Beads implementation workflow. It is subordinate to explicit user, repository, and orchestrator instructions.

1. **File issues for remaining work** - Create beads for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **Handle git/sync by active profile**:
   ```bash
   # Conservative/minimal/default: report status and proposed commands; wait for approval.
   git status

   # Team-maintainer opt-in only, unless current instructions forbid it:
   git pull --rebase
   bd dolt push
   git push
   git status
   ```
5. **Hand off** - Summarize changes, validation, issue status, and any blocked sync/commit/push step

**Critical rules:**
- Explicit user or orchestrator instructions override this Beads block.
- Do not commit or push without clear authority from the active profile or the current user request.
- If a required sync or push is blocked, stop and report the exact command and error.
<!-- END BEADS INTEGRATION -->
