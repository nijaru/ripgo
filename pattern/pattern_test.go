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

	// regex lowercase → case-insensitive
	m, err = New(Config{Pattern: `[a-z]+`, SmartCase: true})
	if err != nil {
		t.Fatal(err)
	}
	rm, ok := m.(*RegexMatcher)
	if !ok {
		t.Fatalf("expected regex matcher, got %T", m)
	}
	// Verify behavior via matching
	if !rm.re.Match([]byte("ABC")) {
		t.Error("expected case-insensitive regex match for lowercase pattern")
	}

	// regex with uppercase → case-sensitive
	m, err = New(Config{Pattern: `[A-Z]+`, SmartCase: true})
	if err != nil {
		t.Fatal(err)
	}
	rm, ok = m.(*RegexMatcher)
	if !ok {
		t.Fatal("expected regex matcher")
	}
	if rm.re.Match([]byte("abc")) {
		t.Error("expected case-sensitive regex match for uppercase pattern")
	}
}

func TestPCREMatcher(t *testing.T) {
	cfg := Config{Pattern: `(?<=foo)\d+`, Pcre2: true}
	m, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if m.Name() != "pcre" {
		t.Fatalf("expected pcre matcher, got %s", m.Name())
	}

	locs, ok := m.Match([]byte("foo123bar"))
	if !ok {
		t.Fatal("expected match with lookbehind")
	}
	if locs[0] != 3 || locs[1] != 6 {
		t.Fatalf("expected [3,6], got %v", locs)
	}

	// lookbehind fails when prefix absent
	_, ok = m.Match([]byte("bar123foo"))
	if ok {
		t.Fatal("expected no match without lookbehind prefix")
	}
}

func TestPCREMatcherLookahead(t *testing.T) {
	cfg := Config{Pattern: `\w+(?=\.)`, Pcre2: true}
	m, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	locs, ok := m.Match([]byte("file.txt"))
	if !ok {
		t.Fatal("expected match with lookahead")
	}
	if locs[0] != 0 || locs[1] != 4 {
		t.Fatalf("expected [0,4], got %v", locs)
	}
}

func TestPCREMatcherCaseInsensitive(t *testing.T) {
	cfg := Config{Pattern: `(?<=foo)\d+`, Pcre2: true, IgnoreCase: true}
	m, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	_, ok := m.Match([]byte("FOO123"))
	if !ok {
		t.Fatal("expected case-insensitive PCRE match")
	}
}

func TestPCREMatcherInvalidPattern(t *testing.T) {
	cfg := Config{Pattern: `(?P<name`, Pcre2: true}
	_, err := New(cfg)
	if err == nil {
		t.Fatal("expected error for invalid PCRE pattern")
	}
}

func TestPCREMatchFile(t *testing.T) {
	cfg := Config{Pattern: `(?<=start)\s+\w+`, Pcre2: true}
	m, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if !m.MatchFile([]byte("start hello end")) {
		t.Fatal("expected MatchFile to find lookbehind match")
	}
	if m.MatchFile([]byte("no prefix here")) {
		t.Fatal("expected MatchFile to return false")
	}
}

func TestLiterals_LiteralMatcher(t *testing.T) {
	m, _ := New(Config{Pattern: "hello", FixedStrings: true})
	lits := m.Literals()
	if len(lits) != 1 || string(lits[0]) != "hello" {
		t.Fatalf("expected [hello], got %v", lits)
	}

	// case-insensitive → nil
	m, _ = New(Config{Pattern: "hello", FixedStrings: true, IgnoreCase: true})
	if m.Literals() != nil {
		t.Fatal("expected nil for case-insensitive literal")
	}
}

func TestLiterals_RegexAlternation(t *testing.T) {
	m, _ := New(Config{Pattern: `(foo|bar|baz)`})
	lits := m.Literals()
	if len(lits) != 3 {
		t.Fatalf("expected 3 literals, got %d: %v", len(lits), lits)
	}
	seen := map[string]bool{}
	for _, l := range lits {
		seen[string(l)] = true
	}
	for _, want := range []string{"foo", "bar", "baz"} {
		if !seen[want] {
			t.Errorf("missing literal %q", want)
		}
	}
}

func TestLiterals_RegexAlternation_Two(t *testing.T) {
	m, _ := New(Config{Pattern: `(abc|def)`})
	lits := m.Literals()
	if len(lits) != 2 {
		t.Fatalf("expected 2 literals, got %d: %v", len(lits), lits)
	}
}

func TestLiterals_RegexNonAlternation(t *testing.T) {
	m, _ := New(Config{Pattern: `foo.*bar`})
	if m.Literals() != nil {
		t.Fatal("expected nil for non-alternation regex")
	}

	m, _ = New(Config{Pattern: `\d+`})
	if m.Literals() != nil {
		t.Fatal("expected nil for \\d+")
	}
}

func TestLiterals_RegexAlternationCaseInsensitive(t *testing.T) {
	m, _ := New(Config{Pattern: `(foo|bar)`, IgnoreCase: true})
	if m.Literals() != nil {
		t.Fatal("expected nil for case-insensitive alternation")
	}
}

func TestLiterals_RegexWithPrefix(t *testing.T) {
	// LiteralPrefix returns "foo" — Literals should return nil since single literal is handled by Literal()
	m, _ := New(Config{Pattern: `foo.*bar`})
	if m.Literals() != nil {
		// foo.*bar has LiteralPrefix "foo", so Literals should be nil
		// (the single-literal path already handles this)
		t.Fatal("expected nil when LiteralPrefix already provides a prefix")
	}
}

func BenchmarkPattern(b *testing.B) {
	lines := [][]byte{
		[]byte("func main() {"),
		[]byte("	fmt.Println(\"hello world\")"),
		[]byte("}"),
		[]byte("// TODO: refactor this"),
		[]byte("var x = 42"),
	}

	b.Run("literal_compile", func(b *testing.B) {
		for b.Loop() {
			New(Config{Pattern: "fmt"})
		}
	})

	m, _ := New(Config{Pattern: "fmt"})
	b.Run("literal_match", func(b *testing.B) {
		for b.Loop() {
			for _, line := range lines {
				m.Match(line)
			}
		}
	})

	b.Run("regex_compile", func(b *testing.B) {
		for b.Loop() {
			New(Config{Pattern: `func\s+\w+`})
		}
	})

	re, _ := New(Config{Pattern: `func\s+\w+`})
	b.Run("regex_match", func(b *testing.B) {
		for b.Loop() {
			for _, line := range lines {
				re.Match(line)
			}
		}
	})

	mci, _ := New(Config{Pattern: "fmt", IgnoreCase: true})
	b.Run("literal_match_ci", func(b *testing.B) {
		for b.Loop() {
			for _, line := range lines {
				mci.Match(line)
			}
		}
	})
}
