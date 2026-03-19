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
	// The caller MUST NOT retain the locs slice after the call.
	Match(line []byte) (locs []int, ok bool)
	// MatchFile searches the entire file contents for the pattern.
	MatchFile(data []byte) bool
	// Name returns the matcher implementation name ("literal" or "regex").
	Name() string
}

// RegexMatcher wraps stdlib regexp for pattern matching.
// Users should generally create Matchers via New().
type RegexMatcher struct {
	re   *regexp.Regexp
	locs [2]int // reusable locs to avoid allocation
}

// Match searches for the pattern in a single line.
func (m *RegexMatcher) Match(line []byte) (locs []int, ok bool) {
	loc := m.re.FindIndex(line)
	if loc == nil {
		return nil, false
	}
	return loc, true
}

// MatchFile searches the entire file contents for the pattern.
func (m *RegexMatcher) MatchFile(data []byte) bool {
	return m.re.Match(data)
}

// Name returns "regex".
func (m *RegexMatcher) Name() string { return "regex" }

// LiteralMatcher uses byte search for fixed-string matching.
// It is significantly faster than RegexMatcher for simple patterns.
// Users should generally create Matchers via New().
type LiteralMatcher struct {
	pattern  []byte
	lower    []byte
	caseFold bool
	locs     [2]int // reusable locs to avoid allocation
}

// Match searches for the pattern in a single line.
func (m *LiteralMatcher) Match(line []byte) (locs []int, ok bool) {
	if m.caseFold {
		idx := indexCaseInsensitive(line, m.pattern)
		if idx < 0 {
			return nil, false
		}
		m.locs[0] = idx
		m.locs[1] = idx + len(m.pattern)
		return m.locs[:], true
	}
	idx := bytes.Index(line, m.pattern)
	if idx < 0 {
		return nil, false
	}
	m.locs[0] = idx
	m.locs[1] = idx + len(m.pattern)
	return m.locs[:], true
}

// MatchFile searches the entire file contents for the pattern.
func (m *LiteralMatcher) MatchFile(data []byte) bool {
	if m.caseFold {
		return indexCaseInsensitive(data, m.pattern) >= 0
	}
	return bytes.Contains(data, m.pattern)
}

// Name returns "literal".
func (m *LiteralMatcher) Name() string { return "literal" }

// CaseFold returns true if this matcher performs case-insensitive matching.
func (m *LiteralMatcher) CaseFold() bool { return m.caseFold }

// New compiles a Matcher from the given config.
func New(cfg Config) (Matcher, error) {
	pattern := cfg.Pattern
	if pattern == "" {
		pattern = "()"
	}

	if cfg.SmartCase && !cfg.IgnoreCase {
		cfg.IgnoreCase = !hasUppercase(pattern)
	}

	if cfg.FixedStrings {
		lit := []byte(pattern)
		return &LiteralMatcher{
			pattern:  lit,
			caseFold: cfg.IgnoreCase,
		}, nil
	}

	if IsLiteral(pattern) {
		lit := []byte(pattern)
		return &LiteralMatcher{
			pattern:  lit,
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

// indexCaseInsensitive performs a non-allocating case-insensitive byte search.
// It is optimized for ASCII.
func indexCaseInsensitive(line, pattern []byte) int {
	if len(pattern) == 0 {
		return 0
	}
	if len(pattern) > len(line) {
		return -1
	}

	first := pattern[0]
	firstAlt := first
	if first >= 'a' && first <= 'z' {
		firstAlt = first - ('a' - 'A')
	} else if first >= 'A' && first <= 'Z' {
		firstAlt = first + ('a' - 'A')
	}

	for i := 0; i <= len(line)-len(pattern); i++ {
		c := line[i]
		if c == first || c == firstAlt {
			if equalFold(line[i:i+len(pattern)], pattern) {
				return i
			}
		}
	}
	return -1
}

// equalFold reports whether s and t, interpreted as UTF-8 strings,
// are equal under Unicode case-folding, but optimized for byte slices.
func equalFold(s, t []byte) bool {
	if len(s) != len(t) {
		return false
	}
	for i := 0; i < len(s); i++ {
		sc, tc := s[i], t[i]
		if sc == tc {
			continue
		}
		if sc >= 'a' && sc <= 'z' {
			sc -= 'a' - 'A'
		}
		if tc >= 'a' && tc <= 'z' {
			tc -= 'a' - 'A'
		}
		if sc != tc {
			return false
		}
	}
	return true
}
