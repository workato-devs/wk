package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/workato-devs/wk/internal/version"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the wk CLI version",
		Example: `  wk version
  wk version --json`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}

			info := map[string]string{
				"version": version.Version(),
				"commit":  version.Commit(),
				"date":    version.Date(),
			}

			if flagJSON {
				return rctx.Formatter.Format(cmd.OutOrStdout(), info)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "wk %s\n", version.Version())
			return nil
		},
	}
}
