// Package printer implements various output formats for search results.
package printer

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/nijaru/ripgo/search"
	"github.com/nijaru/ripgo/stats"
)

// Printer outputs search results.
type Printer interface {
	// PrintResult prints a single file's results.
	PrintResult(search.Result) error
	// Finish is called after all results have been printed.
	Finish(*stats.Stats) error
}

// TextPrinter outputs results in the default format.
type TextPrinter struct {
	w          io.Writer
	lineNumber bool
	column     bool
	heading    bool
	color      bool
}

// TextConfig holds text printer options.
type TextConfig struct {
	// Writer is the output destination. Defaults to os.Stdout.
	Writer io.Writer
	// LineNumber includes line numbers in output.
	LineNumber bool
	// Column includes column offsets in output.
	Column bool
	// Heading shows filename once per group of matches.
	Heading bool
	// Color enables colorized output.
	Color bool
}

const (
	colorReset  = "\033[0m"
	colorBold   = "\033[1m"
	colorPath   = "\033[35m" // Magenta
	colorLine   = "\033[32m" // Green
	colorMatch  = "\033[1;31m" // Bold Red
	colorSep    = "\033[36m" // Cyan
)

func colorize(s string, color string, enabled bool) string {
	if !enabled {
		return s
	}
	return color + s + colorReset
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
		heading:    cfg.Heading,
		color:      cfg.Color,
	}
}

// PrintResult outputs search results for a single file.
func (p *TextPrinter) PrintResult(r search.Result) error {
	if len(r.Matches) == 0 && len(r.Entries) == 0 {
		return nil
	}

	if p.heading {
		fmt.Fprintln(p.w, colorize(r.Path, colorPath, p.color))
	}

	// Use entries if available (context lines), otherwise fall back to matches.
	if len(r.Entries) > 0 {
		return p.printEntries(r)
	}

	for _, m := range r.Matches {
		p.printMatchLine(r.Path, m.Line, m.Column, m.LineBytes, m.Submatches)
	}
	return nil
}

// printEntries prints ordered match + context entries with group separators.
func (p *TextPrinter) printEntries(r search.Result) error {
	prevLine := 0
	for _, e := range r.Entries {
		// Insert group separator for non-contiguous lines.
		if prevLine > 0 && e.Line != prevLine+1 {
			fmt.Fprintln(p.w, colorize("--", colorSep, p.color))
		}

		if e.Kind == search.EntryMatch {
			// If we have a column but no submatches (common in context mode),
			// synthesize a single submatch for highlighting.
			var sub [][2]int
			if e.Column > 0 {
				// We don't know the exact end of the match here from Entry alone,
				// but for TextPrinter we can't easily get it without refactoring Entry.
				// For now, we skip highlighting in context-mode match lines or refactor Entry.
			}
			p.printMatchLine(r.Path, e.Line, e.Column, e.LineBytes, sub)
		} else {
			p.printContextLine(r.Path, e.Line, string(e.LineBytes))
		}
		prevLine = e.Line
	}
	return nil
}

// formatMatch highlights matches within a line using ANSI escapes.
func (p *TextPrinter) formatMatch(line []byte, submatches [][2]int) string {
	if !p.color || len(submatches) == 0 {
		return string(line)
	}

	var sb strings.Builder
	last := 0
	
	// Only highlight the full match (index 0) to avoid issues with
	// overlapping or nested capture groups.
	full := submatches[0]
	if full[0] < 0 || full[0] > len(line) || full[1] > len(line) || full[0] > full[1] {
		return string(line)
	}

	sb.Write(line[last:full[0]])
	sb.WriteString(colorMatch)
	sb.Write(line[full[0]:full[1]])
	sb.WriteString(colorReset)
	sb.Write(line[full[1]:])
	
	return sb.String()
}

// printMatchLine prints a line with a match (colon separator).
func (p *TextPrinter) printMatchLine(path string, line, col int, content []byte, sub [][2]int) {
	sep := colorize(":", colorSep, p.color)
	pathStr := colorize(path, colorPath, p.color)
	lineStr := colorize(fmt.Sprint(line), colorLine, p.color)
	colStr := colorize(fmt.Sprint(col), colorLine, p.color)
	formatted := p.formatMatch(content, sub)

	if p.heading {
		switch {
		case p.lineNumber && p.column:
			fmt.Fprintf(p.w, "%s%s%s%s%s\n", lineStr, sep, colStr, sep, formatted)
		case p.lineNumber:
			fmt.Fprintf(p.w, "%s%s%s\n", lineStr, sep, formatted)
		default:
			fmt.Fprintf(p.w, "%s\n", formatted)
		}
		return
	}

	switch {
	case p.lineNumber && p.column:
		fmt.Fprintf(p.w, "%s%s%s%s%s%s%s\n", pathStr, sep, lineStr, sep, colStr, sep, formatted)
	case p.lineNumber:
		fmt.Fprintf(p.w, "%s%s%s%s%s\n", pathStr, sep, lineStr, sep, formatted)
	default:
		fmt.Fprintf(p.w, "%s%s%s\n", pathStr, sep, formatted)
	}
}

// printContextLine prints a context line (dash separator).
func (p *TextPrinter) printContextLine(path string, line int, content string) {
	sep := colorize("-", colorSep, p.color)
	pathStr := colorize(path, colorPath, p.color)
	lineStr := colorize(fmt.Sprint(line), colorLine, p.color)

	if p.heading {
		if p.lineNumber {
			fmt.Fprintf(p.w, "%s%s%s\n", lineStr, sep, content)
		} else {
			fmt.Fprintf(p.w, "%s\n", content)
		}
		return
	}

	if p.lineNumber {
		fmt.Fprintf(p.w, "%s%s%s%s%s\n", pathStr, sep, lineStr, sep, content)
	} else {
		fmt.Fprintf(p.w, "%s%s%s\n", pathStr, sep, content)
	}
}

// Finish is a no-op for TextPrinter.
func (p *TextPrinter) Finish(_ *stats.Stats) error {
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
func (p *CountPrinter) Finish(_ *stats.Stats) error {
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
func (p *FilesPrinter) Finish(_ *stats.Stats) error {
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
func (p *JSONPrinter) Finish(_ *stats.Stats) error {
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
