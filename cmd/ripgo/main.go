package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/nijaru/ripgo"
	findpkg "github.com/nijaru/ripgo/find"
	"github.com/nijaru/ripgo/ignore"
	"github.com/nijaru/ripgo/internal/action"
	"github.com/nijaru/ripgo/internal/cli"
	"github.com/nijaru/ripgo/internal/config"
	"github.com/nijaru/ripgo/printer"
	"github.com/nijaru/ripgo/search"
	"github.com/nijaru/ripgo/stats"
)

func run(ctx context.Context) int {
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "find" {
		var opts cli.FindOptions
		parser, err := kong.New(&opts, kong.Name("ripgo find"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 2
		}
		if _, err := parser.Parse(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 2
		}
		if opts.Version {
			fmt.Println(cli.Version)
			return 0
		}
		normalizeFindPositionals(&opts)
		return runFind(ctx, opts)
	}
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		var root cli.CLI
		parser, err := kong.New(&root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 2
		}
		if _, err := parser.Parse(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 2
		}
		return 0
	}
	if len(args) > 0 && args[0] == "search" {
		args = args[1:]
	}

	var opts cli.Options
	parser, err := kong.New(&opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 2
	}
	if _, err := parser.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 2
	}
	if opts.Version {
		fmt.Println(cli.Version)
		return 0
	}
	return runSearch(ctx, opts)
}

func runSearch(ctx context.Context, opts cli.Options) int {
	cfg, err := config.New(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 2
	}

	if cfg.TypeList {
		for t, patterns := range ignore.FileTypes {
			fmt.Printf("%s: %s\n", t, strings.Join(patterns, ", "))
		}
		return 0
	}

	// Translate internal/config to ripgo high-level options
	searchOpts := []ripgo.Option{
		ripgo.WithThreads(cfg.Threads),
		ripgo.WithFS(cfg.FS),
		ripgo.WithIgnoreCase(cfg.Pattern.IgnoreCase),
		ripgo.WithFixedStrings(cfg.Pattern.FixedStrings),
		ripgo.WithPcre2(cfg.Pattern.Pcre2),
		ripgo.WithContext(cfg.Search.Before, cfg.Search.After),
		ripgo.WithMaxCount(cfg.Search.MaxCount),
		ripgo.WithHidden(cfg.Ignore.Hidden),
		ripgo.WithNoIgnore(cfg.Ignore.NoIgnore),
		ripgo.WithTypes(cfg.Ignore.Types),
		ripgo.WithTypesNot(cfg.Ignore.TypesNot),
		ripgo.WithFollowSymlinks(cfg.Walk.FollowSymlinks),
		ripgo.WithMaxFileSize(cfg.Walk.MaxFileSize),
		ripgo.WithMultiline(cfg.Pattern.Multiline),
		ripgo.WithWordRegexp(cfg.Pattern.WordRegexp),
		ripgo.WithOnlyMatching(cfg.Search.OnlyMatching),
		ripgo.WithReplace(cfg.Search.Replace),
		ripgo.WithEncoding(cfg.Search.Encoding),
		ripgo.WithGlobIncludes(cfg.Ignore.GlobIncludes...),
		ripgo.WithGlobExcludes(cfg.Ignore.GlobExcludes...),
	}
	prn := newPrinter(cfg)
	var st stats.Stats
	var results []search.Result

	// Use the new iterator-based API
	for res, err := range ripgo.Search(ctx, cfg.Pattern.Pattern, cfg.Paths, searchOpts...) {
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error searching %s: %v\n", res.Path, err)
			continue
		}

		if cfg.Sort == "" || cfg.Sort == "none" {
			if err := prn.PrintResult(res); err != nil {
				continue
			}
		} else {
			results = append(results, res)
		}
		st.RecordMatch(res)
	}

	if cfg.Sort != "" && cfg.Sort != "none" {
		sortResults(results, cfg.Sort)
		for _, res := range results {
			if err := prn.PrintResult(res); err != nil {
				continue
			}
		}
	}

	if err := prn.Finish(&st); err != nil {
		fmt.Fprintf(os.Stderr, "Error finishing printer: %v\n", err)
	}

	if st.TotalMatches() == 0 && len(results) == 0 {
		return 1
	}

	return 0
}

