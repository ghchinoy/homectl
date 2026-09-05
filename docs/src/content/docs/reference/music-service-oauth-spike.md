---
title: "Research Spike: Music Service OAuth & Track Search"
description: Architectural investigation into Spotify Web API, Apple MusicKit, Sonos SMAPI, and local stream relays for cloud track search and playback orchestration.
---

## 1. Executive Summary & Problem Statement

`homectl` provides local UPnP/SOAP playback, volume control, queue management, and **Sonos Favorites (`FV:2`)** browsing. While Favorites allows one-click playback of pinned playlists and stations without third-party credentials, it does not support ad-hoc track search (e.g. *"Play Bohemian Rhapsody by Queen"*).

Local Sonos speakers do not index provider catalogs locally. To support universal track search across the Web UI, CLI, and AI agent MCP tools, `homectl` requires a bridge between streaming service search APIs and Sonos playback endpoints.

---

## 2. Integration Architecture Options

```text
+-----------------------+        +-----------------------------------------------+
|  User / Web UI / MCP  | -----> |             homectl Search Bridge             |
+-----------------------+        +-----------------------------------------------+
                                                         |
         +-----------------------------------------------+-----------------------------------+
         |                                               |                                   |
         v                                               v                                   v
[Option A: Provider REST APIs]              [Option B: Sonos SMAPI]             [Option C: Local Stream Relay]
• Spotify / Apple Music REST                • Device-bound SOAP                 • YouTube / YouTube Music / Web
• Client Credentials (zero user login)      • Requires hardware session tokens  • yt-dlp resolver + HTTP stream relay
• Returns native provider URI:              • Private encryption keys           • Direct HTTP audio endpoint:
  x-sonos-spotify:spotify:track:<id>...     • High XML parse overhead             http://<homectl-ip>:<homectl-port>/stream
         |                                               |                                   |
         | SetAVTransportURI()                           |                                   | PlayStream()
         v                                               v                                   v
+------------------------------------------------------------------------------------------------------------+
|                                        Sonos AVTransport (Port 1400)                                       |
+------------------------------------------------------------------------------------------------------------+
```

### Option A: Direct Provider REST APIs (Recommended for Spotify / Apple)
Query Spotify Web API or Apple MusicKit directly over HTTPS REST, retrieve universal track URIs, and construct Sonos-compatible playback URIs for `SetAVTransportURI`.

### Option B: Sonos SMAPI (Sonos Music API)
Query the speaker's internal SMAPI proxy via SOAP (`/MusicServices/Control`).
* **Verdict:** Highly complex, poorly documented outside Sonos developer NDAs, and requires reverse-engineering device-specific user authentication blobs. **Not recommended.**

### Option C: Local Stream Relay & Media Resolver (Personal / Self-Hosted Use — See Caveats)
Resolve ad-hoc search queries via a local media resolver (`yt-dlp` / YouTube Data API), stream the audio through `homectl serve`'s built-in HTTP server, and hand the stream to Sonos via `PlayStream()`.
* **Verdict:** Credential-free audio resolution for YouTube, YouTube Music, podcasts, and web audio. However, it introduces significant Terms of Service (ToS) constraints, higher ongoing maintenance overhead, and architectural dependencies on a local streaming daemon. Recommended only as an opt-in fallback for personal/self-hosted installations (see §5.5).

---

## 3. Spotify Web API Deep Dive

### 3.1 Authentication Flows

Spotify offers two relevant authentication flows:

1. **Client Credentials Flow (Search-Only — Zero User Login):**
   * **Mechanism:** Server exchanges `client_id` + `client_secret` with `https://accounts.spotify.com/api/token` for an app token (`grant_type=client_credentials`).
   * **Capabilities:** Search the entire global catalog for tracks, albums, artists, and public playlists.
   * **User Friction:** Zero. Users only configure `spotify_client_id` and `spotify_client_secret` once in `~/.config/homectl/config.json`.
2. **Authorization Code with PKCE (Personal Library):**
   * **Mechanism:** Browser-based OAuth redirect flow to access user private playlists and saved tracks.
   * **Capabilities:** User personal library access.
   * **User Friction:** Requires web redirect, local callback server, and refresh token rotation.

