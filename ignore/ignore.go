package ignore

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gobwas/glob"
)

// Config holds ignore engine options.
type Config struct {
	// GlobIncludes is the list of glob patterns to include.
	GlobIncludes []string
	// GlobExcludes is the list of glob patterns to exclude.
	GlobExcludes []string
	// NoIgnore disables .gitignore/.ignore file loading.
	NoIgnore bool
	// Hidden includes hidden files and directories.
	Hidden bool
}

// Engine manages ignore rules and glob filters for file traversal.
type Engine struct {
	cfg      Config
	includes []glob.Glob
	excludes []glob.Glob
	ignores  map[string][]string
	mu       sync.Mutex
}

// NewEngine creates an ignore engine from the given config.
func NewEngine(cfg Config) (*Engine, error) {
	engine := &Engine{
		cfg:      cfg,
		ignores:  make(map[string][]string),
		includes: make([]glob.Glob, 0, len(cfg.GlobIncludes)),
		excludes: make([]glob.Glob, 0, len(cfg.GlobExcludes)),
	}

	for _, p := range cfg.GlobIncludes {
		g, err := glob.Compile(p)
		if err != nil {
			return nil, err
		}
		engine.includes = append(engine.includes, g)
	}

	for _, p := range cfg.GlobExcludes {
		g, err := glob.Compile(p)
		if err != nil {
			return nil, err
		}
		engine.excludes = append(engine.excludes, g)
	}

	return engine, nil
}

// ShouldIgnore returns true if the path should be excluded from traversal.
func (e *Engine) ShouldIgnore(path string, isDir bool) bool {
	relPath, err := filepath.Rel(".", path)
	if err != nil {
		return false
	}

	if !e.cfg.Hidden {
		parts := strings.Split(relPath, string(filepath.Separator))
		for _, p := range parts {
			if len(p) > 0 && p[0] == '.' {
				return true
			}
		}
	}

	for _, g := range e.excludes {
		if g.Match(relPath) {
			return true
		}
	}

	if len(e.includes) > 0 {
		matched := false
		for _, g := range e.includes {
			if g.Match(relPath) {
				matched = true
				break
			}
		}
		if !matched {
			return true
		}
	}

	return false
}

// LoadIgnoreFile reads .gitignore and .ignore files from the given directory.
func (e *Engine) LoadIgnoreFile(dir string) error {
	if e.cfg.NoIgnore {
		return nil
	}

	dir = filepath.Clean(dir)

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.ignores[dir]; ok {
		return nil
	}

	var patterns []string
	for _, name := range []string{".gitignore", ".ignore"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			patterns = append(patterns, line)
		}
	}

	e.ignores[dir] = patterns
	return nil
}

// GetIgnoreRules returns the loaded ignore rules for a directory.
func (e *Engine) GetIgnoreRules(dir string) []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.ignores[dir]
}
