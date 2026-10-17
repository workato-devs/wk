package commands

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

func newConnectorsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "connectors",
		Aliases: []string{"connector"},
		Short:   "Manage custom SDK connectors (use 'wk connections' for workspace connections)",
	}
	cmd.AddCommand(newConnectorsListCmd())
	return cmd
}

func newConnectorsListCmd() *cobra.Command {
	var search string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List custom SDK connectors",
		Example: `  wk connectors list
  wk connectors list --search taxonomy --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}

			connectors, err := client.Connectors().List(cmd.Context(), search)
			if err != nil {
				return err
			}

			if flagJSON {
				return rctx.Formatter.Format(os.Stdout, connectors)
			}

			headers := []string{"ID", "NAME", "TITLE", "RELEASED"}
			var rows [][]string
			for _, c := range connectors {
				released := "never"
				if c.LatestReleasedVersion != nil {
					released = fmt.Sprintf("v%d", *c.LatestReleasedVersion)
				}
				rows = append(rows, []string{strconv.Itoa(c.ID), c.Name, c.Title, released})
			}
			return rctx.Formatter.FormatList(os.Stdout, connectors, headers, rows)
		},
	}

	cmd.Flags().StringVar(&search, "search", "", "Filter by name or title (case-insensitive substring)")
	return cmd
}
