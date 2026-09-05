---
title: Client Setup Guide (OpenCode & Claude)
description: Configure OpenCode, Claude Desktop, Cursor, or custom agent harnesses to use homectl's MCP servers.
---

`homectl` exposes its IoT control capabilities via standard Model Context Protocol (MCP) servers communicating over standard input/output (stdio). This guide explains how to connect your preferred AI agent environment to `homectl`.

---

## 1. Automated Global Installation (Recommended)

`homectl` provides an automated installer that compiles all MCP servers, installs them to `~/.local/bin/`, and automatically registers them into your OpenCode configuration.

From the repository root, run:

```bash
# Build all binaries into ./bin/
make build

# Install to ~/.local/bin and auto-register in ~/.config/opencode/opencode.jsonc
make install-mcp
```

### What `make install-mcp` does:
1. Compiles `cmd/mcp-sonos` to `./bin/mcp-sonos`.
2. Copies the binary to `~/.local/bin/homectl-mcp-sonos` with executable permissions.
3. Automatically backs up and updates `~/.config/opencode/opencode.jsonc` using absolute binary paths, allowing OpenCode to launch the MCP server regardless of which working directory you are in.

---

## 2. OpenCode Configuration

If configuring OpenCode manually or scoping the MCP server to a specific repository, edit either `~/.config/opencode/opencode.jsonc` (global) or `.opencode/opencode.jsonc` (project-local):

```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "sonos": {
      "type": "local",
      "command": ["~/.local/bin/homectl-mcp-sonos"],
      "enabled": true
    }
  }
}
```

*Note: If running directly from a clone of the `homectl` repository, you can point to the local binary directly:*
```jsonc
{
  "mcp": {
    "sonos": {
      "type": "local",
      "command": ["./bin/mcp-sonos"],
      "enabled": true
    }
  }
}
```

---

## 3. Claude Desktop Configuration

To use `homectl`'s MCP servers with Anthropic's Claude Desktop application:

1. Open Claude Desktop settings: **Claude > Settings > Developer > Edit Config**.
2. Edit `claude_desktop_config.json`:
   * **macOS:** `~/Library/Application Support/Claude/claude_desktop_config.json`
   * **Windows:** `%APPDATA%\Claude\claude_desktop_config.json`
3. Add the server under `mcpServers`:

```json
{
  "mcpServers": {
    "homectl-sonos": {
      "command": "/Users/<your-username>/.local/bin/homectl-mcp-sonos"
    }
  }
}
```

4. Restart Claude Desktop. The hammer icon in Claude Desktop will now show the registered `homectl-sonos` tools.

---

## 4. Cursor / VS Code Copilot / Windsurf

For editor-based AI coding agents:

### Cursor (`.cursor/mcp.json` or `~/.cursor/mcp.json`):
```json
{
  "mcpServers": {
    "homectl-sonos": {
      "command": "/Users/<your-username>/.local/bin/homectl-mcp-sonos"
    }
  }
}
```

### VS Code Copilot (`.vscode/mcp.json`):
```json
{
  "servers": {
    "homectl-sonos": {
      "type": "stdio",
      "command": "/Users/<your-username>/.local/bin/homectl-mcp-sonos"
    }
  }
}
```

---

## 5. Direct CLI Testing (Without an LLM)

You can verify that the MCP server compiles and responds to JSON-RPC protocol messages directly from your terminal using stdio piping:

```bash
# Send an MCP initialize handshake and request tools/list:
printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}\n{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}\n{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}\n' | ./bin/mcp-sonos
```

Expected output:
```json
{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{"tools":{}},"serverInfo":{"name":"homectl-sonos","version":"1.0.0"}}}
{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"sonos_list_speakers",...},{"name":"sonos_get_now_playing",...}]}}
```

---

## 6. Example Agent Interactions

Once configured, your AI agent can autonomously discover and control your hardware using plain natural language:

### Status Queries
> **User:** *"What is playing on the Move 2 speaker right now?"*
>
> **Agent Action:** Invokes `sonos_get_now_playing(ip: "192.168.1.120")`
>
> **Agent Response:** *"The Move 2 is currently playing 'Poison' by Alice Cooper at 18% volume (0:00 / 4:29)."*

### Playback & Intent Routing
> **User:** *"Play my Morning Jazz favorite in the kitchen and set the volume to 30%."*
>
> **Agent Action:**
> 1. Invokes `sonos_list_speakers` to resolve "Kitchen" to `192.168.1.121`.
> 2. Invokes `sonos_list_favorites` to find the favorite ID for "Morning Jazz".
> 3. Invokes `sonos_set_volume(ip: "192.168.1.121", volume: 30)`.
> 4. Invokes `sonos_play_favorite(ip: "192.168.1.121", favorite_id: "FV:2/3")`.
>
> **Agent Response:** *"Adjusted Kitchen volume to 30% and launched 'Morning Jazz' from your Sonos Favorites."*

### Multi-Room Grouping
> **User:** *"Group the patio and deck speakers with the living room soundbar."*
>
> **Agent Action:**
> 1. Invokes `sonos_get_topology` to find the coordinator for "Living Room".
> 2. Invokes `sonos_control(ip: "192.168.1.130", action: "join", master: "192.168.1.100")`.
> 3. Invokes `sonos_control(ip: "192.168.1.131", action: "join", master: "192.168.1.100")`.
>
> **Agent Response:** *"Joined the Patio and Deck speakers into the Living Room group."*
