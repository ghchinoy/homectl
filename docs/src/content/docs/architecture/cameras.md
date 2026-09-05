---
title: Security Camera & Transcoding Vertical Architecture
description: Architecture of concurrent camera discovery, RTSP streaming, FFmpeg transcoding, and multimodal LLM inspection.
---

The security camera vertical enables discovery, browser playback, and multimodal AI visual inspection across IP security cameras (Alarm.com, Hikvision, Dahua, Axis, and generic ONVIF devices).

---

## Architectural Diagram

![Security Camera Vertical Architecture](../../../assets/architecture/camera.webp)

---

## Key Components & Protocol Flow

### 1. Agent Skill & Multimodal Vision (`skills/camera-inspector`)
* **Single-Frame Extraction:** Instead of streaming heavy continuous video to an LLM, the agent extracts an on-demand single JPEG frame via `camera_snapshot`.
* **Multimodal Reasoning:** Feeds the captured image directly to multimodal models (e.g., Gemini 2.5/3 Pro Image) for visual reasoning ("Is there a package on the porch?", "Is the garage door open?").
* **Token Efficiency:** Consumes standard image input tokens only when triggered, avoiding continuous video context consumption.

### 2. Standalone MCP Server (`cmd/mcp-camera`)
* `camera_list` (🔒 Read-Only): Lists all discovered RTSP and ONVIF cameras with IP addresses and model identifiers.
* `camera_stream_url` (🔒 Read-Only): Returns the authenticated local RTSP URL for media players like VLC.
* `camera_snapshot` (🔒 Read-Only): Captures and returns a single JPEG snapshot.

### 3. Go Engine & On-Demand Transcoding (`modules/camera` / `pkg/camera`)
* **Concurrent Discovery Pipeline:**
  * Dispatches mDNS browsing (`_rtsp._tcp`, `_axis-video._tcp`) and ONVIF WS-Discovery (UDP `3702`) concurrently.
  * Runs a parallel subnet port 554 scanner using a **50-worker concurrency semaphore**. Decoupled via `sync.WaitGroup` so port probing completes in under 300ms without being blocked by mDNS timeouts.
* **On-Demand FFmpeg MJPEG Proxy (`cmd/serve.go`):**
  * Spawns an `ffmpeg` child process:
    ```bash
    ffmpeg -rtsp_transport tcp -i rtsp://user:pass@192.168.1.200:554 \
      -f mpjpeg -q:v 3 -vcodec mjpeg pipe:1
    ```
  * Pipes the output directly to the browser as `multipart/x-mixed-replace`.
  * **Context Lifecycle:** Wraps the process in `exec.CommandContext(r.Context(), ...)`. When the browser tab closes or stream is stopped, the context automatically signals `SIGKILL` to `ffmpeg`, eliminating orphaned background transcoding processes.
* **Credential Injection:** Loads global camera credentials (`camera_auth` in `config.json`) and safely injects them into outbound RTSP URLs.

### 4. Physical Cameras & Protocols
* **RTSP (TCP Port 554):** Standard streaming protocol delivering H.264 video.
* **Omnivision (TCP Port 6080):** Proprietary control API identifying Alarm.com cameras (`Server: OV Ready`).
* **ONVIF WS-Discovery (UDP 3702):** Multicast discovery protocol returning camera hardware profiles and RTSP scope URLs.
