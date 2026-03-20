package search

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nijaru/ripgo/pattern"
)

func TestSearchFile(t *testing.T) {
	m, err := pattern.New(pattern.Config{Pattern: "func"})
	if err != nil {
		t.Fatal(err)
	}
	s := NewSearcher(nil, Config{}, m)

	result, err := s.SearchPath("search.go", nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Matches) == 0 {
		t.Fatal("expected matches in search.go")
	}
}

func TestSearchNoMatch(t *testing.T) {
	m, err := pattern.New(pattern.Config{Pattern: "xyznonexistent"})
	if err != nil {
		t.Fatal(err)
	}
	s := NewSearcher(nil, Config{}, m)

	result, err := s.SearchPath("search.go", nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(result.Matches))
	}
}

func TestSearchMaxCount(t *testing.T) {
	m, err := pattern.New(pattern.Config{Pattern: "func"})
	if err != nil {
		t.Fatal(err)
	}
	s := NewSearcher(nil, Config{MaxCount: 1}, m)

	result, err := s.SearchPath("search.go", nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Matches) != 1 {
		t.Errorf("expected exactly 1 match with MaxCount=1, got %d", len(result.Matches))
	}
}

func TestSearchNonexistentFile(t *testing.T) {
	m, err := pattern.New(pattern.Config{Pattern: "test"})
	if err != nil {
		t.Fatal(err)
	}
	s := NewSearcher(nil, Config{}, m)

	_, err = s.SearchPath("nonexistent_file_xyz.go", nil)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

// createTempFile creates a temporary file with the given content.
func createTempFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSearchContextBefore(t *testing.T) {
	m, err := pattern.New(pattern.Config{Pattern: "MATCH"})
	if err != nil {
		t.Fatal(err)
	}
	s := NewSearcher(nil, Config{Before: 2}, m)

	path := createTempFile(t, "line1\nline2\nMATCH\nline4\nline5\n")
	result, err := s.SearchPath(path, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(result.Matches))
	}

	if len(result.Entries) != 3 {
		t.Fatalf("expected 3 entries (2 context + 1 match), got %d", len(result.Entries))
	}

	// Entries should be: line1 (context), line2 (context), MATCH (match)
	if result.Entries[0].Kind != EntryContext || result.Entries[0].Line != 1 {
		t.Errorf("entry 0: expected context line 1, got kind=%d line=%d", result.Entries[0].Kind, result.Entries[0].Line)
	}
	if result.Entries[1].Kind != EntryContext || result.Entries[1].Line != 2 {
		t.Errorf("entry 1: expected context line 2, got kind=%d line=%d", result.Entries[1].Kind, result.Entries[1].Line)
	}
	if result.Entries[2].Kind != EntryMatch || result.Entries[2].Line != 3 {
		t.Errorf("entry 2: expected match line 3, got kind=%d line=%d", result.Entries[2].Kind, result.Entries[2].Line)
	}
}

func TestSearchContextAfter(t *testing.T) {
	m, err := pattern.New(pattern.Config{Pattern: "MATCH"})
	if err != nil {
		t.Fatal(err)
	}
	s := NewSearcher(nil, Config{After: 2}, m)

	path := createTempFile(t, "line1\nMATCH\nline3\nline4\nline5\n")
	result, err := s.SearchPath(path, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Entries) != 3 {
		t.Fatalf("expected 3 entries (1 match + 2 context), got %d", len(result.Entries))
	}

	// Entries: MATCH (match), line3 (context), line4 (context)
	if result.Entries[0].Kind != EntryMatch || result.Entries[0].Line != 2 {
		t.Errorf("entry 0: expected match line 2")
	}
	if result.Entries[1].Kind != EntryContext || result.Entries[1].Line != 3 {
		t.Errorf("entry 1: expected context line 3")
	}
	if result.Entries[2].Kind != EntryContext || result.Entries[2].Line != 4 {
		t.Errorf("entry 2: expected context line 4")
	}
}

func TestSearchContextBoth(t *testing.T) {
	m, err := pattern.New(pattern.Config{Pattern: "MATCH"})
	if err != nil {
		t.Fatal(err)
	}
	s := NewSearcher(nil, Config{Before: 1, After: 1}, m)

	path := createTempFile(t, "line1\nMATCH\nline3\n")
	result, err := s.SearchPath(path, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(result.Entries))
	}
}

func TestSearchContextOverlap(t *testing.T) {
	m, err := pattern.New(pattern.Config{Pattern: "X"})
	if err != nil {
		t.Fatal(err)
	}
	s := NewSearcher(nil, Config{Before: 2, After: 2}, m)

	// Two matches close together: their context overlaps
	path := createTempFile(t, "a\nX\nc\nd\nX\nf\n")
	result, err := s.SearchPath(path, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(result.Matches))
	}

	// Entries: a(ctx), X(match), c(ctx), d(ctx), X(match), f(ctx)
	// The context between matches should not duplicate.
	if len(result.Entries) != 6 {
		t.Fatalf("expected 6 entries, got %d", len(result.Entries))
	}

	// Verify no duplicate lines
	seen := make(map[int]bool)
	for _, e := range result.Entries {
		if seen[e.Line] {
			t.Errorf("duplicate entry for line %d", e.Line)
		}
		seen[e.Line] = true
	}
}

