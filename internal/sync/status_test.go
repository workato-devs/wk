package sync

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/workato-devs/wk-cli-beta/internal/config"
)

func TestStatusUnchanged(t *testing.T) {
	dir := t.TempDir()
	content := []byte("hello world")

	// Write asset file
	os.WriteFile(filepath.Join(dir, "recipe.json"), content, 0644)

	// Write meta with matching hash
	meta := &AssetMeta{
		ServerPath:  "test/recipe.json",
		ContentHash: ComputeHash(content),
	}
	WriteMeta(MetaFileName(filepath.Join(dir, "recipe.json")), meta)

	engine := &SyncEngine{projectRoot: dir}
	entry := config.SyncEntry{LocalPath: "."}

	statuses, err := engine.Status(entry)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].Status != StatusUnchanged {
		t.Errorf("expected unchanged, got %s", statuses[0].Status)
	}
}

func TestStatusModified(t *testing.T) {
	dir := t.TempDir()

	// Write asset file
	os.WriteFile(filepath.Join(dir, "recipe.json"), []byte("modified content"), 0644)

	// Write meta with old hash
	meta := &AssetMeta{
		ServerPath:  "test/recipe.json",
		ContentHash: ComputeHash([]byte("original content")),
	}
	WriteMeta(MetaFileName(filepath.Join(dir, "recipe.json")), meta)

	engine := &SyncEngine{projectRoot: dir}
	entry := config.SyncEntry{LocalPath: "."}

	statuses, err := engine.Status(entry)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].Status != StatusModified {
		t.Errorf("expected modified, got %s", statuses[0].Status)
	}
}

func TestStatusNew(t *testing.T) {
	dir := t.TempDir()

	// Write asset file with no meta
	os.WriteFile(filepath.Join(dir, "new-recipe.json"), []byte("new content"), 0644)

	engine := &SyncEngine{projectRoot: dir}
	entry := config.SyncEntry{LocalPath: "."}

	statuses, err := engine.Status(entry)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].Status != StatusNew {
		t.Errorf("expected new, got %s", statuses[0].Status)
	}
}
