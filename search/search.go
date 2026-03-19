package search

import (
	"bytes"
	"io/fs"
	"os"
	"syscall"

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

// Searcher scans files for matches against a compiled pattern.
type Searcher struct {
	fs      fs.FS
	cfg     Config
	matcher pattern.Matcher
}

// NewSearcher creates a searcher with the given config and matcher.
// If fsys is nil, it defaults to the local OS filesystem.
func NewSearcher(fsys fs.FS, cfg Config, matcher pattern.Matcher) *Searcher {
	return &Searcher{
		fs:      fsys,
		cfg:     cfg,
		matcher: matcher,
	}
}

// Search reads a file and returns all matches.
// It uses ReadFile + bytes.Split to ensure all line data lives in a single
// contiguous buffer. Lines are sub-slices of this buffer, avoiding per-line allocations.
// For large files (>1MB) without a custom VFS, it uses syscall.Mmap.
func (s *Searcher) Search(path string) (Result, error) {
	result := Result{Path: path}

	var data []byte
	var err error
	var mapped bool

	if s.fs == nil {
		f, err := os.Open(path)
		if err == nil {
			defer f.Close()
			info, err := f.Stat()
			// Mmap files > 1MB. Must be large enough to justify syscall overhead.
			if err == nil && info.Size() > 1024*1024 {
				data, err = syscall.Mmap(int(f.Fd()), 0, int(info.Size()), syscall.PROT_READ, syscall.MAP_SHARED)
				if err == nil {
					mapped = true
					defer syscall.Munmap(data)
				}
			}
		}
	}

	if !mapped {
		if s.fs != nil {
			data, err = fs.ReadFile(s.fs, path)
		} else {
			data, err = os.ReadFile(path)
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

	        // If we already hit max count, we are just gathering 'after' context lines.
	        // We don't need to match anymore.
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
	                        match := Match{
	                                Line:       lineNum,
	                                Column:     locs[0] + 1,
	                                LineBytes:  content,
	                                Submatches: [][2]int{{locs[0], locs[1]}},
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
	lastAfter := 0 // tracks last line added as after-context

	for i := 0; i < numLines && mIdx < len(matchLines); i++ {
		lineNum := i + 1
		mLine := matchLines[mIdx]

		if lineNum == mLine {
			// Add before-context lines
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

			// Add the match line
			var content []byte
			col := 0
			for _, m := range matches {
				if m.Line == mLine {
					col = m.Column
					content = m.LineBytes // already cloned if mapped
					break
				}
			}
			entries = append(entries, Entry{
				Kind:      EntryMatch,
				Line:      mLine,
				LineBytes: content,
				Column:    col,
			})

			// Add after-context lines
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
