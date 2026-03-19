package printer

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/nijaru/ripgo/search"
	"github.com/nijaru/ripgo/stats"
)

// --- TextPrinter ---

func TestTextPrinterBasic(t *testing.T) {
	var buf bytes.Buffer
	p := NewTextPrinter(TextConfig{Writer: &buf})

	r := search.Result{
		Path: "file.txt",
		Matches: []search.Match{
			{Line: 1, Column: 5, LineBytes: []byte("hello world")},
		},
	}
	if err := p.PrintResult(r); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	want := "file.txt:hello world\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTextPrinterWithLineNumber(t *testing.T) {
	var buf bytes.Buffer
	p := NewTextPrinter(TextConfig{Writer: &buf, LineNumber: true})

	r := search.Result{
		Path: "main.go",
		Matches: []search.Match{
			{Line: 42, Column: 1, LineBytes: []byte("func main() {")},
		},
	}
	if err := p.PrintResult(r); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	want := "main.go:42:func main() {\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTextPrinterWithColumn(t *testing.T) {
	var buf bytes.Buffer
	p := NewTextPrinter(TextConfig{Writer: &buf, LineNumber: true, Column: true})

	r := search.Result{
		Path: "a.txt",
		Matches: []search.Match{
			{Line: 3, Column: 10, LineBytes: []byte("find it here")},
		},
	}
	if err := p.PrintResult(r); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	want := "a.txt:3:10:find it here\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTextPrinterEmptyResult(t *testing.T) {
	var buf bytes.Buffer
	p := NewTextPrinter(TextConfig{Writer: &buf})

	r := search.Result{Path: "empty.txt"}
	if err := p.PrintResult(r); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty output, got %q", buf.String())
	}
}

func TestTextPrinterMultipleMatches(t *testing.T) {
	var buf bytes.Buffer
	p := NewTextPrinter(TextConfig{Writer: &buf, LineNumber: true})

	r := search.Result{
		Path: "multi.txt",
		Matches: []search.Match{
			{Line: 1, Column: 1, LineBytes: []byte("alpha beta")},
			{Line: 3, Column: 1, LineBytes: []byte("gamma delta")},
			{Line: 5, Column: 1, LineBytes: []byte("epsilon beta")},
		},
	}
	if err := p.PrintResult(r); err != nil {
		t.Fatal(err)
	}
	want := "multi.txt:1:alpha beta\nmulti.txt:3:gamma delta\nmulti.txt:5:epsilon beta\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

// --- Entries (context lines) ---

func TestTextPrinterEntriesWithContext(t *testing.T) {
	var buf bytes.Buffer
	p := NewTextPrinter(TextConfig{Writer: &buf, LineNumber: true})

	r := search.Result{
		Path: "ctx.txt",
		Entries: []search.Entry{
			{Kind: search.EntryContext, Line: 1, LineBytes: []byte("before line")},
			{Kind: search.EntryMatch, Line: 2, LineBytes: []byte("match line"), Column: 1},
			{Kind: search.EntryContext, Line: 3, LineBytes: []byte("after line")},
		},
	}
	if err := p.PrintResult(r); err != nil {
		t.Fatal(err)
	}
	want := "ctx.txt-1-before line\nctx.txt:2:match line\nctx.txt-3-after line\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestTextPrinterGroupSeparator(t *testing.T) {
	var buf bytes.Buffer
	p := NewTextPrinter(TextConfig{Writer: &buf, LineNumber: true})

	r := search.Result{
		Path: "sep.txt",
		Entries: []search.Entry{
			{Kind: search.EntryMatch, Line: 2, LineBytes: []byte("first match"), Column: 1},
			{Kind: search.EntryMatch, Line: 10, LineBytes: []byte("second match"), Column: 1},
		},
	}
	if err := p.PrintResult(r); err != nil {
		t.Fatal(err)
	}
	want := "sep.txt:2:first match\n--\nsep.txt:10:second match\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestTextPrinterEntriesPreferOverMatches(t *testing.T) {
	var buf bytes.Buffer
	p := NewTextPrinter(TextConfig{Writer: &buf, LineNumber: true})

	r := search.Result{
		Path: "prefer.txt",
		Matches: []search.Match{
			{Line: 1, Column: 1, LineBytes: []byte("from matches")},
		},
		Entries: []search.Entry{
			{Kind: search.EntryMatch, Line: 2, LineBytes: []byte("from entries"), Column: 1},
		},
	}
	if err := p.PrintResult(r); err != nil {
		t.Fatal(err)
	}
	// When entries exist, matches are ignored
	want := "prefer.txt:2:from entries\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

// --- CountPrinter ---

func TestCountPrinter(t *testing.T) {
	var buf bytes.Buffer
	p := NewCountPrinter(&buf)

	r := search.Result{
		Path: "file.txt",
		Matches: []search.Match{
			{Line: 1}, {Line: 3}, {Line: 7},
		},
	}
	if err := p.PrintResult(r); err != nil {
		t.Fatal(err)
	}
	want := "file.txt:3\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestCountPrinterZeroMatches(t *testing.T) {
	var buf bytes.Buffer
	p := NewCountPrinter(&buf)

	r := search.Result{Path: "empty.txt"}
	if err := p.PrintResult(r); err != nil {
		t.Fatal(err)
	}
	want := "empty.txt:0\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestCountPrinterFinish(t *testing.T) {
	var buf bytes.Buffer
	p := NewCountPrinter(&buf)

	if err := p.Finish(stats.Stats{}); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Errorf("Finish should produce no output, got %q", buf.String())
	}
}

// --- FilesPrinter ---

func TestFilesPrinter(t *testing.T) {
	var buf bytes.Buffer
	p := NewFilesPrinter(&buf)

	r := search.Result{
		Path:    "main.go",
		Matches: []search.Match{{Line: 1}},
	}
	if err := p.PrintResult(r); err != nil {
		t.Fatal(err)
	}
	want := "main.go\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestFilesPrinterDedup(t *testing.T) {
	var buf bytes.Buffer
	p := NewFilesPrinter(&buf)

	r1 := search.Result{Path: "a.go", Matches: []search.Match{{Line: 1}}}
	r2 := search.Result{Path: "b.go", Matches: []search.Match{{Line: 1}}}
	r3 := search.Result{Path: "a.go", Matches: []search.Match{{Line: 2}}} // duplicate

	p.PrintResult(r1)
	p.PrintResult(r2)
	p.PrintResult(r3)

	want := "a.go\nb.go\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestFilesPrinterNoMatches(t *testing.T) {
	var buf bytes.Buffer
	p := NewFilesPrinter(&buf)

	r := search.Result{Path: "empty.txt"}
	if err := p.PrintResult(r); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output for file with no matches, got %q", buf.String())
	}
}

