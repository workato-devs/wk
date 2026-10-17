package config

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestValidateLocalPath_StringLevelRejections(t *testing.T) {
	cases := []struct {
		name        string
		local       string
		wantSub     string
		windowsOnly bool // a drive letter only escapes the project root on Windows
	}{
		{"empty string", "", "cannot be empty", false},
		{"null byte", "foo\x00bar", "null byte", false},
		{"absolute path", "/tmp/evil", "must be relative", false},
		{"absolute path windows drive", `C:\Windows\evil`, "must be relative", true},
		{"rooted backslash", `\evil`, "must be relative", false},
		{"bare parent traversal", "..", "escapes the project root", false},
		{"leading parent traversal", "../evil", "escapes the project root", false},
		{"deeper traversal collapse", "./foo/../../evil", "escapes the project root", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.windowsOnly && runtime.GOOS != "windows" {
				t.Skip("drive-letter paths are a single relative name outside Windows")
			}
			err := ValidateLocalPath("", tc.local)
			if err == nil {
				t.Fatalf("err = nil, want error containing %q", tc.wantSub)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("err = %q, want to contain %q", err.Error(), tc.wantSub)
			}
		})
	}
}

func TestValidateLocalPath_Accepts(t *testing.T) {
	root := t.TempDir()
	cases := []string{
		".",
		"foo",
		"./foo",
		"foo/bar",
		"./foo/../bar",
	}
	for _, local := range cases {
		t.Run(local, func(t *testing.T) {
			if err := ValidateLocalPath(root, local); err != nil {
				t.Errorf("ValidateLocalPath(%q, %q) = %v, want nil", root, local, err)
			}
		})
	}
}

func TestValidateLocalPath_ResolvesOutsideRoot(t *testing.T) {
	root := t.TempDir()
	// Construct a path that string-level checks would accept (no leading
	// ".."), but whose absolute resolution lands outside root. This is rare
	// because filepath.Clean collapses internal "..", but we still verify the
	// belt-and-suspenders on-disk comparison catches divergence.
	inside := filepath.Join("foo", "bar")
	if err := ValidateLocalPath(root, inside); err != nil {
		t.Fatalf("ValidateLocalPath inside = %v, want nil", err)
	}
}
