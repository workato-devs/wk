# Changelog

All notable changes to `wk` will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- Plugin commands and subcommands may declare an optional `renderer` JSON-RPC
  method for human-readable output. Text mode passes the canonical command
  result to the renderer in the same plugin process; `--json` continues to emit
  the primary result without invoking the renderer. ([#90](https://github.com/workato-devs/wk/issues/90))

### Fixed

- Plugin commands without a renderer now fall back to deterministic indented
  JSON instead of exposing nested values as Go `map[...]` syntax. Renderer
  failures warn and use the same fallback without changing the primary command
  result or exit code. ([#90](https://github.com/workato-devs/wk/issues/90))

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
