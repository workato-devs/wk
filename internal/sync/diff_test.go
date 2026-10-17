package sync

import "testing"

// Regression for #102: pull writes normalized JSON (normalizeJSON) and hashes
// the normalized bytes, so diff must hash remote JSON the same way. Otherwise
// every JSON asset reports "modified" immediately after a clone or pull, while
// non-JSON assets report "same".
func TestHashZipContentsMatchesPulledFiles(t *testing.T) {
	rawJSON := []byte(`{"name":"My Recipe","code":{"b":1,"a":2}}`) // unsorted, compact: as the server sends it
	rawRuby := []byte("puts 'hello'\n")
	badJSON := []byte(`{"name": "truncated"`) // fails to parse, so pull writes it as-is
	zipData := buildZipBytes(t, map[string][]byte{
		"Folder/my_recipe.recipe.json": rawJSON,
		"Folder/connector.rb":          rawRuby,
		"Folder/broken.json":           badJSON,
	})

	remote, err := hashZipContents(zipData)
	if err != nil {
		t.Fatalf("hashZipContents() error: %v", err)
	}

	normalized, err := normalizeJSON(rawJSON)
	if err != nil {
		t.Fatalf("normalizeJSON() error: %v", err)
	}
	pulled := map[string]string{
		"Folder/my_recipe.recipe.json": ComputeHash(normalized), // what extractOneFile writes for JSON
		"Folder/connector.rb":          ComputeHash(rawRuby),    // non-JSON is written as-is
		"Folder/broken.json":           ComputeHash(badJSON),    // unparseable JSON is written as-is
	}

	for path, want := range pulled {
		if got := remote[path]; got != want {
			t.Errorf("remote hash for %s = %s, want %s (the hash of the file pull writes)", path, got, want)
		}
	}
}
