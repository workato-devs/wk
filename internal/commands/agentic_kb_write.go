package commands

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/workato-devs/wk/internal/api"
)

// Knowledge base write commands (ADR-009 Decision 1). The API ignores some
// input without saying so (an unknown source_type, a blank name or
// description), so wk rejects that input up front and re-reads after update.

func addKBWriteCmds(cmd *cobra.Command) {
	cmd.AddCommand(newKBCreateCmd(), newKBUpdateCmd(), newKBDeleteCmd())
}

// kbSourceTypes are the source types the API honors; it turns any other value
// into "recipe" and still answers 201.
var kbSourceTypes = []string{"recipe", "matrix"}

func newKBCreateCmd() *cobra.Command {
	var name, description, folderID, sourceType string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a knowledge base",
		Long: `Create a knowledge base and print it as the API returns it.

--source-type is recipe (the default) or matrix. wk can't set a Matrix
knowledge base's data sources: the API accepts them but doesn't apply them.`,
		Example: `  wk agentic kb create --name "Guest FAQ" --folder-id 12345
  wk agentic kb create --name "Guest FAQ" --folder-id 12345 --description "Hotel policies" --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			set := cmd.Flags().Changed
			if !set("name") || !set("folder-id") {
				return fmt.Errorf("--name and --folder-id are required")
			}
			if strings.TrimSpace(name) == "" {
				return fmt.Errorf("--name can't be blank")
			}
			in := api.KBInput{Name: &name, FolderID: &folderID}
			if set("description") {
				in.Description = &description
			}
			if set("source-type") {
				if !slices.Contains(kbSourceTypes, sourceType) {
					return fmt.Errorf("--source-type must be one of %s, got %q", strings.Join(kbSourceTypes, ", "), sourceType)
				}
				in.SourceType = &sourceType
			}
			rctx, client, err := agenticContext(cmd)
			if err != nil {
				return err
			}
			kb, err := client.KnowledgeBases().Create(cmd.Context(), in)
			if err != nil {
				return err
			}
			return printKB(rctx, kb, fmt.Sprintf("Created knowledge base %s", kb.ID))
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Knowledge base name (unique in the workspace)")
	cmd.Flags().StringVar(&description, "description", "", "Description")
	cmd.Flags().StringVar(&folderID, "folder-id", "", "Folder to create the knowledge base in (it can't be moved later)")
	cmd.Flags().StringVar(&sourceType, "source-type", "", "recipe or matrix (default recipe)")
	return cmd
}

func newKBUpdateCmd() *cobra.Command {
	var name, description string
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Rename a knowledge base or change its description",
		Long: `Change the name or description, then print the knowledge base as the API now
reports it. These are the only fields the API lets you change: a knowledge base
can't be moved to another folder or change its source type.

The API can't clear a description: it ignores a blank one and still reports
success, so wk rejects a blank --name or --description.`,
		Example: `  wk agentic kb update kb-AbJzNhoC-krNnNo-AB --name "Guest FAQ (2026)"`,
		Args:    requireArgs(1, "knowledge base ID is required, e.g.: wk agentic kb update <id> --name …"),
		RunE: func(cmd *cobra.Command, args []string) error {
			set := cmd.Flags().Changed
			var in api.KBInput
			if set("name") {
				if strings.TrimSpace(name) == "" {
					return fmt.Errorf("--name can't be blank")
				}
				in.Name = &name
			}
			if set("description") {
				if strings.TrimSpace(description) == "" {
					return fmt.Errorf("--description can't be blank: the API ignores a blank description, so it can't be cleared; pass the new text instead")
				}
				in.Description = &description
			}
			if in == (api.KBInput{}) {
				return fmt.Errorf("nothing to update; pass --name or --description")
			}
			rctx, client, err := agenticContext(cmd)
			if err != nil {
				return err
			}
			id := args[0]
			if err := client.KnowledgeBases().Update(cmd.Context(), id, in); err != nil {
				return err
			}
			kb, err := client.KnowledgeBases().Get(cmd.Context(), id)
			if err != nil {
				return fmt.Errorf("updated knowledge base %s, but re-reading it failed: %w", id, err)
			}
			if err := printKB(rctx, kb, fmt.Sprintf("Updated knowledge base %s", id)); err != nil {
				return err
			}
			return checkKBUpdate(in, kb)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "New name (unique in the workspace)")
	cmd.Flags().StringVar(&description, "description", "", "New description")
	return cmd
}

// checkKBUpdate reports a field the API said it changed but didn't.
func checkKBUpdate(in api.KBInput, kb *api.KnowledgeBase) error {
	var off []string
	if in.Name != nil && kb.Name != *in.Name {
		off = append(off, fmt.Sprintf("name is %q, not %q", kb.Name, *in.Name))
	}
	if in.Description != nil && kb.Description != *in.Description {
		off = append(off, fmt.Sprintf("description is %q, not %q", kb.Description, *in.Description))
	}
	if len(off) > 0 {
		return fmt.Errorf("update returned success, but the knowledge base's %s", strings.Join(off, "; "))
	}
	return nil
}

type kbDeleteReport struct {
	ID       string   `json:"id"`
	Deleted  bool     `json:"deleted"`
	Detached []string `json:"detached_genie_ids"` // genies it was removed from
}

func newKBDeleteCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a knowledge base",
		Long: `Delete a knowledge base.

The API deletes a knowledge base even when genies use it, detaching it from
them while they run. So wk refuses when any genie uses it and names those
genies; detach it first, or pass --force to delete it anyway.`,
		Example: `  wk agentic kb delete kb-AbJzNhoC-krNnNo-AB
  wk agentic kb delete kb-AbJzNhoC-krNnNo-AB --force --json`,
		Args: requireArgs(1, "knowledge base ID is required, e.g.: wk agentic kb delete <id>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, client, err := agenticContext(cmd)
			if err != nil {
				return err
			}
			id := args[0]
			kb, err := client.KnowledgeBases().Get(cmd.Context(), id)
			if err != nil {
				return err
			}
			genies := make([]string, 0, len(kb.Genies))
			for _, g := range kb.Genies {
				genies = append(genies, g.GenieID)
			}
			if len(genies) > 0 && !force {
				return kbInUseError(id, genies)
			}
			if err := client.KnowledgeBases().Delete(cmd.Context(), id); err != nil {
				return err
			}
			if flagJSON {
				return rctx.Formatter.Format(os.Stdout, kbDeleteReport{ID: id, Deleted: true, Detached: genies})
			}
			fmt.Fprintf(os.Stderr, "Deleted knowledge base %s\n", id)
			if len(genies) > 0 {
				fmt.Fprintf(os.Stderr, "Detached it from genies: %s\n", strings.Join(genies, ", "))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Delete even if genies use it (detaches it from them)")
	return cmd
}

func kbInUseError(id string, genies []string) error {
	return fmt.Errorf("knowledge base %s is used by %d genie(s): %s. Deleting it detaches it from them, even while they're running.\n"+
		"Detach it first: wk agentic genies remove-knowledge-bases <genie-id> %s\n"+
		"Or delete it anyway: wk agentic kb delete %s --force",
		id, len(genies), strings.Join(genies, ", "), id, id)
}

func printKB(rctx *RunContext, kb *api.KnowledgeBase, note string) error {
	if flagJSON {
		return rctx.Formatter.Format(os.Stdout, kb)
	}
	fmt.Fprintln(os.Stderr, note)
	headers := []string{"ID", "NAME", "SOURCE", "RECIPES", "GENIES", "FOLDER ID"}
	return rctx.Formatter.FormatList(os.Stdout, kb, headers, [][]string{kbRow(*kb)})
}
