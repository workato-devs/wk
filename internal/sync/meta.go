package sync

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// AssetMeta is the sidecar metadata stored alongside each synced asset.
type AssetMeta struct {
	ServerPath   string    `json:"server_path"`
	ZipName      string    `json:"zip_name"`
	Folder       string    `json:"folder"`
	Type         string    `json:"type"` // "recipe", "connection"
	Version      int       `json:"version"`
	ContentHash  string    `json:"content_hash"` // SHA256
	LastPulledAt time.Time `json:"last_pulled_at"`
}

const metaSuffix = ".wk-meta.json"

// MetaFileName returns the .wk-meta.json filename for an asset file.
// e.g., "my_recipe.recipe.json" -> "my_recipe.recipe.json.wk-meta.json"
func MetaFileName(assetPath string) string {
	return assetPath + metaSuffix
}

// IsMetaFile reports whether a filename is a .wk-meta.json sidecar file.
func IsMetaFile(name string) bool {
	return strings.HasSuffix(name, metaSuffix)
}

// AssetForMeta returns the asset filename that a meta file describes.
func AssetForMeta(metaPath string) string {
	return strings.TrimSuffix(metaPath, metaSuffix)
}

// ReadMeta reads and unmarshals a .wk-meta.json file.
func ReadMeta(metaPath string) (*AssetMeta, error) {
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, fmt.Errorf("reading meta %s: %w", metaPath, err)
	}
	var meta AssetMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parsing meta %s: %w", metaPath, err)
	}
	return &meta, nil
}

// WriteMeta marshals and writes a .wk-meta.json file.
func WriteMeta(metaPath string, meta *AssetMeta) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding meta: %w", err)
	}
	data = append(data, '\n')
	return os.WriteFile(metaPath, data, 0644)
}

// ComputeHash returns the SHA256 hex digest of data.
func ComputeHash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// FindMetaFiles scans a directory for .wk-meta.json files and returns a map
// of asset filename (relative to dir) to its parsed AssetMeta.
func FindMetaFiles(dir string) (map[string]*AssetMeta, error) {
	result := make(map[string]*AssetMeta)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !IsMetaFile(info.Name()) {
			return nil
		}

		meta, err := ReadMeta(path)
		if err != nil {
			return fmt.Errorf("reading meta %s: %w", path, err)
		}

		// The asset file is the meta path minus the suffix.
		assetPath := AssetForMeta(path)
		rel, err := filepath.Rel(dir, assetPath)
		if err != nil {
			return err
		}
		result[rel] = meta
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scanning %s for meta files: %w", dir, err)
	}
	return result, nil
}
