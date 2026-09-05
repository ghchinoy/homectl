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
| `POST` | `/api/sonos/control` | Control transport or volume. Body: `{"ip": "192.168.1.120", "action": "play"}` |
| `GET` | `/api/sonos/art` | Proxies and caches album art. Query params: `?ip=192.168.1.120&path=/getaa?...` |
| `GET` | `/api/sonos/favorites` | Returns pinned cloud favorites (playlists, albums, radio). Query param: `?ip=...` |
| `POST` | `/api/sonos/play-favorite` | Plays a pinned favorite by ID. Body: `{"ip": "...", "favorite_id": "FV:2/1"}` |
| `GET` | `/api/sonos/services` | Returns music services catalog and configured default. Query param: `?ip=...` |
| `POST` | `/api/sonos/play-stream` | Plays direct HTTP/HTTPS audio stream. Body: `{"ip": "...", "url": "...", "title": "..."}` |
| `POST` | `/api/sonos/queue-add` | Enqueues track URI to queue. Body: `{"ip": "...", "uri": "...", "as_next": true}` |

### Google Cast Endpoints

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/api/cast/devices` | Returns discovered Google Cast / Nest devices |
| `GET` | `/api/cast/status` | Returns active receiver status. Query param: `?ip=192.168.1.140` |
| `POST` | `/api/cast/control` | Media and volume actions: `{"ip": "...", "action": "volume", "volume": 50}` |

### Security Camera Endpoints

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/api/security/cameras` | Returns all discovered RTSP and ONVIF security cameras |
| `GET` | `/api/security/stream` | Streams real-time MJPEG video transcoded via FFmpeg. Query param: `?ip=...` |

---

## Web UI Architecture (Lit + Vite)

The web dashboard is located under `ui/` in the repository and built with [Lit WebComponents](https://lit.dev) and [Vite](https://vitejs.dev):

* **Centralized API Client (`ui/src/api.ts`):** Single typed Axios wrapper for all backend calls.
* **Component Isolation:** Each card inherits base styling from `BaseCard` (`ui/src/components/base-card.ts`):
  * `<lutron-card>`: Dynamic brightness illumination color and slider.
  * `<sonos-card>`: Album art, track metadata, and volume.
  * `<sonos-source-panel>`: Shared music source controller with real-time search/filter over favorites, one-click playback, direct stream player, and music services catalog.
  * `<cast-card>`: Active app indicator and playback buttons.
  * `<camera-card>`: On-demand stream toggle and direct RTSP link.
* **Dumb Component / Smart Parent Pattern:** Child cards emit CustomEvents (`level-change`, `volume-change`, `control-change`), and the parent `<homectl-dashboard>` handles API dispatch and optimistic updates.
