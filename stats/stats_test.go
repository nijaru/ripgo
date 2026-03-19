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
	if st.TotalFiles() != 1 {
		t.Errorf("TotalFiles = %d, want 1", st.TotalFiles())
	}
}

func TestRecordNoMatch(t *testing.T) {
	var st Stats
	st.RecordMatch(search.Result{Path: "empty.go", Matches: nil})

	if st.TotalMatches() != 0 {
		t.Errorf("TotalMatches = %d, want 0", st.TotalMatches())
	}
	if st.TotalFiles() != 0 {
		t.Errorf("TotalFiles = %d, want 0", st.TotalFiles())
	}
}

func BenchmarkRecordMatch(b *testing.B) {
	r := search.Result{
		Path:    "test.go",
		Matches: []search.Match{{Line: 1}, {Line: 2}, {Line: 3}},
	}
	b.ResetTimer()
	for range b.N {
		var st Stats
		st.RecordMatch(r)
	}
}

func BenchmarkRecordNoMatch(b *testing.B) {
	r := search.Result{Path: "empty.go"}
	b.ResetTimer()
	for range b.N {
		var st Stats
		st.RecordMatch(r)
	}
}

func BenchmarkRecordMany(b *testing.B) {
	results := make([]search.Result, 100)
	for i := range results {
		results[i] = search.Result{
			Path:    "file.go",
			Matches: []search.Match{{Line: i + 1}},
		}
	}
	b.ResetTimer()
	for range b.N {
		var st Stats
		for _, r := range results {
			st.RecordMatch(r)
		}
	}
}