func TestSearchContextNoMatch(t *testing.T) {
	m, err := pattern.New(pattern.Config{Pattern: "NOMATCH"})
	if err != nil {
		t.Fatal(err)
	}
	s := NewSearcher(nil, Config{Before: 2, After: 2}, m)

	path := createTempFile(t, "a\nb\nc\n")
	result, err := s.SearchPath(path, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(result.Matches))
	}
	if len(result.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(result.Entries))
	}
}

func TestSearchContextAtStart(t *testing.T) {
	m, err := pattern.New(pattern.Config{Pattern: "MATCH"})
	if err != nil {
		t.Fatal(err)
	}
	s := NewSearcher(nil, Config{Before: 3, After: 1}, m)

	path := createTempFile(t, "MATCH\nline2\nline3\n")
	result, err := s.SearchPath(path, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Before context clamped to start of file — no lines before line 1
	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 entries (match + 1 after), got %d", len(result.Entries))
	}
	if result.Entries[0].Kind != EntryMatch || result.Entries[0].Line != 1 {
		t.Errorf("entry 0: expected match line 1")
	}
}

func TestSearchContextAtEnd(t *testing.T) {
	m, err := pattern.New(pattern.Config{Pattern: "MATCH"})
	if err != nil {
		t.Fatal(err)
	}
	s := NewSearcher(nil, Config{Before: 1, After: 3}, m)

	path := createTempFile(t, "line1\nline2\nMATCH\n")
	result, err := s.SearchPath(path, nil)
	if err != nil {
		t.Fatal(err)
	}

	// After context clamped to end of file
	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 entries (1 before + match), got %d", len(result.Entries))
	}
	if result.Entries[0].Kind != EntryContext || result.Entries[0].Line != 2 {
		t.Errorf("entry 0: expected context line 2")
	}
	if result.Entries[1].Kind != EntryMatch || result.Entries[1].Line != 3 {
		t.Errorf("entry 1: expected match line 3")
	}
}

func TestSearchContextWithMaxCount(t *testing.T) {
	m, err := pattern.New(pattern.Config{Pattern: "X"})
	if err != nil {
		t.Fatal(err)
	}
	s := NewSearcher(nil, Config{Before: 1, After: 1, MaxCount: 1}, m)

	path := createTempFile(t, "a\nX\nc\nd\nX\nf\n")
	result, err := s.SearchPath(path, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Matches) != 1 {
		t.Fatalf("expected 1 match (MaxCount), got %d", len(result.Matches))
	}

	// Should still have context for the one match
	if len(result.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(result.Entries))
	}
}

