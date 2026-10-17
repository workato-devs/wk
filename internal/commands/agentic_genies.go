package commands

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/workato-devs/wk/internal/api"
	"github.com/workato-devs/wk/internal/output"
)

// stateNote explains the genie state vocabularies (ADR-009 Decision 3).
const stateNote = `The genie API reports state differently by endpoint, and wk shows each as
reported: list says "stopped"/"started", get says "inactive"/"active", for the
same genie. The --state filter takes the get vocabulary: active or inactive.`

// genieStateFilter validates --state. The API filters only on active/inactive
// and silently ignores any other value (returning every genie), so wk rejects
// the rest rather than let a filter quietly not filter. Verified 2026-10-16:
// active matched exactly the genies list shows as "started", inactive exactly
// the "stopped" ones.
func genieStateFilter(v string) error {
	switch v {
	case "", "active", "inactive":
		return nil
	case "started":
		return fmt.Errorf(`--state started isn't a filter value; use --state active (list shows active genies as "started")`)
	case "stopped":
		return fmt.Errorf(`--state stopped isn't a filter value; use --state inactive (list shows inactive genies as "stopped")`)
	default:
		return fmt.Errorf("unknown --state %q; valid: active, inactive", v)
	}
}

func newGeniesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "genies",
		Aliases: []string{"genie"},
		Short:   "Manage Agent Studio genies",
	}
	cmd.AddCommand(newGeniesListCmd())
	cmd.AddCommand(newGeniesGetCmd())
	addGenieWriteCmds(cmd)
	return cmd
}

func newGeniesListCmd() *cobra.Command {
	var (
		folderID, projectID, state string
		page, perPage              int
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List genies",
		Long:  "List the workspace's genies.\n\n" + stateNote,
		Example: `  wk agentic genies list
  wk agentic genies list --state active --json
  wk agentic genies list --folder-id 12345 --page 2 --per-page 50`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := genieStateFilter(state); err != nil {
				return err
			}
			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}
			genies, pg, err := client.Genies().List(cmd.Context(), &api.GenieListOptions{
				FolderID: folderID, ProjectID: projectID, State: state, Page: page, PerPage: perPage,
			})
			if err != nil {
				return err
			}

			headers := []string{"ID", "NAME", "STATE", "AI MODEL", "SKILLS", "KBS", "FOLDER ID"}
			rows := make([][]string, 0, len(genies))
			for _, g := range genies {
				rows = append(rows, []string{
					g.ID, g.Name, g.State, g.AIModel,
					strconv.Itoa(g.SkillsCount), strconv.Itoa(g.KnowledgeBasesCount), strconv.Itoa(g.FolderID),
				})
			}
			meta := output.PageMeta{Page: pg.Page, PerPage: pg.PerPage, HasNext: pg.HasNext()}
			if !flagJSON && pg.HasNext() && !rctx.Quiet {
				defer fmt.Fprintf(os.Stderr, "Showing %d of %d; next page: --page %d\n", len(genies), pg.Total, pg.Page+1)
			}
			if genies == nil {
				genies = []api.Genie{}
			}
			return rctx.Formatter.FormatPage(os.Stdout, genies, headers, rows, meta)
		},
	}
	cmd.Flags().StringVar(&folderID, "folder-id", "", "Only genies in this folder")
	cmd.Flags().StringVar(&projectID, "project-id", "", "Only genies in this project")
	cmd.Flags().StringVar(&state, "state", "", "Only genies in this state: active or inactive (list output calls these started and stopped)")
	cmd.Flags().IntVar(&page, "page", 0, "Page number (starts at 1)")
	cmd.Flags().IntVar(&perPage, "per-page", 0, "Genies per page (max 50)")
	return cmd
}

func newGeniesGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get a genie by ID",
		Long:  "Get one genie, including the fields only the detail endpoint returns\n(project, folders, user groups, model label, provider).\n\n" + stateNote,
		Example: `  wk agentic genies get gin-AbzGALWh-9h9Q9W-AB
  wk agentic genies get gin-AbzGALWh-9h9Q9W-AB --json`,
		Args: requireArgs(1, "genie ID is required, e.g.: wk agentic genies get <id> (IDs look like gin-…; see wk agentic genies list)"),
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}
			g, err := client.Genies().Get(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if flagJSON {
				return rctx.Formatter.Format(os.Stdout, g)
			}
			headers := []string{"ID", "NAME", "STATE", "AI MODEL", "SKILLS", "KBS", "USER GROUPS", "FOLDER ID", "PROJECT ID"}
			rows := [][]string{{
				g.ID, g.Name, g.State, g.AIModelLabel,
				strconv.Itoa(g.SkillsCount), strconv.Itoa(g.KnowledgeBasesCount), strconv.Itoa(g.UserGroupsCount),
				strconv.Itoa(g.FolderID), strconv.Itoa(g.ProjectID),
			}}
			return rctx.Formatter.FormatList(os.Stdout, g, headers, rows)
		},
	}
}
