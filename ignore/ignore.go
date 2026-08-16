// Package ignore implements gitignore-compatible file filtering.
//
// It supports the full gitignore rule set: negation (!), directory-only (/),
// anchored (/prefix), recursive globs (**), and negation re-inclusion.
// Rules cascade via a parent chain — child rules take precedence.
package ignore

import (
	"fmt"
	"io/fs"
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
	// Types is the list of file types to include (e.g., "go").
	Types []string
	// TypesNot is the list of file types to exclude.
	TypesNot []string
	// NoIgnore disables .gitignore/.ignore file loading.
	NoIgnore bool
	// Hidden includes hidden files and directories.
	Hidden bool
}

// FileTypes maps type names to glob patterns.
var FileTypes = map[string][]string{
	"go":      {"*.go"},
	"rust":    {"*.rs"},
	"python":  {"*.py", "*.pyi"},
	"js":      {"*.js", "*.jsx", "*.mjs", "*.cjs"},
	"ts":      {"*.ts", "*.tsx", "*.mts", "*.cts"},
	"c":       {"*.c", "*.h"},
	"cpp":     {"*.cpp", "*.hpp", "*.cc", "*.hh", "*.cxx", "*.hxx"},
	"zig":     {"*.zig", "*.zon"},
	"java":    {"*.java"},
	"kotlin":  {"*.kt", "*.kts"},
	"scala":   {"*.scala", "*.sc"},
	"swift":   {"*.swift"},
	"csharp":  {"*.cs"},
	"ruby":    {"*.rb", "*.rake", "Rakefile", "Gemfile"},
	"php":     {"*.php", "*.phtml"},
	"lua":     {"*.lua"},
	"html":    {"*.html", "*.htm"},
	"css":     {"*.css", "*.scss", "*.sass", "*.less"},
	"md":      {"*.md", "*.markdown"},
	"json":    {"*.json", "*.jsonc", "*.json5"},
	"yaml":    {"*.yaml", "*.yml"},
	"toml":    {"*.toml"},
	"sql":     {"*.sql"},
	"sh":      {"*.sh", "*.bash", "*.zsh"},
	"fish":    {"*.fish"},
	"make":    {"Makefile", "*.mk", "GNUmakefile"},
	"docker":  {"Dockerfile", "*.dockerfile"},
	"proto":   {"*.proto"},
	"graphql": {"*.graphql", "*.gql"},
}

// IgnoreRule is a single parsed gitignore rule.
type IgnoreRule struct {
	// Pattern is the glob pattern to match.
	Pattern string
	// Negated is true if the pattern starts with !.
	Negated bool
	// DirectoryOnly is true if the pattern ends with /.
	DirectoryOnly bool
	// Anchored is true if the pattern starts with /.
	Anchored bool
	// Source is the path to the file that defined this rule.
	Source string
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
	// Recursive matcher using indices to avoid allocations
	return matchGlobStarAt(pattern, path, 0, 0)
}

