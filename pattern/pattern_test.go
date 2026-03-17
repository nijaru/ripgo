package pattern

import "testing"

func TestLiteralMatcher(t *testing.T) {
	cfg := Config{Pattern: "hello", FixedStrings: true}
	m, err := New(cfg)
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
	cfg := Config{Pattern: "hello", FixedStrings: true, IgnoreCase: true}
	m, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	_, ok := m.Match([]byte("HELLO"))
	if !ok {
		t.Fatal("expected case-insensitive match")
	}
}

func TestRegexMatcher(t *testing.T) {
	cfg := Config{Pattern: `\d+`}
	m, err := New(cfg)
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

func TestIsLiteral(t *testing.T) {
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
		if got := IsLiteral(tt.pattern); got != tt.literal {
			t.Errorf("IsLiteral(%q) = %v, want %v", tt.pattern, got, tt.literal)
		}
	}
}

func TestMatchFile(t *testing.T) {
	cfg := Config{Pattern: "found"}
	m, err := New(cfg)
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

func TestSmartCase(t *testing.T) {
	// all-lowercase → case-insensitive literal
	m, err := New(Config{Pattern: "hello", SmartCase: true})
	if err != nil {
		t.Fatal(err)
	}
	lm, ok := m.(*LiteralMatcher)
	if !ok {
		t.Fatal("expected literal matcher for lowercase pattern")
	}
	if !lm.CaseFold() {
		t.Error("expected caseFold for all-lowercase pattern with smart case")
	}

	// contains uppercase → case-sensitive literal (no regex meta → still literal)
	m, err = New(Config{Pattern: "Hello", SmartCase: true})
	if err != nil {
		t.Fatal(err)
	}
	lm, ok = m.(*LiteralMatcher)
	if !ok {
		t.Fatal("expected literal matcher for 'Hello' (no metacharacters)")
	}
	if lm.CaseFold() {
		t.Error("expected case-sensitive for pattern with uppercase")
	}
}
