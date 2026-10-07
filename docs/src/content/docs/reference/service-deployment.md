---
title: Systemd Service Deployment
description: Production daemon setup, unit file configuration, and systemd management.
---

For continuous background execution and network availability, `homectl` can run as a Linux `systemd` service.

## Automated Setup

The repository provides an installation script in `scripts/install.sh`:

```bash
chmod +x ./scripts/install.sh
./scripts/install.sh
```

---

## Unit File Configuration

The installer places the service unit at `/etc/systemd/system/homectl.service`:

```ini
[Unit]
Description=homectl Smart Home API Server
After=network.target

[Service]
ExecStart=/usr/local/bin/homectl serve --port 8086 --ui /usr/local/share/homectl/ui
Restart=always
User=homectl-user
Environment=XDG_CONFIG_HOME=/home/homectl-user/.config

[Install]
WantedBy=multi-user.target
```

### Critical Directives:
* **`ExecStart`**: Starts `homectl serve` pointing to the shared UI asset directory. By default, it binds to `0.0.0.0` (all network interfaces).
* **`Restart=always`**: Automatically recovers if network connectivity drops or the host restarts.
* **`Environment=XDG_CONFIG_HOME`**: Ensures the service uses the correct user directory where certificates and `config.json` are stored.

### Network Isolation & Tailscale Binding:
Because the API server does not require authentication, binding to `0.0.0.0` exposes endpoints to all devices on the local subnet. For enhanced security:
* **Loopback only (`127.0.0.1`):** Append `--host 127.0.0.1` to `ExecStart` if using a local reverse proxy (like Nginx or Caddy) with authentication.
* **Tailscale overlay (`100.x.y.z`):** Append `-H 100.x.y.z` to `ExecStart` to restrict API access strictly to your authenticated Tailscale tailnet.
* **Persistent configuration:** Set `"api_host": "100.x.y.z"` in `~/.config/homectl/config.json`.

---

## Service Management Commands

```bash
# Start the daemon
sudo systemctl start homectl

# Stop the daemon
sudo systemctl stop homectl

# Restart after code update or configuration edit
sudo systemctl restart homectl

# Check status and health
systemctl status homectl

# Follow live log output
journalctl -u homectl -f

# Review logs since yesterday
journalctl -u homectl --since "yesterday"
```
