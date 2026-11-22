---
title: Web UI & REST API
description: REST endpoints specification and Lit WebComponent dashboard.
---

When running `homectl serve`, the application exposes a REST API on the configured port (default `8080`) and serves static Web UI assets.

## REST API Reference

### Lutron Endpoints

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/api/lutron/devices` | Returns all Lutron devices with assigned nicknames and zone hrefs |
| `GET` | `/api/lutron/status` | Batch query returning current status and dimming level for all zones |
| `POST` | `/api/lutron/set` | Sets brightness for a specific zone. Body: `{"href": "/zone/1", "level": 75}` |
| `POST` | `/api/lutron/all` | Sets brightness for all zones simultaneously. Body: `{"level": 0}` |

### Sonos Endpoints

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/api/sonos/devices` | Returns discovered Sonos speakers with custom nicknames |
| `GET` | `/api/sonos/status` | Returns playback state, volume, track title, artist, album, and art URL |
| `POST` | `/api/sonos/control` | Control transport, volume, or seek. Actions: `play`, `pause`, `stop`, `next`, `prev`, `volume`, `seek_track`, `seek_time` |
| `GET` | `/api/sonos/art` | Proxies and caches album art. Query params: `?ip=192.168.1.120&path=/getaa?...` |
| `GET` | `/api/sonos/favorites` | Returns pinned cloud favorites (playlists, albums, radio). Query param: `?ip=...` |
| `POST` | `/api/sonos/play-favorite` | Plays a pinned favorite by ID. Body: `{"ip": "...", "favorite_id": "FV:2/1"}` |
| `GET` | `/api/sonos/services` | Returns music services catalog and configured default. Query param: `?ip=...` |
| `POST` | `/api/sonos/play-stream` | Plays direct HTTP/HTTPS audio stream. Body: `{"ip": "...", "url": "...", "title": "..."}` |
| `POST` | `/api/sonos/queue-add` | Enqueues track URI to queue. Body: `{"ip": "...", "uri": "...", "as_next": true, "metadata": "..."}` |
| `GET` | `/api/sonos/queue` | Returns paginated tracks from the playback queue. Query params: `?ip=...&start=0&count=20` |

### Google Cast Endpoints

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/api/cast/devices` | Returns discovered Google Cast / Nest devices |
| `GET` | `/api/cast/status` | Returns active receiver status. Query param: `?ip=192.168.1.140` |
| `POST` | `/api/cast/control` | Media and volume actions: `{"ip": "...", "action": "volume", "volume": 50}` |

### Roborock Endpoints

Automatically routes to `homectl roborock serve` (`:8889`) if running, or falls back directly to encrypted LAN TCP (`:58867`) using `~/.config/homectl/roborock/cloud_devices.json` (`homectl roborock login`):

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/api/roborock/devices` | Returns fleet snapshots (`{count, devices}`) with online status, battery %, state, model, firmware, and LAN IP |
| `GET` | `/api/roborock/status` | Queries live `get_status` and `get_consumable`. Query param: `?id=<duid-or-name>` |
| `GET` | `/api/roborock/rooms` | Queries `get_room_mapping` enriched with cloud room names. Query param: `?id=<duid-or-name>` |
| `POST` | `/api/roborock/control` | Executes vacuum actions (`start`, `pause`, `stop`, `dock`, `find`, `clean_rooms`, `fan_power`, `water_mode`). Supports `dry_run` |
| `GET` | `/api/roborock/camera` | Inspects TURN relay and 15-character camera bootstrap info (local server mode) |
| `GET` | `/api/roborock/events` | Server-Sent Events (SSE) stream of live decoded MQTT telemetry (local server mode) |

### Security Camera Endpoints

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/api/security/cameras` | Returns all discovered RTSP and ONVIF security cameras |
| `GET` | `/api/security/stream` | Streams real-time MJPEG video transcoded via FFmpeg. Query param: `?ip=...` |

### Security & Request Guards

All mutating `POST` endpoints enforce:
* **Local / Same-Origin Guard:** Browser requests must originate from the server's `Host`, `localhost` / loopback, or configured `api_host` (cross-origin `Origin` and `Sec-Fetch-Site: cross-site` requests are rejected with `403 Forbidden`).
* **Bounded Payload Limits:** JSON request bodies are capped at `64 KiB` (`413 Request Entity Too Large`), and album art proxy responses are capped at `10 MiB` with media header allowlisting (`nosniff`).
* **Unicast Device IP Validation:** Device `ip` parameters reject hostnames, explicit ports, loopback, link-local (`169.254.0.0/16`), multicast, and unspecified addresses.

---

## Web UI Architecture (Lit + Lustrous Design System + Vite)

The web dashboard lives under `ui/` and is built with [Lit WebComponents](https://lit.dev), the **Lustrous Design System (`@ghchinoy/lustrous-lit`)** (WCAG 2.1 AA Mineral Slate & Sapphire tokens, bundled Google Sans Flex/Code WOFF2 fonts, zero inline `style=` attributes), and [Vite](https://vitejs.dev):

* **Boot Spinner & Progressive Discovery (`ui/index.html`, `ui/src/dashboard.ts`):** Displays an immediate pre-hydration `<progress>` spinner before JS loads, transitions to a `<lustrous-spinner>` + `<lustrous-skeleton>` boot hero, and resolves each subsystem (`Lutron`, `Sonos`, `Cast`, `Roborock`, `Cameras`) independently so fast providers render without blocking on slower scans.
* **Sticky Section Jump Navigation & Theming:** Provides a state-aware sticky `<nav>` pill bar with `IntersectionObserver` scroll-spy (`aria-current="location"`), light/dark/system theme switching with FOUC prevention, and `<lustrous-snackbar>` action feedback.
* **Centralized API Client (`ui/src/api.ts`):** Zero-dependency typed browser `fetch` client for all backend calls.
* **Component Isolation:** Each card subclasses `BaseCard` (`LustrousElement` in `ui/src/components/base-card.ts`):
  * `<lutron-card>`: Ambient brightness strip, quick On/Off & All Off actions, and accessible range slider.
  * `<sonos-card>` & `<sonos-source-panel>`: Album art, transport icon buttons, volume control, searchable favorites grid, direct stream player, and music services table.
  * `<cast-card>`: Active app indicator, playback controls, and volume slider.
  * `<roborock-card>`: Model/firmware/online/error badges, `<lustrous-progress-bar>` battery meter, and Clean/Pause/Dock/Find actions. Off-LAN account vacuums automatically tuck into a collapsed-by-default `<lustrous-collapsible-card>` persisted in `localStorage`.
  * `<camera-card>`: On-demand MJPEG stream toggle with connecting `<lustrous-spinner>` overlay and direct RTSP link.
* **Dumb Component / Smart Parent Pattern:** Child cards emit CustomEvents (`level-change`, `volume-change`, `control-change`, `roborock-control`), and `<homectl-dashboard>` handles API dispatch, optimistic updates, and snackbar notifications.
