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
