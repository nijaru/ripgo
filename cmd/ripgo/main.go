package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/nijaru/ripgo"
	findpkg "github.com/nijaru/ripgo/find"
	"github.com/nijaru/ripgo/ignore"
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
		return 1
	}
	if _, err := parser.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return runSearch(ctx, opts)
}

func runSearch(ctx context.Context, opts cli.Options) int {
	cfg, err := config.New(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
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
	cfg, err := config.NewFind(opts)
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

	pathPrinter := printer.NewPathPrinter(printer.PathConfig{
		Writer:   os.Stdout,
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
	switch cfg.OutputMode() {
	case config.OutputJSON:
		return printer.NewJSONPrinter(os.Stdout)
	case config.OutputCount:
		return printer.NewCountPrinter(os.Stdout)
	case config.OutputFiles:
		return printer.NewFilesPrinter(os.Stdout)
	case config.OutputQuiet:
		return discardPrinter{}
	default:
		return printer.NewTextPrinter(printer.TextConfig{
			Writer:     os.Stdout,
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
