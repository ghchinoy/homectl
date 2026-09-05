---
title: Terminal UI (Bubble Tea)
description: The interactive multi-mode terminal dashboard for homectl.
---

`homectl ui` launches an interactive, responsive terminal dashboard powered by [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Lip Gloss](https://github.com/charmbracelet/lipgloss).

```bash
./homectl ui
```

## Views & Navigation

The TUI features a tabbed navigation system across three primary operational views:

### 1. Lights View
Displays all discovered Lutron Caseta / RA2 Select dimmers and switches:
* **Visual Progress Bars:** Shows brightness level from 0% (OFF) to 100%.
* **Master Switch:** Top-level `ALL LIGHTS` control to clear or illuminate the entire home.
* **Device Details Sidebar:** Displays zone href, model number, device type, and assigned nickname.

### 2. Music View
Displays discovered Sonos speakers:
* **Now Playing:** Shows live track name, artist, and album.
* **Live Volume Bar:** Visual indicator of speaker volume with real-time feedback.
* **Transport Indicators:** Shows `PLAYING`, `PAUSED_PLAYBACK`, or `STOPPED`.

### 3. Areas View
Filters and groups devices by their assigned physical rooms or areas on the Lutron bridge.

---

## Keyboard Shortcuts

| Key | Context | Action |
| :--- | :--- | :--- |
| **`Tab`** | Global | Cycle forward between Lights, Music, and Areas tabs |
| **`0` – `9`** | Lights / Music | Set level or volume: `1`=10%, `5`=50%, `0`=100% |
| **`Space`** | Music | Toggle Play / Pause on the selected speaker |
| **`n`** | Music | Skip to Next track |
| **`p`** | Music | Skip to Previous track |
| **`e`** | Any Device | Edit custom nickname (opens inline text input) |
| **`Enter`** | Editing Nickname | Save and persist nickname to `nicknames.json` |
| **`Esc`** | Editing Nickname | Cancel nickname edit |
| **`q` / `Ctrl+C`** | Global | Quit `homectl ui` |

---

## Nickname Editing in the TUI

To assign a custom friendly name to a device:
1. Navigate to the device using the arrow keys (`↑` / `↓`).
2. Press **`e`**.
3. Type the desired nickname (up to 32 characters).
4. Press **`Enter`**.

The change updates the display immediately and saves to `~/.config/homectl/nicknames.json`. The custom name will now appear consistently across the CLI, TUI, and Web UI.
