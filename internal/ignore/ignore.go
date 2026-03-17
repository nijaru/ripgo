package ignore

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gobwas/glob"
	"github.com/nijaru/ripgo/internal/config"
)

type Engine struct {
	cfg      *config.Config
	includes []glob.Glob
	excludes []glob.Glob
	ignores  map[string][]string
	mu       sync.Mutex
}

type Rule struct {
	Pattern  string
	Negated  bool
	Source   string
	Compiled glob.Glob
}

func NewEngine(cfg *config.Config) (*Engine, error) {
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

	patterns := []string{}

	ignoreFiles := []string{".gitignore", ".ignore"}
	for _, name := range ignoreFiles {
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

func (e *Engine) GetIgnoreRules(dir string) []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	if rules, ok := e.ignores[dir]; ok {
		return rules
	}
	return nil
}

func (e *Engine) MatchIgnore(path, dir string) bool {
	rules := e.GetIgnoreRules(dir)
	if rules == nil {
		return false
	}

	relPath, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}

	for _, rule := range rules {
		negated := strings.HasPrefix(rule, "!")
		pattern := rule
		if negated {
			pattern = rule[1:]
		}

		if matchPattern(pattern, relPath) {
			return !negated
		}
	}
	return false
}

func matchPattern(pattern, path string) bool {
	if pattern == "" {
		return false
	}

	if pattern == "**" {
		return true
	}

	hasPrefix := strings.HasPrefix(pattern, "/")
	hasSuffix := strings.HasSuffix(pattern, "/")

	if hasPrefix {
		pattern = pattern[1:]
	}

	if hasSuffix {
		pattern = pattern[:len(pattern)-1]
		if path != "" {
			parts := strings.Split(path, "/")
			last := parts[len(parts)-1]
			if last == pattern || pattern == "**" {
				return true
			}
		}
	}

	if strings.Contains(pattern, "**") {
		parts := strings.Split(pattern, "**")
		if len(parts) == 2 {
			before, after := parts[0], parts[1]
			if before != "" && !strings.HasPrefix(path, before) {
				return false
			}
			if after != "" && !strings.HasSuffix(path, after) {
				return false
			}
			return true
		}
	}

	if strings.Contains(pattern, "*") {
		matched, err := filepath.Match(pattern, filepath.Base(path))
		if err == nil && matched {
			return true
		}
	}

	return path == pattern || strings.HasSuffix(path, "/"+pattern)
}
