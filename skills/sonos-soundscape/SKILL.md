---
name: sonos-soundscape
description: Autonomous whole-home audio management, playback orchestration, volume safety guardrails, and queue-restoration workflows for Sonos speakers.
license: Apache-2.0
metadata:
  kind: skill-with-code
  domain: smart-home-audio
  version: "1.0.0"
---

# Sonos Soundscape Management Skill

This skill guides AI agents in discovering, querying, and controlling whole-home audio across networked Sonos speakers. It enforces strict volume safety policies, optimal prompt token economics, and playback recovery workflows.

---

## 1. Token Economics & The "Skill with Code" Pattern

Sonos devices communicate via UPnP SOAP and emit verbose XML metadata (DIDL-Lite fragments) containing tracking IDs, namespace declarations, and nested attributes. A single raw XML position query can consume **1,200–2,000 prompt tokens**.

### Rules for Context Optimization:
1. **Never request or ingest raw SOAP XML:** Always use the dedicated MCP tool `sonos_get_now_playing` or the bundled deterministic helper script `scripts/summarize_metadata.py`.
2. **Compact JSON Transformation:** The helper script strips namespaces and unescapes HTML entities, transforming an 800-byte XML block into a compact 40-byte JSON summary:
   ```json
   {
     "title": "So What",
     "artist": "Miles Davis",
     "album": "Kind of Blue"
   }
   ```
   **Token Reduction:** ~94% reduction per status check.

---

## 2. Audio Safety & Volume Boundaries

Audio volume is a physical actuator with immediate real-world acoustic impact. Agents must observe the following constraints:

1. **Volume Clamping:**
   - Default listening volumes must stay between **15% and 40%**.
   - Absolute volumes exceeding **60%** require explicit user confirmation.
   - Volumes exceeding **80%** are forbidden under normal operation.
2. **Relative Adjustments:**
   - Prefer relative volume adjustments (`delta: +5` or `delta: -5`) over large absolute jumps.
   - When asked to "turn it up" or "turn it down", use a step delta of **±5%**.
3. **Late-Night Sound Rules (10:00 PM – 7:00 AM):**
   - Automatically cap volume at **25%** unless user explicitly overrides.

---

## 3. Playback Recovery & Queue Restoration Workflow

When issuing a `play` action to an idle or group-switched Sonos speaker, UPnP may fail with error code `701 (Transition Not Available)`. This occurs when the current transport URI has no active track loaded.

### Recovery Protocol (see issue `control-f1l.18`):
1. **Detect 701 Fault:** If `sonos_control(action: "play")` returns a 701 error, do not immediately fail.
2. **Identify Coordinator & Queue:**
   - Retrieve speaker's RINCON identifier from discovery.
   - Set transport URI to the local queue:
     `x-rincon-queue:<RINCON_ID>#0`
   - Sleep 500ms for buffer stabilization.
3. **Fallback to Radio:**
   - If the local queue is empty (`QueueCount == 0`), fallback to default ambient radio URI:
     `x-sonosapi-radio:sonos:158288?sid=303&flags=0&sn=1`
4. **Retry Play:** Issue `sonos_control(action: "play")` again.

---

## 4. MCP Tools Quick Reference

When interacting with `homectl-sonos-mcp`:

| Tool | Mode | Purpose | Key Parameters |
|---|---|---|---|
| `sonos_list_speakers` | 🔒 Read-Only | Discover speakers on LAN | `refresh: bool` |
| `sonos_get_now_playing`| 🔒 Read-Only | Get track metadata & progress | `ip: string` |
| `sonos_control` | ⚡ Mutating | Play, pause, stop, next, previous | `ip: string`, `action: string` |
| `sonos_set_volume` | ⚡ Mutating | Adjust absolute or relative volume | `ip: string`, `volume?: int`, `delta?: int` |

---

## 5. Pre-flight Execution Checklist for Agents

Before adjusting audio in any room:
- [ ] Has the speaker IP or name been verified via `sonos_list_speakers`?
- [ ] Is the proposed volume change within safe limits (≤ 60% or step delta ≤ 10%)?
- [ ] If pausing/stopping, is playback actually active?
- [ ] If track info is required, did you use `sonos_get_now_playing` to conserve context?
