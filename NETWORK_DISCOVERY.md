# Network Discovery Report & Device Topology

This document outlines the discovery methodology, supported protocols, and baseline device profiles for smart home hardware managed by `homectl`.

> **Note on Local Privacy:** Real physical device MAC addresses, room maps, and static IPs should be maintained in the gitignored `local/NETWORK_DISCOVERY.md` file. The tables below use sanitized documentation addresses and masked MAC identifiers.

---

## Network Architecture & Topology

`homectl` is designed for local private subnets (e.g. `192.168.1.0/24` or `10.0.0.0/24`):

```text
                                 ┌─────────────────────────┐
                                 │   homectl Controller    │
                                 │    (CLI / TUI / Web)    │
                                 └────────────┬────────────┘
                                              │
         ┌──────────────────┬─────────────────┼──────────────────┬─────────────────┐
         │                  │                 │                  │                 │
  ┌──────▼──────┐    ┌──────▼──────┐   ┌──────▼──────┐    ┌──────▼──────┐   ┌──────▼──────┐
  │   Lutron    │    │    Sonos    │   │ Google Cast │    │ RTSP / ONVIF│   │  Qolsys IQ  │
  │   Bridge    │    │   Speakers  │   │   Devices   │    │   Cameras   │   │    Panel    │
  │ (TLS 8081)  │    │ (UPnP 1400) │   │ (mDNS 8009) │    │ (Port 554)  │   │ (WSS 12345) │
  └─────────────┘    └─────────────┘   └─────────────┘    └─────────────┘   └─────────────┘
```

---

## Discovery Methodologies

### 1. Multicast DNS (mDNS / Zeroconf)
Used by Lutron (`_leap._tcp`), Google Cast (`_googlecast._tcp`), and Sonos (`_sonos._tcp`).
* **Strengths:** Zero-configuration service discovery without subnet scanning.
* **Caveat:** On macOS (Darwin), the native `mDNSResponder` service can monopolize UDP port 5353, causing third-party Go mDNS libraries to intermittently drop multicast announcements.

### 2. Simple Service Discovery Protocol (SSDP / UPnP)
Used by Sonos speakers and UPnP Media Renderers.
* **Mechanism:** Multicast M-SEARCH packet sent to `239.255.255.250:1900` querying `ST: urn:schemas-upnp-org:device:ZonePlayer:1`.
* **Strengths:** Operates via ephemeral unicast UDP responses, bypassing macOS mDNS socket restrictions and returning complete XML device descriptors in <500ms.

### 3. Actionable Port Probing
Used for devices that suppress mDNS or advertise via proprietary discovery.
* **RTSP (TCP 554):** Identifies IP cameras across the local subnet.
* **Omnivision (TCP 6080):** Probes for Alarm.com "OV Ready" local server daemons.
* **Qolsys (TCP 12345):** Verifies local alarm panel WebSocket availability.

### 4. MAC OUI Identification
Used to classify silent or non-standard devices by manufacturer OUI:
* `CC:33:31`: Lutron Electronics
* `00:0E:58` / `38:42:0B` / `74:CA:60` / `B0:E4:D5`: Sonos, Inc.
* `B8:3A:9D`: Alarm.com / Video IQ
* `3C:31:78`: Qolsys Inc.
* `F0:C7:7F`: Beijing Roborock Technology Co., Ltd.

---

## Baseline Device Profiles (Sanitized Reference)

### Lighting & Shading
| Subsystem | Example IP | Hardware Model | Masked MAC | Protocol / Port |
| :--- | :--- | :--- | :--- | :--- |
| **Smart Bridge Pro** | `192.168.1.50` | Lutron Smart Bridge 2 | `CC:33:31:XX:XX:XX` | LEAP / TLS (Port 8081) |

### Whole-Home Audio (Sonos)
| Role / Device | Example IP | Model | Masked MAC | Protocol / Port |
| :--- | :--- | :--- | :--- | :--- |
| **Soundbar** | `192.168.1.100` | Sonos Arc (S19) | `38:42:0B:XX:XX:XX` | UPnP / SOAP (Port 1400) |
| **Portable Speaker** | `192.168.1.120` | Sonos Move 2 (S44) | `74:CA:60:XX:XX:XX` | UPnP / SOAP (Port 1400) |
| **Bookshelf Pair** | `192.168.1.101` | Sonos One (S13) | `00:0E:58:XX:XX:XX` | UPnP / SOAP (Port 1400) |
| **Bookshelf Pair** | `192.168.1.102` | Sonos One (S13) | `00:0E:58:XX:XX:XX` | UPnP / SOAP (Port 1400) |
| **Architectural Amp**| `192.168.1.103` | Sonos Amp / Port | `38:42:0B:XX:XX:XX` | UPnP / SOAP (Port 1400) |

### Security Cameras (RTSP & ONVIF)
| Placement | Example IP | Model | Masked MAC | Protocol / Port |
| :--- | :--- | :--- | :--- | :--- |
| **Entryway Doorbell**| `192.168.1.200` | ADC-VDB770 | `B8:3A:9D:XX:XX:XX` | RTSP (554) / OV Ready (6080) |
| **Perimeter Camera** | `192.168.1.201` | ADC-V723x | `B8:3A:9D:XX:XX:XX` | RTSP (554) / OV Ready (6080) |
| **Perimeter Camera** | `192.168.1.202` | ADC-V723x | `B8:3A:9D:XX:XX:XX` | RTSP (554) / OV Ready (6080) |

### Media & Google Cast
| Device | Example IP | Model | Protocol / Port |
| :--- | :--- | :--- | :--- |
| **Smart Display** | `192.168.1.140` | Google Nest Hub Max | Google Cast (Port 8009) |
| **TV Streamer** | `192.168.1.141` | Chromecast with Google TV | Google Cast (Port 8009) |
| **Living Room TV** | `192.168.1.142` | Smart TV (Cast Receiver) | Google Cast (Port 8009) |

### Security Panel
| Subsystem | Example IP | Model | Masked MAC | Protocol / Port |
| :--- | :--- | :--- | :--- | :--- |
| **Alarm Panel** | `192.168.1.30` | Qolsys IQ Panel 4 | `3C:31:78:XX:XX:XX` | WSS (Port 12345) |

---

## Technical Protocol Details & Insights

### 1. Alarm.com Cameras ("OV Ready" + RTSP)
Cameras identify as `Server: OV Ready` on ports 6080 and 6443 using a proprietary Omnivision control protocol. Local RTSP video streaming is supported on port 554, requiring `camera_auth` (`user:pass`) configured in `~/.config/homectl/config.json`.

### 2. Sonos SSDP vs. mDNS
While Sonos speakers advertise `_sonos._tcp`, SSDP M-SEARCH (`239.255.255.250:1900`) is recommended for discovery on macOS workstations. SSDP queries return XML descriptor URLs (`http://<ip>:1400/xml/device_description.xml`) containing model name, room name, and Rincon IDs.

### 3. Qolsys IQ Panel 4 WebSocket API
The IQ Panel exposes a JSON WebSocket server on `wss://<host>:12345` with self-signed TLS. To activate this port, the 3rd Party Control4 Integration setting must be enabled via the panel touchscreen (`Settings > Advanced Settings > Installation > Devices > Security Sensors`).

### 4. Roborock Local UDP Lockout
Modern Roborock vacuums (e.g. Q5+, Saros 10r) refuse or ignore unauthenticated `miio` UDP handshakes on port 54321. Local control requires cloud-extracted tokens and encrypted communication over local MQTT.
