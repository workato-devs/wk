package sync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMatcher_MissingFile(t *testing.T) {
	m, err := LoadMatcher(t.TempDir())
	if err != nil {
		t.Fatalf("LoadMatcher: %v", err)
	}
	if m.Match("anything", false) {
		t.Error("empty matcher should match nothing")
	}
}

func TestMatcher_Patterns(t *testing.T) {
	dir := t.TempDir()
	content := []string{
		"# comment",
		"",
		"*.bak",
		"test_fixtures/",
		"build/**/*.tmp",
		"!build/keep.tmp",
		"docs/**",
	}
	writeIgnore(t, dir, content)

	m, err := LoadMatcher(dir)
	if err != nil {
		t.Fatalf("LoadMatcher: %v", err)
	}

	cases := []struct {
		path  string
		isDir bool
		want  bool
	}{
		{"foo.bak", false, true},
		{"recipes/foo.bak", false, true},
		{"recipes/foo.json", false, false},
		{"test_fixtures", true, true},
		{"test_fixtures/x.json", false, false}, // file itself isn't matched, but walker would SkipDir
		{"build/a/b/x.tmp", false, true},
		{"build/keep.tmp", false, false}, // negated
		{"docs/readme.md", false, true},
		{"docs/sub/file.md", false, true},
		{"src/main.go", false, false},
	}
	for _, tc := range cases {
		got := m.Match(tc.path, tc.isDir)
		if got != tc.want {
			t.Errorf("Match(%q, isDir=%v) = %v, want %v", tc.path, tc.isDir, got, tc.want)
		}
	}
}

func TestMatcher_ShouldSkipWkDir(t *testing.T) {
	m := &Matcher{}
	if !m.ShouldSkip(".wk", true) {
		t.Error(".wk should always be skipped")
	}
	if !m.ShouldSkip(".wk/recipes", true) {
		t.Error(".wk/* should always be skipped")
	}
	if m.ShouldSkip("recipes/slack.recipe.json", false) {
		t.Error("unrelated file should not be skipped by default")
	}
}

func TestMatcher_DirOnly(t *testing.T) {
	dir := t.TempDir()
	writeIgnore(t, dir, []string{"logs/"})
	m, err := LoadMatcher(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Match("logs", true) {
		t.Error("logs/ should match directory")
	}
	if m.Match("logs", false) {
		t.Error("logs/ should NOT match a file named 'logs'")
	}
}

func TestMatcher_Negation(t *testing.T) {
	dir := t.TempDir()
	writeIgnore(t, dir, []string{
		"*.json",
		"!keep.json",
	})
	m, err := LoadMatcher(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Match("foo.json", false) {
		t.Error("foo.json should be ignored")
	}
	if m.Match("keep.json", false) {
		t.Error("keep.json should be re-included via negation")
	}
}

func writeIgnore(t *testing.T, dir string, lines []string) {
	t.Helper()
	var buf []byte
	for _, l := range lines {
		buf = append(buf, l...)
		buf = append(buf, '\n')
	}
	if err := os.WriteFile(filepath.Join(dir, IgnoreFile), buf, 0644); err != nil {
		t.Fatalf("writing %s: %v", IgnoreFile, err)
	}
}
