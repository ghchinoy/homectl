# Project Control: Lutron Integration

This project is a Go-based integration for Lutron Caseta/RA2 Select systems, featuring a Cobra CLI, Bubble Tea TUI, and a Lit Web UI.

## Developer Onboarding

- **Issue Tracking:** This project uses `bd` (beads). Refer to [AGENTS.md](./AGENTS.md) for core workflow instructions and run `bd prime` for full context.
- **Environment:**
    - **Go:** Core application logic and CLI.
    - **Python (via `uv`):** Used for auxiliary tasks like the initial pairing process.
    - **Certs:** Certificates are required for communication and are ignored by git. See [NETWORK_DISCOVERY.md](./NETWORK_DISCOVERY.md) for pairing instructions.

## Application Architecture

- **Project Structure:**
    - `cmd/`: CLI commands (Cobra). `getClient` in `root.go` manages secure connections.
    - `pkg/leap`: Core Lutron LEAP protocol implementation.
    - `pkg/sonos`: Core Sonos UPnP/SOAP protocol implementation.
    - `pkg/tui`: Terminal UI (Bubble Tea) with multi-mode navigation.
    - `secrets/`: Local storage for credentials (ignored by git).
    - `tools/`: Python and Go utilities for discovery and pairing.

## Architectural Principles

### 2. Resilient IoT Pattern
- **Disconnection Handling:** IoT devices (Lutron, Sonos) frequently reset idle connections. The `leap.Client` and `sonos.Client` must implement automatic reconnection logic within their `Request` methods.
- **Batching & Throttling:** Avoid high-concurrency bursts to a single IoT bridge. Sequential processing with small delays (e.g., 50ms) is significantly more stable than concurrent goroutines for bulk commands (e.g. "All Lights").
- **ClientTag Pairing:** In asynchronous protocols like LEAP, every request must include a unique `ClientTag`. The client must loop through incoming messages and only return the response that matches the sent tag to prevent state desync.

### 2. Optimistic UI Updates
- **Snappiness:** To hide network latency, the TUI model should update its local state **immediately** upon user input. The network command is dispatched as an asynchronous `tea.Cmd`, and the TUI only rolls back or alerts if the command fails.

### 3. Mode-Based Hub
- **Navigation:** Use a "Hub" model with `sessionMode` and tabs (Lights, Music, etc.) to handle multiple device categories without cluttering the UI.

## Lessons Learned: Lutron Discovery & LEAP

- **mDNS is Key:** Lutron bridges announce themselves via `_lutron._tcp` and `_leap._tcp`.
- **LEAP Protocol:** Communication happens over TLS on port `8081`. 
- **Strict TLS:** The bridge requires client-side certificates. You cannot even "ping" the LEAP protocol without a valid handshake.
- **Pairing Flow:** Pairing requires a specific socket handshake followed by a physical button press on the bridge to generate a unique certificate/key pair for that client.
- **Asynchronous Responses:** The bridge frequently sends unsolicited `SubscribeResponse` messages (status updates) over the same connection. The client must loop and filter incoming messages to find the specific `ReadResponse` or `CreateResponse` it is waiting for.
- **Batching for Performance:** Polling individual device statuses is slow and prone to timeouts. Using `/zone/status` to get all levels in a single request is significantly more efficient.
- **JSON Structure:** LEAP commands for dimming require a specific nested structure: `{"Command": {"CommandType": "GoToLevel", ...}}`. Missing the outer `Command` wrapper results in a `400 BadRequest`.

### 3. Sonos Integration & SSDP
- **Discovery via SSDP:** Sonos devices are best discovered using SSDP (`urn:schemas-upnp-org:device:ZonePlayer:1`). Fetching the device description from `http://<ip>:1400/xml/device_description.xml` provides the `friendlyName`.
- **UPnP/SOAP Actions:** Transport controls (Play, Pause, Next, Previous) and Rendering controls (Volume) are implemented via SOAP over HTTP on port 1400.
- **State Sync via Refresh:** After bulk operations (like turning off all lights), always trigger a full status refresh (`refreshLights`) to ensure the TUI state matches the hardware state.
- **Dynamic Refresh in TUI:** Implementing a `rediscover` mechanism for IoT devices allows the TUI to recover from network changes without restarting.

## Future Vision

- **Multi-Device Integration:** Expand the CLI/TUI to support Sonos, LG, and GE appliances discovered on the network.
- **Native Go Discovery:** Implement mDNS/SSDP discovery directly in Go to eliminate the Python `uv` dependency for onboarding.
- **Web Dashboard:** A Lit WebComponent UI served directly by the Go binary for cross-platform control.

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

