package search

import (
	"bytes"
	"io/fs"
	"sync"
	"time"

	"github.com/nijaru/ripgo/internal/aho"
	"github.com/nijaru/ripgo/internal/fsref"
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
	// ModTime is the file's last modification time.
	ModTime time.Time
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

// Release returns pooled resources to the searcher.
func (r *Result) Release() {
	for i := range r.Matches {
		if r.Matches[i].Submatches != nil {
			submatchPool.Put(r.Matches[i].Submatches[:0])
			r.Matches[i].Submatches = nil
		}
	}
}

// MappableFS is an optional interface for filesystems that support mmap.
type MappableFS interface {
	fs.FS
	Mmap(name string) ([]byte, func() error, error)
}

var submatchPool = sync.Pool{
	New: func() any {
		return make([][2]int, 0, 16)
	},
}

// Searcher scans files for matches against a compiled pattern.
type Searcher struct {
	fsys      fs.FS
	cfg       Config
	matcher   pattern.Matcher
	prefilter *aho.Machine
}

// NewSearcher creates a searcher with the given config and matcher.
// If fsys is nil, it defaults to the local OS filesystem.
func NewSearcher(fsys fs.FS, cfg Config, matcher pattern.Matcher) *Searcher {
	if fsys == nil {
		fsys = osfs.New()
	}

	// Build Aho-Corasick pre-filter from extracted literals.
	var prefilter *aho.Machine
	if lits := matcher.Literals(); len(lits) >= 2 {
		prefilter = aho.New(lits)
	}

	return &Searcher{
		fsys:      fsys,
		cfg:       cfg,
		matcher:   matcher,
		prefilter: prefilter,
	}
}

// SearchPath is a compatibility shim that creates a pathRef and calls Search.
func (s *Searcher) SearchPath(path string, info fs.FileInfo) (Result, error) {
	ref := fsref.NewPathRef(path, info, s.fsys)
	return s.Search(ref)
}

// Search reads a file via the provided Ref and returns all matches.
func (s *Searcher) Search(ref fsref.Ref) (Result, error) {
	path := ref.DisplayPath()
	result := Result{Path: path}

	var data []byte
	var err error
	var mapped bool
	var unmap func() error

	info := ref.Info()
	if info != nil {
		result.ModTime = info.ModTime()
	}
	// Lower threshold to 128KB to reduce syscall overhead for medium files.
	if info != nil && info.Size() > 128*1024 {
		data, unmap, err = ref.Mmap()
		if err == nil {
			mapped = true
			defer unmap()
		}
	}

	if !mapped {
		data, err = ref.ReadFile()
		if err != nil {
			return result, err
		}
	}

	// Pre-filter: skip files that can't possibly match.
	// Use Aho-Corasick for multiple literals, bytes.Contains for single.
	if s.prefilter != nil {
		if !s.prefilter.MatchesAny(data) {
			return result, nil
		}
	} else if lit := s.matcher.Literal(); len(lit) > 0 {
		if !bytes.Contains(data, lit) {
			return result, nil
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
	for line := range bytes.Lines(data) {
		line = bytes.TrimSuffix(line, []byte("\n"))

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

				submatches := submatchPool.Get().([][2]int)
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
		lineNum := 1
		for line := range bytes.Lines(data) {
			line = bytes.TrimSuffix(line, []byte("\n"))
			if _, ok := s.matcher.Match(line); ok {
				match := Match{
					Line:       lineNum,
					Column:     1,
					LineBytes:  bytes.Clone(line),
					Submatches: [][2]int{{0, len(line)}},
				}
				result.Matches = append(result.Matches, match)
			}
			lineNum++
		}
	}

	return result, nil
}
