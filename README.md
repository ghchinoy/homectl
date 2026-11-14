# homectl

[![CI](https://github.com/ghchinoy/homectl/actions/workflows/ci.yml/badge.svg)](https://github.com/ghchinoy/homectl/actions/workflows/ci.yml)
[![Docs](https://img.shields.io/badge/docs-Starlight-blue)](https://ghchinoy.github.io/homectl/)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.26.11+-00ADD8?logo=go)](go.mod)

A modern, Go-powered toolkit for local smart home management, providing unified control of Lutron Caséta lighting, Sonos whole-home audio, Google Cast devices, and RTSP security cameras via CLI, Terminal UI (TUI), Web UI, and Model Context Protocol (MCP) Agent Plugins.

📖 **Full Documentation & Architecture Deep Dives:** [https://ghchinoy.github.io/homectl/](https://ghchinoy.github.io/homectl/)

---

## Table of Contents

- [Installation](#installation)
  - [Download a Prebuilt Release](#download-a-prebuilt-release-macos--linux)
  - [Build from Source](#build-from-source-macos--linux)
  - [Install All Binaries & Agent Plugins](#install-all-binaries--agent-plugins)
- [Usage](#usage)
  - [Interactive Terminal UI](#interactive-terminal-ui)
  - [CLI Quickstart](#cli-quickstart)
  - [Agent Plugins & MCP Servers](#agent-plugins--mcp-servers)
  - [Web UI & API Server](#web-ui--api-server)
- [Features](#features)
- [Configuration](#configuration)
  - [Config File (`config.json`)](#config-file-configjson)
  - [Credentials & Storage](#credentials--storage)
- [Development](#development)
  - [Prerequisites](#prerequisites)
  - [Building & Testing](#building--testing)
  - [Web UI Development](#web-ui-development)
  - [Documentation Site Development](#documentation-site-development)
- [Deployment (Linux Service & Releases)](#deployment-linux-service--releases)
- [Acknowledgements](#acknowledgements)
- [Contributing](#contributing)
- [License](#license)

---

## Installation

### Download a Prebuilt Release (macOS & Linux)

Download the latest signed archive for your platform (`darwin/arm64`, `darwin/amd64`, `linux/amd64`, `linux/arm64`) from [GitHub Releases](https://github.com/ghchinoy/homectl/releases) and extract `homectl` and `homectl-mcp-sonos` onto your `PATH`.

### Build from Source (macOS & Linux)

```bash
git clone https://github.com/ghchinoy/homectl.git
cd homectl
make build
sudo cp ./bin/homectl /usr/local/bin/
```

### Install All Binaries & Agent Plugins

```bash
# Compiles homectl, mcp-sonos, and sync-skills into ./bin/ with version LDFLAGS
make build

# Installs MCP servers to ~/.local/bin and auto-registers in OpenCode
make install-mcp
```

---

## Usage

### Interactive Terminal UI

Launch the full-screen terminal dashboard:

```bash
homectl ui
```

- **`Tab`**: Cycle views between **Lights**, **Music**, **Areas**, and **Groups**.
- **`0`**: Set brightness or speaker volume to `0%` (OFF); **`1`–`9`** set `10%`–`90%`; **`+` / `-`** adjust by `1%`.
- **`Space`**: Toggle Play / Pause on selected speaker.
- **`n` / `p`**: Skip to Next / Previous track.
- **`e`**: Edit and persist custom nickname for selected device.
- **`q` / `Ctrl+C`**: Exit.

### CLI Quickstart

```bash
# Print build version, commit, and platform metadata
homectl version

# Discover all compatible smart home devices on the local network
homectl discover --limit 10

# Control Lutron lighting (dimming 0-100%)
homectl lutron list devices
homectl lutron set level /zone/1 75
homectl lutron set all 0

# Control Sonos whole-home audio (S1 & S2 households)
homectl sonos list
homectl sonos play 192.168.1.100
homectl sonos volume 192.168.1.100 25
homectl sonos now-playing 192.168.1.100
homectl sonos favorites 192.168.1.100
homectl sonos play-stream 192.168.1.100 https://stream.somafm.com/groovesalad-128-mp3
homectl sonos seek 192.168.1.100 --track 3
homectl sonos seek 192.168.1.100 --time 1:30
homectl sonos queue 192.168.1.100
homectl sonos queue-mode 192.168.1.100 --shuffle on --repeat all
homectl sonos join 192.168.1.101 192.168.1.100
homectl sonos leave 192.168.1.101

# Stream Qolsys alarm panel telemetry (via env vars to keep secrets out of shell history)
HOMECTL_QOLSYS_HOST=192.168.1.30 HOMECTL_QOLSYS_TOKEN=123456 homectl qolsys monitor --json
```

All CLI commands support `--json` (or `-o json`) for machine-readable output, `--limit` for context-window scoping, `--dry-run` for safe pre-flight simulation, and `NO_COLOR` / `HOMECTL_NO_TUI` environment overrides.

### Agent Plugins & MCP Servers

`homectl` provides standalone MCP stdio servers under `./bin/` for AI agents (OpenCode, Claude Desktop, Cursor, Antigravity):

```bash
# Run mcp-sonos directly over stdio
./bin/mcp-sonos
```

Exposes 12 token-budgeted tools (`sonos_list_speakers`, `sonos_get_now_playing`, `sonos_get_topology`, `sonos_control`, `sonos_set_volume`, `sonos_list_favorites`, `sonos_play_favorite`, `sonos_play_stream`, `sonos_add_to_queue`, `sonos_get_queue`, `sonos_queue_edit`, `sonos_list_services`) adhering to SEP-2106 object return schemas. See the [AI Agent Ecosystem Guide](https://ghchinoy.github.io/homectl/agents/overview/) for full client configuration details.

### Web UI & API Server

Start the REST API server (serves the embedded Web UI automatically, or `--ui ./ui/dist` when overriding from disk):

```bash
# Bind to localhost for local or reverse-proxy access
homectl serve -H 127.0.0.1 --port 8080
```

Open `http://127.0.0.1:8080` in your browser to access the responsive Lit card dashboard.

---

## Features

- **Lutron Caséta & RA2 Select:** Local encrypted LEAP protocol communication over mutual TLS (port 8081). Single-roundtrip batch status queries.
- **Sonos Whole-Home Audio:** Transport control (play/pause/stop/next/prev/seek), paginated queue inspection, UPnP GENA real-time push events, album art reverse proxying, Sonos Favorites (Spotify/Apple Music) playback, and direct web audio streaming.
- **Google Cast:** Zero-config mDNS discovery (`_googlecast._tcp`) with IPv4 preference, receiver app tracking, and playback/volume control.
- **Security Cameras:** Concurrent mDNS and subnet port 554 RTSP probing with on-demand, context-managed FFmpeg MJPEG transcoding.
- **Qolsys IQ Panel 4:** Encrypted WebSocket telemetry (`wss://:12345`) with token authentication and anti-replay nonce tracking.
- **Agent Plugins (MCP):** Pre-packaged plugins with "Skill with Code" prompt token optimization (~94% savings) and physical actuator safety boundaries.
- **State Isolation:** XDG-compliant runtime state (`~/.config/homectl/`) paired with a strictly gitignored `local/` folder for private residential inventories.

---

## Configuration

`homectl` complies with the XDG Base Directory specification, storing runtime configuration and state under `~/.config/homectl/` (`$XDG_CONFIG_HOME/homectl`).

### Config File (`config.json`)

Create `~/.config/homectl/config.json`:

```json
{
  "api_host": "127.0.0.1",
  "callback_ip": "192.168.1.100",
  "lutron_bridge": "192.168.1.50",
  "sonos_default_service": "Spotify",
  "camera_auth": "admin:yourpassword",
  "qolsys_host": "192.168.1.30",
  "qolsys_token": "123456"
}
```

* **`api_host`**: (Optional) Host/IP address for `homectl serve` to bind to (`127.0.0.1`, Tailscale `100.x.y.z`, or `0.0.0.0`).
* **`callback_ip`**: (Optional) Local host LAN IP address for inbound Sonos UPnP GENA event notifications.
* **`lutron_bridge`**: (Optional) Static Lutron Smart Bridge IP (overridden by `HOMECTL_LUTRON_BRIDGE` or `--bridge`).
* **`sonos_default_service`**: (Optional) Preferred streaming provider name or ID (e.g. `Spotify`, `Apple Music`).
* **`camera_auth`**: (Optional) Global `username:password` credentials automatically prepended to RTSP camera URLs.
* **`qolsys_host` / `qolsys_token`**: (Optional) Qolsys IQ Panel IP and 6-digit C4 token (overridden by `HOMECTL_QOLSYS_HOST` / `HOMECTL_QOLSYS_TOKEN`).

### Credentials & Storage

The configuration directory also stores:
* **`lutron_client.crt`, `lutron_client.key`, `lutron_ca.crt`**: Mutual TLS certificates for communicating with the Lutron Smart Bridge. See [Lutron Integration Guide](https://ghchinoy.github.io/homectl/integrations/lutron/) for pairing steps.
* **`lutron_cache.json`, `sonos_cache.json`**: Discovered device caches for instant startup.
* **`nicknames.json`**: Custom device names mapped by IP or zone resource path.

> **Privacy Notice:** Real physical device MAC addresses, static LAN IPs, and residential layouts belong in the gitignored `local/NETWORK_DISCOVERY.md` file. See [NETWORK_DISCOVERY.md](NETWORK_DISCOVERY.md) for the sanitized public baseline.

---

## Development

```bash
# Clone the repository
git clone https://github.com/ghchinoy/homectl.git
cd homectl

# Build all binaries into ./bin/
make build

# Run unit and integration tests across the workspace
make test

# Verify agent skill synchronization and manifests
make check-skills
```

### Prerequisites

* **Go:** 1.26.11+
* **Node.js:** 22.23.4+ and `npm` (for the Web UI and Starlight docs)
* **FFmpeg:** Required on the host system for on-demand camera transcoding (`ffmpeg -i rtsp://...`)
* **Graphviz (`dot`):** Optional; required for compiling architecture diagrams (`make diagrams`)

### Web UI Development

The web frontend is written in TypeScript using Lit components and bundled with Vite:

```bash
cd ui
npm ci
npm run dev   # Start Vite dev server on :5173 with API proxy to :8080
npm run build # Build production bundle to ui/dist
```

### Documentation Site Development

The documentation site is built with Astro Starlight, GM3 design tokens, and self-hosted Google Sans Flex/Code fonts:

```bash
cd docs
npm ci
npm run dev   # Start documentation dev server
npm run build # Build static HTML and Pagefind search index
```

---

## Deployment (Linux Service & Releases)

To install `homectl` as a persistent background daemon managed by `systemd` on Linux:

```bash
./scripts/install.sh
```

This automated script:
1. Builds the Go binary with embedded version metadata (`make bin-homectl`) and compiles the Lit Web UI production bundle (`npm ci && npm run build`).
2. Installs the binary to `/usr/local/bin/homectl` and web assets to `/usr/local/share/homectl/ui`.
3. Generates, enables, and starts a system-level `systemd` unit (`/etc/systemd/system/homectl.service`) on port `8086`.

### Publishing a Tagged Release

Pushing an annotated semver tag (`v*.*.*`) triggers `.github/workflows/release.yml`, which builds `ui/dist`, cross-compiles `homectl` and `homectl-mcp-sonos` for `{darwin,linux}/{amd64,arm64}` with tag-aligned `-ldflags`, generates SHA-256 checksums and SLSA build provenance attestations, and publishes a GitHub Release:

```bash
git tag -a v0.4.0 -m "Release v0.4.0"
git push origin v0.4.0
```

### Managing the Service

```bash
sudo systemctl status homectl
sudo systemctl restart homectl
sudo systemctl stop homectl
journalctl -u homectl -f
```

---

## Acknowledgements

* [frankensonos](https://github.com/dickelstoneworth/frankensonos) by [@dickelstoneworth](https://github.com/dickelstoneworth) for Sonos S1/S2 hardware generation classification and non-renderer bridge detection heuristics.

---

## Contributing

Contributions, issue reports, and pull requests are welcome!

1. Fork the repository and create a feature branch (`git checkout -b feat/my-feature`).
2. Verify repository quality gates:
   ```bash
   gofmt -s -w .
   go vet ./...
   make test
   make check-skills
   npm --prefix docs run build
   ```
3. Commit using [Conventional Commits](https://www.conventionalcommits.org/).
4. Push to your branch and open a Pull Request.

Issue tracking in this repository is managed with [Beads (`bd`)](https://github.com/gastownhall/beads). Run `bd ready` to inspect open tasks.

---

## License

This project is licensed under the [Apache License, Version 2.0](LICENSE).
