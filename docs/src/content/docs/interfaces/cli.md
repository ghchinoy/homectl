---
title: CLI Reference
description: Complete command-line interface documentation for homectl.
---

`homectl` provides a modular CLI built with Cobra. All commands support `--help` for inline documentation.

## Global Flags

* `--bridge string`: Lutron Bridge IP address. Defaults to auto-discovery or cached IP.
* `--json`: Output results as machine-readable JSON.
* `--dry-run`: Simulate operation without sending network mutations.

---

## Subcommands Overview

```text
homectl
├── discover         # Network-wide device scan
├── ui               # Launch interactive Bubble Tea TUI
├── serve            # Start REST API server and static UI host
├── lutron           # Control Lutron Caseta / RA2 Select
│   ├── list         # List devices, zones, and areas
│   │   ├── devices
│   │   ├── zones
│   │   └── areas
│   └── set          # Set dimming levels
│       ├── level
│       └── all
├── sonos            # Control Sonos speakers
│   ├── list         # List speakers with transport and volume
│   ├── details      # Show queue and audio stream details
│   ├── play         # Start playback
│   ├── pause        # Pause playback
│   ├── stop         # Stop playback
│   ├── next         # Next track
│   ├── prev         # Previous track
│   ├── now-playing  # Quick track metadata display
│   ├── volume       # Set volume (0-100)
│   ├── favorites    # List pinned cloud favorites
│   ├── play-favorite# Play pinned favorite by ID
│   ├── play-stream  # Play direct HTTP/HTTPS audio stream
│   ├── queue-add    # Enqueue track to playback queue
│   └── services     # List streaming services catalog
└── qolsys           # Qolsys alarm panel integration
    └── monitor      # Stream live panel events via WebSocket
```

---

## Command Details

### `homectl discover`
Runs concurrent discovery across all registered providers:
```bash
homectl discover
```

### `homectl lutron list devices`
Fetches all devices from the bridge alongside their real-time zone status:
```bash
homectl lutron list devices
```

### `homectl lutron set level`
Sets the dimming level (0–100) for a zone:
```bash
homectl lutron set level <zone_href> <0-100>
# Example:
homectl lutron set level /zone/1 75
```

### `homectl lutron set all`
Sets the dimming level across all lights in the home:
```bash
homectl lutron set all 0
```

### `homectl sonos list`
Lists all Sonos speakers, cached or discovered, with volume and now-playing track:
```bash
homectl sonos list
```

### `homectl sonos details <ip>`
Prints full metadata (queue length, format, duration, next track):
```bash
homectl sonos details 192.168.1.120
```

### `homectl sonos volume <ip> <0-100>`
Sets speaker volume percentage:
```bash
homectl sonos volume 192.168.1.120 35
```

### `homectl qolsys monitor`
Connects to a Qolsys IQ Panel and prints incoming event frames:
```bash
homectl qolsys monitor --host 192.168.1.30 --token 123456
```

### `homectl serve`
Starts the HTTP API server:
```bash
homectl serve --port 8080 --ui ./ui/dist
```
Flags:
* `-p, --port int`: Port to listen on (default `8080`).
* `--ui string`: Path to directory containing built Web UI assets (default `./ui/dist`).
