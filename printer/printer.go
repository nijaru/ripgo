// Package printer implements various output formats for search results.
package printer

import (
	"bufio"
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

// PrintResult outputs search results for a single file.
func (p *TextPrinter) PrintResult(r search.Result) error {
	// Use entries if available (context lines), otherwise fall back to matches.
	if len(r.Entries) > 0 {
		return p.printEntries(r)
	}

	if len(r.Matches) == 0 {
		return nil
	}

	for _, m := range r.Matches {
		p.printMatchLine(r.Path, m.Line, m.Column, string(m.LineBytes))
	}
	return nil
}

// printEntries prints ordered match + context entries with group separators.
func (p *TextPrinter) printEntries(r search.Result) error {
	prevLine := 0
	for _, e := range r.Entries {
		// Insert group separator for non-contiguous lines.
		if prevLine > 0 && e.Line != prevLine+1 {
			fmt.Fprintln(p.w, "--")
		}

		if e.Kind == search.EntryMatch {
			p.printMatchLine(r.Path, e.Line, e.Column, string(e.LineBytes))
		} else {
			p.printContextLine(r.Path, e.Line, string(e.LineBytes))
		}
		prevLine = e.Line
	}
	return nil
}

// printMatchLine prints a line with a match (colon separator).
func (p *TextPrinter) printMatchLine(path string, line, col int, content string) {
	switch {
	case p.lineNumber && p.column:
		fmt.Fprintf(p.w, "%s:%d:%d:%s\n", path, line, col, content)
	case p.lineNumber:
		fmt.Fprintf(p.w, "%s:%d:%s\n", path, line, content)
	default:
		fmt.Fprintf(p.w, "%s:%s\n", path, content)
	}
}

// printContextLine prints a context line (dash separator).
func (p *TextPrinter) printContextLine(path string, line int, content string) {
	if p.lineNumber {
		fmt.Fprintf(p.w, "%s-%d-%s\n", path, line, content)
	} else {
		fmt.Fprintf(p.w, "%s-%s\n", path, content)
	}
}

// Finish is a no-op for TextPrinter.
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

// PrintResult outputs the match count for a single file.
func (p *CountPrinter) PrintResult(r search.Result) error {
	fmt.Fprintf(p.w, "%s:%d\n", r.Path, len(r.Matches))
	return nil
}

// Finish is a no-op for CountPrinter.
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

// PrintResult outputs the file path if it has matches and hasn't been printed yet.
func (p *FilesPrinter) PrintResult(r search.Result) error {
	if len(r.Matches) > 0 && !p.seen[r.Path] {
		p.seen[r.Path] = true
		fmt.Fprintln(p.w, r.Path)
	}
	return nil
}

// Finish is a no-op for FilesPrinter.
func (p *FilesPrinter) Finish(_ stats.Stats) error {
	return nil
}

// JSONPrinter outputs results as JSON.
type JSONPrinter struct {
	w     *bufio.Writer
	enc   *json.Encoder
	first bool
}

// NewJSONPrinter creates a JSON printer.
func NewJSONPrinter(w io.Writer) *JSONPrinter {
	if w == nil {
		w = os.Stdout
	}
	return &JSONPrinter{
		w:     bufio.NewWriter(w),
		first: true,
	}
}

// PrintResult streams the result as JSON.
func (p *JSONPrinter) PrintResult(r search.Result) error {
	if p.first {
		if _, err := p.w.Write([]byte("[")); err != nil {
			return err
		}
		p.enc = json.NewEncoder(p.w)
		p.first = false
	} else {
		if _, err := p.w.Write([]byte(",")); err != nil {
			return err
		}
	}
	return p.enc.Encode(r)
}

// Finish writes the closing bracket for the JSON array and flushes.
func (p *JSONPrinter) Finish(_ stats.Stats) error {
	if p.first {
		// No results printed
		if _, err := p.w.Write([]byte("[]")); err != nil {
			return err
		}
	} else {
		if _, err := p.w.Write([]byte("]")); err != nil {
			return err
		}
	}
	return p.w.Flush()
}
