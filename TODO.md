# TODO: wk CLI v2 POC

## Current State (updated 2026-02-18)

**POC is code-complete.** All code items from the PRD's POC scope are implemented and verified against live `app.trial.workato.com`. 32 unit tests pass across 7 packages. Full push round-trip verified: pull → edit → push → re-pull → revert → re-pull.

### What works

- `wk version`, `wk --help` — full command tree including `clone` and dynamic plugin commands
- `wk auth login/status/switch/list` — keychain + env var auth, API connectivity test, formatted text output
- `wk init`, `wk link` — interactive and `--json` non-interactive modes
- `wk recipes list/get/start/stop/export/import` — live API verified, `--json` outputs native API field names/types
- `wk connections list/get` — `--application` client-side filter, `--json` outputs raw API objects
- `wk pull` — RLCM export manifest flow, polls until complete, extracts zip, writes meta sidecars
- `wk push` — builds zip, imports via RLCM, polls until complete. `--preserve-state` (default) wired to `restart_recipes=true`. `--dry-run` supported
- `wk diff` — compares local vs remote hashes
- `wk clone` — creates project dir, generates `wk.toml`, pulls all assets
- `wk status` — local-only, no panic when API client is nil
- `wk plugins install/list/remove` — full lifecycle, dynamic command registration
- `--json` flag — valid JSON on all commands, list commands emit raw API objects (lowercase keys, native types)
- `--verbose` flag — `[debug]` lines for profile, HTTP requests, response codes on stderr
- `WK_TOKEN`/`WK_REGION` env var auth — CI/CD mode verified
- Error handling — 401, 404, missing profile, invalid region all produce clean messages
- Region validation — dynamic list from `auth.ValidRegions()` including `trial`
- 32 unit tests passing (api, auth, config, output, plugin, sync)

### Remaining (non-code)

- **Homebrew tap** — create `workato-devs/homebrew-tap` repo on GitHub so GoReleaser can publish the formula. `.goreleaser.yaml` is already configured.
- **`--toml` output flag** — PRD specifies as cross-cutting flag. Low priority, deferred to Phase 1.

---

## Bugs Found & Fixed During Push Verification

| Bug | File | Fix |
|-----|------|-----|
| `ImportStatus` hit `/packages/import/{id}` (404) | `internal/api/packages.go:106` | Changed to `/packages/{id}` — same endpoint for export and import status |
| `restart_recipes` semantics inverted | `internal/sync/push.go:87` | `restartRecipes := preserveState` (not `!preserveState`). Workato's `restart_recipes=true` = "stop, import, restart" = state preservation |
| `--json` list output used display column names | `internal/commands/recipe.go`, `connection.go` | Branch on `flagJSON`: raw API objects via `Format()` for JSON, string rows via `FormatList()` for text |
| `--preserve-state` flag silently ignored | `internal/api/client.go`, `packages.go`, `push.go` | Added `restartRecipes bool` param to `Import()`, appended `?restart_recipes=%t` to URL |

---

## Completed Work

| Item | Description |
|------|-------------|
| Push round-trip | Pull → edit → push → re-pull verified live. Revert also verified |
| JSON list output | `recipes list --json` and `connections list --json` emit native API field names/types |
| `--preserve-state` | Wired to `restart_recipes` query param on RLCM import endpoint |
| Text output | `auth status`, `recipes get/import`, `connections get` format key-value pairs in text mode |
| RLCM export | `Export()` creates manifest via `POST /api/export_manifests`, then exports from manifest ID |
| RLCM import | `Import()` posts zip as `application/octet-stream`, polls `/packages/{id}` for completion |
| Plugin registration | `registerPluginCommands()` scans `~/.wk/plugins/`, loads manifests, registers Cobra commands |
| Auth connectivity | `auth status` makes `recipes.List(PerPage:1)` call, reports "API: connected" or error |
| Verbose logging | `[debug]` lines for profile/region/base_url resolution and HTTP request/response on stderr |
| Clone command | `wk clone <folder-name> [--workspace] [--local-path]` creates project + pulls. Verified live |
| Region validation | Uses `auth.ValidRegions()` dynamically |
| Recipe import | Verified end-to-end: export → import creates new recipe |
| Connections get | Implemented via list + filter (no single-get API endpoint exists) |
| Connection fields | `provider` → `application`, `connected` → `authorization_status`. Matches real API response |
| Connections test | Removed — no test endpoint in Workato API. Auth status visible via `authorization_status` field |
| ListResult[T] | Not dead code — used by recipes API (`{"items":[...]}`). Documented |

---

## E2E Test Results (2026-02-18)

Tested against `app.trial.workato.com` with region `trial`.

