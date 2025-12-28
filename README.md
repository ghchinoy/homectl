# Control

A modern, Go-powered toolkit for Lutron Caséta and RA2 Select systems. This project provides a robust library, a feature-rich CLI, and a beautiful Terminal UI (TUI) for local smart home management.

## Project Goals

- **Performance:** Native Go implementation for high-speed local control.
- **Interactivity:** A delightful TUI built with Bubble Tea for real-time monitoring and control.
- **Extensibility:** A clean Go SDK (`pkg/leap`) that can be used in other projects.
- **Modern Web:** A server mode that hosts a Lit WebComponent dashboard.

## Prerequisites

- **Go:** 1.25+
- **Lutron Bridge:** Caséta Smart Bridge or RA2 Select Main Repeater.
- **Python/uv:** Required for the initial one-time pairing process.

## Setup & Pairing

Lutron bridges require unique TLS client certificates for security. 

1. **Discovery:**
   The system identifies your bridge using mDNS.
2. **Pairing:**
   Run the pairing script to generate your unique credentials:
   ```bash
   uv run --with pylutron-caseta pair_lutron.py
   ```
   When prompted, press the black button on the back of your bridge. This will generate the necessary `.crt` and `.key` files (which are ignored by git).

## Installation & Usage

### Build
```bash
go build -o control .
```

### CLI Commands (Work in Progress)
The CLI is built with Cobra. Future commands include:

- **List Devices:** `control list devices`
- **Control Lights:** `control set --id <id> --level 50`
- **TUI Mode:** `control ui` (launches the Bubble Tea interface)
- **Server Mode:** `control serve` (starts the HTTP API and Web UI)

## Testing the Client
To run the current integration test that verifies connection to your bridge:
```bash
cd go_test
go run main.go
```

## Documentation
- [Lutron Discovery Report](./lutron_discovery.md): Detailed discovery results and pairing notes.
- [GEMINI.md](./GEMINI.md): Lessons learned, coding conventions, and changelog workflow.
- [AGENTS.md](./AGENTS.md): Issue tracking and developer workflow.
