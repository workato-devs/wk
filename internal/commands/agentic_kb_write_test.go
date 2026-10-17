package commands

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/workato-devs/wk/internal/api"
)

// Input the API would accept with a 200/201 and then ignore is rejected
// before any request is made (the commands fail before resolving a profile).
func TestKBWriteCmds_RejectSilentlyIgnoredInput(t *testing.T) {
	tests := []struct {
		name    string
		cmd     func() *cobra.Command
		args    []string
		wantErr string
	}{
		{"unknown source type becomes recipe", newKBCreateCmd, []string{"--name", "N", "--folder-id", "1", "--source-type", "files"}, "--source-type must be one of recipe, matrix"},
		{"create without folder", newKBCreateCmd, []string{"--name", "N"}, "--name and --folder-id are required"},
		{"blank name on create", newKBCreateCmd, []string{"--name", " ", "--folder-id", "1"}, "--name can't be blank"},
		{"blank name on update", newKBUpdateCmd, []string{"kb-1", "--name", ""}, "--name can't be blank"},
		{"blank description can't clear it", newKBUpdateCmd, []string{"kb-1", "--description", " "}, "can't be cleared"},
		{"nothing to update", newKBUpdateCmd, []string{"kb-1"}, "nothing to update"},
		{"folder can't change", newKBUpdateCmd, []string{"kb-1", "--folder-id", "2"}, "unknown flag: --folder-id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.cmd()
			cmd.SetArgs(tt.args)
			cmd.SilenceErrors, cmd.SilenceUsage = true, true
			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

// The API's 200 on update doesn't mean the change applied, so the re-read is
// compared against what was asked for.
func TestCheckKBUpdate(t *testing.T) {
	name, desc := "New", "Updated"
	if err := checkKBUpdate(api.KBInput{Name: &name, Description: &desc}, &api.KnowledgeBase{Name: "New", Description: "Updated"}); err != nil {
		t.Errorf("applied update: err = %v", err)
	}
	err := checkKBUpdate(api.KBInput{Description: &desc}, &api.KnowledgeBase{Name: "New", Description: "old"})
	if err == nil || !strings.Contains(err.Error(), `description is "old", not "Updated"`) {
		t.Errorf("ignored update: err = %v", err)
	}
}

// The in-use refusal is an agent's only route to a safe delete, so it must
// name the genies and give both runnable next steps.
func TestKBInUseError(t *testing.T) {
	msg := kbInUseError("kb-1", []string{"gin-a", "gin-b"}).Error()
	for _, want := range []string{"gin-a, gin-b", "wk agentic genies remove-knowledge-bases <genie-id> kb-1", "wk agentic kb delete kb-1 --force"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message lacks %q:\n%s", want, msg)
		}
	}
}
