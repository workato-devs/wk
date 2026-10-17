package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/workato-devs/wk/internal/api"
	"github.com/workato-devs/wk/internal/output"
)

// Agent Studio read commands (ADR-009 Decision 1): knowledge bases, genie
// clients, guardrails, and conversation logs. Genies are in agentic_genies.go.

func newKBCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "kb",
		Aliases: []string{"knowledge-bases"},
		Short:   "Read Agent Studio knowledge bases",
	}
	cmd.AddCommand(newKBListCmd(), newKBGetCmd(), newKBRecipesCmd(), newKBDataSourcesCmd())
	return cmd
}

func newKBListCmd() *cobra.Command {
	var folderID, projectID, query string
	var page, perPage int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List knowledge bases",
		Example: `  wk agentic kb list
  wk agentic kb list --query helpdesk --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, client, err := agenticContext(cmd)
			if err != nil {
				return err
			}
			kbs, pg, err := client.KnowledgeBases().List(cmd.Context(), &api.KBListOptions{
				FolderID: folderID, ProjectID: projectID, Query: query, Page: page, PerPage: perPage,
			})
			if err != nil {
				return err
			}
			headers := []string{"ID", "NAME", "SOURCE", "RECIPES", "GENIES", "FOLDER ID"}
			rows := make([][]string, 0, len(kbs))
			for _, kb := range kbs {
				rows = append(rows, kbRow(kb))
			}
			if kbs == nil {
				kbs = []api.KnowledgeBase{}
			}
			return formatOffsetPage(rctx, kbs, headers, rows, pg, len(kbs))
		},
	}
	cmd.Flags().StringVar(&folderID, "folder-id", "", "Only knowledge bases in this folder")
	cmd.Flags().StringVar(&projectID, "project-id", "", "Only knowledge bases in this project")
	cmd.Flags().StringVar(&query, "query", "", "Search text")
	addPageFlags(cmd, &page, &perPage, 50)
	return cmd
}

func kbRow(kb api.KnowledgeBase) []string {
	return []string{kb.ID, kb.Name, kb.SourceType, strconv.Itoa(kb.ActiveRecipesCount), strconv.Itoa(kb.GeniesCount), strconv.Itoa(kb.FolderID)}
}

func newKBGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "get <id>",
		Short:   "Get a knowledge base by ID",
		Example: `  wk agentic kb get kb-AbJzNhoC-krNnNo-AB --json`,
		Args:    requireArgs(1, "knowledge base ID is required, e.g.: wk agentic kb get <id> (IDs look like kb-…; see wk agentic kb list)"),
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, client, err := agenticContext(cmd)
			if err != nil {
				return err
			}
			kb, err := client.KnowledgeBases().Get(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if flagJSON {
				return rctx.Formatter.Format(os.Stdout, kb)
			}
			headers := []string{"ID", "NAME", "SOURCE", "RECIPES", "GENIES", "FOLDER ID"}
			return rctx.Formatter.FormatList(os.Stdout, kb, headers, [][]string{kbRow(*kb)})
		},
	}
}

const unverifiedShape = "No populated response has been observed yet, so wk shows each item as the API returns it."

func newKBRecipesCmd() *cobra.Command {
	var page, perPage int
	cmd := &cobra.Command{
		Use:   "recipes <id>",
		Short: "List a knowledge base's recipes",
		Long:  "List the recipes that feed a knowledge base.\n\n" + unverifiedShape,
		Args:  requireArgs(1, "knowledge base ID is required, e.g.: wk agentic kb recipes <id>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, client, err := agenticContext(cmd)
			if err != nil {
				return err
			}
			recipes, pg, err := client.KnowledgeBases().Recipes(cmd.Context(), args[0], &api.PaginationOptions{Page: page, PerPage: perPage})
			if err != nil {
				return err
			}
			if recipes == nil {
				recipes = []json.RawMessage{}
			}
			rows := make([][]string, 0, len(recipes))
			for _, r := range recipes {
				rows = append(rows, []string{compactJSON(r)})
			}
			return formatOffsetPage(rctx, recipes, []string{"RECIPE (as returned)"}, rows, pg, len(recipes))
		},
	}
	addPageFlags(cmd, &page, &perPage, 50)
	return cmd
}

func newKBDataSourcesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "data-sources <id>",
		Short: "Show a Matrix knowledge base's data sources",
		Long:  "Show a Matrix knowledge base's data sources. Recipe knowledge bases have none (the API answers 400).\n\n" + unverifiedShape,
		Args:  requireArgs(1, "knowledge base ID is required, e.g.: wk agentic kb data-sources <id>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, client, err := agenticContext(cmd)
			if err != nil {
				return err
			}
			sources, err := client.KnowledgeBases().DataSources(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return rctx.Formatter.Format(os.Stdout, sources)
		},
	}
}

func newGenieClientsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clients",
		Short: "Read genie Headless API clients",
	}
	var page, perPage int
	list := &cobra.Command{
		Use:     "list",
		Short:   "List genie clients",
		Long:    "List the runtime credentials (API key or OAuth) used with the genie Headless API. A genie has at most one client.",
		Example: `  wk agentic clients list --json`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, client, err := agenticContext(cmd)
			if err != nil {
				return err
			}
			clients, pg, err := client.GenieClients().List(cmd.Context(), &api.PaginationOptions{Page: page, PerPage: perPage})
			if err != nil {
				return err
			}
			headers := []string{"CLIENT ID", "NAME", "TYPE", "STATUS", "GENIES"}
			rows := make([][]string, 0, len(clients))
			for _, c := range clients {
				rows = append(rows, []string{c.ClientID, c.ClientName, c.Type, c.Status, strings.Join(c.GenieIDs, ",")})
			}
			if clients == nil {
				clients = []api.GenieClient{}
			}
			return formatOffsetPage(rctx, clients, headers, rows, pg, len(clients))
		},
	}
	addPageFlags(list, &page, &perPage, 0)
	cmd.AddCommand(list)
	return cmd
}

func newGuardrailsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "guardrails",
		Short: "Read genie guardrails (beta)",
		Long: `Read genie guardrails. Guardrails are beta, and the endpoints answer 404 when
the workspace's guardrails feature is off. wk doesn't write guardrail
policies (ADR-009 Decision 1).`,
	}
	get := &cobra.Command{
		Use:     "get <genie-id>",
		Short:   "Show a genie's guardrail policies",
		Example: `  wk agentic guardrails get gin-AbfG4JXG-Emp9fD-AB --json`,
		Args:    requireArgs(1, "genie ID is required, e.g.: wk agentic guardrails get <genie-id>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, client, err := agenticContext(cmd)
			if err != nil {
				return err
			}
			g, err := client.Guardrails().Get(cmd.Context(), args[0])
			if err != nil {
				return guardrailsError(err)
			}
			if flagJSON {
				return rctx.Formatter.Format(os.Stdout, g)
			}
			if g.ID == nil {
				fmt.Fprintln(os.Stderr, "No guardrail is attached to this genie.")
				return nil
			}
			headers := []string{"POLICY TYPE", "POLICY ID", "CONFIGURATION"}
			rows := make([][]string, 0, len(g.Policies))
			for _, p := range g.Policies {
				rows = append(rows, []string{p.PolicyType, p.ID, compactJSON(p.Configuration)})
			}
			return rctx.Formatter.FormatList(os.Stdout, g, headers, rows)
		},
	}
	pii := &cobra.Command{
		Use:   "pii-entities",
		Short: "List the PII entity types a pii_detection policy can use",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, client, err := agenticContext(cmd)
			if err != nil {
				return err
			}
			entities, err := client.Guardrails().PIIEntities(cmd.Context())
			if err != nil {
				return guardrailsError(err)
			}
			rows := make([][]string, 0, len(entities))
			for _, e := range entities {
				rows = append(rows, []string{e})
			}
			if entities == nil {
				entities = []string{}
			}
			return rctx.Formatter.FormatList(os.Stdout, entities, []string{"ENTITY TYPE"}, rows)
		},
	}
	cmd.AddCommand(get, pii)
	return cmd
}

// guardrailsError explains a 404, which the API returns both for an unknown
// genie and when the guardrails feature flag is off.
func guardrailsError(err error) error {
	var apiErr *api.APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
		return fmt.Errorf("%w. A 404 here means either the genie doesn't exist (or isn't in a folder the profile can see), or guardrails aren't enabled in this workspace (they're beta)", err)
	}
	return err
}

func newLogsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Read genie conversation logs",
		Long:  "Read what a genie actually did: its conversations, and each conversation's events.",
	}
	cmd.AddCommand(newLogsConversationsCmd(), newLogsConversationCmd(), newLogsEventsCmd())
	return cmd
}

func newLogsConversationsCmd() *cobra.Command {
	var cursor, from, to string
	var limit int
	cmd := &cobra.Command{
		Use:   "conversations <genie-id>",
		Short: "List a genie's conversations",
		Example: `  wk agentic logs conversations gin-AbfG4JXG-Emp9fD-AB
  wk agentic logs conversations gin-AbfG4JXG-Emp9fD-AB --limit 100 --json`,
		Args: requireArgs(1, "genie ID is required, e.g.: wk agentic logs conversations <genie-id>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, client, err := agenticContext(cmd)
			if err != nil {
				return err
			}
			convs, cp, err := client.Conversations().List(cmd.Context(), args[0], &api.ConversationListOptions{
				Cursor: api.CursorOptions{After: cursor, Size: limit}, From: from, To: to,
			})
			if err != nil {
				return err
			}
			headers := []string{"CONVERSATION ID", "CREATED", "APP", "STARTED BY", "TOPIC"}
			rows := make([][]string, 0, len(convs))
			for _, c := range convs {
				rows = append(rows, []string{c.ID, c.CreatedAt.String(), c.AppType, c.StartedBy.Name, truncate(c.Topic, 60)})
			}
			if convs == nil {
				convs = []api.Conversation{}
			}
			return formatCursorPage(rctx, convs, headers, rows, cp)
		},
	}
	addCursorFlags(cmd, &cursor, &limit)
	cmd.Flags().StringVar(&from, "from", "", "Only conversations after this time (YYYY-MM-DDTHH:mm:ss.SSSZ)")
	cmd.Flags().StringVar(&to, "to", "", "Only conversations before this time (YYYY-MM-DDTHH:mm:ss.SSSZ)")
	return cmd
}

func newLogsConversationCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "conversation <genie-id> <conversation-id>",
		Short:   "Show one conversation, with its totals",
		Example: `  wk agentic logs conversation gin-AbfG4JXG-Emp9fD-AB conversation-01M3TREXNHYSPPCCJARM9GEZPV`,
		Args:    requireArgs(2, "genie ID and conversation ID are required, e.g.: wk agentic logs conversation <genie-id> <conversation-id>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, client, err := agenticContext(cmd)
			if err != nil {
				return err
			}
			c, err := client.Conversations().Get(cmd.Context(), args[0], args[1])
			if err != nil {
				return err
			}
			if flagJSON {
				return rctx.Formatter.Format(os.Stdout, c)
			}
			headers := []string{"CONVERSATION ID", "CREATED", "LAST MESSAGE", "STARTED BY", "ERRORS", "SKILL CALLS", "KB CALLS", "TOPIC"}
			rows := [][]string{{
				c.ID, c.CreatedAt.String(), c.LastMessageAt.String(), c.StartedBy.Name,
				strconv.Itoa(c.Totals.Errors.Count), strconv.Itoa(c.Totals.SkillCalls.Count), strconv.Itoa(c.Totals.KBCalls.Count),
				truncate(c.Topic, 50),
			}}
			return rctx.Formatter.FormatList(os.Stdout, c, headers, rows)
		},
	}
}

func newLogsEventsCmd() *cobra.Command {
	var cursor, order, runID string
	var limit int
	var eventTypes []string
	cmd := &cobra.Command{
		Use:   "events <genie-id> <conversation-id>",
		Short: "List a conversation's events",
		Long: `List a conversation's events: runs starting and finishing, user and agent
messages, guardrail evaluations, AI jobs. An event's data is a string, JSON for
most event types, and is shown as the API returns it (truncated in the table).`,
		Example: `  wk agentic logs events gin-AbfG4JXG-Emp9fD-AB conversation-01M3TREXNHYSPPCCJARM9GEZPV
  wk agentic logs events gin-AbfG4JXG-Emp9fD-AB conversation-01M3TREXNHYSPPCCJARM9GEZPV --event-type agent_message --json`,
		Args: requireArgs(2, "genie ID and conversation ID are required, e.g.: wk agentic logs events <genie-id> <conversation-id>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, client, err := agenticContext(cmd)
			if err != nil {
				return err
			}
			events, cp, err := client.Conversations().Events(cmd.Context(), args[0], args[1], &api.EventListOptions{
				Cursor: api.CursorOptions{After: cursor, Size: limit}, Order: order, EventTypes: eventTypes, GenieRunID: runID,
			})
			if err != nil {
				return err
			}
			headers := []string{"TIME", "EVENT", "RUN", "DATA"}
			rows := make([][]string, 0, len(events))
			for _, e := range events {
				data := ""
				if e.Data != nil {
					data = truncate(*e.Data, 70)
				}
				rows = append(rows, []string{e.DateTime.String(), e.EventType, truncate(e.GenieRunID, 8), data})
			}
			if events == nil {
				events = []api.ConversationEvent{}
			}
			return formatCursorPage(rctx, events, headers, rows, cp)
		},
	}
	addCursorFlags(cmd, &cursor, &limit)
	cmd.Flags().StringVar(&order, "order", "", "Sort order, as the API accepts it (e.g. asc or desc)")
	cmd.Flags().StringArrayVar(&eventTypes, "event-type", nil, "Only events of this type (repeatable), e.g. agent_message")
	cmd.Flags().StringVar(&runID, "run-id", "", "Only events from this genie run")
	return cmd
}

// ---- shared helpers ----

func agenticContext(cmd *cobra.Command) (*RunContext, api.Client, error) {
	rctx, err := BuildRunContext(cmd)
	if err != nil {
		return nil, nil, err
	}
	client, _, err := resolveAPIClient(cmd)
	if err != nil {
		return nil, nil, err
	}
	return rctx, client, nil
}

func addPageFlags(cmd *cobra.Command, page, perPage *int, max int) {
	cmd.Flags().IntVar(page, "page", 0, "Page number (starts at 1)")
	usage := "Items per page"
	if max > 0 {
		usage += fmt.Sprintf(" (max %d)", max)
	}
	cmd.Flags().IntVar(perPage, "per-page", 0, usage)
}

func addCursorFlags(cmd *cobra.Command, cursor *string, limit *int) {
	cmd.Flags().StringVar(cursor, "cursor", "", "Continue from a previous page's next_cursor")
	cmd.Flags().IntVar(limit, "limit", 0, "Items per page (max 100)")
}

// formatOffsetPage writes a page-numbered list; HasNext comes from the API's
// total, and text output says how to get the next page.
func formatOffsetPage(rctx *RunContext, data any, headers []string, rows [][]string, pg api.OffsetPage, n int) error {
	if !flagJSON && pg.HasNext() && !rctx.Quiet {
		defer fmt.Fprintf(os.Stderr, "Showing %d of %d; next page: --page %d\n", n, pg.Total, pg.Page+1)
	}
	return rctx.Formatter.FormatPage(os.Stdout, data, headers, rows, output.PageMeta{Page: pg.Page, PerPage: pg.PerPage, HasNext: pg.HasNext()})
}

// formatCursorPage writes a cursor-paginated list; text output names the
// cursor to continue from.
func formatCursorPage(rctx *RunContext, data any, headers []string, rows [][]string, cp api.CursorPage) error {
	if !flagJSON && cp.HasNext && !rctx.Quiet {
		defer fmt.Fprintf(os.Stderr, "More results: --cursor %s\n", cp.NextCursor)
	}
	return rctx.Formatter.FormatCursorPage(os.Stdout, data, headers, rows, output.CursorMeta{NextCursor: cp.NextCursor, HasNext: cp.HasNext})
}

func compactJSON(raw json.RawMessage) string {
	var b bytes.Buffer
	if json.Compact(&b, raw) != nil {
		return string(raw)
	}
	return b.String()
}

func truncate(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len([]rune(s)) <= n {
		return s
	}
	return string([]rune(s)[:n-1]) + "…"
}
