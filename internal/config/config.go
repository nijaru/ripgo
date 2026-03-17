package config

import (
	"errors"
	"runtime"
	"strconv"
	"strings"
	"unicode"

	"github.com/nijaru/ripgo/internal/cli"
)

type OutputMode int

const (
	OutputModeNormal OutputMode = iota
	OutputModeJSON
	OutputModeCount
	OutputModeFiles
	OutputModeQuiet
)

type Config struct {
	Paths            []string
	Pattern          string
	FixedStrings     bool
	IgnoreCase       bool
	SmartCase        bool
	Multiline        bool
	Hidden           bool
	NoIgnore         bool
	SearchBinary     bool
	OnlyBinary       bool
	FollowSymlinks   bool
	ContextBefore    int
	ContextAfter     int
	Context          int
	LineNumber       bool
	Column           bool
	Count            bool
	FilesWithMatches bool
	Quiet            bool
	MaxFileSize      int64
	MaxCount         int
	GlobIncludes     []string
	GlobExcludes     []string
	OutputMode       OutputMode
	Threads          int
}

func New(opts cli.Options) (*Config, error) {
	cfg := &Config{
		Paths:            opts.Paths,
		Pattern:          opts.Pattern,
		FixedStrings:     opts.FixedStrings,
		IgnoreCase:       opts.IgnoreCase,
		SmartCase:        opts.SmartCase,
		Multiline:        opts.Multiline,
		Hidden:           opts.Hidden,
		NoIgnore:         opts.NoIgnore,
		SearchBinary:     !opts.NoBinary && !opts.OnlyBinary,
		OnlyBinary:       opts.OnlyBinary,
		FollowSymlinks:   opts.FollowSymlinks,
		ContextBefore:    opts.ContextBefore,
		ContextAfter:     opts.ContextAfter,
		LineNumber:       opts.LineNumber,
		Column:           opts.Column,
		Count:            opts.Count,
		FilesWithMatches: opts.FilesWithMatches,
		Quiet:            opts.Quiet,
		MaxCount:         opts.MaxCount,
		GlobIncludes:     opts.GlobInclude,
		GlobExcludes:     opts.GlobExclude,
		Threads:          opts.Threads,
	}

	if len(cfg.Paths) == 0 {
		cfg.Paths = []string{"."}
	}

	if opts.Context > 0 {
		cfg.ContextBefore = opts.Context
		cfg.ContextAfter = opts.Context
	}

	switch {
	case opts.Json:
		cfg.OutputMode = OutputModeJSON
	case opts.Count:
		cfg.OutputMode = OutputModeCount
	case opts.FilesWithMatches:
		cfg.OutputMode = OutputModeFiles
	case opts.Quiet:
		cfg.OutputMode = OutputModeQuiet
	}

	if cfg.Threads <= 0 {
		cfg.Threads = runtime.GOMAXPROCS(0)
	}

	if opts.MaxFileSize != "" {
		size, err := parseSize(opts.MaxFileSize)
		if err != nil {
			return nil, err
		}
		cfg.MaxFileSize = size
	}

	if cfg.SmartCase && !cfg.IgnoreCase && !cfg.FixedStrings {
		cfg.IgnoreCase = !hasUppercase(cfg.Pattern)
	}

	return cfg, nil
}

func hasUppercase(s string) bool {
	for _, r := range s {
		if unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

func parseSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("empty size")
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