func runFind(ctx context.Context, opts cli.FindOptions) int {
	normalizeFindPositionals(&opts)
	cfg, err := config.NewFind(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 2
	}

	mode, command, err := prepareFindAction(opts, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 2
	}

	findOpts := []ripgo.FindOption{
		ripgo.WithFindGlob(cfg.Find.Matcher.Glob),
		ripgo.WithFindFixedStrings(cfg.Find.Matcher.FixedStrings),
		ripgo.WithFindIgnoreCase(cfg.Find.Matcher.IgnoreCase),
		ripgo.WithFindFullPath(cfg.Find.Matcher.FullPath),
		ripgo.WithFindTypes(cfg.Find.Types...),
		ripgo.WithFindExtensions(cfg.Find.Extensions...),
		ripgo.WithFindMinSize(cfg.Find.MinSize),
		ripgo.WithFindMinDepth(cfg.Find.Walk.MinDepth),
		ripgo.WithFindFollowSymlinks(cfg.Find.Walk.FollowSymlinks),
		ripgo.WithFindHidden(cfg.Find.Ignore.Hidden),
		ripgo.WithFindNoIgnore(cfg.Find.Ignore.NoIgnore),
		ripgo.WithFindThreads(cfg.Find.Walk.Threads),
	}
	if cfg.Find.MaxSizeSet {
		findOpts = append(findOpts, ripgo.WithFindMaxSize(cfg.Find.MaxSize))
	}
	if cfg.Find.Walk.MaxDepthSet {
		findOpts = append(findOpts, ripgo.WithFindMaxDepth(cfg.Find.Walk.MaxDepth))
	}

	if mode != findActionNone {
		return runFindAction(ctx, cfg, findOpts, mode, command, opts.ExecBatchSize)
	}

	pathPrinter := printer.NewPathPrinter(printer.PathConfig{
		Writer:   bufio.NewWriterSize(os.Stdout, 64*1024),
		Absolute: cfg.Absolute,
		Null:     cfg.Print0,
		Color:    cfg.Color,
	})
	var results []findpkg.Result
	found := false
	hadError := false
	for result, err := range ripgo.Find(ctx, cfg.Pattern, cfg.Paths, findOpts...) {
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error finding paths: %v\n", err)
			hadError = true
			continue
		}
		found = true
		if cfg.Sort == "path" {
			results = append(results, result)
			continue
		}
		if err := pathPrinter.PrintResult(result); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing paths: %v\n", err)
			return 2
		}
	}

	if cfg.Sort == "path" {
		slices.SortFunc(results, func(a, b findpkg.Result) int {
			return strings.Compare(a.Path, b.Path)
		})
		for _, result := range results {
			if err := pathPrinter.PrintResult(result); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing paths: %v\n", err)
				return 2
			}
		}
	}
	if err := pathPrinter.Finish(); err != nil {
		fmt.Fprintf(os.Stderr, "Error finishing path printer: %v\n", err)
		return 2
	}
	if hadError {
		return 2
	}
	if !found {
		return 1
	}
	return 0
}

type findActionMode uint8

const (
	findActionNone findActionMode = iota
	findActionExec
	findActionExecBatch
	findActionDelete
	findActionDeleteDryRun
)

func prepareFindAction(opts cli.FindOptions, cfg *config.FindConfig) (findActionMode, *action.Template, error) {
	if opts.Exec != "" && opts.ExecBatch != "" {
		return findActionNone, nil, fmt.Errorf("--exec and --exec-batch are mutually exclusive")
	}
	if opts.Delete && (opts.Exec != "" || opts.ExecBatch != "") {
		return findActionNone, nil, fmt.Errorf("--delete cannot be combined with --exec or --exec-batch")
	}
	if opts.DryRun && !opts.Delete {
		return findActionNone, nil, fmt.Errorf("--dry-run requires --delete")
	}
	if opts.Delete && opts.Print0 && !opts.DryRun {
		return findActionNone, nil, fmt.Errorf("--print0 is only valid with --delete when --dry-run is set")
	}

	mode := findActionNone
	var command *action.Template
	if opts.Exec != "" {
		if cfg.Sort != "" && cfg.Sort != "none" {
			return findActionNone, nil, fmt.Errorf("--sort cannot be combined with --exec")
		}
		if opts.Print0 {
			return findActionNone, nil, fmt.Errorf("--print0 cannot be combined with --exec")
		}
		parsed, err := action.Parse(opts.Exec, false)
		if err != nil {
			return findActionNone, nil, err
		}
		mode, command = findActionExec, &parsed
	}
	if opts.ExecBatch != "" {
		if cfg.Sort != "" && cfg.Sort != "none" {
			return findActionNone, nil, fmt.Errorf("--sort cannot be combined with --exec-batch")
		}
		if opts.Print0 {
			return findActionNone, nil, fmt.Errorf("--print0 cannot be combined with --exec-batch")
		}
		if opts.ExecBatchSize <= 0 || opts.ExecBatchSize > 10_000 {
			return findActionNone, nil, fmt.Errorf("--exec-batch-size must be between 1 and 10000")
		}
		parsed, err := action.Parse(opts.ExecBatch, true)
		if err != nil {
			return findActionNone, nil, err
		}
		mode, command = findActionExecBatch, &parsed
	}
	if opts.Delete {
		if cfg.Sort != "" && cfg.Sort != "none" {
			return findActionNone, nil, fmt.Errorf("--sort cannot be combined with --delete")
		}
		if err := action.ValidateDeleteTypes(cfg.Find.Types); err != nil {
			return findActionNone, nil, err
		}
		if opts.DryRun {
			mode = findActionDeleteDryRun
		} else {
			mode = findActionDelete
		}
	}
	return mode, command, nil
}

