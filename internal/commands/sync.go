package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/workato-devs/wk-cli-beta/internal/config"
	wkerrors "github.com/workato-devs/wk-cli-beta/internal/errors"
	"github.com/workato-devs/wk-cli-beta/internal/sync"
)

// resolveSyncEntry finds the matching sync entry in config.
// If folder is empty, returns the first entry.
func resolveSyncEntry(cfg *config.Config, folder string) (config.SyncEntry, error) {
	if len(cfg.Sync) == 0 {
		return config.SyncEntry{}, wkerrors.ErrNoSyncEntries
	}
	if folder == "" {
		return cfg.Sync[0], nil
	}
	for _, entry := range cfg.Sync {
		if entry.ServerPath == folder || entry.LocalPath == folder {
			return entry, nil
		}
	}
	return config.SyncEntry{}, fmt.Errorf("no sync entry matching %q", folder)
}

func newPullCmd() *cobra.Command {
	var (
		flagFolder string
		flagForce  bool
	)

	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Pull remote assets to local project",
		Long:  "Download assets from the Workato workspace into the local project directory.",
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}
			if rctx.Config == nil {
				return wkerrors.ErrNotInProject
			}

			entry, err := resolveSyncEntry(rctx.Config, flagFolder)
			if err != nil {
				return err
			}

			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}

			engine := sync.NewSyncEngine(rctx.ProjectRoot, rctx.Config, client)
			results, err := engine.Pull(entry, flagForce)
			if err != nil {
				return err
			}

			if len(results) == 0 {
				if !rctx.Quiet {
					fmt.Fprintln(os.Stderr, "No assets to pull.")
				}
				return nil
			}

			headers := []string{"FILE", "ACTION"}
			var rows [][]string
			for _, r := range results {
				rows = append(rows, []string{r.FilePath, r.Action})
			}
			return rctx.Formatter.FormatList(os.Stdout, headers, rows)
		},
	}

	cmd.Flags().StringVar(&flagFolder, "folder", "", "Sync entry filter (server_path or local_path)")
	cmd.Flags().BoolVar(&flagForce, "force", false, "Overwrite local modifications")

	return cmd
}

func newPushCmd() *cobra.Command {
	var (
		flagFolder        string
		flagDryRun        bool
		flagPreserveState bool
	)

	cmd := &cobra.Command{
		Use:   "push",
		Short: "Push local changes to remote workspace",
		Long:  "Upload modified local assets to the Workato workspace.",
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}
			if rctx.Config == nil {
				return wkerrors.ErrNotInProject
			}

			entry, err := resolveSyncEntry(rctx.Config, flagFolder)
			if err != nil {
				return err
			}

			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}

			engine := sync.NewSyncEngine(rctx.ProjectRoot, rctx.Config, client)
			results, err := engine.Push(entry, flagDryRun, flagPreserveState)
			if err != nil {
				return err
			}

			if len(results) == 0 {
				if !rctx.Quiet {
					fmt.Fprintln(os.Stderr, "No changes to push.")
				}
				return nil
			}

			if flagDryRun && !rctx.Quiet {
				fmt.Fprintln(os.Stderr, "Dry run -- no changes were pushed.")
			}

			headers := []string{"FILE", "ACTION"}
			var rows [][]string
			for _, r := range results {
				rows = append(rows, []string{r.FilePath, r.Action})
			}
			return rctx.Formatter.FormatList(os.Stdout, headers, rows)
		},
	}

	cmd.Flags().StringVar(&flagFolder, "folder", "", "Sync entry filter (server_path or local_path)")
	cmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Show what would be pushed without uploading")
	cmd.Flags().BoolVar(&flagPreserveState, "preserve-state", true, "Preserve recipe active state on import")

	return cmd
}

func newStatusCmd() *cobra.Command {
	var flagFolder string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show sync status of the current project",
		Long:  "Show which local files are new, modified, or deleted compared to the last pull.",
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}
			if rctx.Config == nil {
				return wkerrors.ErrNotInProject
			}

			entry, err := resolveSyncEntry(rctx.Config, flagFolder)
			if err != nil {
				return err
			}

			// Status is local-only; no API client needed.
			engine := sync.NewSyncEngine(rctx.ProjectRoot, rctx.Config, nil)
			statuses, err := engine.Status(entry)
			if err != nil {
				return err
			}

			if len(statuses) == 0 {
				if !rctx.Quiet {
					fmt.Fprintln(os.Stderr, "No synced assets found.")
				}
				return nil
			}

			headers := []string{"FILE", "STATUS", "SERVER PATH"}
			var rows [][]string
			for _, s := range statuses {
				rows = append(rows, []string{s.FilePath, string(s.Status), s.ServerPath})
			}
			return rctx.Formatter.FormatList(os.Stdout, headers, rows)
		},
	}

	cmd.Flags().StringVar(&flagFolder, "folder", "", "Sync entry filter (server_path or local_path)")

	return cmd
}

func newDiffCmd() *cobra.Command {
	var flagFolder string

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Show differences between local and remote",
		Long:  "Compare local assets against the remote workspace to show what has changed.",
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}
			if rctx.Config == nil {
				return wkerrors.ErrNotInProject
			}

			entry, err := resolveSyncEntry(rctx.Config, flagFolder)
			if err != nil {
				return err
			}

			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}

			engine := sync.NewSyncEngine(rctx.ProjectRoot, rctx.Config, client)
			diffs, err := engine.Diff(entry)
			if err != nil {
				return err
			}

			if len(diffs) == 0 {
				if !rctx.Quiet {
					fmt.Fprintln(os.Stderr, "No differences found.")
				}
				return nil
			}

			headers := []string{"PATH", "STATUS", "LOCAL HASH", "REMOTE HASH"}
			var rows [][]string
			for _, d := range diffs {
				localHash := truncateHash(d.LocalHash)
				remoteHash := truncateHash(d.RemoteHash)
				rows = append(rows, []string{d.Path, string(d.Type), localHash, remoteHash})
			}
			return rctx.Formatter.FormatList(os.Stdout, headers, rows)
		},
	}

	cmd.Flags().StringVar(&flagFolder, "folder", "", "Sync entry filter (server_path or local_path)")

	return cmd
}

// truncateHash shortens a SHA256 hash for display (first 8 chars).
func truncateHash(h string) string {
	if len(h) > 8 {
		return h[:8]
	}
	return h
}
