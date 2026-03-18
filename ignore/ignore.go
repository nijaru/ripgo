// Package ignore implements gitignore-compatible file filtering.
//
// It supports the full gitignore rule set: negation (!), directory-only (/),
// anchored (/prefix), recursive globs (**), and negation re-inclusion.
// Rules cascade via a parent chain — child rules take precedence.
package ignore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gobwas/glob"
)

// Config holds ignore engine options.
type Config struct {
	// GlobIncludes is the list of glob patterns to always include.
	GlobIncludes []string
	// GlobExcludes is the list of glob patterns to always exclude.
	GlobExcludes []string
	// NoIgnore disables .gitignore/.ignore file loading.
	NoIgnore bool
	// Hidden includes hidden files and directories.
	Hidden bool
}

// IgnoreRule is a single parsed gitignore rule.
type IgnoreRule struct {
	Pattern       string
	Negated       bool
	DirectoryOnly bool
	Anchored      bool
	Source        string // source file path
}

// Match reports whether relPath matches this rule's pattern.
// Returns raw pattern match; negation is handled by the caller.
func (r *IgnoreRule) Match(relPath string, isDir bool) bool {
	if r.DirectoryOnly && !isDir {
		return false
	}
	return matchPattern(r.Pattern, r.Anchored, relPath, isDir)
}

// matchPattern handles the core glob matching for a single pattern.
func matchPattern(pattern string, anchored bool, relPath string, isDir bool) bool {
	try := func(path string) bool {
		ok := matchGlob(pattern, path)
		if !ok && isDir {
			ok = matchGlob(pattern, path+"/")
		}
		return ok
	}

	if anchored || strings.ContainsRune(pattern, '/') {
		return try(relPath)
	}

	// Non-anchored, no slash: match against full path and basename
	if try(relPath) {
		return true
	}

	// Try basename — patterns like *.txt, build match at any depth
	if idx := strings.LastIndexByte(relPath, '/'); idx >= 0 {
		if try(relPath[idx+1:]) {
			return true
		}
	}

	return false
}

// matchGlob matches a path against a gitignore-style glob pattern.
// Supports * (within segment), ** (recursive), ? (single char), and literals.
// Uses forward-slash path separators.
func matchGlob(pattern, path string) bool {
	if strings.Contains(pattern, "**") {
		return matchGlobStar(pattern, path)
	}

	// Simple patterns: use filepath.Match (handles *, ?, [abc], literals)
	ok, err := filepath.Match(pattern, path)
	if err != nil {
		return false
	}
	return ok
}

// matchGlobStar handles patterns containing **.
// ** matches zero or more path components (including /).
func matchGlobStar(pattern, path string) bool {
	// Split on ** without pre-allocating: extract segments lazily.
	// Pattern "a/**/b" has before="", after="b" between ** markers.
	pathParts := strings.Split(path, "/")
	return matchGlobStarPartsOpt(pattern, pathParts, 0)
}

// matchGlobStarPartsOpt matches a glob pattern containing ** against path segments.
// It extracts pattern segments between ** markers on demand instead of pre-splitting.
func matchGlobStarPartsOpt(pattern string, pathParts []string, si int) bool {
	if pattern == "" {
		return si == len(pathParts)
	}

	// Find the next ** marker
	starIdx := strings.Index(pattern, "**")
	if starIdx < 0 {
		// No more ** — remaining pattern must match remaining path as a single unit
		remaining := strings.Join(pathParts[si:], "/")
		ok, _ := filepath.Match(pattern, remaining)
		return ok
	}

	// Extract the segment before ** (what must match before the wildcard)
	before := pattern[:starIdx]
	// Remainder after ** (strip leading slash if present)
	after := pattern[starIdx+2:]
	if len(after) > 0 && after[0] == '/' {
		after = after[1:]
	}

	if before == "" {
		// Pattern starts with ** — try matching zero or more components
		if after == "" {
			return true // trailing ** matches everything
		}
		// Try consuming 0, 1, 2, ... path components
		for i := si; i <= len(pathParts); i++ {
			if matchGlobStarPartsOpt(after, pathParts, i) {
				return true
			}
		}
		return false
	}

	// 'before' must match starting from si.
	// If before contains a slash, join path components and match.
	// Otherwise, match a single path component.
	if strings.Contains(before, "/") {
		for end := si + 1; end <= len(pathParts); end++ {
			candidate := strings.Join(pathParts[si:end], "/")
			ok, _ := filepath.Match(before, candidate)
			if ok {
				if after == "" {
					return end == len(pathParts)
				}
				if matchGlobStarPartsOpt(after, pathParts, end) {
					return true
				}
			}
		}
		return false
	}

	// Single-segment: before must match exactly one path component
	if si >= len(pathParts) {
		return false
	}
	ok, _ := filepath.Match(before, pathParts[si])
	if !ok {
		return false
	}

	if after == "" {
		return si+1 == len(pathParts)
	}

	return matchGlobStarPartsOpt(after, pathParts, si+1)
}

