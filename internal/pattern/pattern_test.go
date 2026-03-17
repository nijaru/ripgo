package pattern

import (
	"testing"

	"github.com/nijaru/ripgo/internal/config"
)

func TestLiteralMatcher(t *testing.T) {
	cfg := &config.Config{Pattern: "hello", FixedStrings: true}
	m, err := NewMatcher(cfg)
	if err != nil {
		t.Fatal(err)
	}

	locs, ok := m.Match([]byte("say hello world"))
	if !ok {
		t.Fatal("expected match")
	}
	if locs[0] != 4 || locs[1] != 9 {
		t.Fatalf("expected [4,9], got %v", locs)
	}

	_, ok = m.Match([]byte("say hi world"))
	if ok {
		t.Fatal("expected no match")
	}
}

func TestLiteralCaseInsensitive(t *testing.T) {
	cfg := &config.Config{Pattern: "hello", FixedStrings: true, IgnoreCase: true}
	m, err := NewMatcher(cfg)
	if err != nil {
		t.Fatal(err)
	}

	_, ok := m.Match([]byte("HELLO"))
	if !ok {
		t.Fatal("expected case-insensitive match")
	}
}

func TestRegexMatcher(t *testing.T) {
	cfg := &config.Config{Pattern: `\d+`}
	m, err := NewMatcher(cfg)
	if err != nil {
		t.Fatal(err)
	}

	locs, ok := m.Match([]byte("test 123 abc"))
	if !ok {
		t.Fatal("expected match")
	}
	if locs[0] != 5 || locs[1] != 8 {
		t.Fatalf("expected [5,8], got %v", locs)
	}
}

func TestSmartMatcherLiterals(t *testing.T) {
	tests := []struct {
		pattern string
		literal bool
	}{
		{"hello", true},
		{"hello.world", false},
		{"test[0-9]", false},
		{"a*b", false},
		{"path/to/file", true},
	}

	for _, tt := range tests {
		if got := isLiteral(tt.pattern); got != tt.literal {
			t.Errorf("isLiteral(%q) = %v, want %v", tt.pattern, got, tt.literal)
		}
	}
}

func TestMatchFile(t *testing.T) {
	cfg := &config.Config{Pattern: "found"}
	m, err := NewMatcher(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if !m.MatchFile([]byte("this has found in it")) {
		t.Fatal("expected MatchFile to return true")
	}
	if m.MatchFile([]byte("nothing here")) {
		t.Fatal("expected MatchFile to return false")
	}
}
