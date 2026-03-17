package main

import (
	"context"
	"io"
	"os"
	"os/signal"
	"sync"

	"github.com/alecthomas/kong"

	"github.com/nijaru/ripgo/ignore"
	"github.com/nijaru/ripgo/internal/cli"
	"github.com/nijaru/ripgo/internal/config"
	"github.com/nijaru/ripgo/pattern"
	"github.com/nijaru/ripgo/printer"
	"github.com/nijaru/ripgo/search"
	"github.com/nijaru/ripgo/stats"
	"github.com/nijaru/ripgo/walk"
)

func run(ctx context.Context) int {
	var opts cli.Options
	kong.Parse(&opts)

	cfg, err := config.New(opts)
	if err != nil {
		os.Stderr.WriteString("Error: " + err.Error() + "\n")
		return 1
	}

	matcher, err := pattern.New(cfg.Pattern)
	if err != nil {
		os.Stderr.WriteString("Error: " + err.Error() + "\n")
		return 1
	}

	ignoreEngine, err := ignore.NewEngine(cfg.Ignore)
	if err != nil {
		os.Stderr.WriteString("Error: " + err.Error() + "\n")
		return 1
	}

	w := walk.NewWalker(cfg.Walk, ignoreEngine)
	searcher := search.NewSearcher(cfg.Search, matcher)
	prn := newPrinter(cfg)
	var st stats.Stats

	fileCh := make(chan string, 1024)
	resultCh := make(chan search.Result, 1024)

	var scanWg sync.WaitGroup

	// walker
	scanWg.Add(1)
	go func() {
		defer scanWg.Done()
		w.Run(ctx, cfg.Paths, fileCh)
	}()

	// scanners
	for range cfg.Threads {
		scanWg.Add(1)
		go func() {
			defer scanWg.Done()
			for path := range fileCh {
				result, err := searcher.Search(path)
				if err != nil {
					continue
				}
				if len(result.Matches) > 0 {
					select {
					case <-ctx.Done():
						return
					case resultCh <- result:
					}
				}
			}
		}()
	}

	go func() {
		scanWg.Wait()
		close(resultCh)
	}()

	// printer
	for result := range resultCh {
		if err := prn.PrintResult(result); err != nil {
			continue
		}
		st.RecordMatch(result)
	}

	if err := prn.Finish(st); err != nil {
		os.Stderr.WriteString("Error: " + err.Error() + "\n")
	}

	return exitCode(cfg, st)
}

func newPrinter(cfg *config.Config) printer.Printer {
	switch cfg.OutputMode() {
	case config.OutputJSON:
		return printer.NewJSONPrinter()
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
		})
	}
}

func exitCode(cfg *config.Config, st stats.Stats) int {
	if st.TotalMatches() > 0 {
		return 0
	}
	return 1
}

// discardPrinter silently consumes results (for -q / --quiet).
type discardPrinter struct{}

func (discardPrinter) PrintResult(search.Result) error { return nil }
func (discardPrinter) Finish(stats.Stats) error        { return nil }

var _ printer.Printer = discardPrinter{}
var _ io.Writer = discardPrinter{}

func (discardPrinter) Write(p []byte) (int, error) { return len(p), nil }

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(run(ctx))
}
