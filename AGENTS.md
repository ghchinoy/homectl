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