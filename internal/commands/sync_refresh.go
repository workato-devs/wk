package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/workato-devs/wk-cli-beta/internal/config"
	wkerrors "github.com/workato-devs/wk-cli-beta/internal/errors"
	"github.com/workato-devs/wk-cli-beta/internal/sync"
)

// newSyncRefreshCmd reconciles cached folder_id values in wk.toml
// against the current workspace state (ADR-007 Decision 11). It is the
// postcommit companion to `wk init --verify` — same hierarchy walk,
// same cache write-through. Cache validation is walk-based (the
// Workato folders API has no single-folder-by-ID endpoint): each
// entry's server_path is resolved and the result compared against
// the cached folder_id.
//
// Exits zero even when some entries are not-found — partial results
// are the useful outcome. --prune opts into removing the not-found
// entries from wk.toml (interactive confirm or --yes).
func newSyncRefreshCmd() *cobra.Command {
	var (
		flagPrune bool
		flagYes   bool
	)

	cmd := &cobra.Command{
		Use:   "refresh",
		Short: "Reconcile cached folder_id values against the workspace",
		Long: `Walk every [[sync]] entry in wk.toml and classify it against current
server state:

  found       uncached entry — walk succeeded; cache populated with the
              resolved folder_id (first-time lookup, no drift involved)
  current     cached entry — walk returns the same folder_id; no change
  repaired    cached entry — walk returned a different folder_id; the
              cached value was wrong (renamed folder, recreated with a
              new ID, etc.) and has been rewritten to the fresh ID
  not-found   walk failed — server_path does not describe a real folder.
              Entries that had a cached folder_id still show it in the
              output so "used to work" stays distinct from "never
              worked". --prune removes.

Runs without push/pull traffic — folder-list API only. Partial results
are a useful outcome, so refresh exits zero even when some entries are
not-found. Use 'wk sync list' first if you only want to see cache
state.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}
			if rctx.Config == nil {
				return wkerrors.ErrNotInProject
			}
			if len(rctx.Config.Sync) == 0 {
				if !rctx.Quiet && !flagJSON {
					fmt.Fprintln(os.Stderr, "No [[sync]] entries to refresh.")
				}
				if flagJSON {
					return rctx.Formatter.Format(os.Stdout, []sync.RefreshResult{})
				}
				return nil
			}

			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}

			engine := sync.NewSyncEngine(rctx.ProjectRoot, rctx.Config, client)
			// Memoize GET /folders?parent_id=... for the duration of this
			// sweep — N entries in the same workspace share folder lists at
			// most parent_ids, so one fetch per unique parent beats N
			// fetches of the same data.
			engine.EnableFolderListCache()

			results := make([]sync.RefreshResult, 0, len(rctx.Config.Sync))
			entries := make([]config.SyncEntry, len(rctx.Config.Sync))
			copy(entries, rctx.Config.Sync)
			for _, entry := range entries {
				r, err := engine.ClassifyEntry(cmd.Context(), entry)
				if err != nil {
					return err
				}
				results = append(results, r)
			}

			var foundCount, currentCount, repairedCount, notFoundCount int
			for _, r := range results {
				switch r.State {
				case sync.RefreshStateFound:
					foundCount++
				case sync.RefreshStateCurrent:
					currentCount++
				case sync.RefreshStateRepaired:
					repairedCount++
				case sync.RefreshStateNotFound:
					notFoundCount++
				}
			}

			// Any "found" or "repaired" classifications mutated
			// rctx.Config.Sync in place via ClassifyEntry's write-through.
			// Persist once per sweep.
			if foundCount+repairedCount > 0 {
				if err := config.Save(config.ProjectConfigPath(rctx.ProjectRoot), rctx.Config); err != nil {
					return fmt.Errorf("saving wk.toml: %w", err)
				}
			}

			// Prune runs before the report so the JSON branch returns the
			// full classification unchanged, but the human-readable prune
			// summary line is deferred until AFTER the report so the
			// ordering reads "classification → summary → prune outcome"
			// rather than "prune outcome → classification" (which is
			// confusing when the report still shows pruned rows).
			pruneRemoved := 0
			if flagPrune && notFoundCount > 0 {
				if !flagYes {
					if err := confirmPrune(cmd, results); err != nil {
						return err
					}
				}
				pruneRemoved = pruneEntries(rctx.Config, results)
				if err := config.Save(config.ProjectConfigPath(rctx.ProjectRoot), rctx.Config); err != nil {
					return fmt.Errorf("saving wk.toml after prune: %w", err)
				}
			}

			if flagJSON {
				return rctx.Formatter.Format(os.Stdout, results)
			}

			fmt.Fprintf(os.Stdout, "Refreshed %d sync entr%s:\n", len(results), pluralY(len(results)))
			for _, r := range results {
				switch r.State {
				case sync.RefreshStateFound:
					fmt.Fprintf(os.Stdout, "  found       %-40s  (folder_id=%d)\n", r.ServerPath, r.FolderID)
				case sync.RefreshStateCurrent:
					fmt.Fprintf(os.Stdout, "  current     %-40s  (folder_id=%d)\n", r.ServerPath, r.FolderID)
				case sync.RefreshStateRepaired:
					fmt.Fprintf(os.Stdout, "  repaired    %-40s  (folder_id=%d; %s)\n", r.ServerPath, r.FolderID, r.Message)
				case sync.RefreshStateNotFound:
					if r.FolderID != 0 {
						fmt.Fprintf(os.Stdout, "  not-found   %-40s  (cached folder_id=%d; %s)\n", r.ServerPath, r.FolderID, r.Message)
					} else {
						fmt.Fprintf(os.Stdout, "  not-found   %-40s  (%s)\n", r.ServerPath, r.Message)
					}
				}
			}
			fmt.Fprintf(os.Stdout, "\nSummary: %d found, %d current, %d repaired, %d not-found.\n",
				foundCount, currentCount, repairedCount, notFoundCount)
			if pruneRemoved > 0 && !rctx.Quiet {
				fmt.Fprintf(os.Stdout, "Pruned %d not-found entr%s from wk.toml.\n",
					pruneRemoved, pluralY(pruneRemoved))
			}
			if notFoundCount > 0 && !flagPrune {
				verb := "needs"
				if notFoundCount != 1 {
					verb = "need"
				}
				fmt.Fprintf(os.Stdout, "%d not-found entr%s %s attention. Run `wk sync refresh --prune` to remove.\n",
					notFoundCount, pluralY(notFoundCount), verb)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&flagPrune, "prune", false,
		"Remove not-found entries from wk.toml (requires confirmation or --yes)")
	cmd.Flags().BoolVar(&flagYes, "yes", false,
		"Skip the interactive confirmation prompt when --prune would remove entries")
	return cmd
}

// confirmPrune lists the entries that --prune will remove and asks the
// user to confirm. Errors if stdin is not a TTY — CI callers must pass
// --yes to opt in without a prompt.
func confirmPrune(cmd *cobra.Command, results []sync.RefreshResult) error {
	var toRemove []sync.RefreshResult
	for _, r := range results {
		if r.State == sync.RefreshStateNotFound {
			toRemove = append(toRemove, r)
		}
	}
	if len(toRemove) == 0 {
		return nil
	}

	if !isInteractiveStdin() || flagJSON || flagNoInput {
		return fmt.Errorf("--prune would remove %d entr%s; pass --yes to confirm in non-interactive mode",
			len(toRemove), pluralY(len(toRemove)))
	}

	fmt.Fprintf(cmd.OutOrStdout(), "The following %d entr%s will be removed from wk.toml:\n",
		len(toRemove), pluralY(len(toRemove)))
	for _, r := range toRemove {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s  (%s)\n", r.ServerPath, r.State)
	}
	fmt.Fprint(cmd.OutOrStdout(), "Proceed? [y/N]: ")
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.ToLower(strings.TrimSpace(answer))
	if answer != "y" && answer != "yes" {
		return fmt.Errorf("aborted: no entries removed")
	}
	return nil
}

// pruneEntries drops not-found entries from cfg.Sync and returns the
// number removed. Matches on (server_path, local_path) so entries with
// duplicate server_paths (possible under Decision 5 rule 3) are
// disambiguated correctly.
func pruneEntries(cfg *config.Config, results []sync.RefreshResult) int {
	remove := make(map[string]bool, len(results))
	for _, r := range results {
		if r.State == sync.RefreshStateNotFound {
			remove[r.ServerPath+"\x00"+r.LocalPath] = true
		}
	}
	kept := cfg.Sync[:0]
	removed := 0
	for _, e := range cfg.Sync {
		if remove[e.ServerPath+"\x00"+e.LocalPath] {
			removed++
			continue
		}
		kept = append(kept, e)
	}
	cfg.Sync = kept
	return removed
}
