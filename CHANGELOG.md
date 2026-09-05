# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and [Common Changelog](https://common-changelog.org/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0] - 2026-09-05

_Major architectural release introducing disaggregated Go modules, standalone Agent Plugins (MCP), Catppuccin Astro Starlight documentation, and robust audio stream management._

### Added
- **Agent Plugins & MCP Server:** Implemented standalone `cmd/mcp-sonos` stdio server with 11 token-optimized tools (`sonos_list_speakers`, `sonos_get_now_playing`, `sonos_get_topology`, `sonos_control`, `sonos_set_volume`, `sonos_list_favorites`, `sonos_play_favorite`, `sonos_play_stream`, `sonos_add_to_queue`, `sonos_get_queue`, `sonos_list_services`) adhering to MCP SEP-2106 object return schemas ([control-ro4.7], [control-ro4.10], [control-18l.6], [control-ul9]).
- **Sonos Soundscape Skill:** Packaged canonical `skills/sonos-soundscape/` with deterministic metadata compression script (`summarize_metadata.py`) delivering ~94% prompt token savings, volume boundaries, and error recovery protocols ([control-ro4.8]).
- **Astro Starlight Documentation Site:** Scaffolded complete documentation site under `docs/` themed with official `@catppuccin/starlight` (Mocha + Sky), featuring 30 documentation pages, lightbox image zoom (`starlight-image-zoom`), and automated GitHub Pages deployment on push.
- **Architecture Diagrams (Graphviz):** Added 6 system and vertical `.dot` diagrams with automated `.webp` compilation toolchain via `make diagrams`.
- **Audio Streaming & Cloud Favorites:** Enabled browsing and launching cloud playlists/stations from Spotify, Apple Music, and Sonos Radio via `ContentDirectory:Browse("FV:2")`, plus direct web audio streaming via `SetAVTransportURI` ([control-18l.1], [control-18l.3]).
- **Queue Inspection & Track Seeking:** Added `homectl sonos queue` for paginated playback queue inspection, `homectl sonos seek` for track jumping and time seeking (`[H:]MM:SS`), and exposed `seek_track` and `seek_time` actions on the `sonos_control` MCP tool ([control-ul9], [control-mdo]).
- **Container Favorites Queue Replacement:** Implemented automatic 4-step queue replacement protocol for cloud container playlists (Spotify, YouTube Music Liked Music, Apple Music) when invoking `PlayFavorite`.
- **Structured JSON & Dry-Run Flags:** Added `--json` flag to `list`, `sonos`, and `discover` commands and `--dry-run` simulation to mutating actions (`set level`, `set all`, `sonos volume`) ([control-4iw.4], [control-4iw.5]).
- **Dual Manifests & Packaging Tooling:** Automated skill mirroring via `cmd/sync-skills` generating dual `mcp.json` and `opencode.jsonc` manifests with CI consistency verification gate (`make check-skills`) ([control-ro4.19], [control-ro4.21]).
- **License:** Added official Apache License Version 2.0 (`LICENSE`).

### Changed
- **Monorepo Architecture:** Disaggregated monolithic structure into a multi-module workspace (`go.work`) with interface-only `modules/core` and domain `modules/sonos` ([control-ro4.1], [control-ro4.3]).
- **Stereo-Pair Follower Resolution:** Automatically redirects follower speaker queries to authoritative group coordinators, eliminating false-positive empty track status ([control-84r]).
- **Bridge IP Resolution:** Replaced hardcoded default IP with strict precedence: CLI flag > env var > config.json > cache > mDNS discovery ([control-4iw.6]).
- **Concurrency & Discovery:** Converted camera discovery to concurrent port 554 scanning to prevent mDNS starvation ([control-g50]), and added SSDP M-SEARCH fallback for Sonos on macOS ([control-ra2]).
- **Error Propagation:** Converted Cobra CLI commands from `log.Fatalf` to idiomatic `RunE` returning wrapped errors (`%w`).

### Fixed
- **TUI Data-Loss Bug:** Fixed `saveNicknames()` in `pkg/tui/tui.go` which previously initialized an empty map and wiped `nicknames.json`.
- **Format String Error:** Corrected `cmd/list.go` printf formatting to properly display zone brightness percentages.
- **Error 701 Self-Healing:** Added automatic local queue and radio restoration when resuming stopped or unjoined speakers ([control-f1l.18]).
- **SSRF Hardening in Art Proxy:** Added strict IP parsing, loopback/link-local/metadata blocking, and 5-second timeouts to `/api/sonos/art`.
- **Tool Package Collision:** Added `//go:build ignore` to one-off diagnostic tools in `tools/` resolving `go vet ./...` package conflicts.

### Security
- **PII Scrubbing & State Isolation:** Purged all historical MAC addresses, serial numbers, and physical entryway placements across 57 commits using `git-filter-repo`. Established gitignored `local/` directory for physical hardware inventories ([control-b81]).
- **Config Permissions:** Hardened directory permissions in `pkg/config.EnsureDir()` to `0700`.

---

## [0.1.0] - 2025-12-31

### Added
- Compose Video/Cast section with real-time status in Web UI ([control-wlu]).
- Google Cast playback and volume controls in Web UI ([control-aqv]).
- Google Cast "Now Playing" status parsing ([control-tkc]).

---

## [0.0.1] - 2025-12-28

### Added
- Lit Web UI dashboard with dynamic card themes for light levels ([control-9d7], [control-26a]).
- Sonos transport and volume controls in Web UI ([control-f5z]).
- Bubble Tea interactive Terminal UI (TUI) with split-screen details ([control-ibt], [control-1cg]).
- Real-time UPnP GENA event listener and subscriptions for Sonos ([control-f1l.11], [control-f1l.12]).
- Unified Go-native discovery engine for mDNS, SSDP, and RTSP ([control-f1l.4]).
- XDG Base Directory configuration storage under `~/.config/homectl/` ([control-4ye]).

### Fixed
- Fixed concurrency panic in LEAP client during batch requests ([control-1nq]).
- Fixed Sonos volume progress bar flicker on TUI refresh ([control-9os]).
- Resolved Sonos 402 Invalid Args SOAP error ([control-us2]).

---

## [0.0.0] - 2025-12-27

### Added
- Core Go LEAP client for Lutron Caseta / RA2 Select ([control-z3s]).
- Scaffolded Cobra CLI (`list`, `set`, `discover`) ([control-bm3]).
- Master "All Lights" control in CLI and TUI ([control-45p]).
- Lutron pairing scripts and certificate helpers ([control-unc]).

[Unreleased]: https://github.com/ghchinoy/homectl/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/ghchinoy/homectl/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/ghchinoy/homectl/compare/v0.0.1...v0.1.0
[0.0.1]: https://github.com/ghchinoy/homectl/compare/v0.0.0...v0.0.1
[0.0.0]: https://github.com/ghchinoy/homectl/releases/tag/v0.0.0
