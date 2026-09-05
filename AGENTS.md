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
- **Port Probing:** If mDNS is unavailable or hidden, use throttled TCP port probes (e.g., 554 for RTSP) to identify live IoT hosts on the subnet.

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
- **CI Consistency Gate:** Run `make check-skills` (or `go run ./cmd/sync-skills --check`). CI verifies that all plugin skills match canonical sources and asserts that all `scripts/...` paths referenced in `SKILL.md` exist inside the bundle.
- **Binary Artifacts:** All compiled binaries (`homectl`, `mcp-sonos`, `sync-skills`) build into `./bin/` via `make build`. The `./bin` directory is strictly gitignored.

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

<!-- BEGIN BEADS CODEX SETUP: generated by bd setup codex -->
## Beads Issue Tracker

Use Beads (`bd`) for durable task tracking in repositories that include it. Use the `beads` skill at `.agents/skills/beads/SKILL.md` (project install) or `~/.agents/skills/beads/SKILL.md` (global install) for Beads workflow guidance, then use the `bd` CLI for issue operations.

### Quick Reference

```bash
bd ready                # Find available work
bd show <id>            # View issue details
bd update <id> --claim  # Claim work
bd close <id>           # Complete work
bd prime                # Refresh Beads context
```

### Rules

- Use `bd` for all task tracking; do not create markdown TODO lists.
- Run `bd prime` when Beads context is missing or stale. Codex 0.129.0+ can load Beads context automatically through native hooks; use `/hooks` to inspect or toggle them.
- Keep persistent project memory in Beads via `bd remember`; do not create ad hoc memory files.

**Architecture in one line:** issues live in a local Dolt DB; sync uses `refs/dolt/data` on your git remote; `.beads/issues.jsonl` is a passive export. See https://github.com/gastownhall/beads/blob/main/docs/core-concepts/sync-concepts.md for details and anti-patterns.
<!-- END BEADS CODEX SETUP -->
