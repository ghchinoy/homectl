---
title: Introduction & Quickstart
description: Get up and running with homectl in under 2 minutes.
---

`homectl` is a local-first smart home management suite written in Go. It interacts directly with local IoT protocols (Lutron LEAP, Sonos UPnP/GENA, Google Cast mDNS, and RTSP cameras) without requiring cloud accounts, external bridges, or internet connectivity.

## Quickstart

### 1. Clone & Build

Ensure you have **Go 1.25+** installed:

```bash
git clone https://github.com/ghchinoy/homectl.git
cd homectl
go build -o homectl .
```

### 2. Discover Devices on Your Network

Run the multi-protocol discovery engine to scan your subnet for compatible devices:

```bash
./homectl discover
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
./homectl ui
```

- Use **`Tab`** to switch between **Lights**, **Music**, and **Areas**.
- Use number keys **`0`–`9`** to adjust dimming or speaker volume.
- Press **`Space`** to toggle music playback.
- Press **`e`** to set a custom nickname for any device.

### 4. Basic CLI Usage

Control your devices directly from the command line:

```bash
# List all Lutron lighting devices with their real-time dimming levels
./homectl lutron list devices

# Set a light to 50%
./homectl lutron set level /zone/1 50

# Play music on a Sonos speaker
./homectl sonos play 192.168.1.120

# View detailed playback metadata
./homectl sonos details 192.168.1.120

# Jump to track 3 in the queue
./homectl sonos seek 192.168.1.120 --track 3
```

### 5. Launch the Web UI & API Server

Serve the responsive Lit web dashboard and local REST API:

```bash
./homectl serve --port 8080 --ui ./ui/dist
```

Navigate to `http://localhost:8080` to interact with your dashboard from any web browser on your network.
