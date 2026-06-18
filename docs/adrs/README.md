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
| [001](ADR-001-wk-cli-v2-foundational-architecture.md) | Workato CLI v2 (`wk`) — Foundational Architecture | Proposed | Greenfield rewrite: language, command model, distribution, and the `wk` name. |
| [002](ADR-002-sync-engine.md) | Sync Engine — RLCM-Based Pull/Push with Sidecar Metadata | Accepted | Pull/push model with `.meta.json` sidecars powering `status` and `diff`. |
| [003](ADR-003-mcp-strategy.md) | MCP Strategy — No Auto-Delegation, Protocol-Level Tooling Only | Accepted | MCP as protocol-level tooling; no automatic agent delegation. |
| [004](ADR-004-plugin-repo-structure.md) | Plugin Repo Structure — Separate Repos, Not a Monorepo | Accepted | Plugins are independent repos; JSON-RPC over stdio is the only contract. *(Amended June 2026: repos renamed.)* |
| [005](ADR-005-project-scaffolding.md) | Project Scaffolding — Container Folder, `.wk/`, Ignore Semantics | Accepted | `wk init` scaffolding, `.wk/` state dir, folder-ID caching, `.wkignore`. *(Partially superseded by ADR-007.)* |
| [006](ADR-006-profile-identity-model.md) | Profile Identity Model — Workspace & Environment Metadata | Proposed | Profile = name + workspace + environment + region; introspected login. |
| [007](ADR-007-greenfield-onboarding.md) | Greenfield Project Onboarding — Sync Entry Lifecycle | Accepted | `wk sync` command group and push-create-on-demand for new projects. |

**Status vocabulary:** `Proposed` (drafted) · `Accepted` (agreed, in effect) · `Superseded`
(replaced by a later ADR). Implementation is tracked separately via an optional `Implemented:`
date in each ADR's header.
