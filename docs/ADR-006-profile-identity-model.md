# ADR-006: Profile Identity Model — Workspace & Environment Metadata

**Status:** Proposed
**Date:** April 9, 2026
**Authors:** Zayne Turner, Chris Miller
**Deciders:** DevRel Engineering, Platform CLI Team
**References:** ADR-001 (Decision 4: Project Model, Decision 5: Credential Storage)

---

## Context

The original profile model (ADR-001) used a single `--name` flag as both the profile identifier and the implicit workspace reference. The `Profile` struct contained `name`, `region`, `store_type`, and `base_url`. The `wk.toml` project config had a `workspace` field that stored a profile name, and `checkWorkspaceMatch` compared the active profile name against it.

This conflated three distinct concepts:

1. **Identity** — how the CLI looks up a profile (the `name` field)
2. **Targeting** — which Workato account and environment the profile points at
3. **Project binding** — which profile a project is pinned to

As multi-workspace and multi-environment workflows become standard, the CLI needs explicit `workspace` and `environment` fields on profiles. Without them, a developer managing profiles for `acme-corp/dev`, `acme-corp/prod`, and `partner-inc/dev` has no structured way to distinguish what each profile targets — only the name they happened to choose.

Additionally, the `wk.toml` field `workspace` stored a profile name but was named after a concept that now has its own distinct meaning, creating ambiguity in documentation, help text, and developer mental models.

---

## Decision

### Profile struct expansion

Add `workspace` (required) and `environment` (required) as metadata fields on the `Profile` struct. The `name` field remains the primary key — it is required, developer-chosen, and is what the CLI uses for lookup, keyring storage, active profile tracking, and project binding.

**Before:**
```go
type Profile struct {
    Name      string    `json:"name"`
    Region    Region    `json:"region"`
    StoreType StoreType `json:"store_type"`
    BaseURL   string    `json:"base_url"`
    CreatedAt time.Time `json:"created_at"`
}
```

**After:**
```go
type Profile struct {
    Name        string    `json:"name"`
    Workspace   string    `json:"workspace"`
    Environment string    `json:"environment"`
    Region      Region    `json:"region"`
    StoreType   StoreType `json:"store_type"`
    BaseURL     string    `json:"base_url"`
    CreatedAt   time.Time `json:"created_at"`
}
```

This schema is universal — every profile carries these fields regardless of credential store type (keychain, file, encrypted file, secrets manager). The store type determines where the profile metadata and credential are persisted, not what fields the profile contains. See Sub-decision 10 for the full four-tier storage pattern.

### Login command flags

`wk auth login` accepts the following flags:

- `--name` — developer-chosen profile identifier (primary key; prompted interactively if omitted)
- `--workspace` — Workato account name (prompted if omitted)
- `--environment` — target environment within the workspace (prompted if omitted)
- `--region` — Workato region (defaults to `us`)
- `--token` — API token (prompted if omitted)
- `--store-type` — credential backend: `keychain` (default) or `file` (see Sub-decision 3)
- `--force` — skip overwrite confirmation if a profile with the same name already exists

When flags are omitted in interactive mode (TTY), the CLI prompts for them in struct field order: name, workspace, environment, region, token. In non-interactive mode (piped/CI), all required flags must be provided explicitly.

**Overwrite behavior:** If a profile with the given `--name` already exists, the CLI warns and prompts for confirmation before overwriting. The `--force` flag acknowledges the overwrite in advance, skipping the prompt. Developers who prefer explicit separation can delete the existing profile first (`wk auth delete <name>`) and re-create it.

### wk.toml field rename

The `wk.toml` field that pins a project to a profile is renamed from `workspace` to `profile`, since "workspace" now has a distinct meaning (the Workato account).

**Before:**
```toml
name = "my-project"
workspace = "dev"
```

**After:**
```toml
name = "my-project"
profile = "dev"
```

The Go struct field is renamed from `Config.Workspace` to `Config.Profile` with the TOML tag `toml:"profile"`.

### What does NOT change

- The `CredentialStore` interface — still keyed by a single `profileName string`
- Keyring storage keys — still the profile `name`
- `~/.wk/active_profile` — still a single name string
- `~/.wk/keyring_profiles.json` — still a string list of names
- The `--profile` global flag — still accepts a profile name
- `auth switch <name>` — still a single argument

### What DOES change

