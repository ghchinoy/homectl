---
title: Developer Guide & Beads Workflow
description: Contributing guidelines, quality gates, and Beads issue management.
---

This guide outlines standards and procedures for contributing to `homectl`.

## Tooling & Quality Gates

Before submitting changes, all code must pass repository quality gates:

```bash
# 1. Format Go code
gofmt -s -w .

# 2. Run static analysis
go vet ./...

# 3. Execute unit and integration tests
go test -v ./...

# 4. Verify TypeScript and Web UI build
npm --prefix ui run build

# 5. Verify Documentation site build
npm --prefix docs run build
```

---

## Beads (`bd`) Issue Tracker

`homectl` uses **[Beads (`bd`)](https://github.com/gastownhall/beads)** for decentralized issue tracking directly inside the repository.

### Quick Reference

```bash
# View all available tasks ready for work
bd ready

# View details for a specific task
bd show <id>

# Claim a task
bd update <id> --status in_progress

# Close a completed task
bd close <id>
```

Issues are stored in Dolt version-controlled database tables under `.beads/` and passively exported to `.beads/issues.jsonl`.

---

## Coding Standards

1. **Go Readability:**
   - Adhere strictly to the Google Go Style Guide.
   - Use `RunE: func(cmd *cobra.Command, args []string) error` for all Cobra commands. Return wrapped errors (`%w`) instead of calling `log.Fatalf`.
   - Never use `Get` prefixes for network/expensive operations; use `Fetch` or descriptive verbs.
   - Capitalize acronyms (`AppID`, `DeviceID`, `RinconID`).
   - Add package doc comments (`// Package <name> provides ...`) to all packages.
2. **Web UI Standards:**
   - Keep Lit components atomic and isolated in `ui/src/components/`.
   - Use `import type` when importing TypeScript interfaces under `verbatimModuleSyntax`.
   - All network calls must pass through `ui/src/api.ts`.
3. **Commit Messages:**
   - Follow [Conventional Commits](https://www.conventionalcommits.org/):
     * `feat: add lifx device discovery`
     * `fix: prevent camera mDNS context cancellation race`
     * `docs: update network topology reference`
