package main

import (
	"context"
	"os"
	"os/signal"
	"sync"

	"github.com/alecthomas/kong"

	"github.com/nijaru/ripgo/internal/cli"
	"github.com/nijaru/ripgo/internal/config"
	"github.com/nijaru/ripgo/internal/ignore"
	"github.com/nijaru/ripgo/internal/pattern"
	"github.com/nijaru/ripgo/internal/printer"
	"github.com/nijaru/ripgo/internal/search"
	"github.com/nijaru/ripgo/internal/stats"
	"github.com/nijaru/ripgo/internal/walk"
)

func run(ctx context.Context) int {
	var cliOpts cli.Options
	kong.Parse(&cliOpts)

	cfg, err := config.New(cliOpts)
	if err != nil {
		os.Stderr.WriteString("Error: " + err.Error() + "\n")
		return 1
	}

	matcher, err := pattern.NewMatcher(cfg)
	if err != nil {
		os.Stderr.WriteString("Error: " + err.Error() + "\n")
		return 1
	}

	ignoreEngine, err := ignore.NewEngine(cfg)
	if err != nil {
		os.Stderr.WriteString("Error: " + err.Error() + "\n")
		return 1
	}

	w := walk.NewWalker(cfg, ignoreEngine)
	prn := printer.NewPrinter(cfg)
	searcher := search.NewSearcher(cfg, matcher)
	st := stats.NewStats(cfg)

	fileCh := make(chan string, 1024)
	resultCh := make(chan search.Result, 1024)

	var walkWg, scanWg sync.WaitGroup

	// walker goroutine
	walkWg.Add(1)
	go func() {
		defer walkWg.Done()
		w.Run(ctx, cfg.Paths, fileCh)
	}()

	// scanner workers
	scanWg.Add(cfg.Threads)
	for range cfg.Threads {
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

	// close resultCh when all scanners done
	go func() {
		scanWg.Wait()
		close(resultCh)
	}()

	// process results (single goroutine for stable output)
	for result := range resultCh {
		if err := prn.PrintResult(result); err != nil {
			continue
		}
		st.RecordMatch(result)
	}

	return st.ExitCode()
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(run(ctx))
}
