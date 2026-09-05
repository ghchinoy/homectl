---
title: Google Cast
description: Integrate Chromecast and Nest Audio devices for status monitoring and media control.
---

`homectl` integrates with Google Cast devices (Chromecast with Google TV, Nest Hub, Google Home speakers) via mDNS and the Cast V2 protocol.

## Discovery

Cast devices broadcast their presence on the local subnet via mDNS under the service `_googlecast._tcp`.

`homectl` reads the TXT records to extract device metadata:
* `fn`: Friendly Name (e.g. `Living Room TV`)
* `md`: Model Name (e.g. `Chromecast`, `Google Nest Hub`)
* `id`: Unique Cast Device ID

Run the discovery command:
```bash
./homectl discover
```

---

## Status & Web API

Google Cast devices are fully exposed via the `homectl serve` REST API:

### 1. Device Listing
`GET /api/cast/devices`
Returns all discovered Cast devices:
```json
[
  {
    "id": "c7a8b9...",
    "name": "Living Room TV",
    "ip": "192.168.1.140",
    "provider": "googlecast",
    "type": "Chromecast",
    "model": "Chromecast with Google TV"
  }
]
```

### 2. Device Status
`GET /api/cast/status?ip=192.168.1.140`
Returns active application info and audio levels:
```json
{
  "app_id": "CC1AD845",
  "display_name": "Default Media Receiver",
  "volume": 65,
  "is_muted": false,
  "status_text": "Casting: Nature Documentary"
}
```

### 3. Media & Volume Control
`POST /api/cast/control`
```json
{
  "ip": "192.168.1.140",
  "action": "volume",
  "volume": 50
}
```
Supported actions: `play`, `pause`, `stop`, `volume`, `mute`, `unmute`.

---

## Web Dashboard Card

The Lit Web UI includes a dedicated `<cast-card>` component featuring:
- Green Cast status badge
- Active application indicator
- Real-time volume slider (0–100%)
- Play/Pause/Stop transport controls
