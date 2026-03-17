package pattern

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/nijaru/ripgo/internal/config"
)

type Matcher interface {
	Match(line []byte) (locs []int, ok bool)
	MatchFile(data []byte) bool
	Name() string
}

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

func (m *LiteralMatcher) Name() string { return "literal" }

type SmartMatcher struct {
	inner Matcher
}

func (m *SmartMatcher) Match(line []byte) (locs []int, ok bool) {
	return m.inner.Match(line)
}

func (m *SmartMatcher) MatchFile(data []byte) bool {
	return m.inner.MatchFile(data)
}

func (m *SmartMatcher) Name() string { return "smart" }

func NewMatcher(cfg *config.Config) (Matcher, error) {
	pattern := cfg.Pattern
	if pattern == "" {
		pattern = "()"
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

	if isLiteral(pattern) {
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

func isLiteral(s string) bool {
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

func IsRegexMeta(s string) bool {
	meta := ".*+?^${}[]|()\\"
	return strings.ContainsAny(s, meta)
}