### 3.2 Translating Search Results to Sonos Playback

When a user searches for `"So What Miles Davis"`:

1. `homectl` calls `GET https://api.spotify.com/v1/search?q=So+What+Miles+Davis&type=track&limit=1`.
2. Spotify returns track URI: `spotify:track:4cOdK2wGLETKBW3PvgPWqT`.
3. `homectl` transforms the Spotify URI into Sonos's native UPnP playback format:
   ```text
   x-sonos-spotify:spotify%3atrack%3a4cOdK2wGLETKBW3PvgPWqT?sid=9&flags=8224&sn=1
   ```
   *(where `sid=9` is the standard Sonos Music Service ID for Spotify)*.
4. `homectl` invokes `SetAVTransportURI` on the Sonos group coordinator with synthetic DIDL-Lite metadata and triggers `Play()`.

---

## 4. Apple MusicKit Deep Dive

Apple Music requires a dual-token model:
1. **Developer Token (JWT):** Signed by an Apple Developer Team private key (`.p8`) using `ES256`. Valid for up to 6 months.
2. **Music User Token (Optional for catalog search):** User OAuth token required only for personalized library content.

### Catalog Search Endpoint:
```http
GET https://api.music.apple.com/v1/catalog/{storefront}/search?term=So+What&types=songs
Authorization: Bearer <DEVELOPER_JWT>
```

Sonos formats Apple Music tracks using `sid=204`:
```text
x-sonos-http:song%3a<SONG_ID>.mp4?sid=204&flags=8224&sn=1
```

---

## 5. Local Stream Relay & Media Resolver Deep Dive (YouTube & Web Audio)

### 5.1 The YouTube Music Challenge on Sonos

While Spotify and Apple Music use predictable track IDs that Sonos can translate into direct CDN playback URIs, **YouTube Music (Sonos Service ID `284`) behaves differently**:

* **Opaque SMAPI Track Tokens:** Loaded track URIs appear as:
  ```text
  x-sonos-http:ALkSOiFKFEOxEVc9dgW2mtY9klA_tJHTLXDCVppGZFkHtMyd.unknown?sid=284&flags=0&sn=2
  ```
  The track identifier is an opaque, ephemeral cryptographic token minted exclusively by Google’s backend (`music.googleapis.com`). It cannot be synthesized from a public YouTube video ID.
* **Hardware-Encrypted Tokens:** Sonos speakers store OAuth refresh tokens in their local encrypted hardware keychain (`SystemProperties`), which is inaccessible to LAN clients over UPnP.
* **No Direct Web Streaming:** Sonos speakers cannot decode YouTube HTML pages or adaptive DASH/WebM manifests natively; they require direct MP3, AAC, FLAC, or HLS audio streams.

### 5.2 Local Relay Architecture (Option C)

To enable zero-credential ad-hoc playback of YouTube tracks without relying on SMAPI:

```text
+-----------------------+     1. Search & resolve query
|  Agent / CLI / Web UI | ---------------------------------> [ homectl Media Resolver ]
+-----------------------+                                            | (yt-dlp / API)
                                                                     |
                                                                     | 2. Extract direct audio stream
                                                                     v
+-----------------------------+     4. HTTP GET /api/audio/stream?id=..  +----------------------------+
| Sonos Speaker (Living Room) | ---------------------------------------> | homectl HTTP Audio Relay   |
| (192.168.1.100)             | <--------------------------------------- | (homectl serve on :<port>)  |
+-----------------------------+     5. Direct AAC / MP3 audio stream     +----------------------------+
            ^                                                                |
            |                 3. PlayStream(url, title)                      |
            +----------------------------------------------------------------+
```

### 5.3 Technical Implementation Details

1. **Resolver Layer (`pkg/streamer` or `pkg/media`):**
   * Invokes `yt-dlp` in JSON simulation mode (`--dump-single-json --default-search ytsearch:`) to extract track metadata (title, artist, duration, thumbnail) and audio stream URLs.
   * **Audio Format Negotiation:** Filters for native AAC audio (`format 140` / `.m4a`). All Sonos hardware models (Play:1, Play:3, Arc, Move 2, Port) natively decode AAC without requiring local CPU-intensive transcoding. If only Opus is available, pipe through `ffmpeg` to transcode to 192k MP3 or AAC on the fly.
