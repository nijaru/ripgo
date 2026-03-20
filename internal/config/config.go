package config

import (
	"runtime"
	"strconv"
	"strings"
	"unicode"

	"github.com/nijaru/ripgo/ignore"
	"github.com/nijaru/ripgo/internal/cli"
	"github.com/nijaru/ripgo/pattern"
	"github.com/nijaru/ripgo/search"
	"github.com/nijaru/ripgo/walk"
)

// Config holds all validated runtime configuration, translating CLI flags
// into library-specific configs.
type Config struct {
	Paths            []string
	Pattern          pattern.Config
	Search           search.Config
	Walk             walk.Config
	Ignore           ignore.Config
	LineNumber       bool
	Column           bool
	Count            bool
	FilesWithMatches bool
	Quiet            bool
	Json             bool
	Heading          bool
	Sort             string
	TypeList         bool
	Threads          int
}

// OutputMode returns the selected output mode.
func (c *Config) OutputMode() OutputMode {
	switch {
	case c.Json:
		return OutputJSON
	case c.Count:
		return OutputCount
	case c.FilesWithMatches:
		return OutputFiles
	case c.Quiet:
		return OutputQuiet
	default:
		return OutputNormal
	}
}

type OutputMode int

const (
	OutputNormal OutputMode = iota
	OutputJSON
	OutputCount
	OutputFiles
	OutputQuiet
)

// New translates CLI options into validated library configs.
func New(opts cli.Options) (*Config, error) {
	paths := opts.Paths
	if len(paths) == 0 {
		paths = []string{"."}
	}

	pcfg := pattern.Config{
		Pattern:      opts.Pattern,
		FixedStrings: opts.FixedStrings,
		IgnoreCase:   opts.IgnoreCase,
		SmartCase:    opts.SmartCase,
		Pcre2:        opts.Pcre2,
	}

	scfg := search.Config{
		MaxCount:     opts.MaxCount,
		SearchBinary: !opts.NoBinary && !opts.OnlyBinary,
		OnlyBinary:   opts.OnlyBinary,
	}

	if opts.Context > 0 {
		scfg.Before = opts.Context
		scfg.After = opts.Context
	} else {
		scfg.Before = opts.ContextBefore
		scfg.After = opts.ContextAfter
	}

	wcfg := walk.Config{
		Threads:        opts.Threads,
		FollowSymlinks: opts.FollowSymlinks,
	}

	if opts.MaxFileSize != "" {
		size, err := parseSize(opts.MaxFileSize)
		if err != nil {
			return nil, err
		}
		wcfg.MaxFileSize = size
	}

	icfg := ignore.Config{
		GlobIncludes: opts.GlobInclude,
		GlobExcludes: opts.GlobExclude,
		Types:        opts.Type,
		TypesNot:     opts.TypeNot,
		NoIgnore:     opts.NoIgnore,
		Hidden:       opts.Hidden,
	}

	threads := opts.Threads
	if threads <= 0 {
		threads = runtime.GOMAXPROCS(0)
	}

	return &Config{
		Paths:            paths,
		Pattern:          pcfg,
		Search:           scfg,
		Walk:             wcfg,
		Ignore:           icfg,
		LineNumber:       opts.LineNumber,
		Column:           opts.Column,
		Count:            opts.Count,
		FilesWithMatches: opts.FilesWithMatches,
		Quiet:            opts.Quiet,
		Json:             opts.Json,
		Heading:          opts.Heading,
		Sort:             opts.Sort,
		TypeList:         opts.TypeList,
		Threads:          threads,
	}, nil
}

func parseSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, strconv.ErrSyntax
	}

	multiplier := int64(1)
	if len(s) > 1 {
		last := s[len(s)-1]
		switch last {
		case 'k', 'K':
			multiplier = 1024
			s = s[:len(s)-1]
		case 'm', 'M':
			multiplier = 1024 * 1024
			s = s[:len(s)-1]
		case 'g', 'G':
			multiplier = 1024 * 1024 * 1024
			s = s[:len(s)-1]
		}
	}

	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, err
	}
	return n * multiplier, nil
}

func hasUppercase(s string) bool {
	for _, r := range s {
		if unicode.IsUpper(r) {
			return true
		}
	}
	return false
}
