# Changelog

All notable changes to `wk` will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Changed

- All API and MCP HTTP requests now send a `wk-cli/<version>` User-Agent header
  instead of the hardcoded `wk/dev`. Build-time version info injected via
  goreleaser ldflags now flows through to the HTTP clients, making release
  traffic distinguishable in backend telemetry. The MCP client previously sent
  no User-Agent at all. ([#83](https://github.com/workato-devs/wk/pull/83))

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
