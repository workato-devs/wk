# AGENTS.md

This file provides guidance to coding agents (and humans) working in this repository. It is the
canonical instructions file; vendor-specific files such as `CLAUDE.md` import it.

## What This Is

`wk` is Workato's developer CLI — a single statically-compiled Go binary with no runtime
dependencies. It covers workspace operations (`pull`, `push`, `diff`, `status`, `sync`),
profile/auth management, and MCP protocol tooling, and it hosts plugins (such as `recipe-lint`)
over JSON-RPC on stdio.

## Commands

```bash
go build ./...      # Build
go test ./...       # Run all tests
```

See `CONTRIBUTING.md` for the API-resource lifecycle contracts: coverage tests in `internal/api/`
and `internal/sync/` enforce that struct fields, list-table columns, and the `.meta.json` sidecar
stay in sync. Read it before adding a resource type, adding a field, or teaching `wk pull` a new
export-file extension — a failing coverage test names exactly what to fix.

## Architecture

The authoritative architecture lives in the ADRs (`docs/adrs/`, index: `docs/adrs/README.md`). Treat each
as a living hypothesis to verify against the code (see Decision Records below), not as ground truth.

- **ADR-001** — foundational architecture: Go, Cobra/Viper, TOML config, single static binary,
  JSON-RPC-over-stdio plugins.
- **ADR-002** — sync engine: pull/push with `.meta.json` sidecars powering `status` and `diff`.
- **ADR-003** — MCP strategy: protocol-level tooling only, no auto-delegation.
- **ADR-004** — plugins are separate repos; JSON-RPC over stdio is the only contract.
- **ADR-005 / ADR-007** — project scaffolding (`.wk/`, ignore semantics) and greenfield onboarding
  (the `wk sync` entry lifecycle, push-create-on-demand).
- **ADR-006** — profile identity model: name + workspace + environment + region.

## Decision Records (ADRs)

Architectural decisions live in `docs/adrs/` as `ADR-NNN-*.md` (index: `docs/adrs/README.md`). They are
**living records, not settled truth** — many were written as a hypothesis ahead of implementation.

- **Verify before relying.** Treat an ADR's claims as a hypothesis to check against the current
  code, not as ground truth — even one marked `Accepted`.
- **Amend in the same change.** If your change contradicts what an ADR says, amend that ADR in
  place — a dated `> **Amendment (Month Year): …**` blockquote that preserves the original text —
  as part of the same PR. The PR template carries a checkbox for this. Don't silently let the
  record drift.
- **Attribution is point-in-time.** `Author(s)` is frozen to who made the *original* decision; if
  you join by amending, add yourself to `Amended-by` (with your `role`/`harness`/`model` and the
  date), never to `Author(s)`.

See `docs/adrs/ADR-000-how-we-use-adrs.md` for the full convention (status vocabulary, when to amend
vs. write a new ADR, header schema, authorship).
