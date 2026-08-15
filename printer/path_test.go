package printer

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/nijaru/ripgo/find"
)

func TestPathPrinter(t *testing.T) {
	tests := []struct {
		name string
		cfg  PathConfig
		want string
	}{
		{name: "newline", want: "one.txt\ntwo.txt\n"},
		{name: "null", cfg: PathConfig{Null: true}, want: "one.txt\x00two.txt\x00"},
		{name: "color", cfg: PathConfig{Color: true}, want: "\033[35mone.txt\033[0m\n\033[35mtwo.txt\033[0m\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			tt.cfg.Writer = &buf
			printer := NewPathPrinter(tt.cfg)
			if err := printer.PrintResult(find.Result{Path: "one.txt"}); err != nil {
				t.Fatal(err)
			}
			if err := printer.PrintResult(find.Result{Path: "two.txt"}); err != nil {
				t.Fatal(err)
			}
			if err := printer.Finish(); err != nil {
				t.Fatal(err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("output = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPathPrinterAbsolute(t *testing.T) {
	var buf bytes.Buffer
	printer := NewPathPrinter(PathConfig{Writer: &buf, Absolute: true})
	if err := printer.PrintResult(find.Result{Path: "relative/file.txt"}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(buf.String(), "/relative/file.txt\n") {
		t.Fatalf("absolute output = %q, want absolute suffix", buf.String())
	}
	if strings.HasPrefix(buf.String(), "relative/") {
		t.Fatalf("absolute output remained relative: %q", buf.String())
	}
}

type failingWriter struct {
	err error
}

func (w failingWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func TestPathPrinterWriteError(t *testing.T) {
	want := errors.New("write failed")
	printer := NewPathPrinter(PathConfig{Writer: failingWriter{err: want}})
	if err := printer.PrintResult(find.Result{Path: "file.txt"}); !errors.Is(err, want) {
		t.Fatalf("PrintResult() error = %v, want %v", err, want)
	}
}

func TestWriteAllShortWrite(t *testing.T) {
	if err := writeAll(shortWriter{}, []byte("data")); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("writeAll() error = %v, want %v", err, io.ErrShortWrite)
	}
}

type shortWriter struct{}

func (shortWriter) Write([]byte) (int, error) { return 0, nil }
