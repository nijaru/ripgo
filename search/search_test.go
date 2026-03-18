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
	s := NewSearcher(Config{}, m)

	result, err := s.Search("search.go")
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
	s := NewSearcher(Config{}, m)

	result, err := s.Search("search.go")
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
	s := NewSearcher(Config{MaxCount: 1}, m)

	result, err := s.Search("search.go")
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
	s := NewSearcher(Config{}, m)

	_, err = s.Search("nonexistent_file_xyz.go")
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
	s := NewSearcher(Config{Before: 2}, m)

	path := createTempFile(t, "line1\nline2\nMATCH\nline4\nline5\n")
	result, err := s.Search(path)
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
	s := NewSearcher(Config{After: 2}, m)

	path := createTempFile(t, "line1\nMATCH\nline3\nline4\nline5\n")
	result, err := s.Search(path)
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
	s := NewSearcher(Config{Before: 1, After: 1}, m)

	path := createTempFile(t, "line1\nMATCH\nline3\n")
	result, err := s.Search(path)
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
	s := NewSearcher(Config{Before: 2, After: 2}, m)

	// Two matches close together: their context overlaps
	path := createTempFile(t, "a\nX\nc\nd\nX\nf\n")
	result, err := s.Search(path)
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
	s := NewSearcher(Config{Before: 2, After: 2}, m)

	path := createTempFile(t, "a\nb\nc\n")
	result, err := s.Search(path)
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
	s := NewSearcher(Config{Before: 3, After: 1}, m)

	path := createTempFile(t, "MATCH\nline2\nline3\n")
	result, err := s.Search(path)
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
	s := NewSearcher(Config{Before: 1, After: 3}, m)

	path := createTempFile(t, "line1\nline2\nMATCH\n")
	result, err := s.Search(path)
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
	s := NewSearcher(Config{Before: 1, After: 1, MaxCount: 1}, m)

	path := createTempFile(t, "a\nX\nc\nd\nX\nf\n")
	result, err := s.Search(path)
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
