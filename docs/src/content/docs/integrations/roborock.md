---
title: Roborock Vacuums (Local Server)
description: Configure local DNS, hot-reloaded TLS certificates, onboarding, telemetry monitoring, and WebRTC camera relay for Roborock vacuums.
---

Modern Roborock vacuums reject unauthenticated local MiIO UDP handshakes on port `54321` and periodically reset Wi-Fi or deny commands (`-10002 rrcheck access denied`) if blocked from the cloud. `homectl roborock` runs a local HTTPS + MQTT TLS + TURN stack so your vacuums run 100% locally without rooting or hardware modification.

## Prerequisites

1. **Owned Domain & Local DNS Rewrite:** Point `api-r.example.com` and `r.example.com` in your local DNS (router, Pi-hole, or AdGuard Home) to the LAN IP running `homectl roborock serve`.
2. **Trusted Public TLS Certificate:** A wildcard/SAN certificate covering `*.example.com` (and `example.com` if using apex). Certificates are hot-reloaded automatically on file modification.
3. **Port 443 for WebRTC Camera Models:** Firmware truncates camera bootstrap inputs to **15 characters** (including `:port` and trailing `/`). Running HTTPS on port `443` keeps short hostnames like `r.example.com/` (14 chars) within the 15-character limit for both Saros 10R (`country_domain`) and Saros 20 (`token.r`).
4. **Router WAN Block:** Block WAN internet access for the vacuums' LAN IPs/MACs (essential for Saros 20 models that otherwise bypass DHCP DNS to `8.8.8.8`).

## Configuration (`~/.config/homectl/config.json`)

```json
{
  "roborock": {
    "stack_fqdn": "api-r.example.com",
    "bind_host": "0.0.0.0",
    "https_port": 443,
    "mqtt_tls_port": 8881,
    "region": "us",
    "timezone": "America/Denver",
    "camera_domain": "r.example.com",
    "certs_by_sni": {
      "*": {
        "cert_file": "/path/to/fullchain.pem",
        "key_file": "/path/to/privkey.pem"
      }
    },
    "turn": {
      "mode": "embedded",
      "host": "r.example.com",
      "port": 3478,
      "username": "rrturn",
      "password": "replace-with-strong-secret"
    }
  }
}
```

## Quickstart Workflow

```bash
# 1. Build with native libgmp acceleration on macOS/Linux for fast RSA-2048/4096 onboarding recovery
make build-gmp

# 2. Start the local HTTPS + embedded MQTT + embedded TURN server
./bin/homectl roborock serve

# 3. Verify DNS, TLS chains, MQTT TLS, and 15-char camera bootstrap before touching a vacuum
./bin/homectl roborock onboard --server api-r.example.com --camera-domain r.example.com --model a144 --preflight-only

# 4. Reset vacuum Wi-Fi, join its roborock-vacuum-* hotspot, and send onboarding credentials
./bin/homectl roborock onboard --server api-r.example.com --ssid "MyWiFi24" --password "Secret" --timezone America/Denver --camera-domain r.example.com

# 5. Monitor live decoded MQTT telemetry and control the fleet
./bin/homectl roborock list
./bin/homectl roborock monitor
./bin/homectl roborock status a144
./bin/homectl roborock find a144
./bin/homectl roborock clean a144 --rooms 16,17
./bin/homectl roborock dock a144
```
