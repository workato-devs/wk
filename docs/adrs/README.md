# Architecture Decision Records

Architectural decisions for the `wk` CLI. These are **living records** — many were written as a
hypothesis ahead of implementation and carry dated amendments showing how the decision evolved.
Treat an ADR's claims as something to verify against the code, not as settled truth, and amend an
ADR in the same change that contradicts it. How we use ADRs — header schema, in-place dated
amendments, and authorship (`Author(s)` vs `Amended-by`) — is itself an ADR:
[ADR-000](ADR-000-how-we-use-adrs.md).

Start a new ADR by copying [`ADR-TEMPLATE.md`](ADR-TEMPLATE.md) to `ADR-NNN-kebab-case-title.md`
(next number), and add a row here.

| ADR | Title | Status | Summary |
|-----|-------|--------|---------|
| [000](ADR-000-how-we-use-adrs.md) | How We Use ADRs | Accepted | The ADR convention: living hypotheses, in-place dated amendments, header schema, point-in-time authorship (`Author(s)` vs `Amended-by`). |
| [001](ADR-001-wk-cli-v2-foundational-architecture.md) | Workato CLI v2 (`wk`) — Foundational Architecture | Proposed | Greenfield rewrite: language, command model, distribution, and the `wk` name. *(Amended August 2026: `--raw` added to the cross-cutting flags; reasoning in ADR-009.)* |
| [002](ADR-002-sync-engine.md) | Sync Engine — RLCM-Based Pull/Push with Sidecar Metadata | Accepted | Pull/push model with `.meta.json` sidecars powering `status` and `diff`. *(Amended August 2026: manifest `AssetType`, package payload, and sync-tracked metadata are three separate layers — don't infer one from another.)* |
| [003](ADR-003-mcp-strategy.md) | MCP Strategy — No Auto-Delegation, Protocol-Level Tooling Only | Accepted | MCP as protocol-level tooling; no automatic agent delegation. *(Amended July 2026: `mcp_servers` management API shipped. Amended August 2026: a third server type — `airo_mcp` — is not a REST passthrough; delegation stays cut on safety grounds.)* |
| [004](ADR-004-plugin-repo-structure.md) | Plugin Repo Structure — Separate Repos, Not a Monorepo | Accepted | Plugins are independent repos; JSON-RPC over stdio is the only contract. *(Amended June–July 2026: repos renamed; install resolution hardened; optional plugin-owned text renderers added. Amended October 2026: optional `files` manifest key scopes what install copies.)* |
| [005](ADR-005-project-scaffolding.md) | Project Scaffolding — Container Folder, `.wk/`, Ignore Semantics | Accepted | `wk init` scaffolding, `.wk/` state dir, folder-ID caching, `.wkignore`. *(Partially superseded by ADR-007.)* |
| [006](ADR-006-profile-identity-model.md) | Profile Identity Model — Workspace & Environment Metadata | Proposed | Profile = name + workspace + environment + region; introspected login. *(Amended October 2026: uniqueness compares workspace ID, not display name; `auth list` honors `--store-type`.)* |
| [007](ADR-007-greenfield-onboarding.md) | Greenfield Project Onboarding — Sync Entry Lifecycle | Accepted | `wk sync` command group and push-create-on-demand for new projects. |
| [008](ADR-008-airo-integration.md) | AIRO Integration — Out of the Data Path, Explicit Invocation Only, `wk mcp connect` Owns Setup | Accepted | No auto-delegation and no `wk airo` commands: agents reach `airo_mcp` directly, and builder protocols aren't wrapped because their only viable driver is an LLM. `--bearer` on `wk mcp tools`/`test` for https Workato hosts. `wk mcp connect` is client-neutral by default, installs into Claude Code and Codex only through their own CLIs (Cursor print-only), defaults to OAuth (the signed-in user) with token auth (the workspace owner) opt-in and never a fallback, and never writes a credential to a config file except a managed server's token URL on explicit request. |
| [009](ADR-009-agent-studio-dx-completeness.md) | Agent Studio DX Completeness — Resource Coverage, Cursor Pagination, and Syncability | Accepted | `wk agentic` covers genies, knowledge bases, genie clients, logs and skills over REST (guardrails read-only); cursor pagination as a first-class second paginator; write commands verify outcomes rather than trust the API (reject ignored input, re-read, reconcile assignments, guard deletes of in-use resources); genies/KBs have no RLCM path; `--json` vs `--raw` output contract. |

**Status vocabulary:** `Proposed` (drafted) · `Accepted` (agreed, in effect) · `Superseded`
(replaced by a later ADR). Implementation is tracked separately via an optional `Implemented:`
date in each ADR's header.
