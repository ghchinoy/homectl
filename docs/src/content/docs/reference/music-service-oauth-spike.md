---
title: "Research Spike: Music Service OAuth & Track Search"
description: Architectural investigation into Spotify Web API, Apple MusicKit, and Sonos SMAPI for cloud track search and playback orchestration.
---

## 1. Executive Summary & Problem Statement

`homectl` provides local UPnP/SOAP playback, volume control, queue management, and **Sonos Favorites (`FV:2`)** browsing. While Favorites allows one-click playback of pinned playlists and stations without third-party credentials, it does not support ad-hoc track search (e.g. *"Play Bohemian Rhapsody by Queen"*).

Local Sonos speakers do not index provider catalogs locally. To support universal track search across the Web UI, CLI, and AI agent MCP tools, `homectl` requires a bridge between streaming service search APIs and Sonos playback endpoints.

---

## 2. Integration Architecture Options

```text
+-----------------------+        +--------------------------+
|  User / Web UI / MCP  | -----> | homectl Search Bridge    |
+-----------------------+        +--------------------------+
                                              |
                     +------------------------+------------------------+
                     |                                                 |
                     v                                                 v
        [Option A: Spotify Web API]                       [Option B: Sonos SMAPI]
        • Client Credentials (No Login)                   • Device-bound SOAP
        • Fast HTTP REST JSON                             • Requires session tokens
        • Returns: spotify:track:<id>                     • High XML parse overhead
                     |
                     +------------------------+
                                              v
                              +-------------------------------+
                              | Sonos AVTransport (Port 1400) |
                              | SetAVTransportURI(spotifyURI) |
                              +-------------------------------+
```

### Option A: Direct Provider REST APIs (Recommended)
Query Spotify Web API or Apple MusicKit directly over HTTPS REST, retrieve universal track URIs, and construct Sonos-compatible playback URIs for `SetAVTransportURI`.

### Option B: Sonos SMAPI (Sonos Music API)
Query the speaker's internal SMAPI proxy via SOAP (`/MusicServices/Control`).
* **Verdict:** Highly complex, poorly documented outside Sonos developer NDAs, and requires reverse-engineering device-specific user authentication blobs. **Not recommended.**

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

## 5. Security, Storage & XDG Compliance

Following `homectl` standards (see `AGENTS.md §3`):

1. **Configuration:**
   Credentials reside strictly in `~/.config/homectl/config.json`:
   ```json
   {
     "spotify": {
       "client_id": "...",
       "client_secret": "..."
     }
   }
   ```
2. **Token Cache:**
   Ephemeral bearer tokens are cached in memory or persisted to `~/.config/homectl/token_cache.json` with file permissions `0600`.
3. **Hardware Isolation:**
   No personal tokens, client secrets, or credentials may ever be committed to the repository or logged in CI.

---

## 6. Token Economics for Agent MCP Tools

When an AI agent searches for music:
* **Raw Spotify JSON Response:** ~20–40 KB (3,000–6,000 tokens) containing market availability, hrefs, and duplicate image sizes.
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

## 7. Recommendations for Joint MCP + UI Roadmap

To deliver universal ad-hoc track search:

1. **Phase 1 (Flagship / Low Friction): Spotify Client Credentials Bridge**
   * Add `modules/spotify` implementing Client Credentials search.
   * Add `sonos_search_track` and `sonos_play_track` MCP tools to `cmd/mcp-sonos`.
   * Add `/api/sonos/search` REST endpoint in `cmd/serve.go`.
   * Add a real-time track search input to `<sonos-source-panel>` in the Web UI.
2. **Phase 2: Apple Music Catalog Provider**
   * Add MusicKit JWT signing support for households with Apple Music default.
3. **Phase 3: PKCE User Library Integration**
   * Implement local web callback for personal playlist syncing.
