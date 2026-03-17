package printer

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/nijaru/ripgo/search"
	"github.com/nijaru/ripgo/stats"
)

// Printer outputs search results.
type Printer interface {
	// PrintResult prints a single file's results.
	PrintResult(search.Result) error
	// Finish is called after all results have been printed.
	Finish(stats.Stats) error
}

// TextPrinter outputs results in the default format.
type TextPrinter struct {
	w          io.Writer
	lineNumber bool
	column     bool
}

// TextConfig holds text printer options.
type TextConfig struct {
	// Writer is the output destination. Defaults to os.Stdout.
	Writer io.Writer
	// LineNumber includes line numbers in output.
	LineNumber bool
	// Column includes column offsets in output.
	Column bool
}

// NewTextPrinter creates a text printer.
func NewTextPrinter(cfg TextConfig) *TextPrinter {
	if cfg.Writer == nil {
		cfg.Writer = os.Stdout
	}
	return &TextPrinter{
		w:          cfg.Writer,
		lineNumber: cfg.LineNumber,
		column:     cfg.Column,
	}
}

func (p *TextPrinter) PrintResult(r search.Result) error {
	if len(r.Matches) == 0 {
		return nil
	}

	for _, m := range r.Matches {
		line := string(m.LineBytes)
		switch {
		case p.lineNumber && p.column:
			fmt.Fprintf(p.w, "%s:%d:%d:%s\n", r.Path, m.Line, m.Column, line)
		case p.lineNumber:
			fmt.Fprintf(p.w, "%s:%d:%s\n", r.Path, m.Line, line)
		default:
			fmt.Fprintf(p.w, "%s:%s\n", r.Path, line)
		}
	}
	return nil
}

func (p *TextPrinter) Finish(_ stats.Stats) error {
	return nil
}

// CountPrinter outputs match counts per file.
type CountPrinter struct {
	w io.Writer
}

// NewCountPrinter creates a count printer.
func NewCountPrinter(w io.Writer) *CountPrinter {
	if w == nil {
		w = os.Stdout
	}
	return &CountPrinter{w: w}
}

func (p *CountPrinter) PrintResult(r search.Result) error {
	fmt.Fprintf(p.w, "%s:%d\n", r.Path, len(r.Matches))
	return nil
}

func (p *CountPrinter) Finish(_ stats.Stats) error {
	return nil
}

// FilesPrinter outputs only file paths with matches.
type FilesPrinter struct {
	w    io.Writer
	seen map[string]bool
}

// NewFilesPrinter creates a files-with-matches printer.
func NewFilesPrinter(w io.Writer) *FilesPrinter {
	if w == nil {
		w = os.Stdout
	}
	return &FilesPrinter{w: w, seen: make(map[string]bool)}
}

func (p *FilesPrinter) PrintResult(r search.Result) error {
	if len(r.Matches) > 0 && !p.seen[r.Path] {
		p.seen[r.Path] = true
		fmt.Fprintln(p.w, r.Path)
	}
	return nil
}

func (p *FilesPrinter) Finish(_ stats.Stats) error {
	return nil
}

// JSONPrinter outputs results as JSON.
type JSONPrinter struct {
	results []search.Result
}

// NewJSONPrinter creates a JSON printer.
func NewJSONPrinter() *JSONPrinter {
	return &JSONPrinter{}
}

func (p *JSONPrinter) PrintResult(r search.Result) error {
	p.results = append(p.results, r)
	return nil
}

func (p *JSONPrinter) Finish(_ stats.Stats) error {
	data, err := json.Marshal(p.results)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(data)
	return err
}
