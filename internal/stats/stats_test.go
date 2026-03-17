package stats

import (
	"testing"

	"github.com/nijaru/ripgo/internal/config"
	"github.com/nijaru/ripgo/internal/search"
)

func TestExitCodeWithMatches(t *testing.T) {
	st := NewStats(&config.Config{})
	st.RecordMatch(search.Result{
		Path:    "test.go",
		Matches: []search.Match{{Line: 1, LineBytes: []byte("hello")}},
	})

	if st.ExitCode() != 0 {
		t.Error("expected exit code 0 for files with matches")
	}
	if st.TotalMatches() != 1 {
		t.Errorf("TotalMatches = %d, want 1", st.TotalMatches())
	}
	if st.FilesMatched() != 1 {
		t.Errorf("FilesMatched = %d, want 1", st.FilesMatched())
	}
}

func TestExitCodeNoMatches(t *testing.T) {
	st := NewStats(&config.Config{})

	if st.ExitCode() != 1 {
		t.Error("expected exit code 1 for no matches")
	}
}

func TestExitCodeQuietWithMatch(t *testing.T) {
	st := NewStats(&config.Config{Quiet: true})
	st.RecordMatch(search.Result{
		Path:    "test.go",
		Matches: []search.Match{{Line: 1, LineBytes: []byte("hello")}},
	})

	if st.ExitCode() != 0 {
		t.Error("expected exit code 0 for quiet mode with match")
	}
}

func TestExitCodeFilesWithMatches(t *testing.T) {
	st := NewStats(&config.Config{FilesWithMatches: true})
	st.RecordMatch(search.Result{
		Path:    "test.go",
		Matches: []search.Match{{Line: 1, LineBytes: []byte("hello")}},
	})

	if st.ExitCode() != 0 {
		t.Error("expected exit code 0 for files-with-matches mode")
	}
}

func TestFilesSearched(t *testing.T) {
	st := NewStats(&config.Config{})
	st.RecordMatch(search.Result{Path: "a.go", Matches: nil})
	st.RecordMatch(search.Result{Path: "b.go", Matches: []search.Match{{Line: 1, LineBytes: []byte("x")}}})

	if st.FilesSearched() != 2 {
		t.Errorf("FilesSearched = %d, want 2", st.FilesSearched())
	}
}
