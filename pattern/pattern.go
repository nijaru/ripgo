package pattern

import (
	"bytes"
	"regexp"
	"strings"
)

// Config holds pattern matching options.
type Config struct {
	// Pattern is the search string or regex.
	Pattern string
	// FixedStrings treats the pattern as a literal string.
	FixedStrings bool
	// IgnoreCase makes matching case-insensitive.
	IgnoreCase bool
	// SmartCase makes matching case-insensitive unless the pattern
	// contains uppercase characters.
	SmartCase bool
}

// Matcher performs line-oriented matching.
type Matcher interface {
	// Match searches for the pattern in a single line.
	// Returns the byte offsets of the first match and whether one was found.
	Match(line []byte) (locs []int, ok bool)
	// MatchFile searches the entire file contents for the pattern.
	MatchFile(data []byte) bool
	// Name returns the matcher implementation name ("literal" or "regex").
	Name() string
}

// RegexMatcher wraps stdlib regexp for pattern matching.
type RegexMatcher struct {
	re *regexp.Regexp
}

func (m *RegexMatcher) Match(line []byte) (locs []int, ok bool) {
	loc := m.re.FindIndex(line)
	if loc == nil {
		return nil, false
	}
	return loc, true
}

func (m *RegexMatcher) MatchFile(data []byte) bool {
	return m.re.Match(data)
}

func (m *RegexMatcher) Name() string { return "regex" }

// LiteralMatcher uses byte search for fixed-string matching.
type LiteralMatcher struct {
	pattern  []byte
	lower    []byte
	caseFold bool
}

func (m *LiteralMatcher) Match(line []byte) (locs []int, ok bool) {
	if m.caseFold {
		lower := bytes.ToLower(line)
		idx := bytes.Index(lower, m.lower)
		if idx < 0 {
			return nil, false
		}
		return []int{idx, idx + len(m.pattern)}, true
	}
	idx := bytes.Index(line, m.pattern)
	if idx < 0 {
		return nil, false
	}
	return []int{idx, idx + len(m.pattern)}, true
}

func (m *LiteralMatcher) MatchFile(data []byte) bool {
	_, ok := m.Match(data)
	return ok
}

// CaseFold returns true if this matcher performs case-insensitive matching.
func (m *LiteralMatcher) CaseFold() bool { return m.caseFold }

func (m *LiteralMatcher) Name() string { return "literal" }

// New compiles a Matcher from the given config.
func New(cfg Config) (Matcher, error) {
	pattern := cfg.Pattern
	if pattern == "" {
		pattern = "()"
	}

	if cfg.SmartCase && !cfg.IgnoreCase && !cfg.FixedStrings {
		cfg.IgnoreCase = !hasUppercase(pattern)
	}

	if cfg.FixedStrings {
		lit := []byte(pattern)
		lower := bytes.ToLower(lit)
		return &LiteralMatcher{
			pattern:  lit,
			lower:    lower,
			caseFold: cfg.IgnoreCase,
		}, nil
	}

	if IsLiteral(pattern) {
		lit := []byte(pattern)
		lower := bytes.ToLower(lit)
		return &LiteralMatcher{
			pattern:  lit,
			lower:    lower,
			caseFold: cfg.IgnoreCase,
		}, nil
	}

	flags := ""
	if cfg.IgnoreCase {
		flags += "i"
	}
	if flags != "" {
		pattern = "(?" + flags + ")" + pattern
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return &RegexMatcher{re: re}, nil
}

// IsLiteral reports whether the pattern contains no regex metacharacters.
func IsLiteral(s string) bool {
	for _, r := range s {
		switch r {
		case '.', '*', '+', '?', '^', '$', '{', '}', '[', ']', '|', '(', ')', '\\':
			return false
		default:
			if r <= 31 || r == 127 {
				return false
			}
		}
	}
	return true
}

// IsRegexMeta reports whether s contains any regex metacharacters.
func IsRegexMeta(s string) bool {
	meta := ".*+?^${}[]|()\\"
	return strings.ContainsAny(s, meta)
}

func hasUppercase(s string) bool {
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			return true
		}
	}
	return false
}
