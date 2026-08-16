package config

import (
	"fmt"
	"io/fs"
	"os"
	"runtime"
	"strconv"
	"strings"

	findpkg "github.com/nijaru/ripgo/find"
	"github.com/nijaru/ripgo/ignore"
	"github.com/nijaru/ripgo/internal/cli"
	"github.com/nijaru/ripgo/internal/osfs"
	"github.com/nijaru/ripgo/pattern"
	"github.com/nijaru/ripgo/search"
	"github.com/nijaru/ripgo/walk"
	"golang.org/x/term"
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
	MaxColumns       int
	Json             bool
	Heading          bool
	Color            bool
	Sort             string
	TypeList         bool
	Threads          int
	FS               fs.FS
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

// FindConfig contains validated runtime configuration for path finding.
type FindConfig struct {
	Paths    []string
	Pattern  string
	Find     findpkg.Config
	Absolute bool
	Print0   bool
	Color    bool
	Sort     string
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
	if opts.TypeList {
		// --type-list doesn't need a pattern; return early with just TypeList set.
		return &Config{TypeList: true}, nil
	}

	if opts.Pattern == "" {
		return nil, fmt.Errorf("pattern required")
	}

	isTTY := term.IsTerminal(int(os.Stdout.Fd()))

	paths := opts.Paths
	if len(paths) == 0 {
		paths = []string{"."}
	}

	heading := opts.Heading
	if !heading && isTTY && !opts.Json && !opts.Count && !opts.FilesWithMatches {
		heading = true
	}

	useColor := false
	switch opts.Color {
	case "always":
		useColor = true
	case "never":
		useColor = false
	case "auto":
		useColor = isTTY
	}

	pcfg := pattern.Config{
		Pattern:      opts.Pattern,
		FixedStrings: opts.FixedStrings,
		IgnoreCase:   opts.IgnoreCase,
		SmartCase:    opts.SmartCase,
		Pcre2:        opts.Pcre2,
		Multiline:    opts.Multiline,
		WordRegexp:   opts.WordRegexp,
	}

	if opts.Encoding != "" {
		if _, err := search.LookupEncoding(opts.Encoding); err != nil {
			return nil, err
		}
	}

	scfg := search.Config{
		MaxCount:     opts.MaxCount,
		SearchBinary: (!opts.NoBinary && !opts.OnlyBinary) || opts.Unrestricted >= 3,
		OnlyBinary:   opts.OnlyBinary,
		Multiline:    opts.Multiline,
		OnlyMatching: opts.OnlyMatching,
		Replace:      opts.Replace,
		Encoding:     opts.Encoding,
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
		NoIgnore:     opts.NoIgnore || opts.Unrestricted >= 1,
		Hidden:       opts.Hidden || opts.Unrestricted >= 2,
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
		MaxColumns:       opts.MaxColumns,
		Json:             opts.Json,
		Heading:          heading,
		Color:            useColor,
		Sort:             opts.Sort,
		TypeList:         opts.TypeList,
		Threads:          threads,
		FS:               osfs.New(),
	}, nil
}

// NewFind translates finder CLI options into public finder configuration.
func NewFind(opts cli.FindOptions) (*FindConfig, error) {
	paths := append([]string(nil), opts.Paths...)
	if len(paths) == 0 {
		paths = []string{"."}
	}
	if opts.MinDepth < 0 {
		return nil, fmt.Errorf("minimum depth must not be negative: %d", opts.MinDepth)
	}
	if opts.MaxDepth != nil && *opts.MaxDepth < 0 {
		return nil, fmt.Errorf("maximum depth must not be negative: %d", *opts.MaxDepth)
	}

	types, err := parseFindTypes(opts.Type)
	if err != nil {
		return nil, err
	}

	fcfg := findpkg.Config{
		Matcher: findpkg.MatcherConfig{
			Pattern:      opts.Pattern,
			Glob:         opts.Glob,
			FixedStrings: opts.FixedStrings,
			IgnoreCase:   opts.IgnoreCase,
			FullPath:     opts.FullPath,
		},
		Types:      types,
		Extensions: append([]string(nil), opts.Extension...),
		Walk: walk.Config{
			Threads:        opts.Threads,
			FollowSymlinks: opts.FollowSymlinks,
			MinDepth:       opts.MinDepth,
		},
		Ignore: ignore.Config{
			NoIgnore: opts.NoIgnore,
			Hidden:   opts.Hidden,
		},
	}
	if opts.MaxDepth != nil {
		fcfg.Walk.MaxDepth = *opts.MaxDepth
		fcfg.Walk.MaxDepthSet = true
	}

	if opts.MinSize != "" {
		fcfg.MinSize, err = parseSize(opts.MinSize)
		if err != nil {
			return nil, fmt.Errorf("invalid minimum size: %w", err)
		}
	}
	if opts.MaxSize != "" {
		fcfg.MaxSize, err = parseSize(opts.MaxSize)
		if err != nil {
			return nil, fmt.Errorf("invalid maximum size: %w", err)
		}
		fcfg.MaxSizeSet = true
	}

	isTTY := term.IsTerminal(int(os.Stdout.Fd()))
	useColor := opts.Color == "always" || (opts.Color == "auto" && isTTY)
	return &FindConfig{
		Paths:    paths,
		Pattern:  opts.Pattern,
		Find:     fcfg,
		Absolute: opts.Absolute,
		Print0:   opts.Print0,
		Color:    useColor,
		Sort:     opts.Sort,
	}, nil
}

func parseFindTypes(values []string) ([]findpkg.Type, error) {
	types := make([]findpkg.Type, 0, len(values))
	for _, value := range values {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "f", "file", "regular":
			types = append(types, findpkg.TypeFile)
		case "d", "dir", "directory":
			types = append(types, findpkg.TypeDirectory)
		case "l", "link", "symlink":
			types = append(types, findpkg.TypeSymlink)
		default:
			return nil, fmt.Errorf("unknown finder type %q (use f, d, or l)", value)
		}
	}
	return types, nil
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
