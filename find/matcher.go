// Package find provides filename and path matching for finder mode.
package find

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/gobwas/glob"
)

// MatcherConfig controls how a finder pattern is compiled.
type MatcherConfig struct {
	// Pattern is matched against a basename by default. An empty pattern
	// matches every path.
	Pattern string
	// Glob treats Pattern as a glob expression instead of a regular expression.
	Glob bool
	// FixedStrings treats Pattern as a literal substring.
	FixedStrings bool
	// IgnoreCase enables Unicode-aware case-insensitive matching.
	IgnoreCase bool
	// FullPath matches the normalized path passed to Match instead of its
	// basename. The caller supplies the path relative to its search root.
	FullPath bool
}

// Matcher matches finder paths without reading their contents.
//
// A Matcher is immutable after construction and may be shared by concurrent
// callers.
type Matcher struct {
	fullPath bool
	match    func(string) bool
}

// NewMatcher compiles a finder matcher from cfg.
func NewMatcher(cfg MatcherConfig) (*Matcher, error) {
	if cfg.Glob && cfg.FixedStrings {
		return nil, fmt.Errorf("find: glob and fixed-string modes are mutually exclusive")
	}

	if cfg.Pattern == "" {
		return &Matcher{
			fullPath: cfg.FullPath,
			match: func(string) bool {
				return true
			},
		}, nil
	}

	pattern := cfg.Pattern
	if cfg.IgnoreCase {
		pattern = strings.ToLower(pattern)
	}

	var match func(string) bool
	switch {
	case cfg.FixedStrings:
		match = func(candidate string) bool {
			if cfg.IgnoreCase {
				candidate = strings.ToLower(candidate)
			}
			return strings.Contains(candidate, pattern)
		}
	case cfg.Glob:
		compiled, err := glob.Compile(pattern, '/')
		if err != nil {
			return nil, fmt.Errorf("find: compile glob %q: %w", cfg.Pattern, err)
		}
		match = func(candidate string) bool {
			if cfg.IgnoreCase {
				candidate = strings.ToLower(candidate)
			}
			return compiled.Match(candidate)
		}
	default:
		if cfg.IgnoreCase {
			pattern = "(?i:" + cfg.Pattern + ")"
		}
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("find: compile pattern %q: %w", cfg.Pattern, err)
		}
		match = compiled.MatchString
	}

	return &Matcher{fullPath: cfg.FullPath, match: match}, nil
}

// Match reports whether name matches the configured finder pattern.
// name may use either slash or backslash separators; matching normalizes both
// to slash separators first.
func (m *Matcher) Match(name string) bool {
	if m == nil || m.match == nil {
		return false
	}

	name = NormalizePath(name)
	if !m.fullPath {
		name = path.Base(name)
	}
	return m.match(name)
}

// NormalizePath converts path separators to slash and cleans a path for
// matching. It preserves an absolute path's leading slash.
func NormalizePath(name string) string {
	if name == "" {
		return ""
	}
	name = strings.ReplaceAll(name, "\\", "/")
	return path.Clean(name)
}
