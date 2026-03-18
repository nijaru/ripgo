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

// EntryKind distinguishes match lines from context lines.
type EntryKind int

const (
	EntryMatch   EntryKind = iota // a line with a match
	EntryContext                  // a context line (before or after a match)
)

// Entry is an ordered output line — either a match or context.
type Entry struct {
	Kind      EntryKind
	Line      int
	LineBytes []byte
	Column    int // 1-based; 0 for context lines
}

// Result holds all matches for a single file.
type Result struct {
	Path    string
	Matches []Match
	Entries []Entry // ordered match + context entries (populated when context requested)
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

	needContext := s.cfg.Before > 0 || s.cfg.After > 0

	// For context mode, read the entire file so all line data lives in a
	// single contiguous buffer. Lines are sub-slices of this buffer,
	// avoiding per-line allocations.
	if needContext {
		data, err := os.ReadFile(path)
		if err != nil {
			return result, err
		}

		lines := bytes.Split(data, []byte("\n"))
		// Trim trailing empty element from files ending with \n
		if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
			lines = lines[:len(lines)-1]
		}
		matchCount := 0
		var matchLines []int

		for i, line := range lines {
			lineNum := i + 1
			locs, ok := s.matcher.Match(line)
			if ok && (s.cfg.MaxCount == 0 || matchCount < s.cfg.MaxCount) {
				match := Match{
					Line:       lineNum,
					Column:     locs[0] + 1,
					LineBytes:  line,
					Submatches: [][2]int{{locs[0], locs[1]}},
				}
				result.Matches = append(result.Matches, match)
				matchLines = append(matchLines, lineNum)
				matchCount++
			}
		}

		if len(matchLines) > 0 {
			result.Entries = s.buildEntries(lines, matchLines, result.Matches)
		}

		return result, nil
	}

	// Non-context: stream line-by-line, break early on MaxCount.
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
		if ok && (s.cfg.MaxCount == 0 || matchCount < s.cfg.MaxCount) {
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

	if err := scanner.Err(); err != nil {
		return result, err
	}

	return result, nil
}

// buildEntries constructs the ordered entry list with context lines.
func (s *Searcher) buildEntries(lines [][]byte, matchLines []int, matches []Match) []Entry {
	before, after := s.cfg.Before, s.cfg.After
	numLines := len(lines)
	mIdx := 0
	var entries []Entry
	lastAfter := 0 // tracks last line added as after-context

	for i := 0; i < numLines && mIdx < len(matchLines); i++ {
		lineNum := i + 1
		mLine := matchLines[mIdx]

		if lineNum == mLine {
			// Add before-context lines
			for j := max(mLine-before, lastAfter+1); j < mLine; j++ {
				if j >= 1 && j <= numLines {
					entries = append(entries, Entry{
						Kind:      EntryContext,
						Line:      j,
						LineBytes: lines[j-1],
					})
				}
			}

			// Add the match line
			col := 0
			for _, m := range matches {
				if m.Line == mLine {
					col = m.Column
					break
				}
			}
			entries = append(entries, Entry{
				Kind:      EntryMatch,
				Line:      mLine,
				LineBytes: lines[mLine-1],
				Column:    col,
			})

			// Add after-context lines
			for j := mLine + 1; j <= min(mLine+after, numLines); j++ {
				entries = append(entries, Entry{
					Kind:      EntryContext,
					Line:      j,
					LineBytes: lines[j-1],
				})
			}
			lastAfter = mLine + after
			mIdx++
			i = mLine + after - 1 // loop i++ advances to mLine + after
			continue
		}
	}

	return entries
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
		if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
			lines = lines[:len(lines)-1]
		}
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
