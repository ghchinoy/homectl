# Project Control: Lutron Integration

This project is a Go-based integration for Lutron Caseta/RA2 Select systems, featuring a Cobra CLI, Bubble Tea TUI, and a Lit Web UI.

## Developer Onboarding

- **Issue Tracking:** This project uses `bd` (beads). Refer to [AGENTS.md](./AGENTS.md) for core workflow instructions and run `bd prime` for full context.
- **Environment:**
    - **Go:** Core application logic and CLI.
    - **Python (via `uv`):** Used for auxiliary tasks like the initial pairing process.
    - **Certs:** Certificates are required for communication and are ignored by git. See [lutron_discovery.md](./lutron_discovery.md) for pairing instructions.

## Application Architecture

- **Project Structure:**
    - `cmd/`: CLI commands (Cobra). `getClient` helper in `root.go` manages the secure bridge connection.
    - `pkg/leap`: Core LEAP protocol implementation, data models, and connection management.
    - `pkg/tui`: Terminal UI implementation (Bubble Tea).
    - `secrets/`: Local storage for TLS certificates (ignored by git).
    - `tools/`: Python-based discovery and pairing utilities.
- **LEAP Client (`pkg/leap`):** Handles TLS, JSON messaging, and response filtering. It maps Lutron's hierarchical model (Areas -> Devices -> Zones).
- **TUI (`pkg/tui`):** Built with Bubble Tea. Uses a `list.Model` for navigation and sends asynchronous `tea.Cmd` messages to trigger LEAP commands.

## Lessons Learned: Lutron Discovery & LEAP

- **mDNS is Key:** Lutron bridges announce themselves via `_lutron._tcp` and `_leap._tcp`.
- **LEAP Protocol:** Communication happens over TLS on port `8081`. 
- **Strict TLS:** The bridge requires client-side certificates. You cannot even "ping" the LEAP protocol without a valid handshake.
- **Pairing Flow:** Pairing requires a specific socket handshake followed by a physical button press on the bridge to generate a unique certificate/key pair for that client.
- **Asynchronous Responses:** The bridge frequently sends unsolicited `SubscribeResponse` messages (status updates) over the same connection. The client must loop and filter incoming messages to find the specific `ReadResponse` or `CreateResponse` it is waiting for.
- **Batching for Performance:** Polling individual device statuses is slow and prone to timeouts. Using `/zone/status` to get all levels in a single request is significantly more efficient.
- **JSON Structure:** LEAP commands for dimming require a specific nested structure: `{"Command": {"CommandType": "GoToLevel", ...}}`. Missing the outer `Command` wrapper results in a `400 BadRequest`.

## Future Vision

- **Multi-Device Integration:** Expand the CLI/TUI to support Sonos, LG, and GE appliances discovered on the network.
- **Native Go Discovery:** Implement mDNS/SSDP discovery directly in Go to eliminate the Python `uv` dependency for onboarding.
- **Web Dashboard:** A Lit WebComponent UI served directly by the Go binary for cross-platform control.

## Coding Conventions

- **Packages:** Logic is decoupled into `pkg/`.
    - `pkg/leap`: Core protocol handling and data models.
    - `pkg/tui`: Interactive terminal views.
- **Naming Conventions:** 
    - Always use `tlsConfig` (not `lsConfig`) for `*tls.Config` variables to maintain consistency and avoid repeated typos.
- **Messaging:** All LEAP commands should include a `Header` and an optional `Body`.
- **Concurrency:** The `leap.Client` uses a `sync.Mutex` to protect the connection during read/write operations.
- **TUI Feedback:** Use a `statusMsg` pattern in Bubble Tea to provide non-blocking feedback to the user after network operations.
- **Commits:** Use [Conventional Commits](https://www.conventionalcommits.org/).

## Changelog Management
To generate or update the `CHANGELOG.md` from completed tasks in `bd`, run:

```bash
bd list --status closed --json | jq -r 'sort_by(.closed_at) | reverse | map(select(.closed_at != null)) | group_by(.closed_at[0:10]) | reverse | .[] | "## " + (.[0].closed_at[0:10]) + "\n" + (map("- " + .title + " (" + .id + ")") | join("\n")) + "\n"' > CHANGELOG.md
```

