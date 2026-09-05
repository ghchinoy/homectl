---
title: Lutron Caseta & RA2 Select
description: Connect to Lutron Smart Bridges locally via the encrypted LEAP protocol.
---

`homectl` communicates with Lutron Smart Bridges and RA2 Select main repeaters using the **LEAP** (Lutron Extensible Application Protocol) over TLS on port `8081`.

## Pairing & Certificate Generation

Lutron bridges require mutual TLS authentication. You must pair `homectl` with your bridge once to extract client certificates.

### Automated Pairing Script

The repository includes a Python pairing helper in `tools/pair_lutron.py`:

```bash
# Install Python dependency
pip install pylutron-caseta

# Run pairing script
python3 tools/pair_lutron.py
```

When prompted:
1. The script will search for the Smart Bridge on your network.
2. **Press the physical button** on the back of your Lutron Smart Bridge.
3. The script will securely save three certificate files to `~/.config/homectl/`:
   - `lutron_client.crt`
   - `lutron_client.key`
   - `lutron_ca.crt`

Once saved, `homectl` uses these certificates automatically for all future CLI, TUI, and Web UI interactions.

---

## Discovering the Bridge

If you don't know your bridge IP address, `homectl` discovers it via mDNS (`_leap._tcp`):

```bash
./homectl discover
```

The bridge IP is cached in `~/.config/homectl/lutron_cache.json` for zero-latency startups.

---

## Controlling Lights & Shades

### Listing Resources

```bash
# List all areas/rooms defined on the bridge
./homectl lutron list areas

# List all lighting and shade zones
./homectl lutron list zones

# List all devices with real-time dimming levels
./homectl lutron list devices
```

### Adjusting Levels

Set the dimming level (0–100%) for any zone using its resource path:

```bash
# Turn on a zone to 75%
./homectl lutron set level /zone/1 75

# Turn off a zone
./homectl lutron set level /zone/1 0

# Turn all lights off
./homectl lutron set all 0

# Set all lights to 100%
./homectl lutron set all 100
```

---

## Protocol Details

LEAP uses newline-delimited JSON payloads over TLS:
* **Read Request:** Query `/device`, `/zone`, `/area`, or `/zone/status`.
* **Create Request:** Sent to `/zone/<id>/commandprocessor` with payload:
  ```json
  {
    "Command": {
      "CommandType": "GoToLevel",
      "Parameter": [{ "Type": "Level", "Value": 75 }]
    }
  }
  ```
The connection uses automatic reconnect logic with client tag correlation to maintain low-latency responsiveness.
