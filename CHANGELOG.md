# Changelog

All notable changes to `wk` will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- `wk mcp connect airo` sets up any MCP client's connection to Workato AIRO
  MCP. `wk` isn't involved in any request after that, and never writes a
  credential into a client's config.
  - Without `--client`, it prints the connection details in a client-neutral
    form any MCP client can use: the server URL, how each authentication
    option works and who it acts as, which workspace to sign in to, and the
    commands that supply a token. `--json` gives the same structure.
  - `--client claude-code` and `--client codex` install the server through
    those clients' own CLIs (`claude mcp add-json`, `codex mcp add`).
    `--client cursor` prints the exact `~/.cursor/mcp.json` (or project
    `.cursor/mcp.json`) entry instead of installing it, because `cursor
    --add-mcp` writes a settings file Cursor's agent doesn't read servers
    from. An unknown client gets the neutral details. `--print` shows a
    client's config without installing it; `--force` replaces an existing
    server. `--config-scope` (`local`, `user`, `project`) is where the client
    saves the entry on your machine; `project` is meant to be committed.
  - `--auth oauth` (the default) writes only the server URL. The client
    registers itself, you authorize in the browser, and it then acts as the
    user who authorized. OAuth connects to whichever workspace the browser is
    signed in to, so `wk` names the profile's workspace and environment to
    sign in to first. `wk` checks whether the region's authorization server
    supports client registration. Developer Sandbox (trial) doesn't, so there
    `wk` refuses OAuth and points to `--auth token` rather than writing a
    config that can't sign in. **The OAuth path follows Workato's documented
    setup but hasn't been verified end to end by the `wk` team**, since no
    production workspace was available; reports are welcome.
  - `--auth token` gives the client the profile's API token without copying
    it: Claude Code runs `wk mcp auth-header` to get it, and Codex and Cursor
    read it from an environment variable (default `WORKATO_API_TOKEN`) you set
    from `wk auth token`. A token acts as the **workspace owner**, unrestricted
    by role or project access, so it's opt-in only and never a fallback. Claude
    Code runs the header helper only after you've accepted its trust prompt for
    the folder, so open Claude Code there once before headless use.
  - `wk mcp connect <mcps-handle>` does the same for a managed MCP server, and
    the server's authentication method decides how the client signs in.
    `workato_idp` servers get a URL-only entry, and you sign in as your Workato
    Identity user. `token` servers have the credential in their URL, so `wk`
    writes it into a client config only with `--allow-url-token`, never at
    `project` config scope, and otherwise shows the token as `REDACTED`.
    `hashed_token` servers (the default for new ones) can't be configured
    automatically, because Workato doesn't return their token, so `--print`
    gives the entry with a placeholder for the URL from the Workato UI.
- `wk mcp auth-header` prints `{"Authorization":"Bearer <token>"}` for a
  profile, for MCP clients that run a command to get request headers.
- `wk mcp tools` and `wk mcp test` take `--bearer`, which sends the active
  profile's API token as a bearer header, so they can reach first-party
  Workato MCP endpoints such as AIRO MCP (`https://app.<region>.workato.com/airo_mcp`).
  It's sent only to https Workato hosts; any other URL is refused before the
  token is read.
- `wk agentic genies list` and `wk agentic genies get <id>` list and show
  Agent Studio genies. `list` takes `--folder-id`, `--project-id`, `--state`,
  `--page` and `--per-page`, and reports whether more pages exist from the
  API's `total`. `get` adds the fields only the detail endpoint returns
  (project, folders, user groups, model label, provider). The API reports
  `state` differently per endpoint (`started`/`stopped` from list,
  `active`/`inactive` from get) and `wk` shows each as reported. The `--state`
  filter takes `active` or `inactive`; the API silently ignores any other
  value and returns every genie, so `wk` rejects it.
- Agent Studio read commands, with `--json` output that matches what the API
  sends:
  - `wk agentic kb list|get|recipes|data-sources`: knowledge bases. `list`
    takes `--folder-id`, `--project-id`, `--query`, `--page`, `--per-page`.
  - `wk agentic clients list`: genie Headless API clients.
  - `wk agentic guardrails get <genie-id>|pii-entities`: guardrail policies
    (beta; read only). A `404` explains it can mean guardrails aren't enabled
    in the workspace.
  - `wk agentic logs conversations <genie-id>`, `logs conversation <genie-id>
    <conversation-id>` and `logs events <genie-id> <conversation-id>`: what a
    genie actually did. These are cursor-paginated: `--cursor`/`--limit`, and
    `--json` wraps results as `{"items", "next_cursor", "has_next"}`. `events`
    filters with `--event-type` (repeatable) and `--run-id`.

  Timestamps are RFC 3339 in UTC whichever format the API used (epoch
  milliseconds for conversations, ISO 8601 elsewhere). A `0` timestamp, such
  as `last_message_at` on a conversation with no messages, is `null`.
- Genie write commands: `wk agentic genies create|update|delete|start|stop`,
  and `assign-`/`remove-` `skills`, `knowledge-bases` and `user-groups`.
  `create` and `update` print the genie as the API then reports it (`update`
  sends only the fields you pass). `start` and `stop` report the state the
  genie ends up in and fail if it isn't the expected one. **Assignments are
  checked, not trusted:** the API answers `200` even when it ignored an ID
  (unknown knowledge base and user group IDs are dropped silently), so `wk`
  reads each ID before and after and reports an outcome per ID: `assigned`,
  `already_assigned`, `removed`, `not_assigned`, `not_found` or `not_applied`.
  It exits non-zero if any ID was `not_found` or `not_applied`.
