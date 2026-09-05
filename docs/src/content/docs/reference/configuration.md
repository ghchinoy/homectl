---
title: Configuration & Nicknames
description: Storage paths, configuration files, and custom device nicknames.
---

`homectl` complies strictly with the XDG Base Directory specification. All persistent configuration, cache files, and certificates reside in:

```text
~/.config/homectl/
```
*(or `$XDG_CONFIG_HOME/homectl/` if the environment variable is set).*

---

## Configuration File (`config.json`)

Location: `~/.config/homectl/config.json`

```json
{
  "callback_ip": "192.168.1.100",
  "camera_auth": "admin:MyCameraPassword"
}
```

### Options

| Key | Type | Description |
| :--- | :--- | :--- |
| `callback_ip` | string | **Optional.** The local IP address of your host machine. IoT devices (like Sonos speakers) send inbound HTTP `NOTIFY` events to this IP. Essential when running `homectl` in multi-homed or containerized environments. |
| `camera_auth` | string | **Optional.** Global `username:password` credentials automatically prepended to discovered RTSP camera URLs when streaming. |

---

## Device Nicknames (`nicknames.json`)

Location: `~/.config/homectl/nicknames.json`

`homectl` allows overriding manufacturer hardware names with user-friendly custom names without altering bridge or speaker configurations:

```json
{
  "/zone/1": "Kitchen Island Pendant",
  "/zone/2": "Living Room Ceiling Spots",
  "192.168.1.120": "Family Room Sonos",
  "192.168.1.200": "Driveway Camera"
}
```

### Mapping Keys:
* **Lutron Devices & Areas:** Use the zone/area resource href (`/zone/1`, `/area/3`).
* **Sonos, Cast, & Cameras:** Use the device IPv4 address (`192.168.1.120`).

Nicknames can be set:
1. Interactively in the TUI by pressing **`e`**.
2. By editing `nicknames.json` directly.

---

## Discovery Caches

* **`lutron_cache.json`**: Caches bridge name and IP address discovered via mDNS.
* **`sonos_cache.json`**: Caches speaker names, IPs, models, and Rincon IDs.

These cache files permit instant TUI and CLI startup without waiting for network discovery loops.
