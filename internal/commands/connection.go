package commands

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/workato-devs/wk-cli-beta/internal/api"
)

func newConnectionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "connections",
		Aliases: []string{"connection", "conn"},
		Short:   "Manage Workato connections",
	}
	cmd.AddCommand(newConnectionsListCmd())
	cmd.AddCommand(newConnectionsGetCmd())
	return cmd
}

func newConnectionsListCmd() *cobra.Command {
	var folderID int
	var application string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List connections",
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}

			opts := &api.ConnectionListOptions{}
			if cmd.Flags().Changed("folder") {
				opts.FolderID = &folderID
			}

			conns, err := client.Connections().List(cmd.Context(), opts)
			if err != nil {
				return err
			}

			if application != "" {
				var filtered []api.Connection
				for _, c := range conns {
					if strings.Contains(strings.ToLower(c.Application), strings.ToLower(application)) {
						filtered = append(filtered, c)
					}
				}
				conns = filtered
			}

			if flagJSON {
				return rctx.Formatter.Format(os.Stdout, conns)
			}

			headers := []string{"ID", "NAME", "APPLICATION", "STATUS"}
			var rows [][]string
			for _, c := range conns {
				status := "not connected"
				if c.AuthorizationStatus != nil && *c.AuthorizationStatus == "success" {
					status = "connected"
				} else if c.AuthorizationStatus != nil && *c.AuthorizationStatus != "" {
					status = *c.AuthorizationStatus
				}
				rows = append(rows, []string{
					strconv.Itoa(c.ID),
					c.Name,
					c.Application,
					status,
				})
			}
			return rctx.Formatter.FormatList(os.Stdout, headers, rows)
		},
	}

	cmd.Flags().IntVar(&folderID, "folder", 0, "Filter by folder ID")
	cmd.Flags().StringVar(&application, "application", "", "Filter by application name")
	return cmd
}

func newConnectionsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get connection details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}

			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid connection ID: %s", args[0])
			}

			conn, err := client.Connections().Get(cmd.Context(), id)
			if err != nil {
				return err
			}

			if !flagJSON {
				status := "not connected"
				if conn.AuthorizationStatus != nil && *conn.AuthorizationStatus == "success" {
					status = "connected"
				} else if conn.AuthorizationStatus != nil && *conn.AuthorizationStatus != "" {
					status = *conn.AuthorizationStatus
				}
				fmt.Fprintf(os.Stdout, "ID:          %d\n", conn.ID)
				fmt.Fprintf(os.Stdout, "Name:        %s\n", conn.Name)
				fmt.Fprintf(os.Stdout, "Application: %s\n", conn.Application)
				fmt.Fprintf(os.Stdout, "Folder ID:   %d\n", conn.FolderID)
				fmt.Fprintf(os.Stdout, "Status:      %s\n", status)
				fmt.Fprintf(os.Stdout, "Updated:     %s\n", conn.UpdatedAt.Format(time.RFC3339))
				return nil
			}
			return rctx.Formatter.Format(os.Stdout, conn)
		},
	}
}

