---
title: Sonos Whole-Home Audio
description: Control transport, parse rich metadata, and receive real-time GENA push notifications.
---

`homectl` controls Sonos speakers using UPnP/SOAP over HTTP on port `1400`, paired with a local GENA (General Event Notification Architecture) event listener for real-time state synchronization.

## Discovery & Caching

Sonos speakers are discovered via mDNS (`_sonos._tcp`) and SSDP. Discovered speakers are cached in `~/.config/homectl/sonos_cache.json`.

```bash
# List all discovered Sonos speakers
./homectl sonos list
```

Example output:
```text
NAME                 IP              VOLUME     STATUS          NOW PLAYING
---------------------------------------------------------------------------------------------------------
Living Room          192.168.1.120   30%        PLAYING         Comfortably Numb
Kitchen              192.168.1.121   20%        PAUSED_PLAYBACK -
Bedroom              192.168.1.122   15%        STOPPED         -
```

---

## Playback & Volume Control

```bash
# Transport controls
./homectl sonos play 192.168.1.120
./homectl sonos pause 192.168.1.120
./homectl sonos stop 192.168.1.120
./homectl sonos next 192.168.1.120
./homectl sonos prev 192.168.1.120

# Set volume (0-100)
./homectl sonos volume 192.168.1.120 40
```

---

## Favorites & Cloud Playlists (FV:2)

Sonos Favorites allow you to browse and trigger cloud playlists, albums, and radio stations pinned in the official Sonos app without needing third-party API tokens (e.g. Spotify, Apple Music, YouTube Music).

```bash
# Browse pinned favorites
./homectl sonos favorites 192.168.1.120

# Output as structured JSON
./homectl sonos favorites 192.168.1.120 --json

# Play a favorite by ID (or title)
./homectl sonos play-favorite 192.168.1.120 FV:2/1

# Test with dry-run simulation
./homectl sonos play-favorite 192.168.1.120 FV:2/1 --dry-run
```

**Single-Item vs. Container Playlists:** `homectl` automatically distinguishes between individual radio streams and cloud containers (Spotify playlists, YouTube Music Liked Music, Apple Music albums). For containers, `homectl` clears the queue, enqueues the container with its stored DIDL-Lite metadata, repoints transport to the local queue, and initiates playback.

---

## Audio Streaming & Queue Management

Play arbitrary internet radio streams, podcasts, or TTS voice announcements directly:

```bash
# Stream internet radio with custom title
./homectl sonos play-stream 192.168.1.120 https://stream.somafm.com/groovesalad-128-mp3 --title "SomaFM Groove Salad"

# Enqueue an audio track as next
./homectl sonos queue-add 192.168.1.120 x-file-cifs://nas/music/track.flac --next

# Enqueue a container item with stored metadata
./homectl sonos queue-add 192.168.1.120 x-rincon-cpcontainer:... --metadata '<DIDL-Lite...>'

# View tracks in the playback queue with pagination
./homectl sonos queue 192.168.1.120 --start 0 --count 20

# Jump to track 5 in the playback queue
./homectl sonos seek 192.168.1.120 --track 5

# Seek to 1 minute 30 seconds within current track
./homectl sonos seek 192.168.1.120 --time 1:30
```

---

## Music Services & Default Routing

Inspect all registered music streaming services on the Sonos speaker:

```bash
# List available services
./homectl sonos services 192.168.1.120
```

You can set a default music service in `~/.config/homectl/config.json` so agents automatically choose a provider when no service is explicitly specified:

```json
{
  "sonos_default_service": "Spotify"
}
```

---

## Metadata Extraction

To view detailed stream details, queue position, and audio formats:

```bash
./homectl sonos details 192.168.1.120
```

Example output:
```text
Name:     Living Room
IP:       192.168.1.120
Model:    Sonos One (S13)
ID:       RINCON_000E58F...
Status:   PLAYING
Volume:   30%
Queue:    42 tracks
---------------------------------
Track:    Comfortably Numb
Artist:   Pink Floyd
Album:    The Wall
Format:   http-get:*:audio/x-flac:*
Duration: 0:06:24 (0:02:15)
Next:     Hey You by Pink Floyd
```

---

## Real-Time GENA Push Events & NAT Configuration

Rather than continuously polling speakers, `homectl` starts an embedded HTTP listener on a dynamic port and sends UPnP `SUBSCRIBE` requests to:
- `/MediaRenderer/AVTransport/Event`
- `/MediaRenderer/RenderingControl/Event`

### The NAT & Crostini Caveat
Sonos speakers send events by issuing an **inbound** HTTP `NOTIFY` request back to your machine. If you are running inside a virtual machine, Docker container, or ChromeOS Crostini environment, the speaker cannot route traffic to internal IPs (e.g. `100.115.x.x`).

To configure the correct callback IP, add `callback_ip` to `~/.config/homectl/config.json`:

```json
{
  "callback_ip": "192.168.1.100"
}
```

See the [Network Topology Reference](/homectl/reference/network-topology/) for diagnostic procedures using `tools/gena_debug.go`.

---

## Web UI Source Control

When running `homectl serve`, the dashboard includes a shared **Sonos Source Panel**:
* **Target Speaker Selector:** Choose which speaker or stereo pair to control.
* **Favorites Search & Filter:** Real-time search across all pinned favorites, playlists, and stations with album art and one-click playback.
* **Direct Audio Stream Player:** Stream arbitrary HTTP/HTTPS audio streams or insert into queue.
* **Music Services Catalog:** View registered streaming providers and verify the configured default service.

For architecture details on extending ad-hoc track search via Spotify Web API or Apple MusicKit, see the [Music Service OAuth Research Spike](/homectl/reference/music-service-oauth-spike/).

---

## Jibo Social Robot Voice Integration

`homectl`'s Sonos engine powers the **Jibo Social Robot voice capability** (`jiboplugins/sonos` in the Jibo repository).

* **Shared Discovery Cache:** Jibo Cloud reads `~/.config/homectl/sonos_cache.json` on the local LAN to immediately resolve spoken room names ("the office", "kitchen speaker", "living room") without performing slow active mDNS scans during speech turns.
* **Stereo Follower Resolution:** Automatically detects `x-rincon:` follower speakers and resolves the authoritative group coordinator so Jibo never receives blank metadata or false idle states.
* **Gemini 3.8 Flash Tool Calling:** Exposes 7 conversational tools (`sonos_now_playing`, `sonos_playback_control`, `sonos_adjust_volume`, `sonos_queue_info`, `sonos_play_favorite`, `sonos_track_seek`, `sonos_group_speakers`).
* **Embodied Performance:** Jibo smiles and sways side-to-side (`dance, slowdance`) when music plays, does an upbeat bounce with a spinning disco ball for favorites, and flashes musical note emoji hotframes on its circular screen.

For complete voice phrase patterns, container queue protocols, and troubleshooting, see the [Jibo Sonos Voice Capability Guide](https://github.com/ghchinoy/jibo-2026/blob/main/docs/capabilities/sonos-voice-control.md).
