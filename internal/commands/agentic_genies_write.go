package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/workato-devs/wk/internal/api"
)

// Genie write commands (ADR-009 Decision 1). The API answers most of these
// with an empty body or a bare ID, so each command re-reads what it changed
// and reports that, rather than echoing what was asked for.

func addGenieWriteCmds(cmd *cobra.Command) {
	cmd.AddCommand(newGenieCreateCmd(), newGenieUpdateCmd(), newGenieDeleteCmd(),
		newGenieStateCmd("start", "active"), newGenieStateCmd("stop", "inactive"))
	for _, kind := range []struct {
		kind  api.GenieAssignment
		noun  string
		label string
	}{
		{api.AssignSkills, "skills", "skill"},
		{api.AssignKnowledgeBases, "knowledge-bases", "knowledge base"},
		{api.AssignUserGroups, "user-groups", "user group"},
	} {
		cmd.AddCommand(newGenieAssignCmd(kind.kind, kind.noun, kind.label, true))
		cmd.AddCommand(newGenieAssignCmd(kind.kind, kind.noun, kind.label, false))
	}
}

// genieInputFlags are the documented create/update fields.
type genieInputFlags struct {
	name, description, folderID, instructions, instructionsFile, aiProvider string
	channelMode, channelScope, responseMode                                 string
	channelWhitelist                                                        []string
	sharedAccountID, customOAuthKeyID                                       int
	matrix                                                                  bool
}

func (f *genieInputFlags) register(cmd *cobra.Command) {
	fl := cmd.Flags()
	fl.StringVar(&f.name, "name", "", "Genie name")
	fl.StringVar(&f.description, "description", "", "Description")
	fl.StringVar(&f.folderID, "folder-id", "", "Folder to create the genie in")
	fl.StringVar(&f.instructions, "instructions", "", "Instructions (the genie's system prompt)")
	fl.StringVar(&f.instructionsFile, "instructions-file", "", "Read instructions from this file")
	fl.StringVar(&f.aiProvider, "ai-provider", "", "AI provider, e.g. anthropic")
	fl.IntVar(&f.sharedAccountID, "shared-account-id", 0, "Shared account ID")
	fl.IntVar(&f.customOAuthKeyID, "custom-oauth-key-id", 0, "Custom OAuth key ID")
	fl.BoolVar(&f.matrix, "matrix", false, "Matrix genie")
	fl.StringVar(&f.channelMode, "channel-mode", "", "Channel mode, e.g. dms_only")
	fl.StringVar(&f.channelScope, "channel-scope", "", "Channel scope, e.g. specific")
	fl.StringVar(&f.responseMode, "response-mode", "", "Response mode, e.g. on_mention")
	fl.StringSliceVar(&f.channelWhitelist, "channel-whitelist", nil, "Allowed channels (comma-separated or repeated)")
}

// input builds a GenieInput from the flags the caller actually set, so an
// update sends only what changed.
func (f *genieInputFlags) input(cmd *cobra.Command) (api.GenieInput, error) {
	var in api.GenieInput
	set := cmd.Flags().Changed
	str := func(flag string, v string, dst **string) {
		if set(flag) {
			*dst = &v
		}
	}
	str("name", f.name, &in.Name)
	str("description", f.description, &in.Description)
	str("folder-id", f.folderID, &in.FolderID)
	str("instructions", f.instructions, &in.Instructions)
	str("ai-provider", f.aiProvider, &in.AIProvider)
	str("channel-mode", f.channelMode, &in.ChannelMode)
	str("channel-scope", f.channelScope, &in.ChannelScope)
	str("response-mode", f.responseMode, &in.ResponseMode)
	if set("instructions-file") {
		if set("instructions") {
			return in, fmt.Errorf("use --instructions or --instructions-file, not both")
		}
		data, err := os.ReadFile(f.instructionsFile)
		if err != nil {
			return in, fmt.Errorf("reading --instructions-file: %w", err)
		}
		text := string(data)
		in.Instructions = &text
	}
	if set("shared-account-id") {
		in.SharedAccountID = &f.sharedAccountID
	}
	if set("custom-oauth-key-id") {
		in.CustomOAuthKeyID = &f.customOAuthKeyID
	}
	if set("matrix") {
		in.Matrix = &f.matrix
	}
	if set("channel-whitelist") {
		in.ChannelWhitelist = &f.channelWhitelist
	}
	return in, nil
}

