package sync

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/workato-devs/wk-cli-beta/internal/config"
)

// writeAssetWithMeta writes an asset at <root>/<relPath> and its meta
// at the corresponding .wk/ mirror path.
func writeAssetWithMeta(t *testing.T, root, relPath string, content []byte, meta *AssetMeta) {
	t.Helper()
	assetAbs := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(assetAbs), 0755); err != nil {
		t.Fatalf("mkdir asset dir: %v", err)
	}
	if err := os.WriteFile(assetAbs, content, 0644); err != nil {
		t.Fatalf("write asset: %v", err)
	}
	metaPath, err := MetaPath(root, assetAbs)
	if err != nil {
		t.Fatalf("MetaPath: %v", err)
	}
	if err := WriteMeta(metaPath, meta); err != nil {
		t.Fatalf("WriteMeta: %v", err)
	}
}

func TestStatusUnchanged(t *testing.T) {
	dir := t.TempDir()
	content := []byte("hello world")

	writeAssetWithMeta(t, dir, "recipe.json", content, &AssetMeta{
		ServerPath:  "test/recipe.json",
		ContentHash: ComputeHash(content),
	})

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

	writeAssetWithMeta(t, dir, "recipe.json", []byte("modified content"), &AssetMeta{
		ServerPath:  "test/recipe.json",
		ContentHash: ComputeHash([]byte("original content")),
	})

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