- **Credential routing model** — The `ChainStore` pattern (try env vars, then keychain in fixed order) is replaced by StoreType-driven routing. The profile's `store_type` field determines which credential backend to query. See Sub-decision 7.
- **Environment variable credential support** — Process environment variables (`WK_TOKEN`, `WK_REGION`) are removed. The CLI no longer reads credentials from the process environment. CI/CD pipelines use the project-level file store instead. See Sub-decision 8.
- **`auth login` store type** — No longer hardcoded to `StoreKeychain`. The `--store-type` flag selects the credential backend at profile creation time.
- **`auth list` output** — Adds WORKSPACE and ENVIRONMENT columns. Shows file-store profiles from `profiles.env` when run inside a project directory.
- **`wk init` validation** — Now validates that the named profile exists and that the active profile matches. See Sub-decision 9.

---

## Sub-Decisions

### 1. Workspace + Environment uniqueness

The CLI enforces that no two profiles share the same `(workspace, environment, region)` tuple. This prevents silent misconfiguration where two profiles accidentally target the same remote environment. The `name` field remains independently unique (as it is the primary key).

### 2. Environment field validation

The `environment` field is a freeform string, not a constrained enum. Workato's environment model varies by account tier and configuration, so the CLI should not impose a fixed set. Examples: `dev`, `staging`, `prod`, `sandbox`, `test`.

### 3. File-based credential store (`profiles.env`)

The file-based credential store replaces the previous process-environment-variable approach (`WK_TOKEN`, `WK_REGION`). Profile metadata and credentials are stored together in a project-level `profiles.env` file using key=value syntax.

**Format:**

```
NAME=dev
WORKSPACE=acme-corp
ENVIRONMENT=dev
REGION=us
STORE_TYPE=file
BASE_URL=https://www.workato.com
TOKEN=wk-xxxxx

NAME=prod
WORKSPACE=acme-corp
ENVIRONMENT=prod
REGION=us
STORE_TYPE=file
BASE_URL=https://www.workato.com
TOKEN=wk-yyyyy
```

- Each `NAME=` line starts a new profile record. Fields following it until the next `NAME=` line (or EOF) belong to that profile.
- Keys are unprefixed — no `WK_` prefix. The file is CLI-owned; key names correspond directly to profile struct fields.
- The CLI reads but **does not write** to this file. Developers and CI/CD pipelines create and manage it directly.
- The file lives at project level, alongside `wk.toml`.
- Multi-profile files are supported (designed for) but single-profile is the common case.
- A `.env.example` file in the project documentation shows the expected format and explains file-store-specific capabilities.

**Note:** This is not a standard `.env` file — standard dotenv parsers expect unique keys. This is a CLI-owned file that uses key=value syntax with `NAME=` as a record delimiter. The `.env` extension signals "key=value config with secrets — do not commit," which is the right developer signal.

### 4. Clone command flag rename

The `clone` command's `--workspace` flag (which specifies a server-side path prefix, not an auth workspace) is renamed to `--path-prefix` to avoid collision with the auth concept of workspace.

### 5. Backward compatibility

Old `profiles.json` entries without `workspace` or `environment` fields deserialize with empty strings (Go zero values). The CLI warns on `auth status` and `auth list` when a profile has empty workspace/environment fields and suggests running `wk auth login` to update it. No blocking migration — existing profiles continue to function.

### 6. Terminology alignment

All CLI help text, error messages, prompts, and code comments are updated to use consistent terminology:
- **Profile** = a named auth configuration (workspace + environment + region + credential)
- **Workspace** = a Workato account
- **Environment** = a target within a workspace

The phrase "workspace profile" is eliminated. References to "workspace" in contexts that mean "profile" are corrected.

### 7. StoreType-driven credential routing

The `ChainStore` pattern — which tries env vars then keychain in a fixed, hardcoded order — is replaced by deterministic routing based on the profile's `store_type` field.

**Resolution flow:**

1. Read active profile name from `~/.wk/active_profile` (or `--profile` flag override)
2. Look up profile metadata:
   a. Check `~/.wk/profiles.json` — if found, the profile's `store_type` field determines the credential backend
   b. If not in `profiles.json`, check project-level `profiles.env` — if found, store type is implicitly `file`
   c. If neither, error: profile not found
3. Retrieve credential from the backend indicated by the resolved store type

The `--store-type` global flag can override implicit routing, allowing a developer to explicitly target a specific backend (e.g., `wk pull --profile dev --store-type file` routes to `profiles.env` even if `dev` also exists in `profiles.json`).

**Cross-store name collisions:** If the same profile name exists in both `profiles.json` and `profiles.env`, the lookup order (step 2) provides deterministic priority: keychain wins. Developers can override with `--store-type file`. No collision enforcement is needed — the priority order and explicit flag handle it.

