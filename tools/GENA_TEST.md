# Sonos GENA Event Testing

This document describes how to test real-time event notifications (GENA) from Sonos speakers.

## The Challenge: NAT and Inbound Traffic
Sonos speakers send events by making an **inbound** HTTP `NOTIFY` request to your machine. If you are running `homectl` inside a NATed environment (like ChromeOS/Crostini or a Docker container), the speaker cannot reach your internal IP (e.g., `100.115.x.x`).

## Method 1: Direct Run (Recommended)
Run the diagnostic tool on a machine physically connected to the same network as the speakers (e.g., your Linux server at `192.168.1.50`).

1.  **Pull the code** to the Linux machine.
2.  **Run the tool**:
    ```bash
    go run tools/gena_debug.go -ip <SPEAKER_IP>
    ```
    *The tool will automatically detect your `192.168.1.50` IP and use it for the callback.*

## Method 2: SSH Remote Tunnel
If you want to keep developing in your NATed environment but receive events via the Linux server:

1.  **Setup the Tunnel**: From your NATed environment, run:
    ```bash
    ssh -R 37915:localhost:37915 <USER>@192.168.1.50
    ```
    *Note: The Linux machine's `/etc/ssh/sshd_config` may need `GatewayPorts yes` for the speaker to reach the tunnel.*

2.  **Run the tool**:
    ```bash
    go run tools/gena_debug.go -ip <SPEAKER_IP> -callback-ip 192.168.1.50 -port 37915
    ```

## Diagnostic Tool: `gena_debug.go`
The `tools/gena_debug.go` utility performs the following:
- Starts a raw HTTP server that logs **all** incoming request headers and bodies.
- Attempts to `SUBSCRIBE` to a speaker's `AVTransport` service.
- Reports `412 Precondition Failed` if the speaker cannot reach the provided callback URL.
- Reports `✓ SUBSCRIPTION SUCCESSFUL` if the speaker successfully validated the path.

## Configuration for `homectl`
Once you've confirmed an IP/Port works, you can persist the callback IP in `~/.config/homectl/config.json`:

```json
{
  "callback_ip": "192.168.1.50"
}
```
*Note: homectl currently uses a random port for every session to avoid "address already in use" errors during development.*
