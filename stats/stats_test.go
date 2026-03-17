package stats

import (
	"testing"

	"github.com/nijaru/ripgo/search"
)

func TestRecordMatch(t *testing.T) {
	var st Stats
	st.RecordMatch(search.Result{
		Path:    "test.go",
		Matches: []search.Match{{Line: 1, LineBytes: []byte("hello")}},
	})

	if st.TotalMatches() != 1 {
		t.Errorf("TotalMatches = %d, want 1", st.TotalMatches())
	}
	if st.FilesMatched() != 1 {
		t.Errorf("FilesMatched = %d, want 1", st.FilesMatched())
	}
	if st.FilesSearched() != 1 {
		t.Errorf("FilesSearched = %d, want 1", st.FilesSearched())
	}
}

func TestRecordNoMatch(t *testing.T) {
	var st Stats
	st.RecordMatch(search.Result{Path: "empty.go", Matches: nil})

	if st.TotalMatches() != 0 {
		t.Errorf("TotalMatches = %d, want 0", st.TotalMatches())
	}
	if st.FilesMatched() != 0 {
		t.Errorf("FilesMatched = %d, want 0", st.FilesMatched())
	}
	if st.FilesSearched() != 1 {
		t.Errorf("FilesSearched = %d, want 1", st.FilesSearched())
	}
}
