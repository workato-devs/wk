# Changelog

All notable changes to `wk` will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Fixed

- `wk connectors list` now lists the workspace's custom SDK connectors. It was
  calling `GET /integrations`, which is a lookup of built-in platform connector
  metadata by name: with no `?applications=` it returns `{"items": []}` and HTTP
  200, so the command reported an empty workspace instead of failing, and with
  `--search` it returned the named *native* connector (e.g. Salesforce). It now
  calls `GET /custom_connectors` and decodes that endpoint's `result` envelope.
  `--search` filters client-side, case-insensitively, on name and title, rather
  than using `/custom_connectors/search`, which is case-sensitive and returns
  only the ten most recently released connectors. The table gains `ID` and
  `RELEASED` columns and drops `DESCRIPTION`, which this endpoint does not return;
  `--json` now includes `id`, `latest_released_version`, and `released_versions`.
- `wk workspace audit-log --json` now includes each entry's `resource` and
  `workspace` objects, as returned by `GET /api/activity_logs`. They were
  dropped while decoding the response because the struct didn't model them.
  The table is unchanged. ([#103](https://github.com/workato-devs/wk/issues/103))
- `wk auth login` no longer rejects a new profile because another profile's
  workspace has the same display name. The duplicate check compared workspace
  names from `GET /users/me`, and separate workspaces can share one (for
  example, several trial workspaces under one account), so the second
  workspace couldn't be added and `--force` didn't help. It now compares
  workspace IDs, which also catches the same workspace saved again after a
  rename. Profiles saved before workspace IDs were recorded are skipped by the
  check. When a duplicate is real, the error now gives the workspace ID and the
  two ways forward: use that profile, or `wk auth delete` it first.
  ([#100](https://github.com/workato-devs/wk/issues/100))
- `wk auth list` now honors `--store-type`. It silently discarded the flag
  and listed keychain profiles with exit 0, so `--store-type file` outside a
  project looked like "no file-store profiles" rather than "no project here".
  With no flag the list is unchanged (keychain profiles, plus `profiles.env`
  profiles inside a project). `--store-type keychain` lists only keychain
  profiles. `--store-type file` lists only `profiles.env` profiles, and outside
  a project or without a `profiles.env` it fails with the same error as every
  other command. ([#105](https://github.com/workato-devs/wk/issues/105))
- `wk auth login` no longer leaves a profile with no token behind when the
  token can't be stored in the OS keychain. The profile was saved first and
  kept even when the keychain write failed, so later commands found a profile
  but no credential. Now a new profile is removed and an overwritten one is
  restored. The error, previously just `storing credential: exit status 154`,
  now says that the keychain write failed, whether the profile was left
  unchanged or not saved, and that `--store-type file` keeps the token in a
  project file instead.
- `wk diff` no longer reports every JSON asset as `modified` right after a
  `wk clone` or `wk pull`. Pull writes JSON normalized (sorted keys,
  consistent indentation) and records that hash, but diff hashed the remote
  package's raw JSON, so the two never matched; non-JSON assets were
  unaffected. Diff now hashes remote files exactly as pull writes them,
  through one shared function so the two can't drift apart again. Regression
  from the #13 fix. ([#102](https://github.com/workato-devs/wk/issues/102))
- **`--json` changed shape for `wk diff`, `wk status`, `wk pull`, `wk clone`,
  `wk auth list` and `wk plugins list`.** These commands emitted the text
  table as JSON: each row keyed by its column header, holding display values.
  For example, `wk diff --json` returned `"STATUS"` and `"LOCAL HASH"` with
  hashes cut to 8 characters. They now emit their structs, which is what
  every other command's `--json` already did:
  - `diff`: `path`, `type`, `local_hash`, `remote_hash` (full SHA-256)
  - `status`: `file_path`, `status`, and `server_path` once the file has been
    synced (a new local file has no server path yet, so it's left out)
  - `pull`/`clone`: `file_path`, `action`, and `error`, which was previously
    dropped for per-file failures under `--skip-errors`
  - `auth list`: the profile's fields plus `active` and `shadowed`
  - `plugins list`: `name`, `version`, `dir`

  An empty result is now `[]` on stdout. Previously `diff`, `status`, `pull`,
  `clone` and `plugins list` wrote nothing to stdout under `--json` (only a
  note on stderr), so parsers got an empty string.

  With no profiles, `wk auth list` now prints `No auth profiles found. Add one
  with: wk auth login` instead of a table with only a header row.

  **Scripts that parse the old header keys need updating.** To stop this
  recurring, `Formatter.FormatList` now takes the data alongside the table
  rows, and its JSON mode encodes only the data.

## [1.0.3] - 2026-08-01

### Added

- `wk folders update <id> --name <new>` renames a folder or project, routing to
  `PUT /projects/{id}` or `PUT /folders/{id}` by `is_project` (mirroring
  `delete`). `wk folders list --projects` lists projects via `GET /projects`.
  Because a project is itself a folder (the top-level container), the list now
  reports each project's folder `id` as its constant identity — the id
  `update`/`delete` accept — with `project_id` as an additive field, rather than
  surfacing `GET /projects`' `id` (the project_id) as if it were the folder id.
  This holds for both the table (`FOLDER ID`/`PROJECT ID` columns) and `--json`,
  so an id copied from either path routes correctly instead of hitting
  `PUT /folders/{project_id}`. Completes coverage of the Projects & folders
  permission. ([#91](https://github.com/workato-devs/wk/issues/91))
- `wk folders update <id> --parent <new-parent-id>` reparents a plain folder,
  and can be combined with `--name` in the same call (`FolderService.Update`
  now takes an optional `*string` name and `*int` parentID, sending only the
  fields set). `PUT /folders/{id}` already accepted `parent_id` and performed
  the move; the client just never sent it. Projects are always top-level and
  reject `--parent` rather than silently no-oping.
  ([#97](https://github.com/workato-devs/wk/issues/97))
- `docs/known-limitations.md` — a permission-by-permission map of `wk` CLI
  coverage, organized to mirror the Client Role editor.
- Plugin commands and subcommands may declare an optional `renderer` JSON-RPC
  method for human-readable output. Text mode passes the canonical command
  result to the renderer in the same plugin process; `--json` continues to emit
  the primary result without invoking the renderer. ([#90](https://github.com/workato-devs/wk/issues/90))

### Changed

- `docs/command-reference.md` brought back to parity with the actual command
  tree. It was missing `mcp servers` entirely (create/update/delete/
  create-batch/get/list/token-renew/tools/policies/user-groups), `api clients`
  and `api clients keys`, `agentic skills`, `workspace properties`,
  `sync discover`, `recipes jobs get`/`retry`, `folders update`, and
  `auth token`. Added a **Command reference check** item to the PR template so
  command/flag surface changes come with a docs update in the same PR.
- ADR-003 (MCP Strategy) amended: the platform shipped the `mcp_servers`
  management API assumed unavailable when the ADR was written, so `wk mcp
  servers` now covers the full CRUD/token-renew/policies/tools/user-groups
  lifecycle the ADR had marked "Blocked". Also notes that the `token_renew`
  regression for `auth_type: hashed_token` / `*.apim.mcp.*` servers flagged in
  [#76](https://github.com/workato-devs/wk/issues/76) (token missing from
  `mcp_url`) has been reported fixed by the platform team; no CLI change was
  needed since `wk` only relays whatever `mcp_url` the API returns.

### Fixed

- `wk recipes start` no longer hangs for the full poll timeout when activation
  is blocked: `PUT /recipes/{id}/start` returns HTTP 200 with `success:false`
  and a `code_errors` payload for blocked activations, which was previously
  discarded. The command now decodes it and prints the step-level detail
  immediately instead of polling; bulk/`--no-wait` runs collect blocked
  recipes and continue with the rest of the batch. `wk recipes
  delete`/`update`/`move` had the same exposure — `DELETE`/`PUT
  /recipes/{id}` also return HTTP 200 for refusals (e.g. the recipe is
  running) — and previously proceeded to mutate local state (e.g. removing
  `.recipe.json` on a refused delete) as if the server call had succeeded.
  All four now surface the refusal and exit non-zero instead.
  ([#70](https://github.com/workato-devs/wk/issues/70))
- `wk recipes jobs get` no longer silently drops per-step diagnostics. `JobLine`
  now carries `input`, `output`, `error`, `error_descriptor`, and
  `error_details` (including the downstream `http_response` status/body/headers);
  `LineStat` gains `total`/`details` and `JobDetail` gains `error_parts`/
  `job_correlation_id`. `--json` surfaces the full payload and text output prints
  per-step errors. `ErrorDescriptor`/`ErrorParts` also carried JSON tags that
  didn't match the API's field names, so both silently decoded to their zero
  value; the tags now match the live response shape, and all four diagnostic
  structs are covered by a field-coverage test to catch future renames.
  ([#89](https://github.com/workato-devs/wk/issues/89))
- Plugin commands without a renderer now fall back to deterministic indented
  JSON instead of exposing nested values as Go `map[...]` syntax. Renderer
  failures warn and use the same fallback without changing the primary command
  result or exit code. ([#90](https://github.com/workato-devs/wk/issues/90))
- `ValidateLocalPath` now also rejects Windows-style rooted paths — a drive
  letter via `filepath.VolumeName`, or a leading `/` or `\` — that
  `filepath.IsAbs` doesn't catch on all platforms (e.g. `/tmp/evil` was
  previously accepted as "relative" on Windows).
  ([#88](https://github.com/workato-devs/wk/issues/88))

## [1.0.2] - 2026-07-08

### Fixed

- `wk plugins install <name>` now works on Windows when the plugin was installed
  via Scoop. Previously, `exec.LookPath` resolved the name to Scoop's `shims\`
  directory (a launcher stub, not the plugin root), causing a "no plugin.toml
  found" error. The fix reads the `.shim` sidecar file that Scoop writes
  alongside the stub to redirect to the real executable, then resolves the
  plugin root from there. ([#85](https://github.com/workato-devs/wk/issues/85))
- `wk plugins install <path>` on Windows no longer returns "Incorrect function"
  when given a directory that is an NTFS junction (e.g. Scoop's `current\`
  version alias). `filepath.WalkDir` treats junctions as regular files and
  failed when `copyDir` tried to `ReadFile` a directory handle. The fix resolves
  junctions via `os.Lstat` + `ModeIrregular` before copying. ([#85](https://github.com/workato-devs/wk/issues/85))
- `wk plugins install <name>` now walks up to 3 parent directories from the
  binary's location to find `plugin.toml`, covering layouts where the binary
  lives in a `bin/` subdirectory of the plugin root.

## [1.0.1] - 2026-07-01

### Changed

- All API and MCP HTTP requests now send a `wk-cli/<version>` User-Agent header
  instead of the hardcoded `wk/dev`. Build-time version info injected via
  goreleaser ldflags now flows through to the HTTP clients, making release
  traffic distinguishable in backend telemetry. The MCP client previously sent
  no User-Agent at all. The User-Agent token uses `wk-cli` rather than the bare
  `wk` to avoid collisions with unrelated tokens in nginx access logs.
  ([#83](https://github.com/workato-devs/wk/pull/83))

## [1.0.0] - 2026-06-28

First public release.

### Added

- MCP: full server lifecycle management — create, update, delete, start, stop,
  restart; `project_assets` tools for MCP server configuration
  ([#71](https://github.com/workato-devs/wk/pull/71),
  [#76](https://github.com/workato-devs/wk/pull/76))
- Recipes: `wk recipes move` to relocate a recipe to a different folder
  ([#77](https://github.com/workato-devs/wk/pull/77))
- Recipes: canonical pull/export with actionable start-timeout messaging
  ([#68](https://github.com/workato-devs/wk/pull/68),
  [#70](https://github.com/workato-devs/wk/pull/70))
- API Collections: `wk api collections delete`
  ([#80](https://github.com/workato-devs/wk/pull/80))
- Auth: `wk auth login` hardening — token masking, custom base URL support,
  improved region UX ([#79](https://github.com/workato-devs/wk/pull/79))

### Fixed

- Recipes: `wk recipes copy` now returns the new recipe ID (decoded
  `new_flow_id` from copy response)
- Recipes: `wk recipes copy` sends `folder_id` as a string as the API expects
- API Collections: `project_id` sent as a string to match API expectations
- MCP: quota policy `interval` constrained to the API's allowed value set
- MCP: corrected `server_policies` types and request shape

## [0.1.0-beta] - 2026-04-21

Initial beta release.

### Added

- Project lifecycle: init, link, clone, status, pull, push, diff
- Recipe management: list, get, start, stop, export, import, update, delete,
  jobs, copy, update-connection, validate, versions
- Connection management: list, get, create, update, delete, disconnect
- Folder management: list, create, delete
- Tag management: list, create, update, delete, apply, remove
- API Platform: collections (list, create), endpoints (list, enable, disable)
- MCP: test server connectivity, list tools
- Workspace: info, users, audit-log
- Connectors: list with search
- Sync entry management: add, list, refresh, remove
- Auth: keychain and file-based credential stores, multi-profile management
- Plugin system: install, list, remove, JSON-RPC dispatch, pre-push hooks
- `--json` output on all commands
- Workspace isolation check (prevents cross-workspace operations)
- CI/CD support with file-based credential store (profiles.env)
- Cross-platform binaries: linux/darwin/windows on amd64/arm64
