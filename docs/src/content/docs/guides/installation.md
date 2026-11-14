---
title: Installation
description: Install homectl as a standalone binary or a systemd service.
---

`homectl` can be installed either as a standalone binary on your workstation or as a dedicated system service on a local server.

## Prerequisites

- **Operating System:** Linux (Ubuntu/Debian, Arch, Fedora) or macOS (Darwin arm64/amd64).
- **Go:** 1.26.11+ (when building from source).
- **Node.js:** 22.23.4+ and `npm` (when building the Web UI or documentation from source).
- **FFmpeg:** Optional; required only for RTSP camera transcoding.

---

## Method 1: Download a Prebuilt Binary (macOS & Linux)

Download the latest signed archive for your platform (`darwin/arm64`, `darwin/amd64`, `linux/amd64`, `linux/arm64`) from [GitHub Releases](https://github.com/ghchinoy/homectl/releases), verify its SHA-256 checksum against `checksums.txt`, and place `homectl` and `homectl-mcp-sonos` on your `PATH`. Prebuilt `homectl` binaries bundle the compiled Web UI directly inside the executable.

---

## Method 2: Building from Source

To compile all binaries (`homectl`, `mcp-sonos`, `sync-skills`) with embedded version metadata and the Web UI bundle:

```bash
git clone https://github.com/ghchinoy/homectl.git
cd homectl

# Build UI bundle and Go binaries into ./bin/
make build

# Move homectl to a system path
sudo cp ./bin/homectl /usr/local/bin/
```

Verify your installation:
```bash
homectl version
```

---

## Method 3: Systemd Daemon Installation (Linux)

For dedicated Linux hosts (such as a Raspberry Pi or home server), `homectl` provides an automated installer script:

```bash
./scripts/install.sh
```

### What the installer does:
1. Compiles the Go binary `homectl` with version LDFLAGS (`make bin-homectl`).
2. Runs `npm ci` and builds the production Web UI (`ui/dist/`).
3. Installs the executable to `/usr/local/bin/homectl`.
4. Copies web assets to `/usr/local/share/homectl/ui`.
5. Creates, enables, and starts a system-level `systemd` unit (`/etc/systemd/system/homectl.service`):
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
