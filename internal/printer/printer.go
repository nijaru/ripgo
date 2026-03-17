package printer

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/nijaru/ripgo/internal/config"
	"github.com/nijaru/ripgo/internal/search"
)

type Printer interface {
	PrintResult(search.Result) error
	Finish() error
}

type TextPrinter struct {
	cfg *config.Config
	w   *os.File
}

func NewPrinter(cfg *config.Config) Printer {
	switch cfg.OutputMode {
	case config.OutputModeJSON:
		return &JSONPrinter{cfg: cfg}
	case config.OutputModeCount:
		return &CountPrinter{cfg: cfg, w: os.Stdout}
	case config.OutputModeFiles:
		return &FilesPrinter{cfg: cfg, seen: make(map[string]bool), w: os.Stdout}
	case config.OutputModeQuiet:
		return &QuietPrinter{cfg: cfg}
	}
	return &TextPrinter{cfg: cfg, w: os.Stdout}
}

func (p *TextPrinter) PrintResult(r search.Result) error {
	if len(r.Matches) == 0 {
		return nil
	}

	for _, m := range r.Matches {
		line := string(m.LineBytes)
		if p.cfg.LineNumber {
			if p.cfg.Column {
				fmt.Fprintf(p.w, "%s:%d:%d:%s\n", r.Path, m.Line, m.Column, line)
			} else {
				fmt.Fprintf(p.w, "%s:%d:%s\n", r.Path, m.Line, line)
			}
		} else {
			fmt.Fprintf(p.w, "%s:%s\n", r.Path, line)
		}
	}
	return nil
}

func (p *TextPrinter) Finish() error {
	return nil
}

type JSONPrinter struct {
	cfg     *config.Config
	results []search.Result
}

func (p *JSONPrinter) PrintResult(r search.Result) error {
	p.results = append(p.results, r)
	return nil
}

func (p *JSONPrinter) Finish() error {
	data, err := json.Marshal(p.results)
	if err != nil {
		return err
	}
	os.Stdout.Write(data)
	return nil
}

type CountPrinter struct {
	cfg *config.Config
	w   *os.File
}

func (p *CountPrinter) PrintResult(r search.Result) error {
	count := len(r.Matches)
	fmt.Fprintf(p.w, "%s:%d\n", r.Path, count)
	return nil
}

func (p *CountPrinter) Finish() error {
	return nil
}

type FilesPrinter struct {
	cfg  *config.Config
	w    *os.File
	seen map[string]bool
}

func (p *FilesPrinter) PrintResult(r search.Result) error {
	if len(r.Matches) > 0 && !p.seen[r.Path] {
		p.seen[r.Path] = true
		fmt.Fprintln(p.w, r.Path)
	}
	return nil
}

func (p *FilesPrinter) Finish() error {
	return nil
}

type QuietPrinter struct {
	cfg *config.Config
}

func (p *QuietPrinter) PrintResult(r search.Result) error {
	if len(r.Matches) > 0 {
		os.Exit(0)
	}
	return nil
}

func (p *QuietPrinter) Finish() error {
	os.Exit(1)
	return nil
}

func isASCII(s []byte) bool {
	for _, c := range s {
		if c > 127 {
			return false
		}
	}
	return true
}

func trimRightSpace(s string) string {
	return strings.TrimRight(s, " \t")
}
