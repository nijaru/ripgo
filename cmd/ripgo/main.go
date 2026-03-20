package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/nijaru/ripgo"
	"github.com/nijaru/ripgo/ignore"
	"github.com/nijaru/ripgo/internal/cli"
	"github.com/nijaru/ripgo/internal/config"
	"github.com/nijaru/ripgo/printer"
	"github.com/nijaru/ripgo/search"
	"github.com/nijaru/ripgo/stats"
)

func run(ctx context.Context) int {
	var opts cli.Options
	kong.Parse(&opts)

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
		// Go's fs.FileInfo doesn't expose birth time; fall back to modification time.
		slices.SortFunc(results, func(a, b search.Result) int {
			return a.ModTime.Compare(b.ModTime)
		})
	case "accessed":
		// Go's fs.FileInfo doesn't expose access time portably; fall back to modification time.
		slices.SortFunc(results, func(a, b search.Result) int {
			return a.ModTime.Compare(b.ModTime)
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
		})
	}
}

// discardPrinter silently consumes results (for -q / --quiet).
type discardPrinter struct{}

func (discardPrinter) PrintResult(search.Result) error { return nil }
func (discardPrinter) Finish(*stats.Stats) error       { return nil }

var _ printer.Printer = discardPrinter{}
var _ io.Writer = discardPrinter{}

func (discardPrinter) Write(p []byte) (int, error) { return len(p), nil }

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(run(ctx))
}
