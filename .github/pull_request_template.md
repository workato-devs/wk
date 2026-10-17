## Summary

<!-- What does this change do, and why? Link any related issue (e.g. Closes #NN). -->

## Checklist

- [ ] `go build ./...` and `go test ./...` pass
- [ ] Docs / `CHANGELOG.md` updated as needed
- [ ] **Command reference check:** if this PR adds, removes, or renames a command/subcommand, or
      adds/removes a flag worth surfacing at a glance, I updated
      [docs/command-reference.md](../docs/command-reference.md) to match. `go test ./...` fails on
      a missing or stale command or global flag (`TestCommandReference*`); the descriptions and
      per-command flags are still yours to check. (N/A if the command surface didn't change.)
- [ ] **Coverage check:** if this PR changes which API operations `wk` wires, I updated
      [docs/known-limitations.md](../docs/known-limitations.md) so each Client Role permission's
      row and grade still match. (N/A if API coverage didn't change.)
- [ ] **ADR check:** if this changes behavior documented in an ADR (`docs/adrs/ADR-*.md`), I amended that
      ADR in place — a dated amendment that preserves the original text — in this PR, and recorded
      myself on `Amended-by` (not `Author(s)`).
      See [docs/adrs/ADR-000-how-we-use-adrs.md](../docs/adrs/ADR-000-how-we-use-adrs.md). (N/A if no ADR
      covers this change.)
