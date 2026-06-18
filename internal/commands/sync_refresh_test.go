package commands

import (
	"testing"

	"github.com/workato-devs/wk/internal/config"
	"github.com/workato-devs/wk/internal/sync"
)

// TestSyncRefresh_EmptyConfigNoCrash guards the early-exit path — with no
// [[sync]] entries there is no API call to make, no config write, and the
// command prints a message (or empty JSON) without touching the client.
// Run under --json so we hit the JSON branch; the default path is
// functionally identical for the zero case.
func TestSyncRefresh_EmptyConfigNoCrash(t *testing.T) {
	resetGlobalFlags(t)
	cwd := setupIsolatedHome(t)
	writeProjectSkel(t, cwd, nil)

	flagJSON = true
	t.Cleanup(func() { flagJSON = false })

	root := NewRootCmd()
	root.AddCommand(newSyncCmd())
	root.SetArgs([]string{"sync", "refresh", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("refresh: %v", err)
	}
}

// TestPruneEntries_RemovesNotFound pins the (server_path, local_path)
// disambiguation logic so two entries with the same server path but
// different local paths are correctly sorted into keep and remove
// buckets. Only not-found entries are pruned — found/current/repaired
// are all healthy outcomes.
func TestPruneEntries_RemovesNotFound(t *testing.T) {
	cfg := &config.Config{
		Sync: []config.SyncEntry{
			{ServerPath: "Good", LocalPath: "./good", FolderID: 10},
			{ServerPath: "Same", LocalPath: "./same-a", FolderID: 20},
			{ServerPath: "Same", LocalPath: "./same-b"},
			{ServerPath: "Gone", LocalPath: "./gone", FolderID: 999},
			{ServerPath: "Ghost", LocalPath: "./ghost"},
			{ServerPath: "Renamed", LocalPath: "./renamed", FolderID: 50},
		},
	}
	results := []sync.RefreshResult{
		{ServerPath: "Good", LocalPath: "./good", State: sync.RefreshStateCurrent},
		{ServerPath: "Same", LocalPath: "./same-a", State: sync.RefreshStateCurrent},
		{ServerPath: "Same", LocalPath: "./same-b", State: sync.RefreshStateNotFound},
		{ServerPath: "Gone", LocalPath: "./gone", State: sync.RefreshStateNotFound},
		{ServerPath: "Ghost", LocalPath: "./ghost", State: sync.RefreshStateNotFound},
		{ServerPath: "Renamed", LocalPath: "./renamed", State: sync.RefreshStateRepaired},
	}

	removed := pruneEntries(cfg, results)
	if removed != 3 {
		t.Errorf("removed = %d, want 3 (only not-found entries)", removed)
	}
	if len(cfg.Sync) != 3 {
		t.Fatalf("remaining entries = %d, want 3 (%+v)", len(cfg.Sync), cfg.Sync)
	}
	keptPaths := map[string]bool{}
	for _, e := range cfg.Sync {
		keptPaths[e.ServerPath+"|"+e.LocalPath] = true
	}
	if !keptPaths["Good|./good"] {
		t.Errorf("Good|./good should be kept (current): %+v", cfg.Sync)
	}
	if !keptPaths["Same|./same-a"] {
		t.Errorf("Same|./same-a should be kept (current); only Same|./same-b should be pruned: %+v", cfg.Sync)
	}
	if !keptPaths["Renamed|./renamed"] {
		t.Errorf("Renamed|./renamed should be kept (repaired, not pruned): %+v", cfg.Sync)
	}
}
