---
title: Skills with Code & Safety Guardrails
description: How homectl combines declarative Agent Skills with deterministic helper scripts and physical actuator safety boundaries.
---

Controlling smart home hardware introduces physical consequences that pure software tools do not face. A misplaced volume command can wake a household, while unconstrained XML queries can consume thousands of LLM context window tokens on every turn.

`homectl` addresses this through the **Skill with Code** pattern and hard operational boundaries.

---

## 1. The "Skill with Code" Pattern

Declarative prompt instructions in a `SKILL.md` file are necessary, but when dealing with verbose IoT protocols (like UPnP/SOAP), prompting an LLM to parse and transform raw XML in-context wastes massive amounts of reasoning capacity.

```text
┌─────────────────────────┐
│     AI Coding Agent     │
└────────────┬────────────┘
             │ 1. Receives Raw XML Query
             ▼
┌─────────────────────────┐
│  sonos-soundscape Skill │
└────────────┬────────────┘
             │ 2. Pipes raw XML into deterministic script
             ▼
┌─────────────────────────┐
│  summarize_metadata.py  │ ──> [Strips namespaces, unescapes entities, parses tags]
└────────────┬────────────┘
             │ 3. Emits compact, 40-byte JSON
             ▼
┌─────────────────────────┐
│ Agent Context (~40 B)   │ (94% prompt token savings!)
└─────────────────────────┘
```

### Deterministic Transformation (`scripts/summarize_metadata.py`)
Sonos speakers emit raw DIDL-Lite XML fragments:
```xml
<DIDL-Lite xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/" xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/">
  <item id="-1" parentID="-1" restricted="true">
    <dc:title>So What</dc:title>
    <dc:creator>Miles Davis</dc:creator>
    <upnp:album>Kind of Blue</upnp:album>
    <res protocolInfo="sonos.com-http:*:audio/flac:*">http://192.168.1.100:1400/stream</res>
  </item>
</DIDL-Lite>
```

Feeding this directly into an LLM costs **~1,200–1,800 tokens** per track status check.

By invoking the bundled helper script:
```bash
cat raw_track.xml | python3 skills/sonos-soundscape/scripts/summarize_metadata.py
```

The output is reduced to:
```json
{
  "title": "So What",
  "artist": "Miles Davis",
  "album": "Kind of Blue"
}
```

**Result:** The agent ingests only **~40 bytes (~10 tokens)**, conserving context space for user conversation and complex automation reasoning.

---

## 2. Physical Actuator Safety Boundaries

Because speakers, dimmers, and alarm panels operate in physical residential space, `homectl` codifies strict actuator safety guardrails in its skill packs:

### Acoustic Boundaries (`sonos-soundscape`)
1. **Safe Listening Clamping:**
   * Normal automated adjustments must default to the **15% to 40%** volume window.
   * Volume targets exceeding **60%** require explicit user confirmation.
   * Volume targets exceeding **80%** are forbidden under normal operation.
2. **Relative Adjustments:**
   * Agents are instructed to prefer relative deltas (`delta: +5` or `delta: -5`) when a user asks to *"turn it up"* or *"make it quieter"*, avoiding sudden loud volume jumps.
3. **Late-Night Sound Rules (10:00 PM – 7:00 AM):**
   * Volume is automatically capped at **25%** unless explicitly overridden by the user.

### Security Panel Boundaries (`qolsys-guard`)
1. **Perimeter Verification Pre-Flight:**
   * Before arming the alarm system, the agent must query sensor status to ensure all perimeter doors and windows are closed.
2. **Mandatory PIN Gating:**
   * All arming mutations require an explicit 6-digit access PIN.
3. **Autonomous Disarming Forbidden:**
   * Agents are strictly barred from disarming a residential security system autonomously without real-time human presence and approval.

---

## 3. Self-Healing Fault Recovery

Networked IoT devices frequently enter transient error states. Rather than aborting and returning cryptic UPnP codes to the user, `homectl`'s Go engine and skills include automated recovery loops:

### Recovery from UPnP Error 701 (Transition Not Available)
When a Sonos speaker finishes an album, unjoins a group, or boots from cold standby, its transport state is `STOPPED` with no active track loaded. Calling `play` triggers UPnP Error `701`:

```text
[Play Command] ──> (Fails with 701: No URI Loaded)
                          │
                          ▼
            [Check Local Queue Count]
                 /               \
         (Count > 0)          (Count == 0)
              │                    │
              ▼                    ▼
   [Set x-rincon-queue URI]  [Set Ambient Radio URI]
              │                    │
              └─────────┬──────────┘
                        │
                        ▼
               [Re-execute Play()] ──> (Success!)
```

`modules/sonos` automatically catches error 701:
1. Inspects `GetQueueCount()`.
2. If tracks exist, restores the transport URI to `x-rincon-queue:<rincon>#0`.
3. If the queue is empty, sets ambient Sonos Radio (`x-sonosapi-radio:...`).
4. Re-executes `Play` seamlessly.

### Stereo-Pair & Group Follower Auto-Routing
In Sonos stereo pairs or groups, secondary speakers report their transport as `PLAYING` with `TrackURI: x-rincon:<coordinator>` and empty metadata. 

`homectl` automatically:
1. Identifies follower status (`is_follower: true`).
2. Resolves the group coordinator IP via `GetCoordinatorIP()`.
3. Re-queries the coordinator for authoritative metadata.
4. Routes mutating playback commands to the master speaker, avoiding broken stereo imaging.
