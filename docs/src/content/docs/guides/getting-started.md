---
title: Introduction & Quickstart
description: Get up and running with homectl in under 2 minutes.
---

`homectl` is a local-first smart home management suite written in Go. It interacts directly with local IoT protocols (Lutron LEAP, Sonos UPnP/GENA, Google Cast mDNS, and RTSP cameras) without requiring cloud accounts, external bridges, or internet connectivity.

## Quickstart

### 1. Clone & Build

Ensure you have **Go 1.26.11+** and **Node.js 22.23.4+** installed (or download a prebuilt archive from [GitHub Releases](https://github.com/ghchinoy/homectl/releases)):

```bash
git clone https://github.com/ghchinoy/homectl.git
cd homectl
make build
```

### 2. Discover Devices on Your Network

Run the multi-protocol discovery engine to scan your subnet for compatible devices:

```bash
./bin/homectl discover
```

Example output:
```text
PROVIDER   NAME                 IP              MODEL/ID
--------------------------------------------------------------------------------
lutron     Smart Bridge Pro     192.168.1.90    Smart Bridge 2
sonos      Living Room          192.168.1.120   Sonos One (S13)
sonos      Kitchen              192.168.1.121   Sonos Move (S17)
googlecast Office Display       192.168.1.140   Nest Hub
camera     Front Porch          192.168.1.200   IP Camera

Total devices found: 5
```

### 3. Launch the Interactive Terminal UI (TUI)

Launch the full-screen terminal dashboard built with Bubble Tea:

```bash
./bin/homectl ui
```

- Use **`Tab`** to switch between **Lights**, **Music**, **Areas**, and **Groups**.
- Press **`0`** to turn lights off or mute (`0%`), **`1`–`9`** for `10%`–`90%`, or **`+` / `-`** for `±1%` steps.
- Press **`Space`** to toggle music playback.
- Press **`e`** to set a custom nickname for any device.

### 4. Basic CLI Usage

Control your devices directly from the command line:

```bash
# List all Lutron lighting devices with their real-time dimming levels
./bin/homectl lutron list devices

# Set a light to 50%
./bin/homectl lutron set level /zone/1 50

# Play music on a Sonos speaker
./bin/homectl sonos play 192.168.1.120

# View detailed playback metadata
./bin/homectl sonos details 192.168.1.120

# Jump to track 3 in the queue
./bin/homectl sonos seek 192.168.1.120 --track 3
```

### 5. Launch the Web UI & API Server

Serve the responsive Lit web dashboard (automatically served from the embedded bundle, or via `--ui ./ui/dist` when developing locally) and local REST API:

```bash
./bin/homectl serve -H 127.0.0.1 --port 8080
```

Navigate to `http://127.0.0.1:8080` to interact with your dashboard from any web browser.