func runFindAction(ctx context.Context, cfg *config.FindConfig, findOpts []ripgo.FindOption, mode findActionMode, command *action.Template, batchSize int) int {
	var pathPrinter *printer.PathPrinter
	if mode == findActionDeleteDryRun {
		pathPrinter = printer.NewPathPrinter(printer.PathConfig{
			Writer:   os.Stdout,
			Absolute: cfg.Absolute,
			Null:     cfg.Print0,
			Color:    cfg.Color,
		})
	}

	found := false
	hadError := false
	batch := make([]string, 0, batchSize)
	flushBatch := func() error {
		if len(batch) == 0 {
			return nil
		}
		err := command.RunBatch(ctx, batch, os.Stdout, os.Stderr)
		batch = batch[:0]
		return err
	}

	for result, err := range ripgo.Find(ctx, cfg.Pattern, cfg.Paths, findOpts...) {
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error finding paths: %v\n", err)
			hadError = true
			continue
		}
		found = true

		actionResult := result
		if cfg.Absolute {
			absolute, err := filepath.Abs(result.Path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error resolving path %q: %v\n", result.Path, err)
				return 2
			}
			actionResult.Path = absolute
		}

		switch mode {
		case findActionExec:
			if err := command.Run(ctx, actionResult.Path, os.Stdout, os.Stderr); err != nil {
				fmt.Fprintf(os.Stderr, "Error executing action for %q: %v\n", actionResult.Path, err)
				return 2
			}
		case findActionExecBatch:
			batch = append(batch, actionResult.Path)
			if len(batch) == batchSize {
				if err := flushBatch(); err != nil {
					fmt.Fprintf(os.Stderr, "Error executing batch action: %v\n", err)
					return 2
				}
			}
		case findActionDelete:
			if err := action.Delete(ctx, actionResult); err != nil {
				fmt.Fprintf(os.Stderr, "Error deleting %q: %v\n", actionResult.Path, err)
				return 2
			}
		case findActionDeleteDryRun:
			if err := pathPrinter.PrintResult(actionResult); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing paths: %v\n", err)
				return 2
			}
		}
	}

	if mode == findActionExecBatch {
		if err := flushBatch(); err != nil {
			fmt.Fprintf(os.Stderr, "Error executing batch action: %v\n", err)
			return 2
		}
	}
	if pathPrinter != nil {
		if err := pathPrinter.Finish(); err != nil {
			fmt.Fprintf(os.Stderr, "Error finishing path printer: %v\n", err)
			return 2
		}
	}
	if ctx.Err() != nil {
		return 2
	}
	if hadError {
		return 2
	}
	if !found {
		return 1
	}
	return 0
}

func normalizeFindPositionals(opts *cli.FindOptions) {
	if opts.Pattern == "" {
		return
	}
	if _, err := os.Lstat(opts.Pattern); err != nil {
		return
	}
	opts.Paths = append([]string{opts.Pattern}, opts.Paths...)
	opts.Pattern = ""
}

func sortResults(results []search.Result, mode string) {
	switch mode {
	case "path":
		slices.SortFunc(results, func(a, b search.Result) int {
			return strings.Compare(a.Path, b.Path)
		})
	case "modified":
		slices.SortFunc(results, func(a, b search.Result) int {
			return a.ModTime.Compare(b.ModTime)
		})
	case "created":
		slices.SortFunc(results, func(a, b search.Result) int {
			return a.CreatedAt.Compare(b.CreatedAt)
		})
	case "accessed":
		slices.SortFunc(results, func(a, b search.Result) int {
			return a.AccessedAt.Compare(b.AccessedAt)
		})
	}
}

func newPrinter(cfg *config.Config) printer.Printer {
	var stdoutWriter io.Writer = os.Stdout
	if bw := bufio.NewWriterSize(os.Stdout, 64*1024); bw != nil {
		stdoutWriter = bw
	}

	switch cfg.OutputMode() {
	case config.OutputJSON:
		return printer.NewJSONPrinter(stdoutWriter)
	case config.OutputCount:
		return printer.NewCountPrinter(stdoutWriter)
	case config.OutputFiles:
		return printer.NewFilesPrinter(stdoutWriter)
	case config.OutputQuiet:
		return discardPrinter{}
	default:
		return printer.NewTextPrinter(printer.TextConfig{
			Writer:     stdoutWriter,
			LineNumber: cfg.LineNumber,
			Column:     cfg.Column,
			Heading:    cfg.Heading,
			Color:      cfg.Color,
			MaxColumns: cfg.MaxColumns,
		})
	}
}

// discardPrinter silently consumes results (for -q / --quiet).
type discardPrinter struct{}

func (discardPrinter) PrintResult(search.Result) error { return nil }
func (discardPrinter) Finish(*stats.Stats) error       { return nil }

var _ printer.Printer = discardPrinter{}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(run(ctx))
}