func TestSearchBinary(t *testing.T) {
	m, _ := pattern.New(pattern.Config{Pattern: "hello"})

	// Create binary file
	dir := t.TempDir()
	binPath := filepath.Join(dir, "data.bin")
	if err := os.WriteFile(binPath, []byte("hello\x00world"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("skip_by_default", func(t *testing.T) {
		s := NewSearcher(nil, Config{}, m)
		res, err := s.SearchPath(binPath, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !res.Binary {
			t.Error("expected Binary=true")
		}
		if len(res.Matches) > 0 {
			t.Error("expected 0 matches for binary file by default")
		}
	})

	t.Run("search_binary", func(t *testing.T) {
		s := NewSearcher(nil, Config{SearchBinary: true}, m)
		res, err := s.SearchPath(binPath, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !res.Binary {
			t.Error("expected Binary=true")
		}
		if len(res.Matches) == 0 {
			t.Error("expected matches in binary file when SearchBinary=true")
		}
	})

	t.Run("only_binary", func(t *testing.T) {
		s := NewSearcher(nil, Config{OnlyBinary: true}, m)

		// Binary file should have matches
		res, _ := s.SearchPath(binPath, nil)
		if len(res.Matches) == 0 {
			t.Error("expected matches in binary file")
		}

		// Text file should be skipped
		txtPath := createTempFile(t, "hello world")
		res2, _ := s.SearchPath(txtPath, nil)
		if len(res2.Matches) > 0 {
			t.Error("expected 0 matches for text file when OnlyBinary=true")
		}
	})
}

func TestSearchMmap(t *testing.T) {
	// Create a large file (>1MB) to trigger mmap
	dir := t.TempDir()
	path := filepath.Join(dir, "large.txt")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	content := "some text\nMATCH\nmore text\n"
	// Pad to > 1MB
	padding := make([]byte, 1024*1024+100)
	for i := range padding {
		padding[i] = 'a'
	}
	f.Write([]byte(content))
	f.Write(padding)
	f.Close()

	m, _ := pattern.New(pattern.Config{Pattern: "MATCH"})
	s := NewSearcher(nil, Config{}, m)
	res, err := s.SearchPath(path, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Matches) != 1 {
		t.Errorf("expected 1 match, got %d", len(res.Matches))
	}
}

func TestSearchSubmatches(t *testing.T) {
	// Regex with capture groups
	m, err := pattern.New(pattern.Config{Pattern: `(hello) (world)`})
	if err != nil {
		t.Fatal(err)
	}
	s := NewSearcher(nil, Config{}, m)

	path := createTempFile(t, "line 1: hello world\nline 2: no match\n")
	result, err := s.SearchPath(path, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(result.Matches))
	}

	m0 := result.Matches[0]
	// Submatches should be: [0]: "hello world", [1]: "hello", [2]: "world"
	if len(m0.Submatches) != 3 {
		t.Fatalf("expected 3 submatches, got %d: %v", len(m0.Submatches), m0.Submatches)
	}

	check := func(i int, want string) {
		start, end := m0.Submatches[i][0], m0.Submatches[i][1]
		got := string(m0.LineBytes[start:end])
		if got != want {
			t.Errorf("submatch %d: got %q, want %q", i, got, want)
		}
	}

	check(0, "hello world")
	check(1, "hello")
	check(2, "world")
}

func TestSearchSubmatchesPCRE(t *testing.T) {
	// PCRE with lookahead/lookbehind
	m, err := pattern.New(pattern.Config{Pattern: `(hello)(?= world)`, Pcre2: true})
	if err != nil {
		t.Fatal(err)
	}
	s := NewSearcher(nil, Config{}, m)

	path := createTempFile(t, "hello world")
	result, err := s.SearchPath(path, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(result.Matches))
	}

	m0 := result.Matches[0]
	// [0]: "hello", [1]: "hello"
	if len(m0.Submatches) != 2 {
		t.Fatalf("expected 2 submatches, got %d: %v", len(m0.Submatches), m0.Submatches)
	}

	got := string(m0.LineBytes[m0.Submatches[0][0]:m0.Submatches[0][1]])
	if got != "hello" {
		t.Errorf("full match: got %q, want %q", got, "hello")
	}

	got1 := string(m0.LineBytes[m0.Submatches[1][0]:m0.Submatches[1][1]])
	if got1 != "hello" {
		t.Errorf("group 1: got %q, want %q", got1, "hello")
	}
}

func TestSearchACPrelfilter(t *testing.T) {
	// Pattern with 2 literals triggers Aho-Corasick pre-filter.
	m, err := pattern.New(pattern.Config{Pattern: `(alpha|bravo)`})
	if err != nil {
		t.Fatal(err)
	}
	s := NewSearcher(nil, Config{}, m)

	// Verify the pre-filter was built.
	if s.prefilter == nil {
		t.Fatal("expected AC pre-filter for 2+ literals")
	}

	dir := t.TempDir()

	// File containing one of the literals.
	havePath := filepath.Join(dir, "have.txt")
	if err := os.WriteFile(havePath, []byte("no match here\nalpha is first\nalso bravo here\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// File containing neither literal.
	missPath := filepath.Join(dir, "miss.txt")
	if err := os.WriteFile(missPath, []byte("just some text\nnothing relevant here\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// File with the literal should match.
	res1, err := s.SearchPath(havePath, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res1.Matches) != 2 {
		t.Errorf("expected 2 matches in have.txt, got %d", len(res1.Matches))
	}

	// File without either literal should be skipped by pre-filter.
	res2, err := s.SearchPath(missPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res2.Matches) != 0 {
		t.Errorf("expected 0 matches in miss.txt, got %d", len(res2.Matches))
	}
}

func BenchmarkSearch(b *testing.B) {
	// Search the real search.go file for "func" — representative workload
	m, _ := pattern.New(pattern.Config{Pattern: "func"})
	s := NewSearcher(nil, Config{}, m)

	b.Run("literal", func(b *testing.B) {
		for b.Loop() {
			s.SearchPath("search.go", nil)
		}
	})

	b.Run("with_context", func(b *testing.B) {
		sCtx := NewSearcher(nil, Config{Before: 2, After: 2}, m)
		for b.Loop() {
			sCtx.SearchPath("search.go", nil)
		}
	})
}

func BenchmarkSearchRegex(b *testing.B) {
	m, _ := pattern.New(pattern.Config{Pattern: `func\s+\w+`})
	s := NewSearcher(nil, Config{}, m)

	b.Run("regex", func(b *testing.B) {
		for b.Loop() {
			s.SearchPath("search.go", nil)
		}
	})
}
