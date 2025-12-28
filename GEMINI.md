# Project Control: Lutron Integration

This project is a Go-based integration for Lutron Caseta/RA2 Select systems, featuring a Cobra CLI, Bubble Tea TUI, and a Lit Web UI.

## Developer Onboarding

- **Issue Tracking:** This project uses `bd` (beads). Refer to [AGENTS.md](./AGENTS.md) for core workflow instructions and run `bd prime` for full context.
- **Environment:**
    - **Go:** Core application logic and CLI.
    - **Python (via `uv`):** Used for auxiliary tasks like the initial pairing process.
    - **Certs:** Certificates are required for communication and are ignored by git. See [lutron_discovery.md](./lutron_discovery.md) for pairing instructions.

## Lessons Learned: Lutron Discovery & LEAP

- **mDNS is Key:** Lutron bridges announce themselves via `_lutron._tcp` and `_leap._tcp`.
- **LEAP Protocol:** Communication happens over TLS on port `8081`.
- **Strict TLS:** The bridge requires client-side certificates. You cannot even "ping" the LEAP protocol without a valid handshake.
- **Pairing Flow:** Pairing requires a specific socket handshake followed by a physical button press on the bridge to generate a unique certificate/key pair for that client.

## Coding Conventions

- **Packages:** Logic is decoupled into `pkg/`.
    - `pkg/leap`: Core protocol handling.
- **CLI Commands:** Use Cobra for command structure.
- **TUI:** Use Bubble Tea for interactive terminal components.
- **Commits:** Use [Conventional Commits](https://www.conventionalcommits.org/).

## Changelog Management

To generate or update the `CHANGELOG.md` from completed tasks in `bd`, run:

```bash
bd list --status closed --json | jq -r 'sort_by(.closed_at) | reverse | map(select(.closed_at != null)) | group_by(.closed_at[0:10]) | reverse | .[] | "## " + (.[0].closed_at[0:10]) + "\n" + (map("- " + .title + " (" + .id + ")") | join("\n")) + "\n"' > CHANGELOG.md
```

