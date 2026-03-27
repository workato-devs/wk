# ADR-005: Project Scaffolding — Container Folder Convention

**Status:** Proposed
**Date:** March 26, 2026
**Author:** Zayne Turner
**Deciders:** DevRel Engineering
**References:** ADR-001 (Foundational Architecture), ADR-002 (Sync Engine), Tester Feedback (Greenfield Project Setup)

---

## Context

Tester feedback from the CLI beta identified a gap between expected and actual behavior when setting up a greenfield project (initializing in a completely empty folder with no existing Workato project structure).

**Expected behavior:** Running setup commands in an empty folder should produce a self-contained project directory named after the project. The `wk.toml` config file and all synced artifacts live inside this container folder. Nothing exists outside it.

**Actual behavior (as of ADR-002):** `wk init` creates `wk.toml` in the current working directory with no container folder. The developer's CWD *becomes* the project root, but there is no named directory wrapping the project. `wk clone` is closer — it creates a named directory and places `wk.toml` inside — but the two commands produce structurally different outcomes for what should be a consistent convention.

This inconsistency creates confusion for developers starting new projects. A developer who runs `wk init --name my-project` in `~/workato/` expects to find `~/workato/my-project/wk.toml`, not `~/workato/wk.toml`. The container folder convention also aligns with how `git init <name>`, `cargo new <name>`, and `npm init` behave — creating a named directory is the standard pattern for project scaffolding in CLI tools.

---

## Decision

Adopt a container folder convention: `wk init --name X` creates `X/` in the current working directory and places `wk.toml` inside it. `wk clone` aligns with the same convention. The container folder is the project root — all artifacts, metadata, and configuration live within it.

---

## Key Design Decisions

### Decision 1: Container Folder Is the Project Root

**Decision:** `wk init --name my-project` creates `my-project/` in the CWD and writes `wk.toml` inside `my-project/`. Nothing is created outside the container.

**Expected structure:**

```
~/workato/                              (developer's CWD)
└── my-project/                         (container = project root)
    ├── wk.toml                         (project config)
    ├── recipes/                        (created by first pull)
    │   ├── my_recipe.recipe.json
    │   └── my_recipe.recipe.json.wk-meta.json
    └── connections/                    (created by first pull)
        ├── slack.connection.json
        └── slack.connection.json.wk-meta.json
```

**Why:** Developers expect named projects to produce named directories. This matches established conventions from Git, Cargo, npm, and other CLI tools. It also prevents a common mistake: running `wk init` in a directory that contains other unrelated work, which would make the entire directory a wk project.

**Consequence:** `wk init` no longer treats the CWD as the project root. It always creates (or scaffolds into) a child directory. Developers who want the current directory to be the project root should use `wk clone` from the parent, or manually create the directory and run `wk init` from within it (though this is not the recommended workflow).

### Decision 2: `wk init` Creates the Container Directory

**Decision:** `wk init --name my-project` performs the following steps:

1. Check that the CWD is not already inside an existing wk project (see Decision 3)
2. Compute target path: `<CWD>/<project-name>/`
3. If target directory exists and already contains `wk.toml` → error
4. If target directory exists but has no `wk.toml` → scaffold into it (add `wk.toml`)
5. If target directory does not exist → create it with `os.MkdirAll`
6. Write `wk.toml` inside the target directory

**Why:** Step 4 (scaffold into existing directory) supports the case where a developer has already created a directory or initialized a Git repository before running `wk init`. Requiring a completely empty directory would be unnecessarily restrictive. Step 3 prevents accidentally re-initializing a project that already exists.

**Implementation:** `internal/commands/init.go` — Replace the CWD-based config path with `filepath.Join(cwd, name)` as the project root. Create directory before saving config.

### Decision 3: Error When Already Inside a Project

**Decision:** If `FindProjectRoot(cwd)` successfully locates a `wk.toml` in the CWD or any parent directory, `wk init` exits with an error:

```
Error: already inside wk project at /path/to/wk.toml
Run from outside the project directory.
```

**Why:** Nested wk projects would create ambiguity for every other command (`pull`, `push`, `status`, `diff`) because `FindProjectRoot` walks upward and stops at the first `wk.toml` it finds. A nested project's `wk.toml` would shadow the parent, or vice versa depending on the developer's CWD. Rather than introducing complex resolution logic, prevent nesting entirely.

**Consequence:** Developers who genuinely need multiple wk projects in the same tree must keep them as siblings, not nested. This matches how Git repositories work — nested `.git` directories are an antipattern.

