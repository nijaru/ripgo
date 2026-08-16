// Package ripgo provides high-level APIs for searching file contents and
// finding filesystem paths.
//
// It orchestrates the walk, ignore, pattern, search, and find packages into
// simple, unified interfaces for library consumers.
package ripgo

import (
	"context"
	"fmt"
	"io/fs"
	"iter"
	"path"
	"path/filepath"
	"runtime"
	"sync"

	findpkg "github.com/nijaru/ripgo/find"
	"github.com/nijaru/ripgo/ignore"
	"github.com/nijaru/ripgo/internal/osfs"
	"github.com/nijaru/ripgo/pattern"
	"github.com/nijaru/ripgo/search"
	"github.com/nijaru/ripgo/walk"
)

// Search performs a complete search using the provided configuration and paths.
// It returns an iterator of (Result, error). The error is non-nil if a file-level
// error was encountered (e.g. Permission Denied).
func Search(ctx context.Context, patternStr string, paths []string, opts ...Option) iter.Seq2[search.Result, error] {
	cfg := DefaultConfig(patternStr)
	for _, opt := range opts {
		opt(&cfg)
	}

	return func(yield func(search.Result, error) bool) {
		var matcher pattern.Matcher
		var err error

		if cfg.Matcher != nil {
			matcher = cfg.Matcher
		} else {
			matcher, err = pattern.New(cfg.Pattern)
			if err != nil {
				yield(search.Result{}, err)
				return
			}
		}

		fsys := cfg.FS
		if fsys == nil {
			osfsys := osfs.New()
			fsys = osfsys
			defer osfsys.Close()
		}

		engine, err := ignore.NewEngineFS(fsys, cfg.Ignore)
		if err != nil {
			yield(search.Result{}, err)
			return
		}

		w := walk.NewWalker(fsys, cfg.Walk, engine)
		s := search.NewSearcher(fsys, cfg.Search, matcher)

		fileCh := make(chan walk.Entry, 1024)
		resultCh := make(chan search.Result, 1024)

		// Context cancellation management
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		var scanWg sync.WaitGroup

		// Start walker
		scanWg.Add(1)
		go func() {
			defer scanWg.Done()
			w.Run(ctx, paths, fileCh)
		}()

		// Start scan workers
		workers := cfg.Walk.Threads
		if workers <= 0 {
			workers = runtime.GOMAXPROCS(0)
		}

		for range workers {
			scanWg.Add(1)
			go func() {
				defer scanWg.Done()
				for entry := range fileCh {
					res, err := s.Search(entry.File)
					if err != nil {
						res.Error = err
						// We emit the error but keep searching other files
					}

					// Filter logic
					hasMatches := len(res.Matches) > 0 || len(res.Entries) > 0 || res.Error != nil
					if hasMatches || (res.Binary && (cfg.Search.SearchBinary || cfg.Search.OnlyBinary)) {
						select {
						case <-ctx.Done():
							return
						case resultCh <- res:
						}
					}
				}
			}()
		}

		go func() {
			scanWg.Wait()
			close(resultCh)
		}()

		// Consume results and yield to iterator
		for {
			select {
			case <-ctx.Done():
				return
			case res, ok := <-resultCh:
				if !ok {
					return
				}
				if !yield(res, res.Error) {
					res.Release()
					return // Caller stopped iterating
				}
				res.Release()
			}
		}
	}
}

// Find streams matching paths from the supplied roots without reading file
// contents. It returns an iterator of (Result, error). A missing path or
// traversal failure is yielded as an error; the iterator stops when the caller
// stops yielding.
func Find(ctx context.Context, pattern string, paths []string, opts ...FindOption) iter.Seq2[findpkg.Result, error] {
	cfg := DefaultFindConfig(pattern)
	for _, opt := range opts {
		opt(&cfg)
	}

	return func(yield func(findpkg.Result, error) bool) {
		filter, err := findpkg.NewFilter(cfg)
		if err != nil {
			yield(findpkg.Result{}, err)
			return
		}

		fsys := cfg.FS
		var osfsys *osfs.OSFS
		if fsys == nil {
			osfsys = osfs.New()
			fsys = osfsys
			defer osfsys.Close()
		}

		engine, err := ignore.NewEngineFS(fsys, cfg.IgnoreConfig())
		if err != nil {
			yield(findpkg.Result{}, err)
			return
		}

		roots := append([]string(nil), paths...)
		if len(roots) == 0 {
			roots = []string{"."}
		}

		walker := walk.NewWalker(fsys, cfg.WalkerConfig(), engine)
		fileCh := make(chan walk.Entry, 1024)
		errorCh := make(chan error, 32)
		runCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		done := make(chan struct{})
		go func() {
			defer close(done)
			walker.RunWithErrors(runCtx, roots, fileCh, errorCh)
		}()
		defer func() {
			cancel()
			<-done
		}()

		for fileCh != nil || errorCh != nil {
			select {
			case err, ok := <-errorCh:
				if !ok {
					errorCh = nil
					continue
				}
				if !yield(findpkg.Result{}, err) {
					return
				}
			case entry, ok := <-fileCh:
				if !ok {
					fileCh = nil
					continue
				}
				matchPath := findMatchPath(entry.DisplayPath(), roots)
				if !filter.MatchPath(entry, matchPath) {
					continue
				}
				if entry.Info == nil && !cfg.OmitInfo {
					if _, err := entry.ResolveInfo(); err != nil {
						if !yield(findpkg.Result{}, fmt.Errorf("stat %q: %w", entry.Path, err)) {
							return
						}
						continue
					}
				}
				if !yield(findpkg.NewResult(entry), nil) {
					return
				}
			}
		}
	}
}

