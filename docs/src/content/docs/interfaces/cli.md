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
│   ├── seek         # Seek track or time offset
│   ├── now-playing  # Quick track metadata display
│   ├── volume       # Set volume (0-100)
│   ├── favorites    # List pinned cloud favorites
│   ├── play-favorite# Play pinned favorite by ID
│   ├── play-stream  # Play direct HTTP/HTTPS audio stream
│   ├── queue-add    # Enqueue track to playback queue
│   ├── queue        # View items in playback queue
│   ├── queue-remove # Remove track(s) from playback queue
│   ├── queue-clear  # Clear all tracks from playback queue
│   ├── queue-reorder# Reorder tracks in playback queue
│   ├── queue-mode   # Configure playback modes (shuffle, repeat, crossfade)
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
Lists all Sonos speakers, cached or discovered, with generation (S1/S2), device type (Speaker vs. Bridge), volume, and now-playing track:
```bash
homectl sonos list
```

### `homectl sonos details <ip>`
Prints full metadata (queue length, format, duration, next track):
```bash
homectl sonos details 192.168.1.120
```

### `homectl sonos seek <ip>`
Jumps to a 1-based track number in the queue or seeks to a time offset in the current track. Supports `--dry-run`:
```bash
homectl sonos seek 192.168.1.120 --track 3
homectl sonos seek 192.168.1.120 --time 1:30
homectl sonos seek 192.168.1.120 --track 3 --dry-run
```

### `homectl sonos volume <ip> <0-100>`
Sets speaker volume percentage:
```bash
homectl sonos volume 192.168.1.120 35
```

### `homectl sonos favorites [ip]`
Lists pinned cloud playlists, albums, and radio stations from Sonos Favorites (`FV:2`):
```bash
homectl sonos favorites 192.168.1.120
homectl sonos favorites 192.168.1.120 --json
```

### `homectl sonos play-favorite [ip] <id>`
Starts playback of a pinned favorite by ID (or title). Supports `--dry-run`:
```bash
homectl sonos play-favorite 192.168.1.120 FV:2/1
homectl sonos play-favorite 192.168.1.120 FV:2/1 --dry-run
```

### `homectl sonos play-stream [ip] <url>`
Streams any direct HTTP or HTTPS audio URL (internet radio, podcast, TTS voice):
```bash
homectl sonos play-stream 192.168.1.120 http://stream.somafm.com/groovesalad-128-mp3 --title "SomaFM"
```

### `homectl sonos queue-add [ip] <uri>`
Enqueues a track or container URI without interrupting current playback:
```bash
homectl sonos queue-add 192.168.1.120 x-file-cifs://nas/track.flac --next
```

### `homectl sonos queue [ip]`
Inspects items in the Sonos playback queue with positions, titles, artists, and durations:
```bash
homectl sonos queue 192.168.1.120 --start 0 --count 20
homectl sonos queue 192.168.1.120 --json
```

### `homectl sonos queue-remove [ip]`
Removes one or more tracks from the Sonos playback queue. Supports `--dry-run`:
```bash
homectl sonos queue-remove 192.168.1.120 --track 3 --count 1
homectl sonos queue-remove 192.168.1.120 --track 3 --dry-run
```

### `homectl sonos queue-clear [ip]`
Clears all tracks from the speaker's playback queue. Supports `--dry-run`:
```bash
homectl sonos queue-clear 192.168.1.120
homectl sonos queue-clear 192.168.1.120 --dry-run
```

### `homectl sonos queue-reorder [ip]`
Reorders tracks in the playback queue, moving a track range before a target position or bumping to play next. Supports `--dry-run`:
```bash
homectl sonos queue-reorder 192.168.1.120 --track 8 --as-next
homectl sonos queue-reorder 192.168.1.120 --track 5 --insert-before 2
homectl sonos queue-reorder 192.168.1.120 --track 8 --as-next --dry-run
```

### `homectl sonos queue-mode [ip]`
Configures queue playback modes (shuffle, repeat, crossfade) on the speaker or group coordinator. Supports `--dry-run` and `--json`:
```bash
homectl sonos queue-mode 192.168.1.120 --shuffle on --repeat all --crossfade off
homectl sonos queue-mode 192.168.1.120 --shuffle on --dry-run
homectl sonos queue-mode 192.168.1.120 --repeat one --json
```

### `homectl sonos services [ip]`
Lists the catalog of supported streaming services and identifies the configured default provider:
```bash
homectl sonos services 192.168.1.120 --json
```

### `homectl sonos join <speaker-ip> <target-ip>`
Joins a speaker to another speaker's group. Validates that both devices belong to the same hardware generation (S1 with S1, S2 with S2) and rejects grouping with non-rendering devices (bridges):
```bash
homectl sonos join 192.168.1.121 192.168.1.120
```

### `homectl sonos leave <speaker-ip>`
Unjoins a speaker into an independent standalone coordinator group:
```bash
homectl sonos leave 192.168.1.121
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
* `-H, --host string`: Host/IP address to bind the API server to (default `0.0.0.0`; supports `127.0.0.1`, Tailscale `100.x.y.z`).
* `-p, --port int`: Port to listen on (default `8080`).
* `--ui string`: Path to directory containing built Web UI assets (default `./ui/dist`).

Security examples:
```bash
# Bind to localhost for reverse-proxy or local-only agent access
homectl serve --host 127.0.0.1 --port 8080

# Bind to Tailscale IP for secure off-LAN remote access
homectl serve -H 100.85.12.34 -p 8080
```
