package pattern

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/dlclark/regexp2"
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
	// Pcre2 uses PCRE2-compatible regex engine.
	Pcre2 bool
}

// Matcher performs line-oriented matching.
type Matcher interface {
	// Match searches for the pattern in a single line.
	// Returns the byte offsets of the match and all capture groups.
	// locs[0]:locs[1] is the full match, locs[2]:locs[3] is group 1, etc.
	// The caller MUST NOT retain the locs slice after the call.
	Match(line []byte) (locs []int, ok bool)
	// MatchFile searches the entire file contents for the pattern.
	MatchFile(data []byte) bool
	// Name returns the matcher implementation name ("literal", "regex", or "pcre").
	Name() string
}

// RegexMatcher wraps stdlib regexp for pattern matching.
// Users should generally create Matchers via New().
type RegexMatcher struct {
	re *regexp.Regexp
}

// Match searches for the pattern in a single line.
func (m *RegexMatcher) Match(line []byte) (locs []int, ok bool) {
	locs = m.re.FindSubmatchIndex(line)
	if locs == nil {
		return nil, false
	}
	return locs, true
}

// MatchFile searches the entire file contents for the pattern.
func (m *RegexMatcher) MatchFile(data []byte) bool {
	return m.re.Match(data)
}

// Name returns "regex".
func (m *RegexMatcher) Name() string { return "regex" }

// PCREMatcher wraps regexp2 for PCRE2-compatible pattern matching.
// Users should generally create Matchers via New().
type PCREMatcher struct {
	re *regexp2.Regexp
}

// Match searches for the pattern in a single line.
func (m *PCREMatcher) Match(line []byte) (locs []int, ok bool) {
	s := string(line)
	mt, err := m.re.FindStringMatch(s)
	if err != nil || mt == nil {
		return nil, false
	}

	groups := mt.Groups()
	locs = make([]int, 0, 2*len(groups))

	// Cache rune offsets for the current line to avoid O(N^2) on multiple groups
	var runeToByte []int

	for _, g := range groups {
		if g.Index < 0 {
			locs = append(locs, -1, -1)
			continue
		}

		if runeToByte == nil {
			runeToByte = make([]int, 0, len(s))
			for bi := range s {
				runeToByte = append(runeToByte, bi)
			}
			runeToByte = append(runeToByte, len(s))
		}

		start := runeToByte[g.Index]
		end := runeToByte[g.Index+g.Length]
		locs = append(locs, start, end)
	}
	return locs, true
}

// MatchFile searches the entire file contents for the pattern.
func (m *PCREMatcher) MatchFile(data []byte) bool {
	ok, err := m.re.MatchRunes([]rune(string(data)))
	return err == nil && ok
}

// Name returns "pcre".
func (m *PCREMatcher) Name() string { return "pcre" }

// newPCREMatcher compiles a PCRE2-compatible matcher.
func newPCREMatcher(pattern string, ignoreCase bool) (*PCREMatcher, error) {
	var flags regexp2.RegexOptions
	if ignoreCase {
		flags |= regexp2.IgnoreCase
	}
	re, err := regexp2.Compile(pattern, flags)
	if err != nil {
		return nil, err
	}
	return &PCREMatcher{re: re}, nil
}

// runeToByteOffset converts a rune index to a byte offset in s.
func runeToByteOffset(s string, runeIdx int) int {
	if runeIdx <= 0 {
		return 0
	}
	i := 0
	for bi := range s {
		if i == runeIdx {
			return bi
		}
		i++
	}
	return len(s)
}

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

	if cfg.Pcre2 {
		return newPCREMatcher(pattern, cfg.IgnoreCase)
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
