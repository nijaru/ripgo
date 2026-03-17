package search

import (
	"testing"

	"github.com/nijaru/ripgo/internal/config"
	"github.com/nijaru/ripgo/internal/pattern"
)

func TestSearchFile(t *testing.T) {
	m, err := pattern.NewMatcher(&config.Config{Pattern: "func"})
	if err != nil {
		t.Fatal(err)
	}
	s := NewSearcher(&config.Config{}, m)

	result, err := s.Search("search.go")
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Matches) == 0 {
		t.Fatal("expected matches in search.go")
	}

	found := false
	for _, m := range result.Matches {
		if m.Line > 0 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected at least one match with valid line number")
	}
}

func TestSearchNoMatch(t *testing.T) {
	m, err := pattern.NewMatcher(&config.Config{Pattern: "xyznonexistent"})
	if err != nil {
		t.Fatal(err)
	}
	s := NewSearcher(&config.Config{}, m)

	result, err := s.Search("search.go")
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(result.Matches))
	}
}

func TestSearchMaxCount(t *testing.T) {
	m, err := pattern.NewMatcher(&config.Config{Pattern: "func"})
	if err != nil {
		t.Fatal(err)
	}
	s := NewSearcher(&config.Config{MaxCount: 1}, m)

	result, err := s.Search("search.go")
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Matches) != 1 {
		t.Errorf("expected exactly 1 match with MaxCount=1, got %d", len(result.Matches))
	}
}

func TestSearchNonexistentFile(t *testing.T) {
	m, err := pattern.NewMatcher(&config.Config{Pattern: "test"})
	if err != nil {
		t.Fatal(err)
	}
	s := NewSearcher(&config.Config{}, m)

	_, err = s.Search("nonexistent_file_xyz.go")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}