// FindOption configures Find.
type FindOption func(*findpkg.Config)

// DefaultFindConfig returns a finder configuration with pattern matching
// enabled and the shared walker's zero-value traversal defaults.
func DefaultFindConfig(pattern string) findpkg.Config {
	return findpkg.Config{
		Matcher: findpkg.MatcherConfig{Pattern: pattern},
		Walk:    walk.Config{Threads: 0},
	}
}

// WithFindFS selects the filesystem used by Find.
func WithFindFS(fsys fs.FS) FindOption {
	return func(c *findpkg.Config) { c.FS = fsys }
}

// WithFindThreads sets the finder traversal worker count.
func WithFindThreads(n int) FindOption {
	return func(c *findpkg.Config) { c.Walk.Threads = n }
}

// WithFindGlob enables glob matching.
func WithFindGlob(v bool) FindOption {
	return func(c *findpkg.Config) { c.Matcher.Glob = v }
}

// WithFindFixedStrings enables literal substring matching.
func WithFindFixedStrings(v bool) FindOption {
	return func(c *findpkg.Config) { c.Matcher.FixedStrings = v }
}

// WithFindIgnoreCase enables case-insensitive matching and extension filters.
func WithFindIgnoreCase(v bool) FindOption {
	return func(c *findpkg.Config) { c.Matcher.IgnoreCase = v }
}

// WithFindFullPath matches normalized paths relative to each search root.
func WithFindFullPath(v bool) FindOption {
	return func(c *findpkg.Config) { c.Matcher.FullPath = v }
}

// WithFindType adds a result type filter. Repeated values are ORed.
func WithFindType(typ findpkg.Type) FindOption {
	return func(c *findpkg.Config) { c.Types = append(c.Types, typ) }
}

// WithFindTypes adds result type filters. Repeated values are ORed.
func WithFindTypes(types ...findpkg.Type) FindOption {
	return func(c *findpkg.Config) { c.Types = append(c.Types, types...) }
}

// WithFindExtension adds an extension filter. Repeated values are ORed.
func WithFindExtension(extension string) FindOption {
	return func(c *findpkg.Config) { c.Extensions = append(c.Extensions, extension) }
}

// WithFindExtensions adds extension filters. Repeated values are ORed.
func WithFindExtensions(extensions ...string) FindOption {
	return func(c *findpkg.Config) { c.Extensions = append(c.Extensions, extensions...) }
}

// WithFindMinSize sets the inclusive minimum metadata size.
func WithFindMinSize(size int64) FindOption {
	return func(c *findpkg.Config) { c.MinSize = size }
}

// WithFindMaxSize sets the inclusive maximum metadata size.
func WithFindMaxSize(size int64) FindOption {
	return func(c *findpkg.Config) {
		c.MaxSize = size
		c.MaxSizeSet = true
	}
}

// WithFindMinDepth sets the inclusive minimum root-relative depth.
func WithFindMinDepth(depth int) FindOption {
	return func(c *findpkg.Config) { c.Walk.MinDepth = depth }
}

// WithFindMaxDepth sets the inclusive maximum root-relative depth.
func WithFindMaxDepth(depth int) FindOption {
	return func(c *findpkg.Config) {
		c.Walk.MaxDepth = depth
		c.Walk.MaxDepthSet = true
	}
}

// WithFindFollowSymlinks enables followed directory symlinks.
func WithFindFollowSymlinks(v bool) FindOption {
	return func(c *findpkg.Config) { c.Walk.FollowSymlinks = v }
}

