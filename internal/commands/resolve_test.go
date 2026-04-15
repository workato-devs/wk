package commands

import (
	"strings"
	"testing"

	"github.com/workato-devs/wk-cli-beta/internal/config"
)

func TestCheckProfileMatch_Mismatch(t *testing.T) {
	cfg := &config.Config{Profile: "prod"}
	err := checkProfileMatch(cfg, "dev")
	if err == nil {
		t.Fatal("expected error for profile mismatch")
	}
	msg := err.Error()
	if !strings.Contains(msg, `"dev"`) || !strings.Contains(msg, `"prod"`) {
		t.Errorf("error should mention both profiles, got: %s", msg)
	}
	if !strings.Contains(msg, "wk auth switch prod") {
		t.Errorf("error should suggest auth switch, got: %s", msg)
	}
}

func TestCheckProfileMatch_Match(t *testing.T) {
	cfg := &config.Config{Profile: "prod"}
	err := checkProfileMatch(cfg, "prod")
	if err != nil {
		t.Fatalf("unexpected error for matching profile: %v", err)
	}
}

func TestCheckProfileMatch_EmptyProfile(t *testing.T) {
	cfg := &config.Config{Profile: ""}
	err := checkProfileMatch(cfg, "anything")
	if err != nil {
		t.Fatalf("unexpected error when profile is empty: %v", err)
	}
}