- Knowledge base write commands: `wk agentic kb create|update|delete`.
  `create` takes `--name` and `--folder-id` (required), `--description` and
  `--source-type` (`recipe`, the default, or `matrix`). `update` changes only
  `--name` and `--description`, the only fields the API lets you change, and
  re-reads the knowledge base to confirm the change took. `wk` rejects input
  the API would accept and then ignore: any other `--source-type` (the API
  quietly makes it `recipe`), and a blank name or description (the API answers
  `200` and changes nothing, so a description can't be cleared). **`delete`
  refuses a knowledge base that genies use**, naming them, because the API
  deletes it anyway and detaches it from running genies; `--force` deletes it
  and `--json` lists the genies it was detached from. Data sources for Matrix
  knowledge bases can't be set: the API accepts them but didn't apply any we
  tried.
- Genie client commands: `wk agentic clients get|create|update|delete|
  regenerate|attach|detach`. A client serves at most one genie, and a genie
  can have several clients (Workato's reference says the reverse).
  - `create` takes `--name`, `--type` (`api_key`, the default, or `oauth` with
    `--redirect-url`) and `--ip-allow-list`. `create` and `regenerate` print an
    `api_key` client's key, which the API returns only then; `regenerate`
    makes the old key stop working at once.
  - `update` changes `--name`, `--ip-allow-list` (replaces the list;
    `--clear-ip-allow-list` removes every restriction) and, for OAuth clients,
    `--redirect-url`, then re-reads the client to confirm the change took.
  - `get` adds the OAuth client ID and redirect URLs that `list` leaves out.
  - `attach` reports `already_attached` (exit 0) for a client already on that
    genie. A client on another genie gets an error naming that genie and the
    `detach` command, in place of the API's bare `409`.
  - **`delete` refuses a client that's attached to a genie**, because the API
    deletes it anyway and its credentials stop working while the genie runs;
    `--force` deletes it, and `detach` keeps it.
- `wk agentic genies delete --help` now says that deleting a genie also
  deletes the clients attached to it.
- A global `--raw` flag prints the Workato API's response body verbatim
  instead of `wk`'s JSON. `--json` is `wk`'s modeled output and includes only
  the fields `wk` models (`wk connections list --json` shows 8 fields per
  connection; `--raw` shows the 17 the API sends). `--raw` implies `--json`. A
  command that makes several API requests prints one response per line, in
  order, and one that makes none falls back to `wk`'s JSON with a note on
  stderr. Binary downloads are never printed.

### Changed

- API errors now include Workato's `reason` and what to do about it. A `401`
  with `missing_privilege` says the API client role lacks the permission and
  that the token is fine; `authentication_failed` points to the profile and
  `wk auth login`. Both used to read only `API error 401: Unauthorized`.
- MCP connection errors now include the server's response body and its
  `WWW-Authenticate` challenge. A `401` from a Workato endpoint that wants a
  bearer token says to rerun with `--bearer`, and a rejected token names the
  profile. The MCP client also sends `notifications/initialized` after the
  handshake, as the MCP spec requires, and `wk mcp tools --json` keeps each
  tool's `_meta`.

### Fixed

- **`wk agentic skills --json` changed shape.** It now matches what the API
  returns. It gains `type`, `folders`, `genies`, `created_at` and
  `updated_at`, and loses `recipe_id` and `description`, which the API never
  sends (`recipe_id` was a copy of `provider_id`). The table's `RECIPE ID`
  column still shows the recipe, from `provider_id`. `skills list` now decides
  whether there's another page from the API's `total`, not from whether the
  page came back full.
- Agent Studio API errors now show their message: `API error 400: Data
  sources are only available for Matrix knowledge bases` rather than the raw
  `{"errors":[…]}` body. When the body also has a `detail`, it's included
  too: `Create knowledge base failed: Name has already been taken`, not just
  the generic title.
- `wk mcp servers token-renew --help` no longer says the new URL is printed in
  every case. For `hashed_token` servers (the default for new servers), Workato
  doesn't return the new token: the printed URL has none and won't connect,
  while the old token has already stopped working. The help now says so and
  points to the Workato UI.
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
- `wk plugins install <dir>` can now install only what a plugin needs instead
  of the whole directory. Run from a source checkout, it copied the entire
  repo into `~/.wk/plugins/` (852 files, `.git/` included, for `recipe-lint`).
  `plugin.toml` takes a new optional `files` list, e.g.
  `files = ["profiles/*.json"]`. When it's present, install copies
  `plugin.toml`, the entrypoint and those files. A pattern that matches nothing,
  reaches outside the plugin directory, or a missing entrypoint is an error,
  and the existing installation is left in place. When it's absent, install
  copies the whole directory as before and warns, so existing plugins are
  unaffected. Older `wk` versions ignore the key.
  `wk plugins install --json` adds `copied_all`.
  ([#101](https://github.com/workato-devs/wk/issues/101))
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
