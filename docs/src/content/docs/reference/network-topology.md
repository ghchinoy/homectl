---
title: Network Topology & NAT
description: Multi-subnet routing, mDNS reflection, and NAT workarounds for IoT events.
---

`homectl` is engineered to interact with IoT hardware on local private subnets (`10.0.0.0/8`, `172.16.0.0/12`, and `192.168.0.0/16`). Certain IoT protocols introduce specific routing considerations.

## Inbound vs. Outbound Protocol Flows

| Device / Protocol | Inbound or Outbound | Port | Behavior |
| :--- | :--- | :--- | :--- |
| **Lutron LEAP** | Outbound | 8081 (TCP) | Host connects directly to bridge over TLS. Reconnects automatically on network drops. |
| **Sonos Control** | Outbound | 1400 (TCP) | Host sends standard HTTP SOAP requests to the speaker. |
| **Sonos Events (GENA)** | **Inbound** | Dynamic (TCP) | Speaker makes an inbound HTTP `NOTIFY` request back to `homectl`. |
| **Google Cast** | Outbound | 8009 (TCP) | TLS connection initiated to Cast device. |
| **RTSP Cameras** | Outbound | 554 (TCP) | TCP connection initiated to camera RTSP server. |
| **Qolsys Panel** | Outbound | 12345 (TCP) | WebSocket client connects to panel. |

---

## The Inbound NAT Challenge (GENA Events)

Because Sonos uses UPnP GENA, events are pushed via incoming HTTP calls:

```text
[homectl] ──( SUBSCRIBE )──> [Sonos Speaker]
[homectl] <──( NOTIFY )─────── [Sonos Speaker]
```

When running `homectl` inside a NATed development environment (such as a Docker container, WSL2, or ChromeOS Crostini):
1. The speaker cannot reach container/VM internal IPs (e.g., `100.115.x.x` or `172.17.x.x`).
2. The speaker issues an HTTP NOTIFY that fails with `412 Precondition Failed` or times out.

### Solutions:
1. **Set `callback_ip` in `config.json`:** Set this to your host's actual LAN IP on the physical router.
2. **Reverse SSH Tunneling:**
   Forward the dynamic GENA port through your host machine to your development container:
   ```bash
   ssh -R 37915:localhost:37915 user@192.168.1.100
   ```
3. **Diagnostic Utility:**
   The repository includes `tools/gena_debug.go` to test whether a Sonos speaker can successfully complete a handshake with your callback URL:
   ```bash
   go run tools/gena_debug.go -ip 192.168.1.120 -callback-ip 192.168.1.100 -port 37915
   ```
