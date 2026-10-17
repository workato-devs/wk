# AGENTS.md

This file provides guidance to coding agents (and humans) working in this repository. It is the
canonical instructions file.

## What This Is

`wk` is Workato's developer CLI — a single statically-compiled Go binary with no runtime
dependencies. It covers workspace operations (`pull`, `push`, `diff`, `status`, `sync`),
profile/auth management, and MCP protocol tooling, and it hosts plugins (such as `recipe-lint`)
over JSON-RPC on stdio.

## Commands

```bash
make build                 # Build bin/wk; the committed ./wk at the repo root is usually stale
go test ./...              # Run all tests
GOOS=windows go vet ./...  # Cross-platform check (also GOOS=linux)
```

**No CI runs the tests**; the workflows only build releases. Run them yourself before opening a PR.

## Guiding Principles

`wk` is a Workato Labs project, and the [Labs guiding
principles](https://workato-devs.github.io/labs/principles.html) govern design decisions here. They
are framed as **"not aspirations — operating rules,"** so treat them as constraints on what you
build, not as values to nod at.

**Read the principle, not the label.** The published page numbers them and gives each a one-word
heading — *Agent-first, Open, Craft, Contribute, Velocity*. Those headings are page design: they are
navigation labels, not the principles. "Craft" on its own asserts nothing and cannot be applied to a
decision. The principle is the statement underneath it. In full:

| # | Statement — *this* is the principle |
|---|---|
| 01 | **Built for agent-assisted development.** AI coding agents are becoming primary collaborators — not just autocomplete, but partners that read documentation, generate code, validate output, and iterate. |
| 02 | **Open by default.** Build on open standards so developers can use whatever tools they already have — any AI agent, any IDE, any OS. No lock-in to a specific vendor. |
| 03 | **Developer experience is a first-class feature.** Tools should feel pleasant to use from day one: one-command installation, clear documentation. |
| 04 | **Easy to contribute — for everyone.** The contribution surface is as deliberately designed as the user-facing experience. |
| 05 | **Ship fast, learn in the open.** Release early against real usage signal. No SLAs, no heavyweight release gates. |

When citing one — in an ADR, a PR, or a code comment — quote or paraphrase the statement so the
reasoning is legible. "Per principle 03" tells a reader nothing; "developer experience is a
first-class feature, so the failure mode must be a clear error rather than a silent no-op" is an
argument they can check or dispute.

Two consequences that come up constantly in this repo, both from 01:

- **The caller is probably an agent, not a person typing.** Design for stated intent over inferred
  intent, and never make "what the user typed" the test.
- **Error text is an API.** An agent's only route to self-correction is the text `wk` returns, and
  it will follow bad remediation advice literally. Wrong remediation text is a correctness bug.

## Architecture

The authoritative architecture lives in the ADRs (`docs/adrs/`, index: `docs/adrs/README.md`). Treat each
as a living hypothesis to verify against the code (see Decision Records below), not as ground truth.

- **ADR-001** — foundational architecture: Go, Cobra/Viper, TOML config, single static binary,
  JSON-RPC-over-stdio plugins.
- **ADR-002** — sync engine: pull/push with `.meta.json` sidecars powering `status` and `diff`.
- **ADR-003** — MCP strategy: protocol-level tooling only, no auto-delegation. Three server types;
  `airo_mcp` is *not* a REST passthrough, but delegation stays cut on safety grounds.
- **ADR-004** — plugins are separate repos; JSON-RPC over stdio is the only contract.
- **ADR-005 / ADR-007** — project scaffolding (`.wk/`, ignore semantics) and greenfield onboarding
  (the `wk sync` entry lifecycle, push-create-on-demand).
- **ADR-006** — profile identity model: name + workspace + environment + region.
- **ADR-008** — AIRO: `wk` never routes commands through AIRO and has no `wk airo` commands, so
  it's entirely out of the MCP *data* path (agents talk to `airo_mcp` directly);
  `recipe_builder_*`/`blueprint_*` aren't wrapped because their only viable driver is an LLM. `wk` owns the
  *setup* path — `wk mcp connect` wires a client to AIRO with no credential on disk, OAuth by
  default (acts as the signed-in user); token auth (acts as the workspace owner) is opt-in only.
  It must work with any MCP client: client-neutral details by default, and installs only through a
  client's own CLI, never by editing its config files.
- **ADR-009** — Agent Studio DX: `wk agentic` covers genies, knowledge bases, clients, logs and
  skills over REST; cursor pagination is a distinct paginator from offset; write commands verify
  outcomes instead of trusting the API (it silently ignores unknown IDs and some fields, and
  deletes in-use resources); syncability is an RLCM question with three layers (manifest asset
  type vs. package payload vs. `wk` sidecar handling — don't infer one from another); `--json` is
  `wk`'s modeled contract while `--raw` is the API verbatim.

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

### Implementing from an ADR

An ADR records a *decision*, not a build plan. If it has an **Action Items** section, that's the
sequencing; otherwise its Decision section is the spec.

- **Check the claims first.** An ADR written ahead of implementation may describe an API that has
  since changed. Its "What we'll need to revisit" section lists what was left unverified — read it
  before building on a shape. Absence of evidence in an ADR is not evidence: a 404 or 401 recorded
  during probing means *not found on the paths tried*, not *does not exist*.

See `docs/adrs/ADR-000-how-we-use-adrs.md` for the full convention (status vocabulary, when to amend
vs. write a new ADR, header schema, authorship).

## Adding or changing a command

- **Where code goes.** API surface in `internal/api/<resource>.go` (one service per resource,
  registered on the client in `client.go`); commands in `internal/commands/<resource>.go`. Shared
  response/request types live in `internal/api/types.go`.
- **Patterns to copy.** `internal/api/mcp_servers.go` + `internal/commands/mcp_servers.go` are the
  fullest CRUD example — sub-resources, `{"data": …}` unwrapping, table + `--json` output. For
  writes against an API that can report success without making the change, copy
  `internal/commands/agentic_*_write.go`: reject input the API would ignore, re-read after the
  write, and refuse to delete what something depends on without `--force` (ADR-009 Decision 5).
- **Read `CONTRIBUTING.md` before adding a resource or a field.** Coverage tests in `internal/api/`
  and `internal/sync/` enforce that struct fields, list-table columns, and `.meta.json` sidecar keys
  stay in sync; a failing one names exactly what to fix. They check `wk`'s internal consistency, not
  fidelity to the API — a struct can pass while silently dropping fields the API returned.
- **`--json` must match the wire.** Fields the API sometimes omits are optional in the struct
  (`omitempty`, pointers), never zero-filled. Compare a new command's `--json` with its `--raw`.
- **Update the docs in the same change:** `CHANGELOG.md`, `docs/command-reference.md` (a test
  fails if a command or global flag is missing from it, or listed but gone), and
  `docs/known-limitations.md` when coverage of a Client Role permission changes.

## Working against a live Workato workspace

- **A token writes into whatever workspace it belongs to**, whoever owns it (and on AIRO MCP it acts
  as the workspace owner, unlimited by its role). Write only in a workspace you own, and confirm
  that by the `email` field of `wk workspace info --json`, not by profile or workspace name.
- **Clean up after yourself.** Name scratch assets distinctively (`wk-probe-*`) and delete them
  when done. Some can't be deleted through the API (skills, for one), so check before creating.
- **Never print or commit a credential.** Pass tokens through a variable
  (`TOKEN=$(wk auth token --profile <name>)`), and redact secrets in anything you save.
- **The Client Role editor is the most complete map of the API.** Workspace admin → API clients →
  Client roles lists every endpoint under each permission, including some the API reference
  doesn't document.
- **Keep scratch work in `tmp/`.** It's gitignored, so it's the place for captured responses,
  probe scripts and notes, and nothing there reaches the repo. That also means nobody else can see
  it: anything another contributor needs belongs in an ADR, an issue or the PR.
