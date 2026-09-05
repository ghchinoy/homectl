---
title: Security Cameras (RTSP & ONVIF)
description: Discover IP cameras and stream live video feeds transcoded on the fly.
---

`homectl` provides discovery and streaming capabilities for local security cameras (including Hikvision, Dahua, Axis, Amcrest, Alarm.com, and generic RTSP IP cameras).

## Discovery Pipeline

Security cameras often suppress broadcast announcements or use proprietary discovery protocols. `homectl` uses a dual-engine approach in `pkg/camera/discovery.go`:

1. **mDNS & WS-Discovery Probing:**
   Scans for `_rtsp._tcp`, `_axis-video._tcp`, `_http._tcp`, and ONVIF WS-Discovery probe messages (`239.255.255.250:3702`).
2. **Throttled RTSP Port Probing:**
   Iterates through private subnet hosts (`10.x.x.x`, `172.16-31.x.x`, `192.168.x.x`) with a 50-worker concurrency semaphore testing TCP port `554`.

```bash
./homectl discover
```

---

## Authentication Configuration

RTSP camera streams typically require authentication. You can set global camera credentials in `~/.config/homectl/config.json`:

```json
{
  "camera_auth": "admin:MySecurePassword123"
}
```

When configured, RTSP URLs are constructed with embedded authentication:
```text
rtsp://admin:MySecurePassword123@192.168.1.200:554
```

---

## On-Demand Transcoding & Streaming

Web browsers cannot render raw RTSP H.264 streams directly without plugins or WebRTC gateways. `homectl` provides an on-demand transcoding proxy via FFmpeg.

### Stream Endpoint
`GET /api/security/stream?ip=192.168.1.200`

When a browser connects:
1. The Go server spawns a context-bounded `ffmpeg` process:
   ```bash
   ffmpeg -rtsp_transport tcp -i rtsp://user:pass@192.168.1.200:554 \
     -f mpjpeg -q:v 3 -vcodec mjpeg pipe:1
   ```
2. The output stream is piped to HTTP response headers:
   ```http
   Content-Type: multipart/x-mixed-replace; boundary=ffserver
   ```
3. When the browser tab closes or the user stops streaming in the Web UI, the request context is cancelled, automatically terminating the `ffmpeg` subprocess to avoid CPU leaks.

---

## Web UI Camera Card

In the Web UI, each camera renders as an isolated `<camera-card>`:
* **Stream Offline by default:** Prevents unnecessary network bandwidth consumption.
* **Enable Live Stream:** Dynamically injects the `/api/security/stream?ip=...` endpoint into an `<img>` tag.
* **Direct RTSP Link:** Provides a one-click `rtsp://<ip>:554` link to open the full-resolution stream in VLC or external media players.
