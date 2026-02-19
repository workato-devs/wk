package commands

import (
	"strings"
	"testing"

	"github.com/workato-devs/wk-cli-beta/internal/config"
)

func TestCheckWorkspaceMatch_Mismatch(t *testing.T) {
	cfg := &config.Config{Workspace: "prod"}
	err := checkWorkspaceMatch(cfg, "dev")
	if err == nil {
		t.Fatal("expected error for workspace mismatch")
	}
	msg := err.Error()
	if !strings.Contains(msg, `"dev"`) || !strings.Contains(msg, `"prod"`) {
		t.Errorf("error should mention both profiles, got: %s", msg)
	}
	if !strings.Contains(msg, "wk auth switch prod") {
		t.Errorf("error should suggest auth switch, got: %s", msg)
	}
}

func TestCheckWorkspaceMatch_Match(t *testing.T) {
	cfg := &config.Config{Workspace: "prod"}
	err := checkWorkspaceMatch(cfg, "prod")
	if err != nil {
		t.Fatalf("unexpected error for matching workspace: %v", err)
	}
}

func TestCheckWorkspaceMatch_EmptyWorkspace(t *testing.T) {
	cfg := &config.Config{Workspace: ""}
	err := checkWorkspaceMatch(cfg, "anything")
	if err != nil {
		t.Fatalf("unexpected error when workspace is empty: %v", err)
	}
}