### 8. CI/CD model

Process environment variables (`WK_TOKEN`, `WK_REGION`) are removed. CI/CD pipelines use the project-level file store:

1. The pipeline generates `profiles.env` from its secrets management system (GitHub Secrets, Vault, etc.)
2. The project's committed `wk.toml` references a profile name (e.g., `profile = "ci"`)
3. CLI commands resolve credentials from `profiles.env` at runtime — no `wk auth login` step is needed

This provides consistency across local and CI/CD environments: the same profile schema, the same lookup-by-name behavior. The only difference is who creates the credential file (developer vs. pipeline script).

**Example (GitHub Actions):**

```yaml
steps:
  - uses: actions/checkout@v4
  - name: Write credentials
    run: |
      cat <<EOF > profiles.env
      NAME=ci
      WORKSPACE=acme-corp
      ENVIRONMENT=prod
      REGION=us
      STORE_TYPE=file
      BASE_URL=https://www.workato.com
      TOKEN=${{ secrets.WORKATO_TOKEN }}
      EOF
  - name: Pull recipes
    run: wk pull --profile ci
```

### 9. `wk init` profile validation

`wk init` is updated to validate the named profile and enforce active-profile consistency. These changes are cohesive with ADR-005 (project scaffolding).

**Profile validation:** `init --profile <name>` validates that the named profile exists before writing `wk.toml`. The `--store-type` flag tells `init` which store to check:

- Default (no `--store-type`): checks `~/.wk/profiles.json` (keychain profiles)
- `--store-type file`: checks `profiles.env` in the target project directory. Per ADR-005 Decision 2 step 4, `init` can scaffold into an existing directory — the developer creates the directory and places `profiles.env` in it before running `init`.

If `--store-type file` is specified and no `profiles.env` exists in the target directory, `init` warns ("no profiles.env found — create one before running commands") and defers credential validation to runtime. Profile metadata is not validated in this case.

**Active profile mismatch:** If the active profile (from `~/.wk/active_profile`) does not match the `--profile` argument, `init` fails:

```
Error: active profile "prod" does not match target profile "dev"
```

No corrective action is suggested — the developer resolves the mismatch themselves.

**What `wk.toml` stores:** The profile name only. Workspace, environment, region, and store type are not written to `wk.toml` — they are resolved at runtime from the profile's authoritative store.

### 10. Credential storage pattern (four-tier model)

Each credential store tier follows the same principle: the store is self-contained and authoritative for its own profiles. Profile metadata lives where the store lives.

| Tier | Backend | Metadata Location | Credential Location | CLI Writes? |
|------|---------|-------------------|---------------------|-------------|
| 1 | Secrets manager (Vault, AWS SM) | Remote store | Remote store | Connects + reads; provisioning is external |
| 2 | OS keychain | `~/.wk/profiles.json` | OS keychain | Yes (both) |
| 3 | Project-level file (`profiles.env`) | `profiles.env` | `profiles.env` | No (read-only) |
| 4 | AES-encrypted file | Encrypted file | Encrypted file | Yes (must, since encrypted) |

Keychain (Tier 2) is the only tier where metadata and credential are split across two locations. OS keychains are pure secret stores without structured enumeration — `profiles.json` serves as the enumerable index.

Tier 1 (secrets manager) requires local connection configuration (e.g., Vault address, IAM role) to reach the remote store. This configuration is a CLI-level setting, not profile-specific. Once connected, the secrets manager holds both profile metadata and credentials.

Tier 4 (AES-encrypted file) follows the same co-location pattern as Tier 3 but encrypted. The CLI must write to it since the developer cannot hand-edit an encrypted file. Likely stored at user level (`~/.wk/credentials.enc`) since air-gapped environments typically use a single credential set across projects.

Tiers 1 and 4 are Phase 1 deliverables (per ADR-001 Decision 5). This ADR establishes the storage pattern they must follow.

---

## Alternatives Considered

### Composite key (workspace + environment as the identifier)

Using `workspace/environment` as the profile lookup key instead of a separate `name`. This was rejected because:
- It forces a breaking change to every layer that passes a profile identifier (keyring, active_profile file, wk.toml, CredentialStore interface)
- It makes the `auth switch` command verbose (`wk auth switch acme-corp/dev` vs. `wk auth switch dev`)
- It requires migration of existing keychain entries and config files
- The developer-chosen `name` provides a convenient short alias that can be anything

### Optional workspace and environment