// --- JSONPrinter ---

func TestJSONPrinter(t *testing.T) {
	var buf bytes.Buffer
	p := NewJSONPrinter(&buf)

	r1 := search.Result{
		Path: "a.go",
		Matches: []search.Match{
			{Line: 1, Column: 5, LineBytes: []byte("hello")},
		},
	}
	r2 := search.Result{
		Path: "b.go",
		Matches: []search.Match{
			{Line: 3, Column: 1, LineBytes: []byte("world")},
		},
	}
	if err := p.PrintResult(r1); err != nil {
		t.Fatal(err)
	}
	if err := p.PrintResult(r2); err != nil {
		t.Fatal(err)
	}

	if err := p.Finish(stats.Stats{}); err != nil {
		t.Fatal(err)
	}

	// Verify JSON output
	var out []search.Result
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("failed to unmarshal JSON output: %v", err)
	}
	if len(out) != 2 || out[0].Path != "a.go" || out[1].Path != "b.go" {
		t.Errorf("unexpected unmarshaled results: %+v", out)
	}
}

func TestJSONPrinterResultContent(t *testing.T) {
	var buf bytes.Buffer
	p := NewJSONPrinter(&buf)

	r := search.Result{
		Path: "test.go",
		Matches: []search.Match{
			{Line: 10, Column: 3, LineBytes: []byte("package main")},
		},
	}
	if err := p.PrintResult(r); err != nil {
		t.Fatal(err)
	}

	if err := p.Finish(stats.Stats{}); err != nil {
		t.Fatal(err)
	}

	var out []search.Result
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Path != "test.go" {
		t.Errorf("unexpected unmarshaled result: %+v", out)
	}
}

func TestJSONPrinterEmpty(t *testing.T) {
	var buf bytes.Buffer
	p := NewJSONPrinter(&buf)

	if err := p.Finish(stats.Stats{}); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if got != "[]" {
		t.Errorf("expected empty array [], got %q", got)
	}
}