func matchGlobStarAt(pattern, path string, pi, si int) bool {
	if pi == len(pattern) {
		return si == len(path)
	}

	// Find the next ** marker
	starIdx := strings.Index(pattern[pi:], "**")
	if starIdx < 0 {
		// No more ** — remaining pattern must match remaining path
		ok, _ := filepath.Match(pattern[pi:], path[si:])
		return ok
	}
	starIdx += pi

	// Segment before **
	before := pattern[pi:starIdx]
	// Remainder after ** (strip leading slash)
	after := pattern[starIdx+2:]
	if len(after) > 0 && after[0] == '/' {
		after = after[1:]
	}

	if before == "" {
		// Pattern starts with ** (e.g., "**/*.log")
		if after == "" {
			return true // trailing ** matches everything
		}

		// Try matching 'after' at every possible path segment boundary
		if matchGlobStarAt(after, path, 0, si) {
			return true
		}
		for i := si; i < len(path); i++ {
			if path[i] == '/' {
				if matchGlobStarAt(after, path, 0, i+1) {
					return true
				}
			}
		}
		return false
	}

	// 'before' must match a prefix of the current path segment(s)
	if strings.ContainsRune(before, '/') {
		// before has a slash, it must match one or more segments exactly
		// count slashes in before to know how many segments to take from path
		slashes := strings.Count(before, "/")
		curr := si
		for range slashes {
			idx := strings.IndexByte(path[curr:], '/')
			if idx < 0 {
				return false
			}
			curr += idx + 1
		}
		// now find the end of this segment
		end := strings.IndexByte(path[curr:], '/')
		if end < 0 {
			end = len(path)
		} else {
			end += curr
		}

		ok, _ := filepath.Match(before, path[si:end])
		if !ok {
			return false
		}
		if after == "" {
			return end == len(path)
		}
		// after is handled at next segment
		nextSi := end
		if nextSi < len(path) && path[nextSi] == '/' {
			nextSi++
		}
		return matchGlobStarAt(after, path, 0, nextSi)
	}

	// Single segment 'before'
	end := strings.IndexByte(path[si:], '/')
	if end < 0 {
		end = len(path)
	} else {
		end += si
	}

	ok, _ := filepath.Match(before, path[si:end])
	if !ok {
		return false
	}

	if after == "" {
		return end == len(path)
	}
	nextSi := end
	if nextSi < len(path) && path[nextSi] == '/' {
		nextSi++
	}
	return matchGlobStarAt(after, path, 0, nextSi)
}

// IgnoreSet holds all ignore rules for one ignore file (.gitignore/.ignore).
// Rules are ordered top-to-bottom; the last match wins.
type IgnoreSet struct {
	// Dir is the directory this ignore file lives in.
	Dir string
	// DirBase is the base name of the directory (cached for performance).
	DirBase string
	// Rules is the list of parsed rules from the ignore file.
	Rules []IgnoreRule
	// Parent is the parent directory's ignore set.
	Parent *IgnoreSet
}

