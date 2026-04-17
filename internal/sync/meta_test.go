package sync

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/workato-devs/wk-cli-beta/internal/config"
)

func TestComputeHash(t *testing.T) {
	hash := ComputeHash([]byte("hello world"))
	if len(hash) != 64 { // SHA256 hex = 64 chars
		t.Errorf("hash length = %d, want 64", len(hash))
	}

	// Same input = same hash
	hash2 := ComputeHash([]byte("hello world"))
	if hash != hash2 {
		t.Error("same input should produce same hash")
	}

	// Different input = different hash
	hash3 := ComputeHash([]byte("hello world!"))
	if hash == hash3 {
		t.Error("different input should produce different hash")
	}
}

func TestMetaReadWrite(t *testing.T) {
	dir := t.TempDir()
	metaPath := filepath.Join(dir, "test.recipe.json.meta.json")

	meta := &AssetMeta{
		ServerPath:   "All projects/Test/recipe",
		ZipName:      "Test/recipe.recipe.json",
		Folder:       "Test",
		Type:         "recipe",
		Version:      3,
		ContentHash:  "abc123",
		LastPulledAt: time.Now().Truncate(time.Second),
	}

	if err := WriteMeta(metaPath, meta); err != nil {
		t.Fatalf("WriteMeta: %v", err)
	}

	loaded, err := ReadMeta(metaPath)
	if err != nil {
		t.Fatalf("ReadMeta: %v", err)
	}

	if loaded.ServerPath != meta.ServerPath {
		t.Errorf("ServerPath = %q, want %q", loaded.ServerPath, meta.ServerPath)
	}
	if loaded.Version != meta.Version {
		t.Errorf("Version = %d, want %d", loaded.Version, meta.Version)
	}
	if loaded.ContentHash != meta.ContentHash {
		t.Errorf("ContentHash = %q, want %q", loaded.ContentHash, meta.ContentHash)
	}
}

func TestMetaPath(t *testing.T) {
	root := t.TempDir()
	asset := filepath.Join(root, "recipes", "slack.recipe.json")
	got, err := MetaPath(root, asset)
	if err != nil {
		t.Fatalf("MetaPath: %v", err)
	}
	want := filepath.Join(root, config.ProjectDir, "recipes", "slack.recipe.json.meta.json")
	if got != want {
		t.Errorf("MetaPath = %q, want %q", got, want)
	}
}

func TestMetaPath_OutsideRoot(t *testing.T) {
	root := t.TempDir()
	other := t.TempDir()
	asset := filepath.Join(other, "x.recipe.json")
	if _, err := MetaPath(root, asset); err == nil {
		t.Error("expected error for asset outside project root")
	}
}

func TestFindMetaFiles(t *testing.T) {
	projectRoot := t.TempDir()
	localDir := projectRoot // LocalPath "."

	// Create an asset file at project root.
	assetAbs := filepath.Join(projectRoot, "recipe.json")
	os.WriteFile(assetAbs, []byte(`{"name":"test"}`), 0644)

	// Write its meta under .wk/.
	metaPath, err := MetaPath(projectRoot, assetAbs)
	if err != nil {
		t.Fatalf("MetaPath: %v", err)
	}
	meta := &AssetMeta{
		ServerPath:  "All projects/Test",
		ContentHash: ComputeHash([]byte(`{"name":"test"}`)),
	}
	if err := WriteMeta(metaPath, meta); err != nil {
		t.Fatalf("WriteMeta: %v", err)
	}

	metas, err := FindMetaFiles(projectRoot, localDir)
	if err != nil {
		t.Fatalf("FindMetaFiles: %v", err)
	}
	if len(metas) != 1 {
		t.Fatalf("found %d metas, want 1", len(metas))
	}
	if _, ok := metas["recipe.json"]; !ok {
		t.Errorf("expected meta keyed by \"recipe.json\", got keys: %v", keys(metas))
	}
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
