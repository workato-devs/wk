package commands

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// The PR template asks for docs/command-reference.md to be updated with the
// command surface, but a checkbox didn't keep it current: v1.1's new agentic
// and mcp commands and --raw were all missing. These tests compare the doc to
// the real command tree, so the gap fails `go test` and names what to fix.

const commandReferencePath = "../../docs/command-reference.md"

// docCommandPaths parses the tree in the doc's first code block into command
// paths ("agentic kb create"). A line is "<indent><name> [(aliases)] <desc>",
// indented two spaces per level; "a / b" lists sibling commands on one line.
// A line indented deeper than one level below the previous entry continues
// its description.
func docCommandPaths(t *testing.T) map[string]bool {
	t.Helper()
	data, err := os.ReadFile(commandReferencePath)
	if err != nil {
		t.Fatal(err)
	}
	doc := string(data)
	start := strings.Index(doc, "```\nwk\n")
	end := strings.Index(doc[start+4:], "```")
	if start < 0 || end < 0 {
		t.Fatalf("%s: no command tree code block starting with \"wk\"", commandReferencePath)
	}
	lines := strings.Split(doc[start+len("```\nwk\n"):start+4+end], "\n")

	paths := map[string]bool{}
	var stack []string
	nameRe := regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " ")
		if trimmed == "" {
			continue
		}
		depth := (len(line) - len(trimmed)) / 2 // 1 = top-level command
		if depth < 1 || depth > len(stack)+1 {
			continue // description continuation
		}
		var names []string
		for _, part := range strings.Split(trimmed, " / ") {
			if f := strings.Fields(part); len(f) > 0 && nameRe.MatchString(f[0]) {
				names = append(names, f[0])
			}
		}
		if len(names) == 0 {
			continue
		}
		stack = append(stack[:depth-1], names[len(names)-1])
		parent := strings.Join(stack[:depth-1], " ")
		for _, n := range names {
			paths[strings.TrimSpace(parent+" "+n)] = true
		}
	}
	return paths
}

// cliCommandPaths returns every visible command's path below the root,
// without plugin commands (they come from the user's install, and the doc
// covers them in prose) or cobra's generated help/completion subcommands.
func cliCommandPaths(t *testing.T) (map[string]bool, *cobra.Command) {
	t.Helper()
	t.Setenv("WK_HOME", t.TempDir()) // no installed plugins
	root := NewRootCmd()
	registerAllCommands(root)
	paths := map[string]bool{}
	var walk func(c *cobra.Command, prefix string)
	walk = func(c *cobra.Command, prefix string) {
		for _, sub := range c.Commands() {
			if sub.Hidden || sub.Name() == "help" {
				continue
			}
			p := strings.TrimSpace(prefix + " " + sub.Name())
			paths[p] = true
			walk(sub, p)
		}
	}
	walk(root, "")
	return paths, root
}

func TestCommandReferenceListsEveryCommand(t *testing.T) {
	doc := docCommandPaths(t)
	cli, _ := cliCommandPaths(t)

	var missing, stale []string
	for p := range cli {
		if !doc[p] {
			missing = append(missing, p)
		}
	}
	for p := range doc {
		// completion is added by cobra at run time; the doc lists it.
		if !cli[p] && p != "completion" {
			stale = append(stale, p)
		}
	}
	slices.Sort(missing)
	slices.Sort(stale)
	for _, p := range missing {
		t.Errorf("docs/command-reference.md is missing `wk %s`: add it to the command tree", p)
	}
	for _, p := range stale {
		t.Errorf("docs/command-reference.md lists `wk %s`, which no longer exists: remove it", p)
	}
}

func TestCommandReferenceListsEveryGlobalFlag(t *testing.T) {
	data, err := os.ReadFile(commandReferencePath)
	if err != nil {
		t.Fatal(err)
	}
	doc := string(data)
	i := strings.Index(doc, "## Global flags")
	if i < 0 {
		t.Fatalf("%s: no \"## Global flags\" section", filepath.Base(commandReferencePath))
	}
	table := doc[i:]
	_, root := cliCommandPaths(t)
	root.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if !strings.Contains(table, "`--"+f.Name) {
			t.Errorf("docs/command-reference.md \"Global flags\" is missing `--%s`: add a row", f.Name)
		}
	})
}