| # | Test | Result | Notes |
|---|------|--------|-------|
| 1.1 | `wk version` | PASS | Text and JSON both correct |
| 1.2 | `wk --help` tree | PASS | All commands listed including `clone` |
| 1.3 | `wk init` | PASS | Creates wk.toml, fails if exists, JSON mode works |
| 1.4 | `wk link` | PASS | Updates workspace, fails outside project |
| 1.5 | `wk status` (empty) | PASS | "No synced assets found." |
| 1.6 | Auth profiles | PASS | Login, status, switch, list all work |
| 1.7 | Plugin lifecycle | PASS | Install, list (text+JSON), remove |
| 1.8 | Error messages | PASS | Clean errors, no panics |
| 2.1 | Auth login | PASS | Profile saved |
| 2.2 | Auth status | PASS | Formatted text + JSON, "API: connected" |
| 2.3 | Multiple profiles | PASS | Switch works, active marker correct |
| 2.4 | Recipes list | PASS | Table output, JSON valid, filters work |
| 2.5 | Recipe get | PASS | Formatted key-value output |
| 2.6 | Recipe start/stop | PASS | Toggles state, verified via get |
| 2.7 | Recipe export | PASS | Valid JSON file |
| 2.7b | Recipe import | PASS | Creates recipe, formatted output |
| 2.8a | Connections list | PASS | Application + authorization_status fields populated |
| 2.8b | Connections get | PASS | Filters from list response, formatted output |
| 2.8c | Connections test | REMOVED | No test endpoint in Workato API |
| 2.9 | Sync pull | PASS | Export manifest flow works, 195 files pulled |
| 2.9b | Sync push | PASS | Pull → edit → push → re-pull round-trip confirmed |
| 2.9c | Push revert | PASS | Revert push → re-pull confirms original state restored |
| 2.10 | Env var auth | PASS | 100 recipes loaded |
| 2.11a | Bad token error | PASS | "API error 401: Unauthorized" |
| 2.11b | 404 error | PASS | "API error 404: Not found" |
| 2.11c | Bad profile | PASS | "profile not found" |
| 2.11d | Bad region | PASS | Lists all valid regions dynamically |
| extra | Dynamic plugin cmd | PASS | `wk hello` works after install |
| extra | Verbose logging | PASS | `[debug]` lines on stderr |
| extra | Clone command | PASS | `wk clone Test` creates project + pulls all recipes |
| extra | Sync diff | PASS | Shows same/different hashes for all files |
| extra | Sync status | PASS | Shows unchanged after pull, modified after edit |
| extra | Second pull | PASS | All files show "unchanged" on re-pull |
| extra | Application filter | PASS | `--application jira` filters connections correctly |
| extra | JSON field names | PASS | `recipes list --json` and `connections list --json` use lowercase keys, numeric types |

**Summary:** 33 PASS, 1 REMOVED

---

## Key Files Reference

```
cmd/wk/main.go                    Entry point, ldflags
internal/commands/root.go          RunContext, registerAllCommands, registerPluginCommands
internal/commands/resolve.go       resolveAPIClient() + verbose logging
internal/commands/auth.go          Auth subcommands + connectivity test
internal/commands/recipe.go        Recipe subcommands (JSON branches on list, text on get/import)
internal/commands/connection.go    Connection subcommands (JSON branches on list, --application filter)
internal/commands/sync.go          Pull/push/status/diff (--preserve-state flag)
internal/commands/clone.go         Clone command (init + pull shortcut)
internal/commands/plugin.go        Plugin install/list/remove
internal/api/http_client.go        HTTP transport, auth injection, verbose logging
internal/api/client.go             Service interfaces (PackageService.Import takes restartRecipes bool)
internal/api/recipes.go            RecipeService (Import has code/config stringification)
internal/api/connections.go        ConnectionService (get filters from list, no single-get API)
internal/api/folders.go            FolderService (bare array response)
internal/api/packages.go           PackageService (export manifest, octet-stream import, restart_recipes param)
internal/api/types.go              All API types (Connection uses application/authorization_status)
internal/auth/keyring.go           OS keychain store
internal/auth/env.go               WK_TOKEN/WK_REGION env var store
internal/auth/profile.go           ProfileManager (~/.wk/profiles.json)
internal/sync/engine.go            SyncEngine (nil-safe client)
internal/sync/helpers.go           resolveFolderID (handles "All projects"), waitForPackage/Import
internal/sync/pull.go              Pull: export → poll → download → extract zip
internal/sync/push.go              Push: status → build zip → import → poll (preserveState → restartRecipes)
internal/sync/meta.go              .wk-meta.json sidecar handling
internal/plugin/host.go            PluginHost (JSON-RPC process management)
internal/plugin/rpc.go             JSON-RPC 2.0 client over stdio
internal/plugin/registry.go        Plugin install/list/remove at ~/.wk/plugins/
plugins/example/main.go            Example plugin binary
```

## Test Files

```
internal/api/http_client_test.go   HTTP client: 200, 401, 404, 500, Import restart_recipes param
internal/auth/auth_test.go         Region validation, profile manager, env store
internal/config/config_test.go     Load/save, project root, validation
internal/output/formatter_test.go  JSON/text formatting, struct slice preserves json tags + types
internal/plugin/manifest_test.go   TOML parsing
internal/plugin/registry_test.go   Install, list, remove, edge cases
internal/sync/helpers_test.go      Folder resolution with "All projects" handling
internal/sync/meta_test.go         Hash, read/write meta, find meta files
internal/sync/status_test.go       Unchanged, modified, new file detection
```