2. **HTTP Stream Relay inside `cmd/serve.go`:**
   * Exposes endpoint: `GET /api/audio/stream?id=<video_id>`.
   * Proxies upstream audio chunks or pipes `yt-dlp` stdout directly to the HTTP response writer.
   * Supports `Accept-Ranges: bytes` and `Range` headers so Sonos players can buffer and seek smoothly without timeouts.
3. **LAN Host Address Resolution:**
   * Sonos speakers cannot fetch from `localhost` or `127.0.0.1`.
   * `homectl` inspects outbound network interfaces (`net.InterfaceAddrs()` or UDP socket probe to the speaker IP) to dynamically construct reachable LAN URLs (e.g. `http://<homectl-ip>:<homectl-port>/api/audio/stream?id=...`).
4. **Playback Dispatch via Existing UPnP Tools:**
   * Once the stream URL is constructed, `homectl` calls `client.PlayStream(streamURL, title)` (already implemented in `modules/sonos` and exposed via MCP as `sonos_play_stream`).

### 5.4 MCP Integration Patterns

For agent workflows, two complementary tool designs exist:

1. **Two-Step Search + Play (Primary Architecture):**
   * `sonos_search_track`: Queries the catalog, returns candidate tracks in a token-optimized schema (see §8).
   * `sonos_play_track`: Explicitly selects a candidate ID or URI and initiates playback.
   * **Advantage:** Enables agents to inspect, disambiguate, or confirm track selections before triggering physical actuators.
2. **One-Shot Composite Tool (`sonos_search_and_play`):**
   * A convenience helper for direct natural-language invocations:
     ```json
     {
       "name": "sonos_search_and_play",
       "description": "Searches for a music track or audio stream and begins immediate playback on the specified Sonos speaker or room.",
        "parameters": {
          "ip": "192.168.1.100",
          "query": "Tom Sawyer Rush",
          "service": "youtube"
        }
     }
     ```
   * Resolves the stream, registers it with the local relay, and dispatches playback to the speaker coordinator in a single operation.

### 5.5 Operational Risks, ToS & Maintenance Overhead

While Option C provides an attractive zero-credential user experience, adopting it in production carries material technical and legal tradeoffs:

1. **Terms of Service (ToS) & Legal Risk:**
   * Automated audio extraction via `yt-dlp` bypasses YouTube player clients and violates the YouTube Terms of Service.
   * `homectl` must never ship this capability enabled by default; it should remain strictly an explicit, opt-in configuration for personal, self-hosted environments.
2. **Brittle Upstream Maintenance:**
   * YouTube frequently modifies video player JavaScript, signature decipher algorithms, and throttling tokens. `yt-dlp` requires frequent upstream updates to maintain extraction reliability.
   * Introducing `yt-dlp` and `ffmpeg` creates external runtime binary dependencies that must be managed outside the Go binary toolchain, competing with the project's self-contained binary packaging philosophy.
3. **Daemon Reliability & Live Audio Hop:**
   * Unlike Option A (where Sonos speakers stream audio directly from Spotify CDN edge nodes), Option C forces `homectl` to sit directly in the active audio pipeline.
   * If `homectl serve` restarts, runs out of memory, or the host machine enters sleep, active speaker playback immediately terminates.
4. **Host Resource Utilization:**
   * Each active stream consumes local network bandwidth and CPU (especially when transcoding non-AAC formats via `ffmpeg`). Grouped speaker zones multiplying local audio requests will scale host load accordingly.

---

## 6. Architecture Comparison Matrix