// IgnoreSet holds all ignore rules for one ignore file (.gitignore/.ignore).
// Rules are ordered top-to-bottom; the last match wins.
type IgnoreSet struct {
	Dir    string // directory this ignore file lives in
	Rules  []IgnoreRule
	Parent *IgnoreSet // parent directory's ignore set (nil at root)
}

// IsIgnored checks relPath against this set and all ancestors.
// Returns true if the path should be ignored.
//
// Within each set, last matching rule wins.
// Across the chain, the first set with a matching rule determines the result
// (child set takes precedence over parent).
func (s *IgnoreSet) IsIgnored(relPath string, isDir bool) bool {
	// Step 1: check if any ancestor directory is ignored by parent rules.
	if s.Parent != nil {
		for i := 0; i < len(relPath); i++ {
			if relPath[i] == '/' {
				if s.Parent.isIgnoredBySelf(relPath[:i], true) {
					return true
				}
			}
		}
	}

	// Step 2: walk chain child → parent. First set with a matching rule wins.
	for cur := s; cur != nil; cur = cur.Parent {
		if ignored, ok := cur.matchRules(relPath, isDir); ok {
			return ignored
		}
	}
	return false
}

// isIgnoredBySelf checks only this set's rules (walking up parent chain).
// Returns on first matching rule in the chain.
func (s *IgnoreSet) isIgnoredBySelf(relPath string, isDir bool) bool {
	for cur := s; cur != nil; cur = cur.Parent {
		if ignored, ok := cur.matchRules(relPath, isDir); ok {
			return ignored
		}
	}
	return false
}

// matchRules evaluates rules in order, returning (ignored, matched).
// Last matching rule wins.
func (s *IgnoreSet) matchRules(relPath string, isDir bool) (ignored bool, matched bool) {
	for i := range s.Rules {
		if s.Rules[i].Match(relPath, isDir) {
			ignored = !s.Rules[i].Negated
			matched = true
		}
	}
	return
}

// Engine manages ignore rules and glob filters for file traversal.
type Engine struct {
	cfg          Config
	includes     []glob.Glob
	excludes     []glob.Glob
	sets         map[string]*IgnoreSet
	mu           sync.Mutex
	baseRelSlash string // path from cwd, forward-slash form (cached for ShouldIgnore)
}

// NewEngine creates an ignore engine from the given config.
func NewEngine(cfg Config) (*Engine, error) {
	engine := &Engine{
		cfg:      cfg,
		sets:     make(map[string]*IgnoreSet),
		includes: make([]glob.Glob, 0, len(cfg.GlobIncludes)),
		excludes: make([]glob.Glob, 0, len(cfg.GlobExcludes)),
	}

	// Pre-compute the cwd-relative path to avoid per-file filepath.Rel calls.
	if cwd, err := os.Getwd(); err == nil {
		rel, err := filepath.Rel(".", cwd)
		if err == nil {
			engine.baseRelSlash = filepath.ToSlash(rel)
		}
	}
	if engine.baseRelSlash == "." {
		engine.baseRelSlash = ""
	}

	for _, p := range cfg.GlobIncludes {
		g, err := glob.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("compile include glob %q: %w", p, err)
		}
		engine.includes = append(engine.includes, g)
	}

	for _, p := range cfg.GlobExcludes {
		g, err := glob.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("compile exclude glob %q: %w", p, err)
		}
		engine.excludes = append(engine.excludes, g)
	}

	return engine, nil
}

