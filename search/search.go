package search

import (
	"bufio"
	"bytes"
	"os"

	"github.com/nijaru/ripgo/pattern"
)

// Match represents a single match within a line.
type Match struct {
	// Line is the 1-based line number.
	Line int
	// Column is the 1-based byte offset of the match start.
	Column int
	// LineBytes is the full line content.
	LineBytes []byte
	// Submatches holds [start, end) byte offsets for each submatch.
	Submatches [][2]int
}

// Result holds all matches for a single file.
type Result struct {
	Path    string
	Matches []Match
}

// Config holds search behavior options.
type Config struct {
	// MaxCount limits matches per file. 0 means unlimited.
	MaxCount int
	// Before is the number of context lines before each match.
	Before int
	// After is the number of context lines after each match.
	After int
}

// Searcher scans files for matches against a compiled pattern.
type Searcher struct {
	cfg     Config
	matcher pattern.Matcher
}

// NewSearcher creates a searcher with the given config and matcher.
func NewSearcher(cfg Config, matcher pattern.Matcher) *Searcher {
	return &Searcher{
		cfg:     cfg,
		matcher: matcher,
	}
}

// Search reads a file line-by-line and returns all matches.
func (s *Searcher) Search(path string) (Result, error) {
	result := Result{Path: path}

	file, err := os.Open(path)
	if err != nil {
		return result, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	matchCount := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()

		locs, ok := s.matcher.Match(line)
		if ok {
			match := Match{
				Line:       lineNum,
				Column:     locs[0] + 1,
				LineBytes:  bytes.Clone(line),
				Submatches: [][2]int{{locs[0], locs[1]}},
			}
			result.Matches = append(result.Matches, match)
			matchCount++

			if s.cfg.MaxCount > 0 && matchCount >= s.cfg.MaxCount {
				break
			}
		}
	}

	return result, scanner.Err()
}

// SearchMultiline reads the entire file and matches across line boundaries.
func (s *Searcher) SearchMultiline(path string) (Result, error) {
	result := Result{Path: path}

	data, err := os.ReadFile(path)
	if err != nil {
		return result, err
	}

	if s.matcher.MatchFile(data) {
		lines := bytes.Split(data, []byte("\n"))
		for i, line := range lines {
			if _, ok := s.matcher.Match(line); ok {
				match := Match{
					Line:       i + 1,
					Column:     1,
					LineBytes:  bytes.Clone(line),
					Submatches: [][2]int{{0, len(line)}},
				}
				result.Matches = append(result.Matches, match)
			}
		}
	}

	return result, nil
}
