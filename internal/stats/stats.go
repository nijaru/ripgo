package stats

import (
	"github.com/nijaru/ripgo/internal/config"
	"github.com/nijaru/ripgo/internal/search"
)

type Stats struct {
	cfg           *config.Config
	filesMatched  int
	totalMatches  int
	filesSearched int
}

func NewStats(cfg *config.Config) *Stats {
	return &Stats{
		cfg: cfg,
	}
}

func (s *Stats) RecordMatch(r search.Result) {
	s.filesSearched++
	if len(r.Matches) > 0 {
		s.filesMatched++
		s.totalMatches += len(r.Matches)
	}
}

func (s *Stats) ExitCode() int {
	if s.cfg.Quiet {
		if s.filesMatched > 0 {
			return 0
		}
		return 1
	}

	if s.cfg.FilesWithMatches || s.cfg.OutputMode == config.OutputModeQuiet {
		if s.filesMatched > 0 {
			return 0
		}
		return 1
	}

	if s.totalMatches > 0 {
		return 0
	}
	return 1
}

func (s *Stats) TotalMatches() int {
	return s.totalMatches
}

func (s *Stats) FilesMatched() int {
	return s.filesMatched
}

func (s *Stats) FilesSearched() int {
	return s.filesSearched
}
