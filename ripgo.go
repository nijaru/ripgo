// Package ripgo provides a high-level API for searching files.
//
// It orchestrates the walk, ignore, pattern, and search packages into a
// simple, unified interface for library consumers.
package ripgo

import (
	"context"
	"io/fs"
	"iter"
	"sync"

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
			workers = 4
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
