# Known limitations — `wk` coverage map

This maps what the `wk` CLI covers onto the Workato **Client Role editor**, so you
can size a Client Role for `wk` and see which platform capabilities aren't wired
yet. The structure below mirrors the editor exactly: the three tabs
(**Projects / Tools / Admin**), each section heading, and each permission row as
they appear in the UI.

## Legend

The status grades **`wk` CLI coverage**, not whether the Workato API exists:

- **✅** — `wk` wires everything the API exposes for this permission.
- **⚠️** — `wk` wires *some* operations but not all the API offers.
- **❌** — no `wk` command for it (**available-but-unwired**). This does **not**
  mean the platform API is missing — many ❌ rows have fully working APIs.
- **n/a** — not a capability `wk` is meant to cover (governed elsewhere).

A grade changes only when `wk`'s wiring changes, not when an endpoint is confirmed
to exist. Keep this file in sync when commands are added or removed.

---

## Projects

Access to core recipe-building features within projects.

### Project assets

| Permission | Status | `wk` commands | Notes |
|---|---|---|---|
| Projects & folders | ✅ | `wk folders list [--projects]/create/update/delete` | Create/list/update/delete for folders and projects + `GET /projects`; update/delete route by `is_project`. |
| Connections | ✅ | `wk connections list/get/create/update/delete/disconnect` | |
| Recipes | ✅ | `wk recipes list/get/start/stop/export/import/update/delete/move/copy/pull` | Full CRUD + lifecycle. |
| Genies | ❌ | — | No `wk` command. |
| Knowledge bases | ❌ | — | No `wk` command. |
| Skills | ✅ | `wk agentic skills list/get/create` | Covers the operations the Skills API exposes. |
| MCP servers | ✅ | `wk mcp servers/tools/test/user-groups` (+ policies) | Full management. Platform caveat: new servers default to `hashed_token` with no API-retrievable token (UI-reveal only) — a platform gap, not a CLI gap. |
| Recipe Versions | ✅ | `wk recipes versions list/get/comment` | |
| Jobs | ✅ | `wk recipes jobs list/get/retry` | `jobs get` now surfaces per-step `input`/`output`/`error`/`error_details` (issue #89). |
| Tag Assignments | ✅ | `wk tags assign` | Assign/remove tags on recipes and connections. |
| Test Cases | ❌ | — | No `wk` command. |

### Recipe lifecycle management

| Permission | Status | `wk` commands | Notes |
|---|---|---|---|
| Recipe lifecycle management | ✅ | `wk pull` / `wk push` | Folder-based export/import via the Packages (RLCM) API. |
| Export manifests | ✅ | `wk pull` / `wk push` | Used as part of the package export/import flow. |

### Project access

| Permission | Status | `wk` commands | Notes |
|---|---|---|---|
| Project Grants | ❌ | — | No `wk` command. |

---

## Tools

Access to data and features configured at the workspace level.

### Workspace data

| Permission | Status | `wk` commands | Notes |
|---|---|---|---|
| Lookup tables | ❌ | — | No `wk` command. |
| Data tables | ❌ | — | No `wk` command. |
| Data table records | ❌ | — | See Data tables. |
| Event Streams | ❌ | — | No `wk` command. |
| Event Streams topics | ❌ | — | No `wk` command. |
| Environment properties | ✅ | `wk workspace properties list/set` | |

### API platform

| Permission | Status | `wk` commands | Notes |
|---|---|---|---|
| Certificate bundles | ❌ | — | No `wk` command. |
| API portal | ❌ | — | No `wk` command. |
| Collections & endpoints | ⚠️ | `wk api collections list/create/delete`; `wk api endpoints list/create/enable/disable` | No get/update on either; no pull/push sync for workspace-level collections (**#62**, `blocked: platform-api`). |
| Clients & access profiles | ⚠️ | `wk api clients list/get/create/delete` (+ key create/refresh) | No client update; access profiles not wired. |

### Connector SDKs

| Permission | Status | `wk` commands | Notes |
|---|---|---|---|
| Connector SDKs | ⚠️ | `wk connectors list` | Read-only visibility into existing SDK connectors. Build/publish/versioning is handled by the separate **Workato Connector SDK** tool. |

### Custom OAuth profiles

| Permission | Status | `wk` commands | Notes |
|---|---|---|---|
| Custom OAuth profiles | ❌ | — | No `wk` command. |

### On-prem groups and agents

| Permission | Status | `wk` commands | Notes |
|---|---|---|---|
| On-prem groups | ❌ | — | No `wk` command. |
| On-prem agents | ❌ | — | No `wk` command. |

### Partner marketplace

| Permission | Status | `wk` commands | Notes |
|---|---|---|---|
| Connectors | ❌ | — | Marketplace connector endpoints; not used by `wk`. |

### Workato CLI

| Permission | Status | `wk` commands | Notes |
|---|---|---|---|
| Workato CLI | n/a | — | Not used by `wk`; enabling it is not required for the CLI. |

### Recipe building

| Permission | Status | `wk` commands | Notes |
|---|---|---|---|
| Recipe building | ❌ | — | Recipe-building/validation APIs. `wk lint`/`wk validate` run locally via the recipe-lint plugin and don't call this API. |

---

## Admin

Workspace administration.

### Workspace collaborators

| Permission | Status | `wk` commands | Notes |
|---|---|---|---|
| Collaborators | ❌ | — | No `wk` command. |
| Collaborator roles | ❌ | — | No `wk` command. |
| Collaborator groups | ❌ | — | No `wk` command. |
| Environment roles | ❌ | — | No `wk` command. |
| Project roles | ❌ | — | No `wk` command. |
| Migration of collaborator roles | ❌ | — | No `wk` command. |

### Workspace details

| Permission | Status | `wk` commands | Notes |
|---|---|---|---|
| Workspace details | ✅ | `wk workspace info` | Read (`GET /users/me`). |
| IAM | ❌ | — | No `wk` command. |

### Environment Management

| Permission | Status | `wk` commands | Notes |
|---|---|---|---|
| Secrets management | ❌ | — | Platform secrets-management API; not wired. (Distinct from the planned CLI-side external-secrets backends below.) |
| Audit Log | ✅ | `wk workspace audit-log` | |
| Tags | ✅ | `wk tags list/create/update/delete` | Tag CRUD. (Assigning tags to assets is **Tag Assignments** under Projects.) |

### Developer API clients

| Permission | Status | `wk` commands | Notes |
|---|---|---|---|
| API clients | ❌ | — | Managing the workspace's Workato REST API clients/tokens is unwired. (Distinct from **API platform → Clients & access profiles**.) |
| API client roles | ❌ | — | No `wk` command. |

---

## CLI-only gaps (not tied to a Client Role permission)

`wk` features that are planned or partial, independent of platform APIs:

- **Auth Tier 1** — external secrets-manager backends (Vault, AWS Secrets Manager, Doppler)
- **Auth Tier 4** — encrypted file-based credential store
- **`wk auth rotate`** — credential rotation
- **`wk migrate`** — automated migration from the legacy Python CLI
- **`--toml` output format** — `--json` and text tables are supported today
- **`--no-color`** — accepted on all commands but currently a no-op