// IsIgnored checks relPath against this set and all ancestors.
// Returns true if the path should be ignored.
//
// Within each set, last matching rule wins.
// Across the chain, the first set with a matching rule determines the result
// (child set takes precedence over parent).
func (s *IgnoreSet) IsIgnored(relPath string, isDir bool) bool {
	// Step 1: walk chain child → parent. First set with a matching rule wins.
	currentRel := relPath
	for cur := s; cur != nil; cur = cur.Parent {
		if ignored, ok := cur.matchRules(currentRel, isDir); ok {
			return ignored
		}
		// If we have a parent, we must prepend our directory name to the relative path
		// for the parent's rules to match correctly.
		if cur.Parent != nil {
			if currentRel == "" || currentRel == "." {
				currentRel = cur.DirBase
			} else {
				currentRel = cur.DirBase + "/" + currentRel
			}
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

// Node is a trie node for directory-based ignore set lookups.
type Node struct {
	Children sync.Map // map[string]*Node
	Set      *IgnoreSet
}

func newNode() *Node {
	return &Node{}
}

// Engine manages ignore rules and glob filters for file traversal.
type Engine struct {
	fs           fs.FS
	cfg          Config
	includes     []glob.Glob
	excludes     []glob.Glob
	typeIncludes []glob.Glob
	typeExcludes []glob.Glob
	root         *Node
	mu           sync.RWMutex
	baseRelSlash string // path from cwd, forward-slash form (cached for ShouldIgnore)
}

// NewEngine creates an ignore engine from the given config.
func NewEngine(cfg Config) (*Engine, error) {
	return NewEngineFS(nil, cfg)
}

// NewEngineFS creates an ignore engine from the given config and filesystem.
func NewEngineFS(fsys fs.FS, cfg Config) (*Engine, error) {
	engine := &Engine{
		fs:       fsys,
		cfg:      cfg,
		root:     newNode(),
		includes: make([]glob.Glob, 0, len(cfg.GlobIncludes)),
		excludes: make([]glob.Glob, 0, len(cfg.GlobExcludes)),
	}

	// Pre-compute the cwd-relative path to avoid per-file filepath.Rel calls.
	if fsys == nil {
		if cwd, err := os.Getwd(); err == nil {
			engine.baseRelSlash = filepath.ToSlash(cwd)
		}
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

	// Compile type filters
	for _, t := range cfg.Types {
		patterns, ok := FileTypes[t]
		if !ok {
			return nil, fmt.Errorf("unknown file type %q", t)
		}
		for _, p := range patterns {
			g, err := glob.Compile(p)
			if err != nil {
				return nil, fmt.Errorf("compile type glob %q: %w", p, err)
			}
			engine.typeIncludes = append(engine.typeIncludes, g)
		}
	}

	for _, t := range cfg.TypesNot {
		patterns, ok := FileTypes[t]
		if !ok {
			return nil, fmt.Errorf("unknown file type %q", t)
		}
		for _, p := range patterns {
			g, err := glob.Compile(p)
			if err != nil {
				return nil, fmt.Errorf("compile type glob %q: %w", p, err)
			}
			engine.typeExcludes = append(engine.typeExcludes, g)
		}
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

// IgnoreContext holds the state for ignore lookups in a specific directory.
// It can be passed to ShouldIgnore to avoid redundant trie traversals.
type IgnoreContext struct {
	set *IgnoreSet
}

// LoadIgnoreFile reads .gitignore and .ignore files from the given directory.
// It returns an IgnoreContext that can be used for subsequent ShouldIgnore calls in this directory.
func (e *Engine) LoadIgnoreFile(dir string) (IgnoreContext, error) {
	if e.cfg.NoIgnore {
		return IgnoreContext{}, nil
	}

	dir = filepath.ToSlash(filepath.Clean(dir))

	existing := e.lookup(dir)
	if existing != nil && existing.Dir == dir {
		return IgnoreContext{set: existing}, nil
	}

	var rules []IgnoreRule
	for _, name := range []string{".gitignore", ".ignore", ".rgignore", ".ripgoignore"} {
		var data []byte
		var err error
		if e.fs != nil {
			data, err = fs.ReadFile(e.fs, filepath.Join(dir, name))
		} else {
			data, err = os.ReadFile(filepath.Join(dir, name))
		}
		if err != nil {
			continue
		}
		source := filepath.Join(dir, name)
		rules = append(rules, parseIgnoreLines(string(data), source)...)
	}

	set := &IgnoreSet{
		Dir:     dir,
		DirBase: filepath.Base(dir),
		Rules:   rules,
	}

	// Link to parent chain
	parentSet := e.lookup(filepath.Dir(dir))
	if parentSet != nil {
		set.Parent = parentSet
	}

	e.insert(dir, set)
	return IgnoreContext{set: set}, nil
}

func (e *Engine) insert(path string, set *IgnoreSet) {
	if path == "." || path == "" {
		e.root.Set = set
		return
	}

	parts := strings.Split(path, "/")
	node := e.root
	for _, part := range parts {
		next, _ := node.Children.LoadOrStore(part, newNode())
		node = next.(*Node)
	}
	node.Set = set
}

func (e *Engine) lookup(path string) *IgnoreSet {
	if path == "." || path == "" {
		return e.root.Set
	}

	parts := strings.Split(path, "/")
	node := e.root
	var bestSet *IgnoreSet
	if node.Set != nil {
		bestSet = node.Set
	}

	for _, part := range parts {
		val, ok := node.Children.Load(part)
		if !ok {
			break
		}
		node = val.(*Node)
		if node.Set != nil {
			bestSet = node.Set
		}
	}
	return bestSet
}

// ShouldIgnore returns true if the path should be excluded from traversal.
// path MUST be cleaned and use forward slashes (normalized by Walker).
// If ctx is provided, it avoids redundant trie traversals.
func (e *Engine) ShouldIgnore(path string, isDir bool, ctx ...IgnoreContext) bool {
	hasGlobs := len(e.excludes) > 0 || (!isDir && len(e.includes) > 0) || (!isDir && (len(e.typeExcludes) > 0 || len(e.typeIncludes) > 0))

	if hasGlobs {
		// 1. Calculate path relative to cwd for glob matching
		relSlash := path
		if e.baseRelSlash != "" && strings.HasPrefix(path, e.baseRelSlash) {
			if len(path) > len(e.baseRelSlash) {
				relSlash = path[len(e.baseRelSlash)+1:]
			} else {
				relSlash = ""
			}
		}

		// 2. CLI globs always take precedence
		base := relSlash
		if idx := strings.LastIndexByte(relSlash, '/'); idx >= 0 {
			base = relSlash[idx+1:]
		}

		for i := range e.excludes {
			if e.excludes[i].Match(relSlash) || e.excludes[i].Match(base) {
				return true
			}
		}

		if !isDir && len(e.includes) > 0 {
			matched := false
			for i := range e.includes {
				if e.includes[i].Match(relSlash) || e.includes[i].Match(base) {
					matched = true
					break
				}
			}
			if !matched {
				return true
			}
		}

		// 3. Type filters (files only)
		if !isDir {
			for i := range e.typeExcludes {
				if e.typeExcludes[i].Match(relSlash) || e.typeExcludes[i].Match(base) {
					return true
				}
			}
			if len(e.typeIncludes) > 0 {
				matched := false
				for i := range e.typeIncludes {
					if e.typeIncludes[i].Match(relSlash) || e.typeIncludes[i].Match(base) {
						matched = true
						break
					}
				}
				if !matched {
					return true
				}
			}
		}
	}

	// 4. Hidden file check
	if !e.cfg.Hidden {
		fromWalkCtx := len(ctx) > 0
		if fromWalkCtx {
			if idx := strings.LastIndexByte(path, '/'); idx >= 0 {
				if len(path) > idx+1 && path[idx+1] == '.' {
					return true
				}
			} else if len(path) > 0 && path[0] == '.' {
				return true
			}
		} else {
			relSlash := path
			if e.baseRelSlash != "" && strings.HasPrefix(path, e.baseRelSlash) {
				if len(path) > len(e.baseRelSlash) {
					relSlash = path[len(e.baseRelSlash)+1:]
				} else {
					relSlash = ""
				}
			}
			for i := 0; i < len(relSlash); i++ {
				if relSlash[i] == '.' && (i == 0 || relSlash[i-1] == '/') {
					return true
				}
			}
		}
	}

	if e.cfg.NoIgnore {
		return false
	}

	// 5. Check ignore file rules via chain.
	var bestSet *IgnoreSet
	fromWalkCtx := len(ctx) > 0 && ctx[0].set != nil
	if fromWalkCtx {
		bestSet = ctx[0].set
	} else {
		bestSet = e.lookup(path)
	}

	if bestSet == nil {
		return false
	}

	// 6. If called outside walk context (e.g. standalone API or test), verify
	// ancestor directories are not ignored. During walking, Walker already
	// checked ancestor directories before recursing.
	if !fromWalkCtx {
		for cur := bestSet; cur != nil; cur = cur.Parent {
			if cur.Parent != nil {
				if cur.Parent.IsIgnored(cur.DirBase, true) {
					return true
				}
			}
		}
	}

	// 7. Check rules for the file itself.
	relToSet := path
	if bestSet.Dir != "." && strings.HasPrefix(path, bestSet.Dir) {
		if len(path) > len(bestSet.Dir) {
			relToSet = path[len(bestSet.Dir)+1:]
		} else {
			relToSet = ""
		}
	}

	return bestSet.IsIgnored(relToSet, isDir)
}

// GetIgnoreRules returns the loaded ignore rules for a directory.
func (e *Engine) GetIgnoreRules(dir string) []IgnoreRule {
	dir = filepath.ToSlash(filepath.Clean(dir))
	if set := e.lookup(dir); set != nil && set.Dir == dir {
		return set.Rules
	}
	return nil
}
