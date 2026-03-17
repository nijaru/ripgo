package stats

import "github.com/nijaru/ripgo/search"

// Stats tracks match counts across all files.
type Stats struct {
	filesMatched  int
	totalMatches  int
	filesSearched int
}

// RecordMatch records the result of searching a single file.
func (s *Stats) RecordMatch(r search.Result) {
	s.filesSearched++
	if len(r.Matches) > 0 {
		s.filesMatched++
		s.totalMatches += len(r.Matches)
	}
}

// TotalMatches returns the total number of matches found.
func (s *Stats) TotalMatches() int { return s.totalMatches }

// FilesMatched returns the number of files with at least one match.
func (s *Stats) FilesMatched() int { return s.filesMatched }

// FilesSearched returns the total number of files searched.
func (s *Stats) FilesSearched() int { return s.filesSearched }