| Dimension | Option A: Spotify Client Credentials | Option B: Sonos SMAPI | Option C: Local Stream Relay |
|---|---|---|---|
| **Catalog Scope** | Spotify global catalog (tracks/albums) | All linked Sonos services | YouTube, YouTube Music, web audio |
| **User Authentication** | Free Spotify Dev `client_id` + `secret` | None available (device-locked) | **Zero accounts / credentials required** |
| **Sonos Playback URI** | Native `x-sonos-spotify:spotify:track:...` | Native `x-sonos-http:...` | HTTP stream `http://<homectl-ip>:<homectl-port>/...` |
| **Daemon Required** | No (Sonos connects directly to Spotify) | No | **Yes (`homectl serve` must be running)** |
| **Queue Support** | Full queue addition (`queue-add`) | Full queue addition | Single stream / playlist queue |
| **Hardware Compatibility**| All Sonos models with Spotify linked | All Sonos models | **All Sonos models (universal HTTP)** |
| **Legal / ToS Status** | Compliant (Standard Developer Terms) | Compliant (Sonos Private Terms) | **ToS Violation (Personal Self-Hosted Only)** |
| **Maintenance Burden** | Low (Stable REST JSON API) | Prohibitive (Reverse Engineering) | **High (Frequent Upstream Player Breakages)** |
| **Complexity** | Low (REST JSON + UPnP URI formatting) | Prohibitive (Hardware session crypto) | Moderate initial, high ongoing |

---

## 7. Security, Storage & XDG Compliance

Following `homectl` standards (see `AGENTS.md §3`):

1. **Configuration:**
   Credentials reside strictly in `~/.config/homectl/config.json`:
   ```json
   {
     "spotify": {
       "client_id": "...",
       "client_secret": "..."
     },
     "streamer": {
       "port": "<homectl-port>",
       "preferred_format": "m4a",
       "ytdlp_path": "yt-dlp"
     }
   }
   ```
2. **Token Cache:**
   Ephemeral bearer tokens are cached in memory or persisted to `~/.config/homectl/token_cache.json` with file permissions `0600`.
3. **Hardware Isolation:**
   No personal tokens, client secrets, or credentials may ever be committed to the repository or logged in CI.

---

## 8. Token Economics for Agent MCP Tools

When an AI agent searches for music:
* **Raw API Response:** ~20–40 KB (3,000–6,000 tokens) containing market availability, hrefs, and duplicate image sizes.
* **Optimized MCP Tool Output:** The bridge must extract and return only the top 3–5 candidate records:
  ```json
  {
    "query": "Miles Davis So What",
    "results": [
      {
        "id": "4cOdK2wGLETKBW3PvgPWqT",
        "title": "So What",
        "artist": "Miles Davis",
        "album": "Kind of Blue",
        "uri": "spotify:track:4cOdK2wGLETKBW3PvgPWqT",
        "sonos_uri": "x-sonos-spotify:spotify%3atrack%3a4cOdK2wGLETKBW3PvgPWqT?sid=9&flags=8224&sn=1"
      }
    ]
  }
  ```
  **Token Reduction:** ~95% token savings per agent invocation.

---

## 9. Recommendations for Joint MCP + UI Roadmap

To deliver universal ad-hoc track search and playback:

1. **Phase 1: Spotify Client Credentials Bridge (Flagship / Standard Compliant)**
   * Add `modules/spotify` implementing Client Credentials search against the Spotify Web API.
   * Add `sonos_search_track` and `sonos_play_track` MCP tools to `cmd/mcp-sonos`.
   * Add `/api/sonos/search` REST endpoint in `cmd/serve.go`.
   * Add real-time track search input to `<sonos-source-panel>` in the Web UI.
   * Delivers zero user login, high-fidelity catalog search, and direct speaker-to-CDN streaming without host relay dependencies.
2. **Phase 2: Apple Music Catalog Provider**
   * Add MusicKit JWT signing support for households with Apple Music default.
3. **Phase 3: Local Audio Stream Relay (Opt-In / Personal-Use Fallback)**
   * Provide optional, opt-in stream resolver and HTTP audio relay in `cmd/serve.go` (disabled by default).
   * Add `sonos_search_and_play` composite convenience tool for personal self-hosted environments.
   * Explicitly document the ToS, dependency, and maintenance requirements outlined in §5.5.
4. **Phase 4: PKCE User Library Integration**
   * Implement local web callback for personal playlist and saved library syncing.
