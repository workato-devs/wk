# ADR-010: Telemetry — Anonymous Usage Collection, Device Identity, and Opt-Out

**Author(s):** Chris Miller, Kiro [role: assistant; harness: Kiro IDE; model: Claude Sonnet 4.6]
**Status:** Proposed
**Date:** November 13, 2026
**References:** ADR-001 (Foundational Architecture), ADR-006 (Profile Identity Model), [Workato Labs guiding principles](https://workato-devs.github.io/labs/principles.html) — chiefly *"ship fast, learn in the open"* and *"developer experience is a first-class feature"*

---

## Context

`wk` has no usage signal. Decisions about which commands to invest in, which failure modes are
common, and which features land in CI-heavy workflows versus interactive development are made from
anecdote and internal dogfooding. That's fine early on; it doesn't scale as external adoption grows.

The design challenge is genuine: `wk` has no mandatory login (a profile exists but can be created
without touching a Workato workspace), so there is no authenticated user to associate events with.
Events arrive from shared IPs (offices, CI providers), so IP address alone can't distinguish 50
developers on the same VPN from 50 runs of the same pipeline. And the caller is probably an agent
or a CI script, not a human who reads a first-run consent flow — the caller moves on and may never
see it.

There is also a legal dimension. Even a randomly-generated UUID is arguably personal data under
GDPR Article 4 §1 if it can be linked to a natural person. The design has to treat device
identifiers as personal data in scope of that definition and act accordingly.

The question is therefore: **what is the minimum useful signal, generated in a way that is honest
about what it is, and that can be switched off with no friction?**

---

## Decision

### 1. Collect command-level usage events, not values

Each event records what the user **did**, not what they did **with**:

| Collected | Not collected |
|-----------|---------------|
| Subcommand invoked (`wk push`, `wk agentic genies list`) | Flag values (profile name, folder path, project name) |
| Flags present (`--json`, `--quiet`, `--force`) | Flag values |
| CLI version | |
| OS and architecture (macOS/Linux/Windows, arm64/x86_64) | File paths, project names |
| Number of CPUs | |
| Whether running in CI (see Decision 4) | Env var names or values |
| Command exit code (0 / non-zero) | Error message text |
| Command duration (ms) | Stack traces |
| Go runtime version | Auth tokens, secrets |

This is intentionally narrow. No file contents, no paths, no error message strings (which may
contain user-supplied values), no credentials. The event shape is typed fields, not freeform
strings, so there is no path for accidental leakage.

### 2. Identify devices with a hashed machine ID

The goal is to distinguish 50 developers on the same office network from 50 runs of one CI
pipeline — something a session UUID or IP address can't reliably do. The approach:

1. Read the OS-provided machine identifier:
   - **Linux:** `/var/lib/dbus/machine-id` or `/etc/machine-id`
   - **macOS:** `IOPlatformUUID` from `ioreg`
   - **Windows:** `HKLM\SOFTWARE\Microsoft\Cryptography\MachineGuid`
2. Compute `HMAC-SHA256(key="wk-v1", message=<raw_machine_id>)`.
3. Send the hex digest as `device_id`. Never send the raw machine ID.

The HMAC key is application-specific. Two tools seeing the same machine produce different digests,
so `wk`'s telemetry IDs can't be correlated with another tool's even if both use this technique.
The raw ID is not recoverable from the digest. The ID is stable across reinstalls, unlike a
generated UUID written to a config file.

If the machine ID is unavailable or unreadable (some hardened or ephemeral environments), `wk`
falls back to a UUID generated once and stored in `~/.wk/telemetry.json`. That UUID is
regenerated on reinstall but is better than dropping the event entirely.

Each event also carries a **session ID** — a UUID4 generated once per process invocation — to
correlate the start and completion of the same command run. The session ID is never persisted.

### 3. Opt-out is easy, standard, and respected before any event fires

Three opt-out surfaces, checked in order before any telemetry code runs:

1. **`DO_NOT_TRACK=1`** — the [Console Do Not Track](https://consoledonottrack.com/) standard,
   honored by Git, Homebrew, and others. If set to `1`, telemetry is suppressed.
2. **`WK_TELEMETRY_DISABLED=1`** — `wk`-specific env var. Useful in CI where `DO_NOT_TRACK`
   isn't set globally.
3. **`wk telemetry disable`** — writes a flag to `~/.wk/telemetry.json`; persists across shell
   sessions. `wk telemetry enable` and `wk telemetry status` round out the subcommand.

All three are equivalent: any one of them suppresses all events. Environment variables are checked
first so CI environments can suppress telemetry without touching `~/.wk/`.

### 4. CI events are tagged, not suppressed

`wk` detects CI by checking a standard set of environment variables (`CI`, `GITHUB_ACTIONS`,
`GITLAB_CI`, `CIRCLECI`, `BUILDKITE`, `CI_NAME`, etc.). When running in CI, events carry
`"ci": true`. They are not suppressed: CI usage is real signal — which commands run in pipelines
versus interactively is worth knowing. The `ci` field lets us separate them in analysis.

### 5. First-run disclosure on interactive sessions only

When telemetry is enabled and the ack flag in `~/.wk/telemetry.json` is absent or false, `wk`
prints a one-time notice to stderr **if and only if stdin is a terminal** (`isatty(0)`). The
notice names what is collected, links to the full policy, and gives the opt-out command:

```
Attention: wk collects anonymous usage telemetry.
Run `wk telemetry status` to see what is collected, or `wk telemetry disable` to opt out.
```

It then writes `{"ack": true}` to `~/.wk/telemetry.json` and does not print again. CI and
non-interactive invocations never see the notice — the caller is a script, not a person.

### 6. Debug mode: inspect without sending

`WK_TELEMETRY_DEBUG=1` dumps each would-be event as JSON to stderr and suppresses the network
send. This is the transparency mechanism for anyone who wants to verify exactly what `wk` would
emit before trusting the written policy.

### 7. Data handling

- Events are sent over HTTPS to a Workato-controlled ingest endpoint.
- No event payload contains a value that identifies a person or a workspace.
- Aggregated data only is used for roadmap and feature prioritization decisions.
- The `device_id` HMAC digest is not linked to any authenticated identity server-side.
- Retention, deletion rights, and the full data inventory are governed by Workato's privacy policy;
  this ADR does not substitute for it.

---

## Alternatives considered

### Generated UUID written to `~/.wk/` (rejected as the primary identifier)

This is what AWS CDK and many others do, and it is simpler. The problem is that it doesn't survive
reinstalls and doesn't distinguish two machines sharing a home directory. The hashed machine ID
does both. The generated UUID is kept as a fallback for environments where the machine ID is
inaccessible (see Decision 2).

### Raw machine ID sent as the identifier (rejected)

A MAC address or platform UUID is directly linkable to hardware. Even hashed with SHA-256 alone
it's reversible by enumeration (the space of real MAC addresses is small). HMAC with an
application-specific key is the standard countermeasure.

### Account-linked IDs using the authenticated profile (rejected)

A profile exists when `wk auth login` has run, and it carries a workspace ID. Linking events to
that identifier would be the most precise signal — but it requires auth and explicitly ties
telemetry to a workspace identity. The hashed machine ID achieves device-level disambiguation
without that linkage. If a future ADR adds workspace-linked analytics as an opt-in feature for
workspace admins, that is its own decision.

### Opt-in rather than opt-out (rejected)

Opt-in means no signal until a user affirmatively enables it, which in practice means near-zero
signal from CI and agent-driven workflows. The Labs principle is to *ship fast and learn in the
open*. Opt-out with a clear first-run notice is the standard adopted by Next.js, Angular CLI, and
the .NET SDK — it balances real signal against user control.

### Suppress telemetry in CI entirely (rejected)

CI is a first-class usage context for `wk`, and "which commands run in pipelines" is valuable
signal. Suppressing it hides usage patterns that differ significantly from interactive use. Tagging
with `"ci": true` lets us separate the populations in analysis without discarding either.

---

## Consequences

### What becomes easier

- **Data-driven prioritization.** Which commands are actually used, which flags are common, where
  do pipelines spend time — answers that are currently guesses.
- **CI vs. interactive visibility.** Separate the two populations without discarding either.
- **Detecting adoption across machines.** The HMAC device ID gives a reasonable proxy for distinct
  developers even on shared networks.

### What becomes harder

- **Privacy compliance.** Even a pseudonymous `device_id` is personal data under strict readings of
  GDPR. The HMAC-with-application-key construction is the right countermeasure, but it requires
  care in the ingest pipeline (no joins with identity, retention limits) that this ADR can't
  enforce alone.
- **Maintaining the event schema.** As commands are added, telemetry coverage needs to follow.
  An untested command silently emits no events.
- **User trust.** Any telemetry is a trust question. The opt-out must be easy and the debug mode
  must be honest; if what `WK_TELEMETRY_DEBUG=1` shows ever diverges from what is actually sent,
  trust is gone.

### What we'll need to revisit

- **Whether `DO_NOT_TRACK` is the right standard.** It has broad adoption in developer tools but
  is not a formal spec. If a competing standard emerges, revisit.
- **Ingest endpoint and retention policy.** This ADR sets the shape of the data; the pipeline that
  receives it, stores it, and deletes it is out of scope here.
- **Workspace-linked opt-in analytics.** A future feature for workspace admins who want aggregate
  pipeline stats tied to their workspace — explicitly opted into, not this telemetry scheme.
- **GDPR and other jurisdiction analysis.** The HMAC construction addresses the most common
  concern; a formal legal review against GDPR, CCPA, and applicable regulations should happen
  before the ingest pipeline goes live in production regions.
