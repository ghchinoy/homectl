---
title: Qolsys IQ Panel
description: Real-time event streaming and status monitoring for Qolsys security systems.
---

`homectl` interfaces with Qolsys IQ Panel 2+ and IQ Panel 4 alarm systems through an encrypted WebSocket interface on port `12345`.

## Prerequisites

To connect to your IQ Panel:
1. Enable the **Control4 / 3rd Party Integration** option on your IQ Panel touchscreen (Advanced Settings > Installation > Devices > Security Sensors).
2. Note your panel's local IP address.
3. Obtain your 6-digit access token / user PIN.

---

## Streaming Panel Events

Use the `qolsys monitor` CLI command to establish an authenticated WebSocket stream:

```bash
./homectl qolsys monitor --host 192.168.1.30 --token 123456
```

### Output
```text
Connecting to 192.168.1.30...
Connected! Listening for events (Ctrl+C to stop)...
EVENT: map[action:INFO nonce:1741190400 source:homectl status:SUCCESS user_pin:123456 version:1]
EVENT: map[event:ARMING partition_id:0 status:ARMED_STAY]
EVENT: map[event:ZONE_EVENT state:OPEN zone_id:3 zone_name:Front Door]
```

Events are streamed asynchronously with automatic graceful shutdown on `SIGINT` / `SIGTERM`.

---

## Protocol Architecture

Communication is handled by `pkg/qolsys/client.go`:
- Dialing URL: `wss://<host>:12345`
- TLS Configuration: Self-signed certificate bypass (`InsecureSkipVerify: true`).
- Payload Format: JSON objects containing `action`, `user_pin`, `nonce`, `source`, and optional payload parameters.