// WithFindHidden includes hidden paths.
func WithFindHidden(v bool) FindOption {
	return func(c *findpkg.Config) { c.Ignore.Hidden = v }
}

// WithFindGlobExcludes excludes matching paths from finder traversal.
func WithFindGlobExcludes(globs ...string) FindOption {
	return func(c *findpkg.Config) {
		c.Ignore.GlobExcludes = append(c.Ignore.GlobExcludes, globs...)
	}
}

// WithFindMetadata controls whether finder results resolve file metadata.
// Metadata is enabled by default; disabling it leaves Result.Info nil and
// cannot be combined with size filters.
func WithFindMetadata(v bool) FindOption {
	return func(c *findpkg.Config) { c.OmitInfo = !v }
}

// WithFindNoIgnore disables ignore-file loading.
func WithFindNoIgnore(v bool) FindOption {
	return func(c *findpkg.Config) { c.Ignore.NoIgnore = v }
}

func findMatchPath(entryPath string, roots []string) string {
	entryPath = findpkg.NormalizePath(entryPath)
	for _, root := range roots {
		root = findpkg.NormalizePath(root)
		if root == "" {
			root = "."
		}
		rel, err := filepath.Rel(filepath.FromSlash(root), filepath.FromSlash(entryPath))
		rel = filepath.ToSlash(rel)
		if err != nil || rel == ".." || len(rel) > 3 && rel[:3] == "../" {
			continue
		}
		if rel == "." {
			return path.Base(root)
		}
		return rel
	}
	return entryPath
}

// Config represents the complete search configuration.
type Config struct {
	Pattern pattern.Config
	Search  search.Config
	Walk    walk.Config
	Ignore  ignore.Config
	FS      fs.FS
	Matcher pattern.Matcher
}

// Option is a functional option for configuring the search.
type Option func(*Config)

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig(pat string) Config {
	return Config{
		Pattern: pattern.Config{Pattern: pat},
		Search:  search.Config{SearchBinary: false},
		Walk:    walk.Config{Threads: 0},
		Ignore:  ignore.Config{},
	}
}

// Functional Options

func WithThreads(n int) Option {
	return func(c *Config) { c.Walk.Threads = n }
}

func WithFS(fsys fs.FS) Option {
	return func(c *Config) { c.FS = fsys }
}

func WithIgnoreCase(v bool) Option {
	return func(c *Config) { c.Pattern.IgnoreCase = v }
}

func WithFixedStrings(v bool) Option {
	return func(c *Config) { c.Pattern.FixedStrings = v }
}

func WithPcre2(v bool) Option {
	return func(c *Config) { c.Pattern.Pcre2 = v }
}

func WithContext(before, after int) Option {
	return func(c *Config) {
		c.Search.Before = before
		c.Search.After = after
	}
}

func WithMaxCount(n int) Option {
	return func(c *Config) { c.Search.MaxCount = n }
}

func WithHidden(v bool) Option {
	return func(c *Config) { c.Ignore.Hidden = v }
}

func WithNoIgnore(v bool) Option {
	return func(c *Config) { c.Ignore.NoIgnore = v }
}

func WithTypes(types []string) Option {
	return func(c *Config) { c.Ignore.Types = types }
}

func WithTypesNot(typesNot []string) Option {
	return func(c *Config) { c.Ignore.TypesNot = typesNot }
}

func WithFollowSymlinks(v bool) Option {
	return func(c *Config) { c.Walk.FollowSymlinks = v }
}

func WithMaxFileSize(n int64) Option {
	return func(c *Config) { c.Walk.MaxFileSize = n }
}

func WithMatcher(m pattern.Matcher) Option {
	return func(c *Config) { c.Matcher = m }
}

func WithMultiline(v bool) Option {
	return func(c *Config) {
		c.Pattern.Multiline = v
		c.Search.Multiline = v
	}
}

func WithWordRegexp(v bool) Option {
	return func(c *Config) {
		c.Pattern.WordRegexp = v
	}
}

func WithOnlyMatching(v bool) Option {
	return func(c *Config) {
		c.Search.OnlyMatching = v
	}
}

func WithReplace(v string) Option {
	return func(c *Config) {
		c.Search.Replace = v
	}
}

func WithGlobIncludes(globs ...string) Option {
	return func(c *Config) {
		c.Ignore.GlobIncludes = append(c.Ignore.GlobIncludes, globs...)
	}
}

func WithGlobExcludes(globs ...string) Option {
	return func(c *Config) {
		c.Ignore.GlobExcludes = append(c.Ignore.GlobExcludes, globs...)
	}
}