Making the new fields optional rather than required. This was rejected because optional fields would allow profiles to exist without targeting information, defeating the purpose of the change. Existing profiles are handled via backward compatibility (empty strings with a warning), but new profiles must provide both fields.

---

## Consequences

### What becomes easier

- **Multi-environment workflows**: Developers can see at a glance which workspace and environment each profile targets, rather than relying on naming conventions.
- **CI/CD setup**: Pipelines produce a `profiles.env` file from their secrets manager — no `auth login` step, no keychain dependency, same profile schema as local dev.
- **Safety**: `auth list` output shows the actual target (workspace, environment, region), reducing the risk of operating against the wrong environment. `init` validates profiles exist and enforces active-profile consistency.
- **Consistency across store types**: The profile schema is universal. Whether a developer uses keychain locally or a file store in CI, the same fields are present and the same lookup-by-name behavior applies.
- **Deterministic credential routing**: StoreType-driven routing eliminates the guesswork of the ChainStore pattern. The developer (or their profile configuration) declares which backend to use.

### What becomes harder

- **Profile creation**: `wk auth login` now requires workspace and environment in addition to name/token/region. Mitigated by the interactive prompt flow filling them in when omitted in TTY mode.
- **File store setup**: Developers using the file store must manually create and manage `profiles.env`. The CLI does not scaffold it. Mitigated by the `.env.example` documentation and the simple key=value format.
- **Existing users**: Profiles created before this change lack workspace/environment fields. Mitigated by the non-blocking backward compatibility strategy (warn, don't block).
- **`init` is stricter**: Profile must exist, active profile must match. Developers who previously used arbitrary profile names in `wk.toml` must now create the profile first. This is an intentional safety improvement.

### What we'll need to revisit

- **Profile validation against Workato API**: Once the API supports workspace/environment introspection, the CLI could validate that the workspace and environment specified in a profile actually exist on the server. Deferred until API support is available.
- **wk.toml binding granularity**: Currently the project pins to a profile name. A future enhancement could pin to a workspace (allowing any profile targeting that workspace), enabling environment promotion workflows without changing wk.toml.
- **Interactive profile picker for `init`**: If developer demand warrants it, `init` could present a list of available profiles to select from rather than requiring the name as a flag. Deferred to avoid importing a picker dependency for a single command.
- **Tier 1 and 4 implementation**: The storage pattern is defined here; implementation details (Vault connection config, encrypted file key management) will be specified in dedicated ADRs when those tiers are built.

---

## Action Items

### Profile model
1. [ ] Update `Profile` struct in `internal/auth/types.go` with `Workspace` and `Environment` fields
2. [ ] Add workspace+environment+region uniqueness validation to `ProfileManager.SaveProfile()`
3. [ ] Rename `Config.Workspace` to `Config.Profile` (struct + TOML tag)
4. [ ] Rename `clone --workspace` to `clone --path-prefix`

### Auth commands
5. [ ] Add `--workspace`, `--environment`, `--store-type`, and `--force` flags to `wk auth login`
6. [ ] Add interactive prompt flow to `auth login` (prompt in struct field order when flags omitted in TTY mode)
7. [ ] Add overwrite detection and confirmation prompt to `auth login`
8. [ ] Implement `wk auth delete <name>` command
9. [ ] Update `auth list` — add WORKSPACE, ENVIRONMENT columns; read `profiles.env` when in a project directory
10. [ ] Update `auth status` to resolve from file store when appropriate
11. [ ] Update `auth switch` to validate against both `profiles.json` and `profiles.env` (when in a project directory)

### Credential routing
12. [ ] Replace `ChainStore` usage in `resolveAPIClient` with StoreType-driven routing (Sub-decision 7 resolution flow)
13. [ ] Remove `EnvStore` (process env var credential reader)
14. [ ] Implement `FileStore` — reads `profiles.env`, parses multi-profile key=value format, looks up by `NAME=`
15. [ ] Add `--store-type` as a global flag for explicit backend override

### Init validation (cohesive with ADR-005)
16. [ ] Add profile existence validation to `wk init`
17. [ ] Add active profile mismatch check to `wk init` (hard fail, name the mismatch)
18. [ ] Add `--store-type` flag to `wk init` for file-store profile validation
19. [ ] Warn and defer when `--store-type file` is specified but no `profiles.env` exists

### Documentation and cleanup
20. [ ] Update all help text, error messages, and prompts for terminology consistency
21. [ ] Remove `WK_TOKEN`, `WK_REGION` env var references from code and documentation
22. [ ] Create `.env.example` documenting the `profiles.env` format
23. [ ] Update all tests
