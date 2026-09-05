---
title: Architecture Overview
description: High-level system design and architectural principles of homectl.
---

`homectl` is structured around clean package boundaries, local protocol decoupling, and unified state persistence.

## System Topology

```text
┌────────────────────────────────────────────────────────┐
│                      INTERFACES                        │
├────────────────────┬──────────────────┬────────────────┤
│  Bubble Tea TUI    │    Cobra CLI     │   Lit Web UI   │
│   (Terminal Hub)   │ (Scriptable CLI) │ (Vite + WebSP) │
└─────────┬──────────┴────────┬─────────┴────────┬───────┘
          │                   │                  │
          └───────────────────┼──────────────────┘
                              │
                    ┌─────────▼─────────┐
                    │ homectl Core App  │
                    │   & API Server    │
                    └─────────┬─────────┘
                              │
     ┌────────────────────────┼────────────────────────┐
     │                        │                        │
┌────▼──────┐           ┌─────▼─────┐            ┌─────▼──────┐
│ pkg/leap  │           │ pkg/sonos │            │ pkg/camera │
│  (TLS)    │           │(UPnP/GENA)│            │(RTSP/FFmpeg│
└────┬──────┘           └─────┬─────┘            └─────┬──────┘
     │                        │                        │
┌────▼──────┐           ┌─────▼─────┐            ┌─────▼──────┐
│  Lutron   │           │   Sonos   │            │ Security   │
│  Bridge   │           │  Speakers │            │  Cameras   │
└───────────┘           └───────────┘            └────────────┘
```

## Architectural Principles

### 1. Centralized XDG Storage
All persistent runtime state—including TLS certificates, discovery caches, application configuration, and custom nicknames—resides under the standard XDG path:
```text
~/.config/homectl/
├── config.json          # User configuration (callback IP, camera auth)
├── nicknames.json       # User-assigned friendly device names
├── lutron_cache.json    # Cached bridge discovery
├── sonos_cache.json     # Cached speaker discovery
├── lutron_client.crt    # Client TLS certificate for Lutron LEAP
├── lutron_client.key    # Client private key for Lutron LEAP
└── lutron_ca.crt        # Lutron root authority cert
```
The codebase uses `pkg/config.GetPath(filename)` exclusively, preventing hardcoded paths.

### 2. Multi-Protocol Discovery Pipeline (`pkg/discovery`)
Rather than coupling discovery to individual device drivers, `pkg/discovery` defines a unified interface:
```go
type Provider interface {
    Name() string
    Discover(ctx context.Context) ([]Device, error)
}
```
The `Manager` coordinates parallel discovery runs across:
- **mDNS / Zeroconf:** Lutron (`_leap._tcp`), Sonos (`_sonos._tcp`), Google Cast (`_googlecast._tcp`), RTSP cameras (`_rtsp._tcp`, `_axis-video._tcp`).
- **SSDP / UPnP:** Multicast UDP probe for Sonos renderers.
- **WS-Discovery:** ONVIF camera discovery (`NetworkVideoTransmitter`).
- **Throttled TCP Port Probing:** Direct port 554 scan across the subnet for cameras that omit mDNS advertising.

### 3. API Proxy & Transcoding Pattern
Browsers cannot natively render RTSP video streams or authenticate against UPnP XML endpoints directly. The Go API server acts as a proxy:
- **RTSP Transcoding:** Spawns context-bounded `ffmpeg` pipelines to transcode raw H.264 RTSP feeds to multi-part JPEG (`multipart/x-mixed-replace`) over HTTP.
- **Album Art Proxy:** Normalizes internal Sonos UPnP artwork paths (`/getaa?s=1&...`) with SSRF protections and serves them over standard HTTP GET.
