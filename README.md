# homectl

[![Docs](https://img.shields.io/badge/docs-Starlight-blue)](https://ghchinoy.github.io/homectl/)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

A modern, Go-powered toolkit for local smart home management, providing unified control of Lutron Caseta/RA2 Select lighting, Sonos speakers, Google Cast devices, and RTSP security cameras via CLI, Terminal UI (TUI), Web UI, and Model Context Protocol (MCP) Agent Plugins.

📖 **Full Documentation & Architecture Deep Dives:** [https://ghchinoy.github.io/homectl/](https://ghchinoy.github.io/homectl/)

## Table of Contents

- [Quickstart](#quickstart)
- [Features](#features)
- [Installation](#installation)
  - [Build from Source](#build-from-source)
  - [Systemd Service (Linux)](#systemd-service-linux)
- [Usage](#usage)
  - [Terminal UI](#terminal-ui)
  - [CLI Commands](#cli-commands)
  - [Web UI & API Server](#web-ui--api-server)
- [Configuration](#configuration)
  - [Config File (`config.json`)](#config-file-configjson)
  - [Credentials & Cache](#credentials--cache)
- [Development](#development)
  - [Prerequisites](#prerequisites)
  - [Running Backend & Tests](#running-backend--tests)
  - [Web UI Development](#web-ui-development)
- [Contributing](#contributing)

---

## Quickstart

Build and launch the interactive terminal dashboard in seconds:

```bash
# Clone the repository
git clone https://github.com/ghchinoy/homectl.git
cd homectl

# Build the binary
go build -o homectl .

# Launch the interactive Terminal UI
./homectl ui
```

---

## Features

- **Lutron Caseta / RA2 Select:** Control lights, shades, and scenes locally via the encrypted LEAP protocol.
- **Sonos Whole-Home Audio:** Volume adjustment, playback transport, track metadata, album art proxying, and real-time UPnP GENA push notifications.
- **Google Cast:** Discover Chromecasts and Nest Audio devices to view status and adjust volume/playback.
- **RTSP Security Cameras:** mDNS discovery and subnet port scanning with on-demand MJPEG stream transcoding via `ffmpeg`.
- **Zero-Config Discovery:** Automatically discovers IoT devices on the local subnet via mDNS/zeroconf and SSDP.
- **Agent Plugins & MCP Servers:** Native Model Context Protocol (MCP) stdio micro-servers (`mcp-sonos`) and Agent Skills (`skills/sonos-soundscape`) conforming to the Agent Plugins Spec 1.0.0 for autonomous AI agents.
- **Multi-Interface Access:**
  - **Terminal UI (TUI):** Responsive Bubble Tea dashboard with tab navigation, progress bars, and live volume/lighting feedback.
  - **CLI (Cobra):** Scriptable commands for home automation scripts and terminal workflows.
  - **Web Dashboard:** Fast, modular Lit + Vite single-page application served directly from the Go API server.
- **Centralized XDG Storage:** Standardized state and configuration management at `~/.config/homectl/`.
- **Custom Nicknames:** Assign custom friendly names to devices without modifying bridge hardware.

---

## Installation

### Build from Source

Works on macOS and Linux:

```bash
git clone https://github.com/ghchinoy/homectl.git
cd homectl
go build -o homectl .
sudo mv homectl /usr/local/bin/
```

### Systemd Service (Linux)

To install `homectl` as a persistent background daemon serving the Web UI and REST API on Linux:

```bash
./scripts/install.sh
```

This automated script:
1. Builds the Go binary and compiles the Lit Web UI bundle.
2. Installs the binary to `/usr/local/bin/homectl` and web assets to `/usr/local/share/homectl/ui`.
3. Generates, enables, and starts a `systemd` user service on port `8086`.

#### Managing the Service

```bash
# Start, stop, or restart the service
sudo systemctl start homectl
sudo systemctl stop homectl
sudo systemctl restart homectl

# View real-time logs
journalctl -u homectl -f
```

---

## Usage

### Terminal UI

Launch the full-screen terminal dashboard:

```bash
./homectl ui
```

- **`Tab`**: Cycle views between Lights, Music, and Areas.
- **`0`–`9`**: Quickly adjust dimming or speaker volume.
- **`Space`**: Toggle Play / Pause on selected speaker.
- **`n` / `p`**: Next track / Previous track.
- **`e`**: Edit and persist custom nickname for the selected device.
- **`q` / `Ctrl+C`**: Exit.

### CLI Commands

#### Network Discovery
```bash
# Discover all compatible smart home devices on the network
./homectl discover
```

#### Lutron Lighting & Shades
```bash
# List discovered Lutron devices, zones, and areas
./homectl lutron list devices
./homectl lutron list zones
./homectl lutron list areas

# Set dimming level for a specific zone (0-100%)
./homectl lutron set level /zone/1 75

# Set all lights in the home simultaneously
./homectl lutron set all 0
```

#### Sonos Audio
```bash
# List all Sonos speakers with current status and track info
./homectl sonos list

# Inspect detailed track metadata and queue state
./homectl sonos details 192.168.1.50

# Playback controls
./homectl sonos play 192.168.1.50
./homectl sonos pause 192.168.1.50
./homectl sonos next 192.168.1.50
./homectl sonos volume 192.168.1.50 25
```

#### Qolsys Security Panel
```bash
# Stream live panel events (arming, sensor trips) via secure WebSocket
./homectl qolsys monitor --host 192.168.1.30 --token 123456
```

### Web UI & API Server

Start the REST API server and serve the built Web UI:

```bash
./homectl serve --port 8080 --ui ./ui/dist
```

Open `http://localhost:8080` in your browser to access the control dashboard.

### Agent Plugins & MCP Servers

`homectl` provides standalone MCP stdio servers under `./bin/` (buildable via `make build`):

```bash
# Build all binaries (homectl, mcp-sonos, sync-skills)
make build

# Install MCP binaries to ~/.local/bin and register in OpenCode config
make install-mcp
```

#### Running `mcp-sonos` Directly
```bash
./bin/mcp-sonos
```
Exposes tools for speaker listing, compact now-playing status, volume, cloud favorites, direct audio streaming, and group topology. See [Architecture Documentation](https://ghchinoy.github.io/homectl/architecture/sonos/) for full specifications.

---

## Configuration

`homectl` adheres to the XDG Base Directory specification, storing configuration and state under `~/.config/homectl/` (`$XDG_CONFIG_HOME/homectl`).

### Config File (`config.json`)

Create `~/.config/homectl/config.json`:

```json
{
  "callback_ip": "192.168.1.100",
  "camera_auth": "admin:yourpassword"
}
```

* **`callback_ip`**: (Optional) The local IP address of the machine running `homectl`. Used by Sonos speakers for inbound UPnP GENA event notifications. Required if running behind NAT or on multi-homed systems.
* **`camera_auth`**: (Optional) Global `username:password` used for RTSP camera stream authentication.

### Credentials & Cache

The configuration directory also stores:
* **`lutron_client.crt`, `lutron_client.key`, `lutron_ca.crt`**: TLS client certificates for communicating with the Lutron Smart Bridge. See [NETWORK_DISCOVERY.md](./NETWORK_DISCOVERY.md) for pairing instructions via `tools/pair_lutron.py`.
* **`lutron_cache.json`, `sonos_cache.json`**: Discovered device caches for instant startup.
* **`nicknames.json`**: Device nicknames mapped by IP or zone resource path.

---

## Development

### Prerequisites

* **Go**: 1.25+
* **Node.js**: 20+ and `npm` (for the Web UI)
* **FFmpeg**: Required on the host system for camera transcoding (`ffmpeg -i rtsp://...`)

### Running Backend & Tests

```bash
# Run tests
go test -v ./...

# Run static analysis
go vet ./...

# Run the backend locally
go run main.go serve --port 8080 --ui ./ui/dist
```

### Web UI Development

The web frontend is written in TypeScript using Lit components and bundled with Vite:

```bash
cd ui
npm install

# Start Vite dev server with hot-reloading (proxies API requests to :8080)
npm run dev

# Build production bundle to ui/dist
npm run build
```

---

## Contributing

Contributions, issue reports, and pull requests are welcome!

1. Fork the repository and create a feature branch (`git checkout -b feat/my-feature`).
2. Verify code quality gates:
   ```bash
   gofmt -s -w .
   go vet ./...
   go test -v ./...
   ```
3. Commit using [Conventional Commits](https://www.conventionalcommits.org/) (`feat: add lifx discovery`, `fix: handle timeout on sonos notify`).
4. Push to your branch and open a Pull Request.

Issue tracking in this repository is managed with [Beads (`bd`)](https://github.com/gastownhall/beads). Run `bd ready` to inspect open tasks.

---

## License

This project is licensed under the [Apache License, Version 2.0](LICENSE).
