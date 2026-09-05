---
title: "Agent Ecosystem: Overview & The 3 Pillars"
description: Architectural overview of homectl's AI agent ecosystem, token economics, and the Three Pillars (MCP Servers, Skills with Code, and Agent Plugins).
---

Smart home automation is evolving from static screen-based dashboards to natural-language, autonomous agent interaction:

> *"Dim the living room lights to 20%, group the patio speakers, and play my Morning Jazz favorite."*

Controlling physical smart home hardware through Large Language Models (LLMs) presents unique engineering challenges:
1. **Physical Actuator Impact:** Sending the wrong command or volume spike to an amplifier or alarm system has immediate, irreversible acoustic and real-world consequences.
2. **Context Window Inflation:** IoT protocols (like UPnP/SOAP) emit verbose XML payloads (DIDL-Lite fragments) that can consume **1,200–2,000 prompt tokens** on a single status query.
3. **Tool Sprawl:** Naively exposing every granular device action as a separate MCP tool floods the model's system prompt with massive JSON schemas (e.g. 50+ tools consuming 12,000+ tokens on every conversational turn), degrading model reasoning.

`homectl` solves these challenges through an integrated architecture built around **The Three Pillars**.

---

## The Three Pillars of the Agent Ecosystem

```text
┌────────────────────────────────────────────────────────────────────────┐
│                        THE THREE PILLARS                               │
├─────────────────────┬──────────────────────┬───────────────────────────┤
│ 1. Runtime Layer    │ 2. Policy Layer      │ 3. Distribution Layer     │
│    MCP Servers      │    Agent Skills      │    Agent Plugins          │
│  (bin/mcp-<svc>)    │  (skills/<name>/)    │  (plugins/<svc>/)         │
├─────────────────────┼──────────────────────┼───────────────────────────┤
│ • Pure Go stdio     │ • "Skill with Code"  │ • Agent Plugins Spec 1.0  │
│ • Validated JSON-RPC│ • Token compression  │ • Dual manifests          │
│ • SEP-2106 objects  │ • Actuator safety    │ • Automated skill sync    │
│ • Instant <5ms boot │ • Error 701 recovery │ • CI consistency gates    │
└─────────────────────┴──────────────────────┴───────────────────────────┘
```

### Pillar 1: Model Context Protocol (MCP) Servers
The runtime bridge between LLM agents and device drivers:
* **Pure Go Static Binaries (`bin/mcp-<svc>`):** Micro-servers compiled directly from Go source (`cmd/mcp-<svc>`) using the official `@modelcontextprotocol/go-sdk`. They boot in under **5ms** with zero external runtime dependencies (no Node.js, Python runtime, or heavy `node_modules` trees).
* **Consolidated, Token-Budgeted Tooling:** Rejects granular tool sprawl. Instead of exposing 60 disjoint tools, `homectl` exposes **10 intent-oriented tools** per domain (consuming under **1,500 tokens** total).
* **SEP-2106 Structured Output Compliance:** Every tool returns a structured Go record/object wrapper (`ListSpeakersResult`, `TopologyResult`) rather than bare arrays or unstructured string dumps, preventing schema validation failures in strict clients.

### Pillar 2: Agent Skills ("Skill with Code")
The reasoning, policy, and safety layer stored in `skills/<name>/SKILL.md`:
* **The "Skill with Code" Pattern:** Pairs declarative prompt instructions with deterministic local scripts. For example, `skills/sonos-soundscape/` bundles `scripts/summarize_metadata.py` to strip XML namespaces and unescape entities, reducing raw UPnP track metadata from 1,800 prompt tokens down to 40 bytes of clean JSON (**~94% token savings**).
* **Physical Actuator Safety:** Hardens operations with strict boundaries (clamping standard listening volumes between 15% and 40%, requiring confirmation above 60%, and barring autonomous alarm disarming).
* **Self-Healing Fault Recovery:** Codifies protocol workarounds, such as catching UPnP Error `701 (Transition Not Available)` and automatically restoring local queues or ambient radio before retrying playback.

### Pillar 3: Agent Plugins & Packaging
The distribution and interoperability packaging conforming to the **Agent Plugins Specification 1.0.0**:
* **Self-Contained Bundles (`plugins/<svc>/`):** Encapsulates the skill, manifests, and references to the compiled executable.
* **Dual Manifests:** Generates both the universal `plugin.json` / `mcp.json` standard and the OpenCode-native `opencode.jsonc` configuration.
* **Single Source of Truth:** Canonical skill sources live in top-level `skills/`. Running `make sync-skills` mirrors them into plugin bundles, and the CI gate (`make check-skills`) asserts zero drift and zero missing script dependencies.

---

## Token Economics & Efficiency

A comparison of prompt overhead between unoptimized 1:1 protocol exposure and `homectl`'s intent-budgeted architecture:

| Metric | Direct 1:1 Action Mapping | homectl Intent-Based Architecture | Efficiency Gain |
| :--- | :---: | :---: | :---: |
| **Tool Count** | 50+ individual tools | **10 consolidated tools** | **~80%** fewer tools |
| **Schema Prompt Overhead** | ~40 KB (~12,000 tokens) | **~4.5 KB (~1,500 tokens)** | **~88%** token savings |
| **Track Metadata Inspection** | ~1,800 tokens (raw XML) | **~40 bytes (compact JSON)** | **~94%** token savings |
| **Cold Start Latency** | 1,500ms – 4,000ms (interpreted) | **< 5ms (Go binary)** | **>99%** faster boot |

---

## Next Steps

* [Client Setup Guide](/homectl/agents/client-setup/): Configure OpenCode, Claude Desktop, or Cursor to connect to `homectl`'s MCP servers.
* [MCP Server Reference](/homectl/agents/mcp-servers/): Inspect tools, parameters, and return schemas.
* [Skills & Safety Guardrails](/homectl/agents/skills-and-safety/): Learn about the "Skill with Code" pattern and actuator policies.
* [Plugin Packaging & Authoring](/homectl/agents/packaging/): Guidelines for authoring new agent skills and plugins.
