---
name: roborock-fleet
description: Local-only Roborock vacuum fleet monitoring, telemetry inspection, safe room/whole-home cleaning orchestration, and WebRTC camera diagnostics via homectl's embedded HTTPS/MQTT/TURN server.
license: Apache-2.0
metadata:
  kind: skill
  domain: smart-home-robotics
  version: "1.0.0"
---

# Roborock Fleet Management & Instrumentation Skill

This skill guides AI agents in monitoring, diagnosing, and safely controlling Roborock vacuums via `homectl` — supporting both **Keep Official App (Direct LAN TCP `:58867` + UDP `:58866`)** mode (`homectl roborock login`) and **De-Clouded Local Server** mode (`homectl roborock serve` + `onboard`), across V1 models like Q5 Max+ (`a125`) and V2/L01 models like Saros 10R (`a144`) and Saros 20 (`a288`).

---

## 1. Tool Quick-Reference

| Tool | Read / Mutate | Purpose |
| :--- | :---: | :--- |
| `roborock_list_devices` | Read | Fleet overview: DID/DUID, model, online state, battery %, state, firmware, RSA key-recovery progress |
| `roborock_get_status` | Read | Live RPC `get_status` + `get_consumable` (battery, error code, fan/water mode, brush/filter wear) |
| `roborock_get_telemetry` | Read | Recent decoded MQTT DPS frames, RPC request/response traces, OTA reports, and onboarding events |
| `roborock_get_rooms` | Read | Segment/room ID mapping (`get_room_mapping`) for targeted room cleaning |
| `roborock_camera_info` | Read | WebRTC TURN relay health & 15-character `/fwapi/createca` bootstrap diagnostics |
| `roborock_control` | Mutate | Consolidated command tool: `start`, `pause`, `stop`, `dock`, `find`, `clean_rooms`, `fan_power`, `water_mode`, `request_map` (supports `dry_run`) |

---

## 2. Physical Safety & Pre-Flight Guardrails

Robotic vacuums are mobile physical actuators. Before issuing mutating commands:

1. **Pre-Flight Health Check:** Always call `roborock_list_devices` or `roborock_get_status` first.
   - **Never start cleaning** if `error_code != 0` (e.g. `bumper_stuck`, `robot_trapped`, `clean_water_tank_empty`, `dirty_water_tank_full`) — report the exact fault to the user instead.
   - **Check battery:** Warn or refuse to start a whole-house clean if battery is below **20%**.
2. **Whole-House vs. Targeted Room Cleans:**
   - Prefer `roborock_control(action: "clean_rooms", rooms: [...])` after querying `roborock_get_rooms` when the user asks to clean a specific room.
   - Confirm with the user before starting a full-house clean (`action: "start"`) during quiet hours (10:00 PM – 7:00 AM).
3. **Dry-Run Support:** Use `dry_run: true` to validate parameters when planning multi-vacuum schedules or room sequences.

---

## 3. Onboarding & Model-Specific Diagnostics

When diagnosing offline vacuums or onboarding issues using `roborock_list_devices` and `roborock_get_telemetry`:

- **RSA Public-Key Recovery:** Vacuums require `>= 2` signed `/region` requests (V1 query SHA-256 or V2 header SHA-384) to recover their RSA public key before encrypted `/region` and `/nc` handshakes succeed.
- **48-Hour Access Renewal (`rrcheck`):** `homectl roborock serve` automatically forges `{k, d}` grants on `POST /f/c` and `GET /devices/{did}/sh` so WAN-blocked vacuums never hit `-10002 rrcheck access denied`.
- **DNS Bypass (Saros 20 `a288`):** If a Saros 20 fails to hit the local server after Wi-Fi setup, confirm WAN access for its LAN IP/MAC is blocked at the router so it obeys local DHCP DNS instead of `8.8.8.8`.
- **WebRTC Camera 15-Character Limit:** Use `roborock_camera_info` to confirm that `country_domain` (Saros 10R `a144`) and `token.r` (Saros 20 `a288`) fit within **15 characters** including port and trailing `/` (best achieved on port `443`).
