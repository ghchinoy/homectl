# homectl

A modern, Go-powered toolkit for local smart home management, specializing in Lutron Caseta/RA2 Select and Sonos integration.

## Features

- **Lutron Integration:** Control lights, shades, and scenes via the LEAP protocol.
- **Sonos Control:** Manage volume, transport, and rich metadata for network speakers.
- **Go-Native Discovery:** Zero-config startup using mDNS/zeroconf for device discovery.
- **Centralized Config:** Standard XDG configuration storage at `~/.config/homectl/`.
- **Interactive TUI:** A responsive Bubble Tea terminal interface with multi-mode navigation.
- **Nickname Support:** Add custom nicknames to devices without modifying bridge configuration.

## Getting Started

### Prerequisites

- **Go:** 1.25+
- **Lutron Certificates:** Required for LEAP communication. See [NETWORK_DISCOVERY.md](./NETWORK_DISCOVERY.md) for pairing instructions.

### Installation

```bash
go build -o homectl main.go
./homectl ui
```

### Configuration

All certificates and discovery caches are stored in `~/.config/homectl/`:
- `lutron_client.crt`, `lutron_client.key`, `lutron_ca.crt`: Lutron credentials.
- `lutron_cache.json`, `sonos_cache.json`: Discovery results for fast startup.
- `nicknames.json`: Custom device names.

## Usage

### CLI

```bash
# List resources
./homectl list zones
./homectl sonos list

# Control devices
./homectl set /zone/1 50
./homectl sonos volume 192.168.4.120 20
```

### Terminal UI

Launch the interactive dashboard:
```bash
./homectl ui
```
- **Tab:** Switch between Lights, Music, and Areas.
- **1-9 / 0:** Set dimming or volume level.
- **e:** Edit nickname for the selected device.
- **Space:** Play/Pause music.
- **n / p:** Next/Previous track.
```