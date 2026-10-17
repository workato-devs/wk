# ADR-008: AIRO Integration — Out of the Data Path, Explicit Invocation Only, `wk mcp connect` Owns Setup

**Author(s):** Zayne Turner, Claude [role: assistant; harness: Claude Code; model: Opus 5]
**Amended-by:** Claude [role: assistant; harness: Claude Code; model: Opus 5.5], dir. Zayne Turner — October 2026
**Status:** Proposed
**Date:** August 12, 2026
**References:** ADR-003 (MCP Strategy), ADR-006 (Profile Identity Model), ADR-009 (Agent Studio DX Completeness), [AIRO MCP server docs](https://docs.workato.com/en/airo/mcp), [Workato Labs guiding principles](https://workato-devs.github.io/labs/principles.html) — chiefly *"built for agent-assisted development"* and *"open by default"*

**Amendments:**
- October 2026 — Revised in place before acceptance (never published). The August probe ran in a workspace without AIRO enabled; re-verified 2026-10-16 in AIRO-enabled workspaces against Workato's published AIRO MCP docs. The planned `wk airo` command group, `wk recipes summarize` and the tool allowlist / `--allow-write` were cut; `wk mcp connect` (Decision 8) was added with OAuth as its default. Investigation detail moved to the probe notes. Decision numbers are unchanged, so existing references still hold.

---

## Context

Workato AIRO is a workspace-scoped AI assistant. Its programmatic surface is one MCP endpoint,
`POST /airo_mcp`, documented in the [AIRO MCP server docs](https://docs.workato.com/en/airo/mcp).
The facts the decisions below rest on:

- **Two ways to authenticate, two identities.** An OAuth connection acts as **the signed-in user,
  with that user's permissions**. An API token sent as a bearer header acts as **the workspace
  owner**. One role permission, "AIRO MCP Server", grants every tool, and other role privileges
  and project access do not narrow it. Audit log entries are tagged "(via AIRO)".
- **About 100 tools, and the surface moves fast.** 98 tools on 2026-10-16, up from 84 in August,
  with 36 of the original 84 changing input schemas in between.
- **No tool carries MCP safety annotations** (`readOnlyHint`/`destructiveHint`). The per-tool
  `_meta` field (`"workato.com/tool_type"`) is a category (recipe, genie, workspace, …), not a
  read/write signal; every category has both kinds. By name, 44 of 98 tools mutate state.
- **Builder protocols are stateful and LLM-shaped.** `recipe_builder_*` and `blueprint_*` (and
  `genie_*`) require a caller-chosen `session_id`, and each call's arguments come from reading the
  previous call's rendered output. The server's `initialize` instructions are written for an agent.
  This is what broke ADR-003's premise that MCP delegation buys nothing; ADR-003 was amended, and
  this ADR decides what `wk` builds on top.
- **AIRO's REST surface is effectively gone.** `POST /api/airo/chat` returns `404` with every AIRO
  permission granted and is undocumented; `GET /api/airo/knowledge_bases` answers but is also
  undocumented.

**Out of scope**, and not to be re-derived from AIRO's tool list: recipe building
(`/api/recipe_validators/*`; local `wk lint` is unaffected), Connector SDK work including
`connector_builder_*` (a separate tool; see `docs/known-limitations.md`), and data tables
(plugin-scoped, ADR-009 Decision 8).

*Probe captures and detailed notes are in the gitignored `tmp/api-exploration/airo-2026-10/`. Ask
Zayne if you don't have a copy.*

---

## Decision

### 1. AIRO is reached only by explicit invocation; `wk` never delegates to it

`wk` does not route existing commands through AIRO, does not consult AIRO to decide how to fulfil a
command, and does not drive multi-tool AIRO workflows on a caller's behalf.

This keeps ADR-003's rejection of auto-delegation on new grounds. ADR-003 rejected it because the
Dev API MCP was a 1:1 passthrough, which isn't true of `airo_mcp`. It stays rejected because
**AIRO can mutate workspace state with the workspace owner's authority, and implicit routing into
a mutating, LLM-oriented surface cannot be audited from the invocation.** AI coding agents are
`wk`'s primary callers, and they rely on `wk` being deterministic and compose commands from
`--help` and error text, which can't express "sometimes this calls an assistant." `wk` does not
make judgment calls between tool calls; the caller, usually already an agent, does that better.

### 2. No `wk airo` command group: `wk` stays out of AIRO's data path

There are no `wk airo tools`, `call`, `docs`, `knowledge-bases` or `ask` commands. Once
`wk mcp connect` exists (Decision 8), each would only insert `wk` between an agent and `airo_mcp`:

- **`tools` and `docs` duplicate native MCP.** A connected agent already has `tools/list`,
  `docs_get` and the resource corpus. Diagnostics are `wk mcp tools`/`test --bearer` (Decision 3).
- **`call` would do it on the riskier identity.** It could only use the profile's API token (the
  workspace owner), while the connection `wk mcp connect` sets up acts as the signed-in user.
  Shell access to AIRO should wait until `wk` can act as the user, i.e. until `wk` is its own OAuth
  client (deferred; see Alternatives).
- **`knowledge-bases` and `ask` rest on undocumented endpoints**, and `ask`'s returns `404`.
  Agent Studio knowledge bases are covered by `wk agentic kb` (ADR-009).

`wk` sets the connection up and can check it; the agent does the rest.

### 3. First-party MCP endpoints authenticate with the profile bearer token, via explicit `--bearer`

`airo_mcp` requires `Authorization: Bearer <token>`. The managed servers ADR-003 was built for
carry their credential in the URL, so `internal/mcp/client.go` sent no auth header. The MCP client
gains optional bearer auth; this is a client capability, not an AIRO special case. On the CLI it is
an explicit `--bearer` flag on `wk mcp tools` and `wk mcp test` that sends the active profile's
token and is **refused for anything but an https Workato host** (`*.workato.com`,
`*.workatoapp.cn`), so a mistyped or hostile URL never receives it. Nothing attaches the token
implicitly, and for first-party endpoints the token never goes in, or is echoed into, the URL.

### 4. No tool-safety classification in `wk`, because `wk` calls no AIRO tools

The tool allowlist and `--allow-write` existed only to guard `wk airo call`, which Decision 2 cut.
The finding behind them stands for anyone who later builds a `wk` caller of AIRO tools: with no
safety annotations and `_meta` carrying only a category, such a caller must keep a known-read-only
allowlist and fail closed on anything unrecognized. A draft list is in the probe notes.

### 5. Builder protocols are not wrapped: their only viable driver is an LLM

`recipe_builder_*` (27 tools) and `blueprint_*` (8 tools) get no `wk` commands.

- *`wk` doesn't need them.* Recipe creation and code replacement already work over REST
  (`internal/api/recipes.go`). The builders are an authoring layer over capabilities `wk` already
  reaches deterministically.
- *Their shape needs a driver `wk` must not be.* Each step's arguments come from interpreting the
  previous step's output. That loop's natural driver is an LLM, and per Decision 1 `wk` is not one.
  Being LLM-driven is the design, not a flaw. An agent that wants a builder connects to `airo_mcp`
  directly (Decision 8); a `wk` hop over an unstable output format gives it nothing and adds a
  second place for the contract to break.

Per ADR-000 §4 this is an architectural rejection and stays inline: *`wk` does not wrap tool groups
whose only viable driver is an LLM.* Revisiting it means revisiting "should `wk` drive agentic
loops?", not quietly adding builder commands.

There is also no `wk recipes summarize` (an earlier draft wrapped `recipe_builder_pull` for its
readable rendering). AIRO's `recipe_get` now returns that rendering read-only, with no session, and
is where AIRO points agents for inspection.

### 6. AIRO error facts

These apply to `wk mcp tools`/`test --bearer` today and to any future AIRO caller:

- `401` on `airo_mcp` carries `WWW-Authenticate: Bearer resource_metadata="…"`, so an error can
  point at the discovery document; the body carries a `reason` (`authentication_failed` for a
  missing or bad token). `401`s on `/api/*` carry a `reason` too (e.g. `missing_privilege`).
- Tool failures arrive as `result.isError: true`, with text in content block 0 and JSON in block 1.
  On success `isError` is **absent**, not `false`.

### 7. Where AIRO and Agent Studio overlap, REST wins

`genie_*` and `knowledge_base_*` exist both as `airo_mcp` tools and as `/api/agentic/*` REST
endpoints (ADR-009). `wk`'s commands use REST: it's documented, returns structured JSON rather than
markdown, needs no `session_id`, and is deterministic. `wk agentic genies create` is the supported
path.

### 8. `wk mcp connect` wires MCP clients to Workato MCP endpoints: `wk` configures the connection and is not in it

Decisions 1, 2 and 5 keep `wk` out of the **data path**. The **setup path** is different: `wk`
already knows the region→URL map, the active profile, its workspace and environment, and the
credential, so it is the tool positioned to produce a working client config. Wiring a client isn't
delegation: `wk` is absent from every resulting request.

```
wk mcp connect airo                                 # client-neutral details; installs nothing
wk mcp connect airo --client <claude-code|codex|cursor> [--config-scope …] [--auth oauth|token] [--print]
wk mcp connect <mcps-handle> [--client …] [--allow-url-token]   # managed MCP servers
wk mcp auth-header --profile <name>                 # {"Authorization":"Bearer …"} for header helpers
```

**Open by default.** MCP standardizes the protocol, not client configuration; Claude Code, Codex,
Cursor and Claude Desktop each differ on file, format and how auth is expressed. "Open by default
… any AI agent, any IDE, any OS" means every MCP client must get something it can act on:

- **Without `--client`, `wk` prints client-neutral details and installs nothing.** This is the
  default, not a fallback: server URL and transport; the profile's workspace, workspace ID and
  environment; for OAuth, whether it's available (and why not), who it acts as, which workspace to
  sign in to, and the discovery URLs and scope; for token auth, who it acts as, the exact header,
  and the commands that supply it (`wk auth token`, `wk mcp auth-header`). Same content in `--json`.
- **A known `--client`** gets its exact config, installed where `wk` can do that reliably and
  printed where it can't.
- **An unknown `--client`** gets the neutral details and a note that there's no adapter, exit 0.

**`wk` installs only through a client's own CLI and never edits a client's config files.** Each
client owns its files' format, merge rules and scopes; a second, unofficial writer drifts. Where a
client's CLI can't be relied on, the client is print-only.

| Client | How `wk` installs | Config scopes | Token auth mechanism |
|---|---|---|---|
| `claude-code` | `claude mcp add-json` | local (default), user, project | `headersHelper` → `wk mcp auth-header` |
| `codex` | `codex mcp add` | user only (Codex has no other) | `bearer_token_env_var` (default `WORKATO_API_TOKEN`) |
| `cursor` | **print-only**: the `mcp.json` entry, its file, and steps | user (`~/.cursor/mcp.json`), project (`.cursor/mcp.json`) | `headers` reading `${env:WORKATO_API_TOKEN}` |

**`--config-scope` is the client's config scope**, i.e. where the client saves the entry on this
machine (Claude Code: `local`, private per folder; `user`, all folders; `project`, `.mcp.json`,
meant to be committed). It has nothing to do with Workato projects, which is why it isn't called
`--scope`. The server name defaults to `airo-mcp` (the server's `serverInfo.name`);
`WORKATO_API_TOKEN` matches Workato's setup docs and `--token-env-var` overrides it; replacing an
existing entry requires `--force`.

**Invariant: `wk` never writes a credential into a config file** (one narrow, opt-in exception for
managed `token` servers, below).

**OAuth is the default; token auth is opt-in and never a fallback.** The two methods decide whose
authority the client wields. Wiring every agent with a token would quietly give each one
owner-level power over the workspace.

1. **`--auth oauth` (default): the config holds only the URL.** The client registers itself with
   Workato's authorization server (dynamic client registration), runs the browser authorization and
   stores its own tokens. A project-scoped config is safe to commit, and each teammate connects as
   themselves. The AIRO host is the API host in every region except US (API `www.workato.com`,
   AIRO `app.workato.com`).
2. **`wk` says which workspace to authorize.** An OAuth connection binds to the workspace and
   environment active in the *browser session*, not the `wk` profile. Authorizing the wrong one
   without noticing is the likeliest failure, and only `wk` has the facts to prevent it, so the
   human and `--json` output both name it.
3. **`--auth token` is stated intent, never inferred.** The client reaches the token through
   indirection, never a copy: a header helper running `wk mcp auth-header` (Claude Code), or an
   environment variable the user sets from `wk auth token` (Codex, Cursor). The helper lives under
   `wk mcp` because it formats a credential `wk` already holds and changes nothing about auth. The
   output says the client will act as the workspace owner. For Claude Code, token auth refuses
   `--config-scope project` (the helper names a personal profile), and a `profiles.env` profile is
   limited to local scope (the helper can only find it from inside the project). This is the path
   for headless runs and for trial.
4. **Registration support comes from the server's metadata, not a region list.** At connect time
   `wk` reads the region's `/.well-known/oauth-authorization-server`. If `registration_endpoint` is
   absent, `--auth oauth` stops with an error naming the cause and `--auth token` as the
   alternative, including its workspace-owner consequence, and the neutral details report OAuth as
   unavailable. Observed 2026-10-16: present in every production data center (US, EU, JP, SG, AU,
   IL, KR, UK), absent on trial (`POST /oauth2/register` → `403 "Dynamic Client Registration is not
   enabled"`). Without registration a client needs a pre-registered client ID, which Workato offers
   no way to get. If trial gains registration, `wk` just works; if a data center drops it, `wk`
   refuses instead of writing a config that will fail.

**`--auth oauth` ships unverified end to end, labeled as such.** No production workspace was
available, so authorization, token exchange and a tool call acting as the user were not exercised.
Registration was: a public loopback client registered on `app.workato.com`, and Codex, given a
URL-only entry, registered itself and produced a valid authorize URL. What `wk` writes is exactly
Workato's documented setup (a URL-only entry, as in their `claude mcp add --transport http <url>`),
and the flow runs entirely between client and Workato, so a break would hit a user following
Workato's docs by hand too. Per *"ship fast, learn in the open — release early against real usage
signal"*, it ships, and the release notes and `docs/known-limitations.md` say it wasn't verified
end to end and ask for reports.

**Client behaviors the adapters must respect** (observed 2026-10-16):

- **Claude Code** (2.1.302): `headersHelper` runs on each connection and after a `401`/`403`
  (*verified* `✔ Connected` with a token entry). **It runs only after the user accepts the folder
  trust prompt, at local scope too**; until then Claude Code falls back to OAuth, which fails on
  trial. Headless `claude -p` never shows the prompt, so trust must be accepted interactively once;
  `wk` says so. Before running the helper it strips env vars whose names contain TOKEN, SECRET,
  PASSWORD, KEY or AUTH, so `wk mcp auth-header` reads the keychain. A helper returning
  `Authorization` disables OAuth for that server. Re-adding a name fails with `already exists`,
  which `wk` turns into a `--force` hint. Reportedly (unverified) it reads credential-like `${VAR}`s
  as empty in remote `url`/`headers`, so the adapter doesn't rely on `${VAR}` expansion.
- **Codex** (0.156.1): `codex mcp add` with a URL-only entry saves the entry and **then starts an
  OAuth flow that waits for a browser**, which would hang an agent or CI. `wk` stops it once it
  starts and tells the user to run `codex mcp login <name>`. Codex runs in its own process group on
  Unix and the whole group is killed, so an npm-installed Codex (a Node wrapper) can't leave `wk`
  waiting; Windows has no process groups, so a child may outlive the kill, but `wk` stops reading
  once Codex exits. **`codex mcp add` silently overwrites an existing server**, so `wk` checks with
  `codex mcp get` first and requires `--force`. `codex mcp add` has no flag for Codex's
  `http_headers_helper`, hence `--bearer-token-env-var`.
- **Cursor** (3.4.20): **print-only because its CLI doesn't do what it documents.**
  `cursor --add-mcp` ignored `--mcp-workspace` (warned it's unknown though `--help` lists it) and
  wrote VS Code's `User/settings.json` format instead of `~/.cursor/mcp.json`, where Cursor's agent
  reads servers. The OAuth entry follows Workato's docs (`npx -y mcp-remote <url>`).

**Managed MCP servers: the server's authentication method decides, and `--auth` is an error.**
`wk mcp connect <mcps-handle>` reads `authentication_method` (returned by the list endpoint only;
`GET` omits it). The default entry name is `workato-<server-name>`.

| Method | What the API gives `wk` | What `wk mcp connect` does |
|---|---|---|
| `workato_idp` | The URL, no token. OAuth via **Workato Identity** (`id.<region>.workato.com/auth/servers/<id>`), which supports client registration **even on trial**. | URL-only entry, like AIRO OAuth; the user signs in as their Workato Identity user. *Verified:* Claude Code accepted it (`Needs authentication`). |
| `token` | A URL containing `?wkt_token=`: **the URL is the credential.** | Installs only with `--allow-url-token`, never at `project` config scope. Without the flag, output shows the token as `REDACTED` and names the flag. *Verified:* `✔ Connected` in Claude Code. |
| `hashed_token` (default for new servers) | The URL, no token. The token can't be read, and `token_renew` invalidates the old one without returning the new one. | Refuses to install and never renews (renewing would break every existing client and still configure nothing). `--print` gives the entry with a placeholder for the URL copied from the Workato UI. |

**`wk`'s own authentication is unchanged this release.** OAuth happens entirely in the client, and
the token path reads the existing profile credential. Making `wk` its own OAuth client is deferred
(see Alternatives).

---

## Alternatives considered

- **Callers configure clients by hand from Workato's docs (rejected).** The docs require facts
  `wk` already has (region→URL, which profile, header vs. URL credential), and a static token in a
  committed config is one copy-paste away: exactly the default Decision 8 avoids.
- **A static bearer token in the config as the default (rejected).** One code path, but it copies a
  credential into a file whose most useful scope is committed, and rotation breaks it with an error
  that looks like an AIRO outage.
- **One config format, let callers translate (rejected).** Honors "no lock-in" in letter only:
  Codex differs in file, syntax and auth field, and the translation ends in an opaque `401`.
- **`wk` as a local stdio proxy for `airo_mcp` (rejected; the closest call).** Works with any
  stdio client with no per-client adapters, but puts `wk` back in the data path as a live
  dependency inside someone else's agent loop, and the proxy would have to hold the owner-level
  token. Reopen if per-client adapters multiply beyond what's maintainable.
- **`wk` writes client config files directly (rejected).** Uniform and would make Cursor
  installable, but each client owns its format, merge rules and scope semantics (Claude Code's local
  scope lives inside a per-project block of `~/.claude.json`), and they change between releases. Where
  a client's CLI can't be trusted, printing the exact entry is safer than writing it.
- **Default `--client` to one client (rejected).** An earlier draft defaulted to `claude-code`, so
  a Codex or Cursor user got a config their client ignores. Installing requires naming a client.
- **Fall back to token auth when OAuth isn't available (rejected).** It would change whose
  authority the client wields based on `wk`'s inference. The caller is probably an agent that won't
  read a warning in success output; an error naming `--auth token` and its consequence costs one
  retry and leaves the choice with the caller.
- **`wk` as its own OAuth client this release (deferred).** Production data centers support
  registration, so `wk` could run a loopback PKCE login and keep a refresh token, letting `wk` act
  as the signed-in user. It adds a second credential type to the profile model (ADR-006) and a token
  lifecycle to `wk auth`. `wk mcp connect` doesn't need it. Reopen with the `wk auth` work; it is
  also the precondition for any shell-level AIRO command (Decision 2).

---

## Consequences

### What becomes easier

- **Getting an agent onto AIRO is one command**, with no credential on disk, acting by default as
  the signed-in user. At project scope the config is committable and onboards a whole team.
- **First-party MCP endpoints are diagnosable** with `wk mcp tools`/`test --bearer`, closing a gap
  ADR-003 left.
- **Nothing in `wk` tracks AIRO's tool churn.** `wk` neither calls nor parses AIRO tools, so a fast-
  moving, partly specified contract (the docs cover setup and categories, not argument or output
  shapes) can't break a `wk` command.

### What becomes harder

- **All AIRO use, including recipe and blueprint building, happens in the caller's MCP client.**
  There is no shell-only or conversational AIRO from `wk`.
- **Per-client adapters are maintenance.** Each tracks a client CLI's behavior (the Codex OAuth
  hang, Claude Code's trust prompt, Cursor's CLI), and those change between releases.

### What we'll need to revisit

- **The OAuth path, end to end.** Remove the "unverified" label once someone with a production
  workspace confirms: add → authorize → a tool call acting as the user, token refresh, and what
  happens when the browser session's workspace differs from the profile's.
- **Trial in Claude Code** (`claude --debug`): confirm it fails as expected, and whether it tries an
  unadvertised client ID metadata document. **`${VAR}` expansion** in Claude Code remote headers.
- **Codex with a token** was checked only to the config (`codex mcp get`); no tool call yet.
- **`workato_idp` sign-in** wasn't completed (needs a Workato Identity user in the server's groups).
- **`hashed_token`:** if `token_renew` returns the new token again (ADR-003, issue #76), an explicit
  `--renew-token` becomes possible, stating plainly that every client on the old token stops working.
- **Cursor:** check both printed entries in the app; if `--add-mcp` writes `~/.cursor/mcp.json` (or
  `--mcp-workspace` works), Cursor becomes installable.
- **More adapters** (Copilot in VS Code, Copilot CLI, Claude Desktop) when each can be verified
  against the real client; the neutral details cover them today.
- **Windows:** if a Codex child outliving the kill causes problems, use a job object.
- **If AIRO ships MCP tool annotations, or a builder gains a REST or single-call equivalent**,
  Decisions 4 and 5 lose part of their footing; reopen them as the general questions they name.
- **The `401` `reason` vocabulary** before mapping reasons to remediation text: check whether bad
  token, AIRO not enabled and entitlement-gated cases differ.

---

## Internal Action Required

Platform dependencies, following ADR-003's precedent:

1. **Publish MCP tool annotations on `airo_mcp`** (`readOnlyHint`/`destructiveHint`). Without them
   every MCP client must guess which tools are destructive from their names, and a token acts as
   the workspace owner, unnarrowed by role privileges.
2. **Clarify the status of `/api/airo/*`.** `chat` returns `404` in AIRO-enabled workspaces;
   `knowledge_bases` answers but is undocumented. Deprecated, and if so, when does the rest go?
3. **Document the `401` `reason` vocabulary** so clients can map each reason to the right fix.
4. **Enable dynamic client registration on trial (Developer Sandbox), or correct the docs.** The
   AIRO MCP docs list a trial URL with the same URL-only OAuth setup, but registration returns
   `403` there, so the documented setup most likely fails and the only working path is a token
   acting as the workspace owner: the wrong default for developers trying AIRO for the first time.
5. **Say whether the tool inventory is a stable programmatic contract**, and if not, publish a
   deprecation or notice policy.
