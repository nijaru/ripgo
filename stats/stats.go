// Package stats provides search statistics tracking.
package stats

import (
	"sync/atomic"

	"github.com/nijaru/ripgo/search"
)

// Stats tracks match and file counts across a search session.
// It is safe for concurrent use.
type Stats struct {
	files   atomic.Int64
	matches atomic.Int64
}

// RecordMatch updates statistics with the results from a single file.
func (s *Stats) RecordMatch(r search.Result) {
	if len(r.Matches) > 0 {
		s.files.Add(1)
		s.matches.Add(int64(len(r.Matches)))
	}
}

// TotalFiles returns the number of files with at least one match.
func (s *Stats) TotalFiles() int64 {
	return s.files.Load()
}

// TotalMatches returns the total number of matches found across all files.
func (s *Stats) TotalMatches() int64 {
	return s.matches.Load()
}
