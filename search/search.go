package search

import (
	"bytes"
	"io/fs"
	"sync"
	"time"

	"github.com/nijaru/ripgo/internal/aho"
	"github.com/nijaru/ripgo/internal/fsref"
	"github.com/nijaru/ripgo/internal/osfs"
	"github.com/nijaru/ripgo/internal/sys"
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
	// CreatedAt is the file's birth time (creation time).
	CreatedAt time.Time
	// AccessedAt is the file's last access time.
	AccessedAt time.Time
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
	// MmapThreshold is the minimum file size in bytes that triggers mmap
	// instead of a full read. 0 defaults to 128KB.
	MmapThreshold int64
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
	if cfg.MmapThreshold == 0 {
		cfg.MmapThreshold = 128 * 1024
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
// When context lines are requested, a ring buffer retains only the last
// <before> lines, avoiding O(N) memory for large files with few matches.
func (s *Searcher) Search(ref fsref.Ref) (Result, error) {
	path := ref.DisplayPath()
	result := Result{Path: path}

	var data []byte
	var err error
	var mapped bool
	var unmap func() error

	info := ref.Info()
	if info != nil {
		stats := sys.GetStats(info)
		result.ModTime = stats.ModifiedAt
		result.CreatedAt = stats.CreatedAt
		result.AccessedAt = stats.AccessedAt
	}
	if info != nil && info.Size() > s.cfg.MmapThreshold {
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
	needContext := s.cfg.Before > 0 || s.cfg.After > 0

	// Pre-allocate matches slice.
	result.Matches = make([]Match, 0, 16)

	// Ring buffer for context lines — only retains last <before> lines.
	type ringLine struct {
		line     int
		lineData []byte
	}
	beforeCap := min(s.cfg.Before, 32)
	ring := make([]ringLine, beforeCap)
	ringFront := 0 // points to oldest slot
	ringCount := 0

	// Post-match state for emitting after-context.
	afterActive := false
	afterRem := 0
	lastEmitted := 0

	lineNum := 1
	for line := range bytes.Lines(data) {
		line = bytes.TrimSuffix(line, []byte("\n"))

		if needContext && afterActive {
			// Emitting after-context for a previous match.
			afterRem--
			lineToEmit := line
			if mapped {
				lineToEmit = bytes.Clone(line)
			}
			result.Entries = append(result.Entries, Entry{
				Kind:      EntryContext,
				Line:      lineNum,
				LineBytes: lineToEmit,
			})
			lastEmitted = lineNum

			if afterRem <= 0 {
				afterActive = false
				if s.cfg.MaxCount > 0 && matchCount >= s.cfg.MaxCount {
					break
				}
			}
			lineNum++
			continue
		}

		// Check match limit.
		if s.cfg.MaxCount > 0 && matchCount >= s.cfg.MaxCount {
			if !needContext {
				break
			}
			break
		}

		locs, ok := s.matcher.Match(line)
		if !ok {
			// No match — add to before ring buffer.
			if needContext && beforeCap > 0 {
				ring[(ringFront+ringCount)%beforeCap] = ringLine{lineNum, line}
				if ringCount < beforeCap {
					ringCount++
				} else {
					ringFront = (ringFront + 1) % beforeCap
				}
			}
			lineNum++
			continue
		}

		// Match found.
		content := line

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
		matchCount++

		if needContext {
			// Emit before-context from ring buffer.
			for i := 0; i < ringCount; i++ {
				idx := (ringFront + i) % beforeCap
				if ring[idx].line > lastEmitted {
					cl := ring[idx].lineData
					if mapped {
						cl = bytes.Clone(cl)
					}
					result.Entries = append(result.Entries, Entry{
						Kind:      EntryContext,
						Line:      ring[idx].line,
						LineBytes: cl,
					})
					lastEmitted = ring[idx].line
				}
			}

			// Emit match — clone here (once) rather than in the Match struct.
			matchContent := content
			if mapped {
				matchContent = bytes.Clone(content)
			}
			result.Entries = append(result.Entries, Entry{
				Kind:      EntryMatch,
				Line:      lineNum,
				LineBytes: matchContent,
				Column:    match.Column,
			})
			result.Matches[len(result.Matches)-1].LineBytes = matchContent
			lastEmitted = lineNum

			// Start after-context phase.
			if s.cfg.After > 0 {
				afterActive = true
				afterRem = s.cfg.After
			} else if s.cfg.MaxCount > 0 && matchCount >= s.cfg.MaxCount {
				break
			}
		} else {
			// No context — clone match content directly into Match struct.
			if mapped {
				result.Matches[len(result.Matches)-1].LineBytes = bytes.Clone(content)
			}
			if s.cfg.MaxCount > 0 && matchCount >= s.cfg.MaxCount {
				break
			}
		}

		lineNum++
	}

	return result, nil
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
