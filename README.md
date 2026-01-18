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

## Installation

To install `homectl` as a system-wide service (Linux):

```bash
./scripts/install.sh
```

This script:
1. Builds the Go binary and Lit Web UI.
2. Installs the binary to `/usr/local/bin/homectl`.
3. Installs UI assets to `/usr/local/share/homectl/ui`.
4. Configures and starts a `systemd` service.

To update an existing installation, simply run the script again.

### Service Management

Once installed, you can manage the `homectl` service using standard systemd commands:

```bash
# Start/Stop/Restart the service
sudo systemctl start homectl
sudo systemctl stop homectl
sudo systemctl restart homectl

# Check status
systemctl status homectl

# View real-time logs
journalctl -u homectl -f

# View logs since a specific time
journalctl -u homectl --since "1 hour ago"
```

## Development

### Running Locally
For rapid development, you can run the application without installing:

```bash
# Start the API server and serve UI from source
go run main.go serve --ui ./ui/dist
```

### UI Development
The Web UI is built with Lit and Vite.
```bash
cd ui
npm install
npm run dev   # Start Vite dev server for hot-reloading
npm run build # Build for production
```

### Code Structure
- `cmd/`: CLI commands (Cobra).
- `pkg/`: Core logic (leap, sonos, camera, config).
- `ui/`: Web interface (Lit + TypeScript).

### Configuration

homectl uses a configuration file located at `~/.config/homectl/config.json`.

```json
{
  "callback_ip": "192.168.4.80",
  "camera_auth": "admin:yourpassword"
}
```

*   **`callback_ip`**: The local IP of your machine that IoT devices (like Sonos) should send event notifications to. Required if you are behind a NAT or have multiple network interfaces.
*   **`camera_auth`**: A global `username:password` string used to authenticate RTSP streams for security cameras.

### Local Storage
The configuration directory also stores:
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