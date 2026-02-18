package sync

import (
	"os"
	"path/filepath"
	"testing"
	"time"
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
	metaPath := filepath.Join(dir, "test.recipe.json.wk-meta.json")

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

func TestMetaFileName(t *testing.T) {
	name := MetaFileName("my_recipe.recipe.json")
	if name != "my_recipe.recipe.json.wk-meta.json" {
		t.Errorf("MetaFileName = %q, want %q", name, "my_recipe.recipe.json.wk-meta.json")
	}
}

func TestFindMetaFiles(t *testing.T) {
	dir := t.TempDir()

	// Create an asset file and its meta
	assetPath := filepath.Join(dir, "recipe.json")
	os.WriteFile(assetPath, []byte(`{"name":"test"}`), 0644)

	meta := &AssetMeta{
		ServerPath:  "All projects/Test",
		ContentHash: ComputeHash([]byte(`{"name":"test"}`)),
	}
	WriteMeta(filepath.Join(dir, "recipe.json.wk-meta.json"), meta)

	metas, err := FindMetaFiles(dir)
	if err != nil {
		t.Fatalf("FindMetaFiles: %v", err)
	}

	if len(metas) != 1 {
		t.Fatalf("found %d metas, want 1", len(metas))
	}

	if _, ok := metas["recipe.json"]; !ok {
		t.Error("expected meta for recipe.json")
	}
}