### Decision 4: `wk clone` Alignment

**Decision:** `wk clone` already creates a named directory and places `wk.toml` inside it. The following adjustments align it with the `wk init` convention:

- The `Name` field in `wk.toml` should always match the container folder name
- Clone should also check that the CWD is not already inside an existing project (same guard as init)

**Why:** Both commands should produce the same structural outcome. A developer should be able to look at a project directory and not know whether it was created by `init + pull` or by `clone`.

**Implementation:** `internal/commands/clone.go` — Add the `FindProjectRoot` guard. Ensure `cfg.Name` is set from the directory name, not just the server folder name (they may differ if `--local-path` is used).

### Decision 5: `FindProjectRoot` Unchanged

**Decision:** The `FindProjectRoot` function in `internal/config/config.go` requires no modification. It walks upward looking for `wk.toml` and returns the directory containing it — this behavior is correct for the new layout because `wk.toml` lives inside the container folder.

**Why:** The container folder *is* the project root. `FindProjectRoot` returns the directory containing `wk.toml`, which is the container. All relative paths in `wk.toml` (like `local_path = "."`) resolve correctly relative to the container.

### Decision 6: Sync `local_path` Semantics

**Decision:** `local_path` in `[[sync]]` entries is relative to the project root (the directory containing `wk.toml`). With the container convention, this means relative to the container folder. The default `local_path` for init should be `"."` (same as clone), meaning artifacts are extracted directly into the project root.

**Why:** This makes `wk init --name my-project --server-path "Recipes/Prod"` followed by `wk pull` produce the same layout as `wk clone "Recipes/Prod" --local-path my-project`. Consistency between the two workflows.

**Current behavior (init):** `local_path` defaults to `./<leaf of server path>` (e.g., `./Prod`). This creates an extra nesting level inside the container that diverges from clone behavior.

**New behavior (init):** `local_path` defaults to `"."` to match clone. Developers who want subdirectory nesting can still set `--local-path` explicitly.

### Decision 7: Inner Directory Layout Is Not Pre-Scaffolded

**Decision:** `wk init` does not pre-create subdirectories like `recipes/`, `connections/`, etc. These are created by the pull/extract logic when assets are first downloaded.

**Why:** The inner directory structure depends on what assets exist on the server. Pre-creating empty directories would be misleading (implying assets exist when they don't) and would need to be kept in sync with the server's asset types. The RLCM zip extraction already handles directory creation correctly.

---

## Impact on Existing Commands

| Command | Change Required | Details |
|---------|----------------|---------|
| `wk init` | **Yes** | Create container directory, write `wk.toml` inside it, add nesting guard |
| `wk clone` | **Minor** | Add nesting guard, ensure `Name` matches directory name |
| `wk pull` | **None** | Already resolves paths relative to `wk.toml` location |
| `wk push` | **None** | Already resolves paths relative to `wk.toml` location |
| `wk status` | **None** | Already resolves paths relative to `wk.toml` location |
| `wk diff` | **None** | Already resolves paths relative to `wk.toml` location |
| `wk link` | **None** | Operates on an existing `wk.toml`, no scaffolding involved |
| `FindProjectRoot` | **None** | Upward walk for `wk.toml` still works |

---

## Consequences

### What Becomes Easier

- **Greenfield setup**: Developers get a clean, named project directory with a single command. No need to manually create directories or reason about where `wk.toml` should live.
- **Multi-project workspaces**: Developers can have multiple wk projects as siblings in the same parent directory, each in its own named container.
- **Mental model**: "The project name is the folder name" is simple and matches how most CLI tools work.

### What Becomes Harder

- **In-place initialization**: Developers who want the CWD itself to be the project root (no container subdirectory) can no longer do this with `wk init`. This workflow is uncommon and can be worked around by creating the directory manually.

### Migration

This is a **breaking change** for any existing projects initialized with the current `wk init` behavior (where `wk.toml` lives at the CWD level without a container folder). Since the CLI is in beta, this is acceptable. Existing projects can be migrated by:

1. Creating the container directory
2. Moving `wk.toml` and all synced artifacts into it
3. All relative paths in `wk.toml` remain valid (they're relative to `wk.toml`'s location)

---

## Action Items

1. [ ] Update `internal/commands/init.go` — container directory creation, nesting guard, default `local_path = "."`
2. [ ] Update `internal/commands/clone.go` — nesting guard, `Name` alignment
3. [ ] Update tests for init and clone to verify container folder behavior
4. [ ] Update CLI help text for `wk init` and `wk clone` to reflect the new convention
