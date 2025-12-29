# homectl

This project is a Go-based integration for Lutron Caseta/RA2 Select systems and Sonos speakers, featuring a Cobra CLI, Bubble Tea TUI, and a Lit Web UI.

## Developer Onboarding

- **Issue Tracking:** This project uses `bd` (beads).
- **Environment:**
    - **Go:** Core application logic and CLI.
    - **Node.js/npm:** Required for the `ui/` directory (Lit + Vite).
    - **Config:** Standard XDG path `~/.config/homectl/`. Use `pkg/config` for path resolution.
- **Certs:** Certificates are required for communication and should be placed in the config directory.

## Application Architecture

- **Project Structure:**
    - `cmd/`: CLI commands (Cobra) and API Server (`serve`).
    - `pkg/config`: Centralized path management (`~/.config/homectl/`).
    - `pkg/leap`: Core Lutron LEAP protocol implementation.
    - `pkg/sonos`: Core Sonos UPnP/SOAP protocol implementation.
    - `pkg/camera`: Dynamic RTSP camera discovery (Port 554).
    - `pkg/tui`: Terminal UI (Bubble Tea).
    - `ui/`: Web UI (Lit, TypeScript, Vite).

## Architectural Principles

### 1. Centralized Configuration
- **Standard Paths:** Use `os.UserConfigDir()` to store all persistent state.
- **pkg/config:** Always use `config.GetPath("filename")` to ensure consistency.

### 2. API Proxy Pattern
- **IoT-to-Web Bridge:** The Go backend acts as a proxy/transcoder for protocols browsers can't handle (e.g., transcoding RTSP to MJPEG via `ffmpeg`, proxying Sonos Art).
- **Unescaping Metadata:** Always use `html.UnescapeString` before using IoT strings in proxy requests or UI to avoid 404s from double-escaped entities (e.g., `&amp;`).

### 3. Component Isolation (Web UI)
- **Centralized API Service:** Use `ui/src/api.ts` for all HTTP calls.
- **Dumb Components / Smart Parent:** Cards (`lutron-card`, `sonos-card`) should be isolated and emit custom events (e.g., `level-change`). The parent `dashboard` handles the actual API synchronization.
- **TypeScript Types:** When using `verbatimModuleSyntax`, always use `import type` for interfaces to avoid build errors.

### 4. Actionable Discovery
- **Actionable Discovery:** Only display devices for which a `Provider` exists.
- **Port Probing:** For devices that hide mDNS (like some security cameras), use a throttled TCP port probe (e.g., Port 554 for RTSP) to identify live hosts on the subnet.

## Coding Conventions

- **Packages:** Logic is decoupled into `pkg/`.
- **Web UI (Lit):**
    - Prefer `axios` for API communication.
    - Use `...BaseCard.styles` array for CSS inheritance in sub-components.
- **Commits:** Use [Conventional Commits](https://www.conventionalcommits.org/).

### Common Pitfalls
- **RTSP 401 Unauthorized:** Security cameras require `camera_auth` in `config.json` (format: `user:pass`).
- **Art Proxy 404:** Check for double-escaped `&amp;` in the `path` query parameter.
- **Omnivision / OV Ready:** ADC cameras often identify as `Server: OV Ready` on ports 6080/6443. These use a proprietary protocol.

## Changelog Management
To generate or update the `CHANGELOG.md` from completed tasks in `bd`, run:

```bash
bd list --status closed --json | jq -r 'sort_by(.closed_at) | reverse | map(select(.closed_at != null)) | group_by(.closed_at[0:10]) | reverse | .[] | "## " + (.[0].closed_at[0:10]) + "\n" + (map("- " + .title + .id + ")") | join("\n")) + "\n"' > CHANGELOG.md
```