// parseIgnoreRule parses a single non-empty, non-comment line from an ignore file.
func parseIgnoreRule(line, source string) (IgnoreRule, bool) {
	negated := false
	directoryOnly := false
	anchored := false

	if strings.HasPrefix(line, "!") {
		negated = true
		line = line[1:]
	}

	if strings.HasPrefix(line, "/") {
		anchored = true
		line = line[1:]
	}

	if strings.HasSuffix(line, "/") {
		directoryOnly = true
		line = strings.TrimRight(line, "/")
	}

	line = strings.TrimSpace(line)
	if line == "" {
		return IgnoreRule{}, false
	}

	return IgnoreRule{
		Pattern:       line,
		Negated:       negated,
		DirectoryOnly: directoryOnly,
		Anchored:      anchored,
		Source:        source,
	}, true
}

// parseIgnoreLines parses all rules from ignore file content.
func parseIgnoreLines(content, source string) []IgnoreRule {
	var rules []IgnoreRule
	for line := range strings.SplitSeq(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if rule, ok := parseIgnoreRule(line, source); ok {
			rules = append(rules, rule)
		}
	}
	return rules
}

// LoadIgnoreFile reads .gitignore and .ignore files from the given directory.
// It links the resulting IgnoreSet to the parent directory's chain.
func (e *Engine) LoadIgnoreFile(dir string) error {
	if e.cfg.NoIgnore {
		return nil
	}

	dir = filepath.Clean(dir)

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.sets[dir]; ok {
		return nil
	}

	var rules []IgnoreRule
	for _, name := range []string{".gitignore", ".ignore"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		source := filepath.Join(dir, name)
		rules = append(rules, parseIgnoreLines(string(data), source)...)
	}

	set := &IgnoreSet{
		Dir:   dir,
		Rules: rules,
	}

	// Link to parent chain
	if parentDir := filepath.Dir(dir); parentDir != dir {
		if parentSet, ok := e.sets[parentDir]; ok {
			set.Parent = parentSet
		}
	}

	e.sets[dir] = set
	return nil
}

// ShouldIgnore returns true if the path should be excluded from traversal.
func (e *Engine) ShouldIgnore(path string, isDir bool) bool {
	clean := filepath.Clean(path)
	relSlash := filepath.ToSlash(clean)
	if e.baseRelSlash != "" {
		relSlash = relSlash[len(e.baseRelSlash)+1:]
	}

	// CLI globs always take precedence
	for _, g := range e.excludes {
		if g.Match(relSlash) {
			return true
		}
	}

	if len(e.includes) > 0 {
		matched := false
		for _, g := range e.includes {
			if g.Match(relSlash) {
				matched = true
				break
			}
		}
		if !matched {
			return true
		}
	}

	// Hidden file check — scan for path components starting with '.'
	if !e.cfg.Hidden {
		for i := 0; i < len(relSlash); i++ {
			if relSlash[i] == '.' && (i == 0 || relSlash[i-1] == '/') {
				return true
			}
		}
	}

	// Check ignore file rules via chain
	dir := filepath.Dir(clean)

	e.mu.Lock()
	set, ok := e.sets[dir]
	e.mu.Unlock()

	if !ok {
		return false
	}

	// If the directory itself is ignored by parent rules, everything inside is ignored
	if set.Parent != nil {
		dirRelSlash := relSlash
		if idx := strings.LastIndexByte(relSlash, '/'); idx >= 0 {
			dirRelSlash = relSlash[:idx]
		}
		if set.Parent.isIgnoredBySelf(dirRelSlash, true) {
			return true
		}
	}

	// Compute path relative to this ignore set's directory
	var setRelSlash string
	if set.Dir == "." || set.Dir == clean {
		setRelSlash = relSlash
	} else if len(clean) > len(set.Dir)+1 && clean[len(set.Dir)] == '/' && clean[:len(set.Dir)] == set.Dir {
		setRelSlash = clean[len(set.Dir)+1:]
	} else {
		setRel, err := filepath.Rel(set.Dir, clean)
		if err != nil {
			return false
		}
		setRelSlash = filepath.ToSlash(setRel)
	}

	return set.IsIgnored(setRelSlash, isDir)
}

// GetIgnoreRules returns the loaded ignore rules for a directory.
func (e *Engine) GetIgnoreRules(dir string) []IgnoreRule {
	e.mu.Lock()
	defer e.mu.Unlock()
	if set, ok := e.sets[filepath.Clean(dir)]; ok {
		return set.Rules
	}
	return nil
}
