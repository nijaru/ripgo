package search

import (
	"bytes"
	"io/fs"

	"github.com/nijaru/ripgo/internal/osfs"
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
	// EntryMatch is a line containing a match.
	EntryMatch EntryKind = iota
	// EntryContext is a context line (before or after a match).
	EntryContext
)

// Entry is an ordered output line — either a match or context.
type Entry struct {
	// Kind is either EntryMatch or EntryContext.
	Kind EntryKind
	// Line is the 1-based line number.
	Line int
	// LineBytes is the full line content.
	LineBytes []byte
	// Column is the 1-based byte offset of the match start.
	// 0 for context lines.
	Column int
}

// Result holds all matches for a single file.
type Result struct {
	// Path is the file path.
	Path string
	// Matches is the list of matches found in the file.
	Matches []Match
	// Entries is the ordered list of match and context entries.
	// Only populated if context lines are requested.
	Entries []Entry
	// Binary is true if the file was detected as binary.
	Binary bool
	// Error is any error encountered while searching this file.
	Error error
}

// Config holds search behavior options.
type Config struct {
	// MaxCount limits matches per file. 0 means unlimited.
	MaxCount int
	// Before is the number of context lines before each match.
	Before int
	// After is the number of context lines after each match.
	After int
	// SearchBinary includes binary files in results.
	SearchBinary bool
	// OnlyBinary restricts results to binary files only.
	OnlyBinary bool
}

// MappableFS is an optional interface for filesystems that support mmap.
type MappableFS interface {
	fs.FS
	Mmap(name string) ([]byte, func() error, error)
}

// Searcher scans files for matches against a compiled pattern.
type Searcher struct {
	fsys    fs.FS
	cfg     Config
	matcher pattern.Matcher
}

// NewSearcher creates a searcher with the given config and matcher.
// If fsys is nil, it defaults to the local OS filesystem.
func NewSearcher(fsys fs.FS, cfg Config, matcher pattern.Matcher) *Searcher {
	if fsys == nil {
		fsys = osfs.New()
	}
	return &Searcher{
		fsys:    fsys,
		cfg:     cfg,
		matcher: matcher,
	}
}

// Search reads a file and returns all matches.
func (s *Searcher) Search(path string) (Result, error) {
	result := Result{Path: path}

	var data []byte
	var err error
	var mapped bool
	var unmap func() error

	// Try memory mapping if supported
	if mfs, ok := s.fsys.(MappableFS); ok {
		info, err := fs.Stat(s.fsys, path)
		if err == nil && info.Size() > 1024*1024 {
			data, unmap, err = mfs.Mmap(path)
			if err == nil {
				mapped = true
				defer unmap()
			}
		}
	}

	if !mapped {
		if rfs, ok := s.fsys.(fs.ReadFileFS); ok {
			data, err = rfs.ReadFile(path)
		} else {
			data, err = fs.ReadFile(s.fsys, path)
		}
		if err != nil {
			return result, err
		}
	}

	result.Binary = bytes.Contains(data[:min(len(data), 8192)], []byte{0})
	if !s.cfg.SearchBinary && !s.cfg.OnlyBinary && result.Binary {
		return result, nil
	}
	if s.cfg.OnlyBinary && !result.Binary {
		return result, nil
	}

	matchCount := 0
	var matchLines []int
	var lines [][]byte
	needContext := s.cfg.Before > 0 || s.cfg.After > 0

	// Pre-allocate matches slice.
	result.Matches = make([]Match, 0, 16)

	lineNum := 1
	remaining := data
	var line []byte

	for {
		idx := bytes.IndexByte(remaining, '\n')
		if idx == -1 {
			if len(remaining) > 0 {
				line = remaining
			} else {
				break
			}
		} else {
			line = remaining[:idx]
		}

		if needContext {
			lines = append(lines, line)
		}

		if s.cfg.MaxCount > 0 && matchCount >= s.cfg.MaxCount {
			if !needContext {
				break
			}
			lastMatchLine := matchLines[len(matchLines)-1]
			if lineNum >= lastMatchLine+s.cfg.After {
				break
			}
		} else {
			locs, ok := s.matcher.Match(line)
			if ok {
				content := line
				if mapped {
					content = bytes.Clone(line)
				}

				submatches := make([][2]int, 0, len(locs)/2)
				for i := 0; i < len(locs); i += 2 {
					if locs[i] >= 0 {
						submatches = append(submatches, [2]int{locs[i], locs[i+1]})
					} else {
						submatches = append(submatches, [2]int{-1, -1})
					}
				}

				match := Match{
					Line:       lineNum,
					Column:     locs[0] + 1,
					LineBytes:  content,
					Submatches: submatches,
				}
				result.Matches = append(result.Matches, match)
				matchLines = append(matchLines, lineNum)
				matchCount++

				if s.cfg.MaxCount > 0 && matchCount >= s.cfg.MaxCount {
					if !needContext || s.cfg.After == 0 {
						break
					}
				}
			}
		}

		if idx == -1 {
			break
		}
		remaining = remaining[idx+1:]
		lineNum++
	}

	if needContext && len(matchLines) > 0 {
		result.Entries = s.buildEntries(lines, matchLines, result.Matches, mapped)
	}

	return result, nil
}

// buildEntries constructs the ordered entry list with context lines.
func (s *Searcher) buildEntries(lines [][]byte, matchLines []int, matches []Match, mapped bool) []Entry {
	before, after := s.cfg.Before, s.cfg.After
	numLines := len(lines)
	mIdx := 0
	var entries []Entry
	lastAfter := 0

	for i := 0; i < numLines && mIdx < len(matchLines); i++ {
		lineNum := i + 1
		mLine := matchLines[mIdx]

		if lineNum == mLine {
			for j := max(mLine-before, lastAfter+1); j < mLine; j++ {
				if j >= 1 && j <= numLines {
					content := lines[j-1]
					if mapped {
						content = bytes.Clone(content)
					}
					entries = append(entries, Entry{
						Kind:      EntryContext,
						Line:      j,
						LineBytes: content,
					})
				}
			}

			var content []byte
			col := 0
			for _, m := range matches {
				if m.Line == mLine {
					col = m.Column
					content = m.LineBytes
					break
				}
			}
			entries = append(entries, Entry{
				Kind:      EntryMatch,
				Line:      mLine,
				LineBytes: content,
				Column:    col,
			})

			for j := mLine + 1; j <= min(mLine+after, numLines); j++ {
				content := lines[j-1]
				if mapped {
					content = bytes.Clone(content)
					}
				entries = append(entries, Entry{
					Kind:      EntryContext,
					Line:      j,
					LineBytes: content,
				})
			}
			lastAfter = mLine + after
			mIdx++
			i = mLine + after - 1
			continue
		}
	}

	return entries
}

// SearchMultiline reads the entire file and matches across line boundaries.
func (s *Searcher) SearchMultiline(path string) (Result, error) {
	result := Result{Path: path}

	var data []byte
	var err error
	if rfs, ok := s.fsys.(fs.ReadFileFS); ok {
		data, err = rfs.ReadFile(path)
	} else {
		data, err = fs.ReadFile(s.fsys, path)
	}
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
