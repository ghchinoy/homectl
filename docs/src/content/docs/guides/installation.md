---
title: Installation
description: Install homectl as a standalone binary or a systemd service.
---

`homectl` can be installed either as a standalone binary on your workstation or as a dedicated system service on a local server.

## Prerequisites

- **Operating System:** Linux (Ubuntu/Debian, Arch, Fedora) or macOS (Darwin arm64/amd64).
- **Go:** 1.25+ (for building from source).
- **Node.js:** 20+ and `npm` (to build the Web UI).
- **FFmpeg:** Optional; required only for RTSP camera transcoding.

---

## Method 1: Building from Source

To compile the latest binary on your local machine:

```bash
git clone https://github.com/ghchinoy/homectl.git
cd homectl

# Build the binary
go build -o homectl .

# Move to a system path
sudo mv homectl /usr/local/bin/
```

Verify your installation:
```bash
homectl --help
```

---

## Method 2: Systemd Daemon Installation (Linux)

For dedicated Linux hosts (such as a Raspberry Pi or home server), `homectl` provides an automated installer script:

```bash
./scripts/install.sh
```

### What the installer does:
1. Compiles the Go binary `homectl`.
2. Installs npm dependencies and builds the production Web UI (`ui/dist/`).
3. Installs the executable to `/usr/local/bin/homectl`.
4. Copies web assets to `/usr/local/share/homectl/ui`.
5. Creates, enables, and starts a `systemd` user service:
   ```ini
   [Unit]
   Description=homectl Smart Home API Server
   After=network.target

   [Service]
   ExecStart=/usr/local/bin/homectl serve --port 8086 --ui /usr/local/share/homectl/ui
   Restart=always
   User=<your-user>
   Environment=XDG_CONFIG_HOME=/home/<your-user>/.config

   [Install]
   WantedBy=multi-user.target
   ```

### Managing the Service

```bash
# Check running status
systemctl status homectl

# Restart or stop the service
sudo systemctl restart homectl
sudo systemctl stop homectl

# Stream real-time logs
journalctl -u homectl -f
```

To upgrade an existing installation after pulling git updates, rerun `./scripts/install.sh`.
