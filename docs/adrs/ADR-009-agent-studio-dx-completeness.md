# ADR-009: Agent Studio DX Completeness — Resource Coverage, Cursor Pagination, and Syncability

**Author(s):** Zayne Turner, Claude [role: assistant; harness: Claude Code; model: Opus 5]
**Amended-by:** Claude [role: assistant; harness: Claude Code; model: Opus 5.5], dir. Zayne Turner — October 2026
**Status:** Accepted
**Date:** August 12, 2026
**Implemented:** October 17, 2026
**References:** ADR-001 (Foundational Architecture), ADR-002 (Sync Engine), ADR-004 (Plugin Repo Structure), ADR-008 (AIRO Integration), [Agent Studio API reference](https://docs.workato.com/en/workato-api/agent-studio), [Workato Labs guiding principles](https://workato-devs.github.io/labs/principles.html) — chiefly *"built for agent-assisted development"*

**Amendments:**
- October 2026 — Revised in place before acceptance (never published), after re-verifying against AIRO-enabled workspaces and the now-published Agent Studio reference. Write commands were added for genies, knowledge bases and genie clients, and their rules were consolidated into Decision 5. `guardrails set-policy` was cut. Recipe building and data tables moved to an out-of-scope note, so decisions were renumbered. Investigation detail was removed; the probe evidence was not kept.

---

## Context

`wk agentic` covered one resource, `skills` (`list`/`get`/`create`): 3 of the 45 endpoints the
Client Role editor lists under Genies, Genie clients, Knowledge bases and Skills (the API reference
documents 36). ADR-008 Decision 7 routes agentic asset management through REST rather than
AIRO's MCP tools, which is only a real choice once the REST commands exist.

The API's behavior, not its reference, shapes the decisions below:

- **List and get disagree.** Genie `state` is `stopped`/`started` from list and `inactive`/`active`
  from get, for the same genie. Get also returns eight fields list doesn't.
- **Many writes report success for changes they didn't make.** Assignment endpoints accept unknown
  IDs with `200`. Knowledge base create turns an unknown `source_type` into `recipe` with `201`,
  update ignores a blank name or description with `200`, and `data_sources` is accepted and not
  applied. A genie client update ignores redirect URLs on an `api_key` client.
- **Deletes don't check what depends on the resource.** A knowledge base or genie client is
  deleted even while attached to a running genie, and deleting a genie deletes its clients.
- **The reference is wrong in places that matter**, most notably genie client cardinality: a genie
  can have several clients, and a client serves at most one genie.
- **Two pagination styles** (offset, and cursor on conversations and events) and **three timestamp
  formats** (epoch millis, ISO 8601 in UTC, ISO 8601 with an offset).
- **Guardrails are beta and behind a feature flag**, with full-replace `PUT` semantics.
- **Skills can't be deleted through the API**, and outlive their recipe.

**Out of scope:** recipe building (`/api/recipe_validators/*`, including connection health; local
`wk lint` is unaffected) and data tables (plugin-scoped per ADR-004, whatever `airo_mcp` exposes).
`docs/known-limitations.md` records both.

---

## Decision

### 1. Cover Agent Studio over REST

```
wk agentic genies      list | get | create | update | delete | start | stop
                       assign-skills | remove-skills
                       assign-knowledge-bases | remove-knowledge-bases
                       assign-user-groups | remove-user-groups
wk agentic kb          list | get | create | update | delete | recipes | data-sources
wk agentic clients     list | get | create | update | delete | regenerate | start | pause
                       attach | detach
wk agentic guardrails  get | pii-entities
wk agentic logs        conversations <genie-id> | conversation <genie-id> <id> | events <genie-id> <id>
```

Conversation commands take the genie ID because the API scopes conversations under the genie.
`wk agentic logs` is named for what it gives (visibility into what a genie actually did), not for
the API's path.

**Guardrails are read-only.** Policy writes are full replaces against a beta safety control, and a
payload that validates but means something other than intended weakens a genie's protections with
no visible failure.

**Commands that create something the API can't remove say so first.** `skills create` warns that
only the UI can delete a skill. `clients create` and `clients regenerate` print an `api_key`
client's key, because the API returns it only then, and say to store it.

### 2. Cursor pagination is a first-class second paginator

Cursor pagination gets its own option and page types alongside `PaginationOptions`. "Has next"
means `next_cursor` is **non-null and non-empty** (an exhausted page returns `null`), never a
length heuristic. Cursor-paginated commands expose `--cursor`/`--limit`, not `--page`/`--per-page`,
so the command surface tells the truth about what the API supports.

### 3. Separate list and detail structs where the API returns different shapes

`Genie` (list) and `GenieDetail` (get) are distinct types. `state` is an opaque string in both,
shown as each endpoint reported it. Translating one vocabulary into the other would make `--json`
disagree with `--raw` about a value callers branch on. The `--state` filter accepts only `active`
and `inactive`, because the API silently ignores any other value and returns every genie.

Where shapes match, one struct is correct: `Skill` models what the API returns (the recipe is
`provider_id`; there is no `recipe_id`). Optional fields are optional in the types (`omitempty`),
never zero-filled, so `--json` never reports a value the API didn't send.
`GuardrailPolicy.Configuration` stays raw (`json.RawMessage`), so a new policy type displays rather
than failing a read.

### 4. Genies and knowledge bases have no sync path

ADR-002 routes all sync through RLCM, and syncability has three layers: manifest asset type,
package payload, and `wk`'s sidecar handling. None can be inferred from another.

- **Skills** are a valid manifest asset type; `wk`'s sidecar handling of them is a `wk` gap.
- **Genies and knowledge bases are in no layer.** That's a platform boundary, so they're managed as
  live API resources through `wk agentic`, and that's the whole of it.

Even if RLCM added them, they'd be hard to round-trip: a genie's substance is a prose
`instructions` blob plus cross-resource references that need remapping; counts and rollups are
computed, so a sidecar diff would show constant drift; and guardrail policies have server-created
IDs.

### 5. Write commands verify the outcome instead of trusting the response

A command whose purpose is to change something must not report a change that didn't happen. The
caller is probably an agent that moves on believing the response. So every Agent Studio write:

- **Rejects input the API would accept and then ignore**, before calling it: an unknown KB
  `--source-type`, a blank KB name or description (so a description can't be cleared, and `wk`
  says so), `--redirect-url` on a non-OAuth client. It doesn't offer fields the API ignores or
  rejects: KB `data_sources` (no valid source names known), KB folder moves, client type changes.
- **Re-reads after an update** and fails if a requested value didn't take.
- **Reconciles assignments per ID.** `assign-*`/`remove-*` read membership before and after (from
  each skill's or KB's `genies`, and the genie's `user_groups`) and report each ID as `assigned`,
  `already_assigned`, `removed`, `not_assigned`, `not_found` or `not_applied`, exiting non-zero on
  the last two.
- **Refuses to delete a resource something depends on**, unless `--force`. KB `delete` refuses
  while genies use the knowledge base, and `clients delete` while the client is attached. Each
  error names the dependents and gives runnable next steps, and the `--json` report lists what was
  detached. Genie `delete` has no guard, but its help says it deletes the genie's clients.
- **Translates conflicts into what actually happened.** Re-attaching a client to its own genie
  returns `409`; `wk` reports `already_attached` and exits 0. A client on another genie gets an
  error naming that genie and the `detach` command.

These cost extra reads (an assignment reads each ID before and after) against a 1,000/min read
limit. Silent success on a typo or an ignored field is the far worse cost.

### 6. Normalize `wk`'s output, but keep API truth reachable via `--raw`

`--json` is `wk`'s modeled contract: stable across resources, with timestamps normalized to
RFC 3339 UTC by a custom unmarshaller at the service layer. A global `--raw` flag emits the API's
response bodies verbatim, with no struct round-trip, so a field `wk` doesn't model or a value it
normalized is always reachable. The flag itself is recorded in ADR-001 Decision 3.

---

## Alternatives considered

### One `wk genie` top-level group instead of `wk agentic genies` (rejected)

`wk agentic` already ships `skills`. Promoting genies alone would split one product surface across
two levels of the command tree. Renaming the group is its own decision, made all at once with
aliases.

### One `Genie` struct with pointer fields for detail-only data (rejected)

Pointers can't express that `state` uses different vocabularies per endpoint, and they don't let
callers tell "list doesn't return this" from "this genie has none."

### Translate genie `state` into one vocabulary (rejected)

The reference doesn't enumerate the values, so a mapping would need guessing for new ones, and it
would make `--json` disagree with `--raw` on a field callers branch on.

### Generalize `PaginationOptions` to cover both paginators (rejected)

Half the fields would be invalid for any given call, and `--page 3` against a cursor endpoint would
be a silently ignored parameter and a wrong `HasNext`. Two types make the mismatch unrepresentable.

### Model guardrail `configuration` as a typed union (rejected)

An exhaustive union fails closed on an unknown `policy_type`, erroring on a read for a policy it
merely doesn't recognize yet.

### Trust the API's success responses (rejected)

Report whatever the API returns, as most `wk` commands do. Here the API returns the same success
for a typo, an ignored field, and a real change, so trusting it means reporting success for
changes that never happened (Decision 5).

### Sync genies and knowledge bases via a `wk`-invented file format (rejected)

A second, `wk`-specific sync path beside RLCM would make `wk` own dependency remapping, identity and
conflict semantics, which is what RLCM exists to solve, and would diverge from whatever the
platform ships.

---

## Consequences

### What becomes easier

- **ADR-008 Decision 7 holds:** agentic management goes through REST.
- **Genie lifecycle is scriptable** (`create` → `assign-*` → `clients create`/`attach` → `start`),
  and failures are visible rather than silent.
- **Agent runtime is observable:** `wk agentic logs` shows conversations, events and error totals.
- **Guardrails are auditable from a terminal.**

### What becomes harder

- **Two paginators and list/detail struct pairs** are more code, and invite "why are these
  separate?" questions. Decisions 2 and 3 are the answer.
- **Write commands cost extra reads**, and a `--force` step where the API has none.
- **Some things stay UI-only:** configuring guardrails, setting Matrix KB data sources, deleting
  skills, and promoting agentic assets between environments (Decision 4). The last is the sharpest
  user-facing cost.

### What we'll need to revisit

- **Syncability:** if `genie` or `knowledge_base` appears in the RLCM asset types or an export
  package, reopen Decision 4 as an ADR-002 amendment. Skill sync beyond the manifest is unverified.
- **Guardrail writes:** reopen when they leave beta, by exercising them on a scratch genie
  including their `422` validation errors.
- **Matrix KB data sources:** offer them once the valid source names are known.
- **Genie `state` values beyond the four observed.** `start`/`stop` check for `active`/`inactive`.
- **Populated conversation events** haven't been observed live; the types follow the reference.
- **Whether `clients regenerate` invalidates the old key at once** is per the reference, not
  observed.

---

## Internal Action Required

Platform dependencies, following ADR-003's precedent:

1. **Reject what the API currently ignores:** unknown IDs on `assign_*`/`remove_*`; an unknown KB
   `source_type`; a blank KB name or description; KB `data_sources` it doesn't apply; redirect URLs
   on an `api_key` client. Also make a KB description clearable.
2. **Add `DELETE /api/agentic/skills/:id`**, and decide what happens to a skill when its recipe is
   deleted.
3. **Correct the reference:** client cardinality (a genie has several clients; a client serves
   one genie); required fields on genie and KB create, and their defaults; KB `folder_id` isn't
   updatable; update/delete response bodies; `data_sources` is an array; `api_key` clients get no
   `oauth_client_id`; deleting a genie deletes its clients. Document `GET
   /agentic/genies/clients/:id`, client `ip_allow_list` and update's `oauth_redirect_url`, plus the
   endpoints the Client Role editor lists that the reference omits: client start/pause, genie
   interfaces, and knowledge base documents.
4. **Enumerate the genie `state` vocabulary** and say whether list and get are meant to differ;
   make the list `state` filter reject unknown values.
5. **Consider guarding deletes server-side:** a KB or client attached to a running genie, and a
   genie's clients on genie delete. Re-attaching a client to its own genie should succeed or say
   so, not return `409 "Genie has already been taken"`.