func newGenieCreateCmd() *cobra.Command {
	var f genieInputFlags
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a genie",
		Long: `Create a genie, then print it as the API now reports it. The genie is created
stopped; start it with wk agentic genies start <id>.`,
		Example: `  wk agentic genies create --name "Concierge" --folder-id 12345 --instructions-file prompt.md
  wk agentic genies create --name "Concierge" --folder-id 12345 --instructions "Help guests." --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("name") || !cmd.Flags().Changed("folder-id") {
				return fmt.Errorf("--name and --folder-id are required")
			}
			in, err := f.input(cmd)
			if err != nil {
				return err
			}
			rctx, client, err := agenticContext(cmd)
			if err != nil {
				return err
			}
			id, err := client.Genies().Create(cmd.Context(), in)
			if err != nil {
				return err
			}
			return printGenie(cmd, rctx, client, id, fmt.Sprintf("Created genie %s", id))
		},
	}
	f.register(cmd)
	return cmd
}

func newGenieUpdateCmd() *cobra.Command {
	var f genieInputFlags
	cmd := &cobra.Command{
		Use:     "update <id>",
		Short:   "Update a genie's settings",
		Long:    "Change only the fields you pass, then print the genie as the API now reports it.",
		Example: `  wk agentic genies update gin-AbzGALWh-9h9Q9W-AB --instructions-file prompt.md`,
		Args:    requireArgs(1, "genie ID is required, e.g.: wk agentic genies update <id> --name …"),
		RunE: func(cmd *cobra.Command, args []string) error {
			in, err := f.input(cmd)
			if err != nil {
				return err
			}
			if in == (api.GenieInput{}) {
				return fmt.Errorf("nothing to update; pass at least one field flag, e.g. --name or --instructions")
			}
			rctx, client, err := agenticContext(cmd)
			if err != nil {
				return err
			}
			if err := client.Genies().Update(cmd.Context(), args[0], in); err != nil {
				return err
			}
			return printGenie(cmd, rctx, client, args[0], fmt.Sprintf("Updated genie %s", args[0]))
		},
	}
	f.register(cmd)
	return cmd
}

func newGenieDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "delete <id>",
		Short:   "Delete a genie",
		Long:    "Delete a genie. This also deletes the Headless API clients attached to it, so their credentials stop working; to keep a client, detach it first with wk agentic clients detach.",
		Example: `  wk agentic genies delete gin-AbzGALWh-9h9Q9W-AB`,
		Args:    requireArgs(1, "genie ID is required, e.g.: wk agentic genies delete <id>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, client, err := agenticContext(cmd)
			if err != nil {
				return err
			}
			if err := client.Genies().Delete(cmd.Context(), args[0]); err != nil {
				return err
			}
			if flagJSON {
				return rctx.Formatter.Format(os.Stdout, map[string]any{"id": args[0], "deleted": true})
			}
			fmt.Fprintf(os.Stderr, "Deleted genie %s\n", args[0])
			return nil
		},
	}
}

// newGenieStateCmd builds start or stop. Both return an empty body, so the
// command re-reads the genie and reports the state it's actually in. want is
// the get vocabulary ("active"/"inactive"), verified to follow start/stop.
func newGenieStateCmd(verb, want string) *cobra.Command {
	return &cobra.Command{
		Use:     verb + " <id>",
		Short:   strings.ToUpper(verb[:1]) + verb[1:] + " a genie",
		Example: fmt.Sprintf("  wk agentic genies %s gin-AbzGALWh-9h9Q9W-AB", verb),
		Args:    requireArgs(1, fmt.Sprintf("genie ID is required, e.g.: wk agentic genies %s <id>", verb)),
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, client, err := agenticContext(cmd)
			if err != nil {
				return err
			}
			run := client.Genies().Start
			if verb == "stop" {
				run = client.Genies().Stop
			}
			if err := run(cmd.Context(), args[0]); err != nil {
				return err
			}
			g, err := client.Genies().Get(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("%s succeeded but re-reading the genie failed: %w", verb, err)
			}
			if flagJSON {
				if err := rctx.Formatter.Format(os.Stdout, map[string]any{"id": g.ID, "state": g.State}); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(os.Stderr, "Genie %s is now %s\n", g.ID, g.State)
			}
			if g.State != want {
				return fmt.Errorf("%s returned success, but the genie's state is %q, not %q", verb, g.State, want)
			}
			return nil
		},
	}
}

func printGenie(cmd *cobra.Command, rctx *RunContext, client api.Client, id, note string) error {
	g, err := client.Genies().Get(cmd.Context(), id)
	if err != nil {
		return fmt.Errorf("%s, but re-reading it failed: %w", note, err)
	}
	if flagJSON {
		return rctx.Formatter.Format(os.Stdout, g)
	}
	fmt.Fprintln(os.Stderr, note)
	headers := []string{"ID", "NAME", "STATE", "AI MODEL", "FOLDER ID", "PROJECT ID"}
	return rctx.Formatter.FormatList(os.Stdout, g, headers, [][]string{{g.ID, g.Name, g.State, g.AIModelLabel, fmt.Sprint(g.FolderID), fmt.Sprint(g.ProjectID)}})
}

// ---- assignments (ADR-009 Decision 5) ----

// Assignment outcomes, per requested ID.
const (
	outcomeAssigned        = "assigned"
	outcomeAlreadyAssigned = "already_assigned"
	outcomeRemoved         = "removed"
	outcomeNotAssigned     = "not_assigned" // remove of an ID that wasn't attached
	outcomeNotFound        = "not_found"
	outcomeNotApplied      = "not_applied" // exists, but the change didn't happen
)

type assignmentResult struct {
	ID      string `json:"id"`
	Outcome string `json:"outcome"`
}

type assignmentReport struct {
	GenieID string             `json:"genie_id"`
	Action  string             `json:"action"` // "assign" or "remove"
	Kind    string             `json:"kind"`
	OK      bool               `json:"ok"`
	Results []assignmentResult `json:"results"`
}

// membership is, for one ID, whether it exists and whether it's attached to
// the genie.
type membership struct {
	found    bool
	attached bool
}

func newGenieAssignCmd(kind api.GenieAssignment, noun, label string, assign bool) *cobra.Command {
	verb, action := "remove", "remove"
	if assign {
		verb, action = "assign", "assign"
	}
	return &cobra.Command{
		Use:   fmt.Sprintf("%s-%s <genie-id> <%s-id>...", verb, noun, strings.ReplaceAll(label, " ", "-")),
		Short: fmt.Sprintf("%s %ss %s a genie", strings.ToUpper(verb[:1])+verb[1:], label, map[bool]string{true: "to", false: "from"}[assign]),
		Long: fmt.Sprintf(`%s %ss %s a genie, then check each one.

The API reports success even when an ID wasn't applied (unknown IDs are
silently ignored), so wk reads each %s before and after and reports an outcome
per ID: assigned, already_assigned, removed, not_assigned, not_found, or
not_applied. It exits non-zero if any ID was not_found or not_applied.`,
			strings.ToUpper(verb[:1])+verb[1:], label, map[bool]string{true: "to", false: "from"}[assign], label),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return fmt.Errorf("genie ID and at least one %s ID are required, e.g.: wk agentic genies %s-%s <genie-id> <id>...", label, verb, noun)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, client, err := agenticContext(cmd)
			if err != nil {
				return err
			}
			genieID, ids := args[0], slices.Compact(slices.Sorted(slices.Values(args[1:])))
			ctx := cmd.Context()

			before, err := memberships(ctx, client, kind, genieID, ids)
			if err != nil {
				return err
			}
			call := client.Genies().Remove
			if assign {
				call = client.Genies().Assign
			}
			if err := call(ctx, genieID, kind, ids); err != nil {
				return err
			}
			after, err := memberships(ctx, client, kind, genieID, ids)
			if err != nil {
				return fmt.Errorf("%s returned success, but checking the result failed: %w", action, err)
			}

			report := reconcileAssignment(genieID, action, string(kind), ids, before, after, assign)
			if flagJSON {
				if err := rctx.Formatter.Format(os.Stdout, report); err != nil {
					return err
				}
			} else {
				rows := make([][]string, 0, len(report.Results))
				for _, r := range report.Results {
					rows = append(rows, []string{r.ID, r.Outcome})
				}
				if err := rctx.Formatter.FormatList(os.Stdout, report, []string{strings.ToUpper(label) + " ID", "OUTCOME"}, rows); err != nil {
					return err
				}
			}
			if !report.OK {
				var bad []string
				for _, r := range report.Results {
					if r.Outcome == outcomeNotFound || r.Outcome == outcomeNotApplied {
						bad = append(bad, r.ID+" ("+r.Outcome+")")
					}
				}
				return fmt.Errorf("%d of %d %s IDs weren't %sed: %s", len(bad), len(ids), label, action, strings.Join(bad, ", "))
			}
			return nil
		},
	}
}

// reconcileAssignment compares membership before and after the call.
func reconcileAssignment(genieID, action, kind string, ids []string, before, after map[string]membership, assign bool) assignmentReport {
	report := assignmentReport{GenieID: genieID, Action: action, Kind: kind, OK: true}
	for _, id := range ids {
		b, a := before[id], after[id]
		var outcome string
		switch {
		case !b.found && !a.found:
			outcome = outcomeNotFound
		case assign && b.attached:
			outcome = outcomeAlreadyAssigned
		case assign && a.attached:
			outcome = outcomeAssigned
		case assign:
			outcome = outcomeNotApplied
		case !b.attached && !a.attached:
			outcome = outcomeNotAssigned
		case !a.attached:
			outcome = outcomeRemoved
		default:
			outcome = outcomeNotApplied
		}
		if outcome == outcomeNotFound || outcome == outcomeNotApplied {
			report.OK = false
		}
		report.Results = append(report.Results, assignmentResult{ID: id, Outcome: outcome})
	}
	return report
}

// memberships reads, for each ID, whether it exists and is attached to the
// genie. Skills and knowledge bases list the genies they're attached to; the
// genie lists its user groups. A user group that isn't attached can't be told
// apart from one that doesn't exist, so it counts as found.
func memberships(ctx context.Context, client api.Client, kind api.GenieAssignment, genieID string, ids []string) (map[string]membership, error) {
	out := make(map[string]membership, len(ids))
	attachedTo := func(refs []api.GenieRef) bool {
		return slices.ContainsFunc(refs, func(r api.GenieRef) bool { return r.GenieID == genieID })
	}
	switch kind {
	case api.AssignUserGroups:
		g, err := client.Genies().Get(ctx, genieID)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			attached := slices.ContainsFunc(g.UserGroups, func(u api.GenieUserGroup) bool { return u.ID == id })
			out[id] = membership{found: true, attached: attached}
		}
	default:
		for _, id := range ids {
			var refs []api.GenieRef
			var err error
			if kind == api.AssignSkills {
				var s *api.Skill
				if s, err = client.Skills().Get(ctx, id); err == nil {
					refs = s.Genies
				}
			} else {
				var kb *api.KnowledgeBase
				if kb, err = client.KnowledgeBases().Get(ctx, id); err == nil {
					refs = kb.Genies
				}
			}
			var apiErr *api.APIError
			switch {
			case errors.As(err, &apiErr) && apiErr.StatusCode == 404:
				out[id] = membership{}
			case err != nil:
				return nil, err
			default:
				out[id] = membership{found: true, attached: attachedTo(refs)}
			}
		}
	}
	return out, nil
}
