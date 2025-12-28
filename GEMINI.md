# Project Control: homectl

This project is a Go-based integration for Lutron Caseta/RA2 Select systems and Sonos speakers, featuring a Cobra CLI, Bubble Tea TUI, and a Lit Web UI.

## Developer Onboarding

- **Issue Tracking:** This project uses `bd` (beads).
- **Environment:**
    - **Go:** Core application logic and CLI.
    - **Config:** Standard XDG path `~/.config/homectl/`. Use `pkg/config` for path resolution.
- **Certs:** Certificates are required for communication and should be placed in the config directory.

## Application Architecture

- **Project Structure:**
    - `cmd/`: CLI commands (Cobra).
    - `pkg/config`: Centralized path management (`~/.config/homectl/`).
    - `pkg/leap`: Core Lutron LEAP protocol implementation.
    - `pkg/sonos`: Core Sonos UPnP/SOAP protocol implementation.
    - `pkg/tui`: Terminal UI (Bubble Tea) with multi-mode navigation.

## Architectural Principles

### 1. Centralized Configuration
- **Standard Paths:** Use `os.UserConfigDir()` to store all persistent state. Never hardcode paths to `secrets/` or the project root for runtime data.
- **pkg/config:** Always use `config.GetPath("filename")` to ensure consistency across the CLI, TUI, and background services.

### 2. Resilient IoT Pattern
- **Disconnection Handling:** IoT devices frequently reset idle connections. The clients must implement automatic reconnection logic.
- **Batching & Throttling:** Avoid high-concurrency bursts to a single bridge. Sequential processing with small delays (e.g., 50ms) is significantly more stable than concurrent goroutines for bulk commands.
- **ClientTag Pairing:** In asynchronous protocols like LEAP, every request must include a unique `ClientTag`. The client must loop through incoming messages and only return the response that matches the sent tag to prevent state desync.
- **Thread Safety:** When using `bufio.Reader` on a shared connection, the **entire** request/response cycle (including the read loop) must be protected by a mutex to prevent data corruption and slice-out-of-bounds panics.

### 3. Sonos Integration & SSDP/mDNS
- **Go-Native Discovery:** Prefer native Go mDNS (`_sonos._tcp`, `_leap._tcp`) over auxiliary Python scripts for zero-config onboarding.
- **UPnP/SOAP Actions:** Transport controls and metadata retrieval are implemented via SOAP over HTTP on port 1400.
- **Metadata Extraction:** Sonos often embeds escaped XML within XML (DIDL-Lite). Use robust string searching or specialized parsers to extract fields like `streamContent`, `albumArtURI`, and `NextURIMetaData`.
- **State Sync via Refresh:** After bulk operations, always trigger a full status refresh to ensure the UI state accurately reflects the hardware state.

## Future Vision
...
- **Unified Discovery Engine:** A background service to periodically scan for all supported devices.

## Coding Conventions

- **Packages:** Logic is decoupled into `pkg/`.
    - `pkg/leap`: Core protocol handling and data models.
    - `pkg/sonos`: UPnP/SOAP implementation.
    - `pkg/tui`: Interactive terminal views.
- **Naming Conventions:** 
    - Always use `tlsConfig` (not `lsConfig`) for `*tls.Config` variables.
- **Messaging:** All LEAP commands should include a `Header` and an optional `Body`.
- **Concurrency:** The `leap.Client` uses a `sync.Mutex` to protect the connection during read/write operations.
- **TUI Feedback:** Use a `statusMsg` pattern in Bubble Tea to provide non-blocking feedback. For bulk actions, return a `tea.Cmd` that performs the action and then triggers a refresh.
- **Explicit UI Markers:** Terminals vary in rendering colors/borders. Use explicit text markers (e.g., `[ LIGHTS ]`) for critical navigation like tabs.
- **Commits:** Use [Conventional Commits](https://www.conventionalcommits.org/).

### Golden Rules for TUI Development
1. **Pointers for State:** When updating sub-components (like `list.Model`) in a multi-mode TUI, use direct access to the model's fields rather than temporary pointer assignments to avoid syntax errors and "unexpected name" bugs.
2. **Avoid Fprintf for Data:** Never use `fmt.Fprintf` when the string being rendered might contain a data-driven `%` sign (like a progress bar's `80%`). Use `fmt.Fprint` or `fmt.Printf("%s", str)` to avoid `!(NOVERB)` errors.
3. **Clean Writes for Complex UI:** When the TUI structure changes significantly (e.g., adding tabs or modes), prefer a full `write_file` over incremental `replace` calls to ensure file integrity and avoid corruption.

### Common Pitfalls
- **Lutron 400 BadRequest:** Usually caused by a missing outer `Command` wrapper in the JSON body.
- **Sonos 405 Method Not Allowed:** Usually caused by an incorrect `controlURL` path (e.g., using `/RenderingControl` instead of `/MediaRenderer/RenderingControl/Control`).
- **I/O Timeouts:** Occur when polling too many IoT resources sequentially. Always prefer batch/collective endpoints.

## Changelog Management
To generate or update the `CHANGELOG.md` from completed tasks in `bd`, run:

```bash
bd list --status closed --json | jq -r 'sort_by(.closed_at) | reverse | map(select(.closed_at != null)) | group_by(.closed_at[0:10]) | reverse | .[] | "## " + (.[0].closed_at[0:10]) + "\n" + (map("- " + .title + " (" + .id + ")") | join("\n")) + "\n"' > CHANGELOG.md
```

