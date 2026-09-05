---
title: MCP Server Reference & Tool Schemas
description: Complete tool definitions, parameter schemas, and SEP-2106 output specifications for homectl MCP servers.
---

`homectl` provides dedicated, domain-scoped Model Context Protocol (MCP) servers built with the official [`github.com/modelcontextprotocol/go-sdk`](https://github.com/modelcontextprotocol/go-sdk).

---

## 1. Runtime Architecture

Unlike TypeScript or Python MCP servers that require runtime environments (`npx`, `python -m`, virtualenvs) and large dependency trees, `homectl`'s MCP servers compile into **single static binaries**:

* **Instant Cold Starts:** Boots and emits tool definitions over stdio in **<5ms** (vs. 1,500ms–4,000ms for Node.js `npx` wrappers).
* **Low Memory Footprint:** Consumes ~12–18 MB RSS during active operation.
* **Hermetic Execution:** Operates without ambient environment dependencies or uncommitted runtime caches.

---

## 2. Sonos MCP Server (`cmd/mcp-sonos`)

The `mcp-sonos` server exposes **10 focused tools** and **1 live resource**.

### Tool Catalog Summary

| Tool Name | Mode | Purpose | Key Parameters |
| :--- | :---: | :--- | :--- |
| **`sonos_list_speakers`** | 🔒 Read-Only | Discovers or lists cached network speakers | `refresh?: bool` |
| **`sonos_get_now_playing`** | 🔒 Read-Only | Retrieves authoritative track metadata & status | `ip: string` |
| **`sonos_get_topology`** | 🔒 Read-Only | Inspects zone groups, members, and stereo pairs | `ip: string` |
| **`sonos_list_favorites`** | 🔒 Read-Only | Lists pinned cloud tracks, playlists, and radio | `ip?: string` |
| **`sonos_list_services`** | 🔒 Read-Only | Enumerates available music services on household | `ip?: string` |
| **`sonos_control`** | ⚡ Mutating | Basic playback (`play`, `pause`, `stop`, `next`, `prev`) | `ip: string`, `action: string` |
| **`sonos_set_volume`** | ⚡ Mutating | Sets absolute volume (0–100) or applies step delta | `ip: string`, `volume?: int`, `delta?: int` |
| **`sonos_play_favorite`** | ⚡ Mutating | Launches playback of a pinned cloud favorite | `ip: string`, `favorite_id: string` |
| **`sonos_play_stream`** | ⚡ Mutating | Streams an arbitrary HTTP/HTTPS audio URL | `ip: string`, `url: string`, `title?: string` |
| **`sonos_add_to_queue`** | ⚡ Mutating | Enqueues a track URI into the active queue | `ip: string`, `uri: string`, `as_next?: bool` |

---

## 3. Tool Definitions & Schemas

### `sonos_list_speakers` (Read-Only)
Discovers all Sonos speakers on the local subnet or returns the cached topology for instant response.

* **Parameters:**
  ```json
  {
    "type": "object",
    "properties": {
      "refresh": {
        "type": "boolean",
        "description": "Whether to actively re-scan the network via mDNS/SSDP instead of loading cached devices"
      }
    }
  }
  ```
* **Return Schema (`ListSpeakersResult`):**
  ```json
  {
    "count": 2,
    "speakers": [
      {
        "Name": "Living Room",
        "IP": "192.168.1.100",
        "RinconID": "RINCON_000E5800000000001",
        "ModelName": "Sonos Arc",
        "ModelNumber": "S19"
      }
    ]
  }
  ```

---

### `sonos_get_now_playing` (Read-Only)
Returns compact, token-optimized track status, duration, volume, and playback state.

* **Automatic Follower Resolution:** If invoked on a speaker that is a stereo-pair or group follower (reporting an `x-rincon:` URI), the tool automatically resolves the group coordinator IP, queries the coordinator, and returns authoritative metadata with `is_follower: true` and `coordinator_ip`.
* **Parameters:**
  ```json
  {
    "type": "object",
    "properties": {
      "ip": { "type": "string", "description": "IP address of the Sonos speaker (required)" }
    },
    "required": ["ip"]
  }
  ```
* **Return Schema (`NowPlayingResult`):**
  ```json
  {
    "ip": "192.168.1.100",
    "state": "PLAYING",
    "volume": 25,
    "title": "Comfortably Numb",
    "artist": "Pink Floyd",
    "album": "The Wall",
    "duration": "0:06:24",
    "progress": "0:02:15",
    "stream_content": "",
    "is_follower": false
  }
  ```

---

### `sonos_get_topology` (Read-Only)
Exposes the zone group topology, identifying multi-room groups, bonded stereo pairs, and active coordinators.

* **Parameters:**
  ```json
  {
    "type": "object",
    "properties": {
      "ip": { "type": "string", "description": "IP address of any speaker in the household (required)" }
    },
    "required": ["ip"]
  }
  ```
* **Return Schema (`TopologyResult`):**
  ```json
  {
    "count": 1,
    "groups": [
      {
        "id": "RINCON_000E5800000000001:1",
        "coordinator_uuid": "RINCON_000E5800000000001",
        "is_pair": true,
        "members": [
          { "uuid": "RINCON_000E5800000000001", "room_name": "Living Room (L)", "ip": "192.168.1.100", "is_coordinator": true },
          { "uuid": "RINCON_000E5800000000002", "room_name": "Living Room (R)", "ip": "192.168.1.101", "is_coordinator": false }
        ]
      }
    ]
  }
  ```

---

### `sonos_list_favorites` (Read-Only)
Lists pinned cloud media (Spotify playlists, Apple Music albums, radio stations) stored in the household's "Sonos Favorites" container (`FV:2`).

* **Parameters:**
  ```json
  {
    "type": "object",
    "properties": {
      "ip": { "type": "string", "description": "Optional speaker IP. If omitted, uses first discovered speaker." }
    }
  }
  ```
* **Return Schema (`ListFavoritesResult`):**
  ```json
  {
    "count": 2,
    "favorites": [
      { "id": "FV:2/1", "title": "Morning Jazz", "type": "playlist", "album_art_uri": "/getaa?..." },
      { "id": "FV:2/2", "title": "SomaFM Groove Salad", "type": "station" }
    ]
  }
  ```

---

### `sonos_control` (Mutating)
Dispatches playback actions to a speaker. If the target speaker is a follower, the command is automatically routed to the group coordinator.

* **Parameters:**
  ```json
  {
    "type": "object",
    "properties": {
      "ip": { "type": "string", "description": "IP address of the Sonos speaker (required)" },
      "action": {
        "type": "string",
        "enum": ["play", "pause", "stop", "next", "previous"],
        "description": "Playback action to execute"
      }
    },
    "required": ["ip", "action"]
  }
  ```

---

### `sonos_set_volume` (Mutating)
Adjusts speaker volume. Supports both absolute volume setting (0–100) and relative step adjustments (`delta: +5` or `-10`).

* **Parameters:**
  ```json
  {
    "type": "object",
    "properties": {
      "ip": { "type": "string", "description": "IP address of the Sonos speaker (required)" },
      "volume": { "type": "integer", "description": "Target absolute volume between 0 and 100" },
      "delta": { "type": "integer", "description": "Relative volume delta (e.g. +5 or -10). Applied if volume is omitted." }
    },
    "required": ["ip"]
  }
  ```

---

### `sonos_play_favorite` (Mutating)
Loads a pinned favorite by ID and begins playback with automatic coordinator resolution.

* **Parameters:**
  ```json
  {
    "type": "object",
    "properties": {
      "ip": { "type": "string", "description": "IP address of the target speaker (required)" },
      "favorite_id": { "type": "string", "description": "The favorite ID returned by sonos_list_favorites (e.g. 'FV:2/1')" }
    },
    "required": ["ip", "favorite_id"]
  }
  ```

---

### `sonos_play_stream` (Mutating)
Loads and plays any arbitrary HTTP or HTTPS audio stream (internet radio station, podcast episode, or TTS voice announcement) directly on the target speaker.

* **Parameters:**
  ```json
  {
    "type": "object",
    "properties": {
      "ip": { "type": "string", "description": "IP address of the Sonos speaker (required)" },
      "url": { "type": "string", "description": "Direct HTTP or HTTPS stream URL (e.g. 'http://stream.somafm.com/groovesalad-128-mp3')" },
      "title": { "type": "string", "description": "Optional friendly stream title to display on TUI and Web UI" }
    },
    "required": ["ip", "url"]
  }
  ```

---

### `sonos_add_to_queue` (Mutating)
Appends or inserts an audio URI into the speaker's active queue without halting current playback.

* **Parameters:**
  ```json
  {
    "type": "object",
    "properties": {
      "ip": { "type": "string", "description": "IP address of the Sonos speaker (required)" },
      "uri": { "type": "string", "description": "Track or stream URI to add" },
      "as_next": { "type": "boolean", "description": "If true, inserts track as the next song to play; otherwise appends to end" }
    },
    "required": ["ip", "uri"]
  }
  ```

---

## 4. MCP SEP-2106 Structured Output Compliance

Per the Model Context Protocol specification (SEP-2106) and OpenCode client validation rules, tool outputs **must be Go structs/records (JSON objects)**, never bare arrays.

* ❌ **Invalid Schema (Bare Array):**
  ```json
  [ { "name": "Kitchen" }, { "name": "Living Room" } ]
  ```
* ✅ **Valid Schema (`homectl` Standard):**
  ```json
  {
    "count": 2,
    "speakers": [ { "Name": "Kitchen" }, { "Name": "Living Room" } ]
  }
  ```

All `homectl` MCP tools wrap collections inside a top-level record, returning both a natural-language summary block and a typed payload:
```go
return &mcp.CallToolResult{
    Content: []mcp.Content{
        &mcp.TextContent{Text: fmt.Sprintf("Found %d Sonos speaker(s)", len(speakers))},
        &mcp.TextContent{Text: string(jsonData)},
    },
}, resultStruct, nil
```
