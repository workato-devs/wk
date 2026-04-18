package sync

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/workato-devs/wk-cli-beta/internal/config"
)

// IgnoreFile is the filename at the project root whose patterns exclude
// files and directories from sync operations (ADR-005 Decision 10).
const IgnoreFile = ".wkignore"

// Matcher evaluates paths against a set of .wkignore patterns.
//
// Semantics follow the subset of gitignore documented in ADR-005 Decision 10:
//   - `*` matches within a single path component
//   - `**` matches across path separators
//   - a trailing `/` restricts the pattern to directories
//   - a leading `!` negates a previous match
//   - `#` begins a comment; blank lines are ignored
//   - all patterns evaluate relative to the project root
//
// Anything beyond this spec (leading-slash anchoring beyond the default,
// re-inclusion after parent exclusion, character classes) is intentionally
// out of scope.
type Matcher struct {
	patterns []ignorePattern
}

type ignorePattern struct {
	raw      string
	negate   bool
	dirOnly  bool
	segments []string // path split by "/"; "**" preserved as a sentinel
	// anchored is true when the pattern contains "/" somewhere other than
	// at the trailing position — such patterns are matched from the root,
	// not from any subdirectory.
	anchored bool
}

// LoadMatcher reads <projectRoot>/.wkignore and returns a compiled Matcher.
// If the file is absent, returns a non-nil Matcher that matches nothing
// (default: include everything).
func LoadMatcher(projectRoot string) (*Matcher, error) {
	path := filepath.Join(projectRoot, IgnoreFile)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Matcher{}, nil
		}
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

	m := &Matcher{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), " \t\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m.patterns = append(m.patterns, compilePattern(line))
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return m, nil
}

// compilePattern converts a raw pattern line into its parsed form.
func compilePattern(raw string) ignorePattern {
	p := ignorePattern{raw: raw}

	if strings.HasPrefix(raw, "!") {
		p.negate = true
		raw = raw[1:]
	}
	if strings.HasSuffix(raw, "/") {
		p.dirOnly = true
		raw = strings.TrimSuffix(raw, "/")
	}
	// Strip a leading slash — semantically it only means "anchored to root",
	// which is already the default once the pattern has any interior "/".
	raw = strings.TrimPrefix(raw, "/")

	// A pattern is anchored if it contains a path separator after
	// trimming the leading/trailing slashes.
	p.anchored = strings.Contains(raw, "/")
	p.segments = strings.Split(raw, "/")
	return p
}

// Match reports whether relPath (project-root-relative, forward slashes)
// should be ignored. isDir disambiguates directory-only patterns.
//
// Evaluation is last-match-wins, so a later negating pattern can re-include
// a previously matched path.
func (m *Matcher) Match(relPath string, isDir bool) bool {
	if m == nil || len(m.patterns) == 0 {
		return false
	}
	path := filepath.ToSlash(relPath)
	parts := strings.Split(strings.Trim(path, "/"), "/")

	matched := false
	for _, p := range m.patterns {
		if p.dirOnly && !isDir {
			continue
		}
		if p.matchPath(parts) {
			matched = !p.negate
		}
	}
	return matched
}

// matchPath walks the pattern segments against the path segments.
func (p ignorePattern) matchPath(parts []string) bool {
	if p.anchored {
		return matchSegments(p.segments, parts)
	}
	// Unanchored: the pattern may match any suffix of the path, and when
	// the pattern is a single segment it also matches any file/dir along
	// the path (standard gitignore behavior for bare patterns).
	if len(p.segments) == 1 {
		for _, seg := range parts {
			if matchSegments(p.segments, []string{seg}) {
				return true
			}
		}
		return false
	}
	for i := range parts {
		if matchSegments(p.segments, parts[i:]) {
			return true
		}
	}
	return false
}

// matchSegments matches pattern segments against path segments, handling
// "**" as a zero-or-more segment wildcard.
func matchSegments(pattern, path []string) bool {
	for len(pattern) > 0 {
		head := pattern[0]
		if head == "**" {
			// Consume consecutive "**" segments — they collapse.
			for len(pattern) > 0 && pattern[0] == "**" {
				pattern = pattern[1:]
			}
			if len(pattern) == 0 {
				return true // trailing ** matches the rest
			}
			// Try every alignment of the remaining pattern against path.
			for i := 0; i <= len(path); i++ {
				if matchSegments(pattern, path[i:]) {
					return true
				}
			}
			return false
		}
		if len(path) == 0 {
			return false
		}
		ok, err := filepath.Match(head, path[0])
		if err != nil || !ok {
			return false
		}
		pattern = pattern[1:]
		path = path[1:]
	}
	return len(path) == 0
}

// ShouldSkip reports whether the walker should skip a path entirely.
// This combines the implicit .wk/ skip (regardless of .wkignore content,
// ADR-005 Decision 10) with the user's .wkignore rules.
//
// relPath must be project-root-relative using forward slashes.
func (m *Matcher) ShouldSkip(relPath string, isDir bool) bool {
	if relPath == "" || relPath == "." {
		return false
	}
	// .wk/ is always skipped, regardless of .wkignore.
	parts := strings.Split(filepath.ToSlash(relPath), "/")
	if len(parts) > 0 && parts[0] == config.ProjectDir {
		return true
	}
	return m.Match(relPath, isDir)
}
