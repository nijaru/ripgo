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
	// Literal returns the fixed literal string if the matcher is purely literal,
	// or an empty slice if it's a regex. This allows for fast pre-filtering.
	Literal() []byte
	// Literals returns all mandatory literal strings for multi-pattern
	// pre-filtering (e.g., Aho-Corasick). Returns nil if extraction is not
	// possible (regex metacharacters, case-insensitive without a clean set, etc.).
	// When non-nil, every byte slice must appear in the data for a match to be
	// possible. Used by the search pre-filter to skip files early.
	Literals() [][]byte
}

// RegexMatcher wraps stdlib regexp for pattern matching.
// Users should generally create Matchers via New().
type RegexMatcher struct {
	re *regexp.Regexp
}

func (m *RegexMatcher) Literal() []byte {
	prefix, _ := m.re.LiteralPrefix()
	if prefix != "" {
		return []byte(prefix)
	}
	return nil
}

// Literals extracts mandatory literals from simple alternation patterns like
// (foo|bar|baz). Returns nil for non-alternation regexes or case-insensitive
// patterns where Literal() already has a prefix.
func (m *RegexMatcher) Literals() [][]byte {
	src := m.re.String()

	// If the literal prefix already works, no need for multi-literal.
	if p, complete := m.re.LiteralPrefix(); complete {
		return [][]byte{[]byte(p)}
	} else if p != "" {
		return nil
	}

	// Check for case-insensitive flag at the start.
	caseInsensitive := false
	pattern := src
	if strings.HasPrefix(pattern, "(?i:") || strings.HasPrefix(pattern, "(?i)") {
		caseInsensitive = true
	}
	if caseInsensitive {
		return nil // AC can't do case-insensitive matching cheaply.
	}

	lits := extractAlternationLiterals(pattern)
	if len(lits) < 2 {
		return nil
	}
	return lits
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
	re      *regexp2.Regexp
	pattern string
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

func (m *PCREMatcher) Literal() []byte {
	// PCRE patterns often start with (?flags). If so, we can't easily extract a literal prefix
	// that respects those flags without complex parsing.
	if strings.HasPrefix(m.pattern, "(?") {
		return nil
	}
	// Simple prefix extraction: take all characters until the first meta-character.
	for i, r := range m.pattern {
		if strings.ContainsRune(".*+?^${}[]|()\\", r) {
			if i > 0 {
				return []byte(m.pattern[:i])
			}
			return nil
		}
	}
	return []byte(m.pattern)
}

// Literals extracts literals from simple alternation patterns.
func (m *PCREMatcher) Literals() [][]byte {
	if strings.HasPrefix(m.pattern, "(?") {
		return nil
	}
	lits := extractAlternationLiterals(m.pattern)
	if len(lits) < 2 {
		return nil
	}
	return lits
}

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
	return &PCREMatcher{re: re, pattern: pattern}, nil
}

// extractAlternationLiterals parses simple alternation patterns of the form
// (lit1|lit2|lit3) and returns the individual literals. Returns nil if the
// pattern is not a simple alternation of literal branches.
func extractAlternationLiterals(pattern string) [][]byte {
	// Must match: (branch1|branch2|...)
	if len(pattern) < 5 { // min: (a|b)
		return nil
	}
	if pattern[0] != '(' || pattern[len(pattern)-1] != ')' {
		return nil
	}
	inner := pattern[1 : len(pattern)-1]

	var lits [][]byte
	start := 0
	for i := 0; i <= len(inner); i++ {
		if i == len(inner) || inner[i] == '|' {
			branch := inner[start:i]
			if !isLiteralBytes(branch) {
				return nil
			}
			lits = append(lits, []byte(branch))
			start = i + 1
		}
	}
	if len(lits) < 2 {
		return nil
	}
	return lits
}

// isLiteralBytes reports whether s contains no regex metacharacters.
func isLiteralBytes(s string) bool {
	if len(s) == 0 {
		return false
	}
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

// Literal returns the fixed literal string if the matcher is purely literal
// and case-sensitive.
func (m *LiteralMatcher) Literal() []byte {
	if m.caseFold {
		return nil
	}
	return m.pattern
}

// Literals returns the literal as a single-element slice, or nil if
// case-insensitive (AC can't cheaply handle case-folded matching).
func (m *LiteralMatcher) Literals() [][]byte {
	if m.caseFold {
		return nil
	}
	return [][]byte{m.pattern}
}

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
