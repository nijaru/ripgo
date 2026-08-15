package printer

import (
	"io"
	"os"
	"path/filepath"

	"github.com/nijaru/ripgo/find"
)

// PathConfig controls finder path output.
type PathConfig struct {
	// Writer is the output destination. Defaults to os.Stdout.
	Writer io.Writer
	// Absolute converts relative result paths to absolute paths.
	Absolute bool
	// Null terminates each path with NUL instead of newline.
	Null bool
	// Color wraps paths in ANSI color escapes.
	Color bool
}

// PathPrinter writes one finder result path per record.
type PathPrinter struct {
	w        io.Writer
	absolute bool
	null     bool
	color    bool
}

// NewPathPrinter creates a finder path printer.
func NewPathPrinter(cfg PathConfig) *PathPrinter {
	w := cfg.Writer
	if w == nil {
		w = os.Stdout
	}
	return &PathPrinter{
		w:        w,
		absolute: cfg.Absolute,
		null:     cfg.Null,
		color:    cfg.Color,
	}
}

// PrintResult writes a matched path.
func (p *PathPrinter) PrintResult(result find.Result) error {
	path := result.Path
	if p.absolute {
		var err error
		path, err = filepath.Abs(path)
		if err != nil {
			return err
		}
	}
	if p.color {
		path = colorize(path, colorPath, true)
	}

	terminator := byte('\n')
	if p.null {
		terminator = 0
	}
	if err := writeAll(p.w, []byte(path)); err != nil {
		return err
	}
	return writeAll(p.w, []byte{terminator})
}

// Finish flushes any buffered output.
func (p *PathPrinter) Finish() error {
	return flushWriter(p.w)
}

func writeAll(w io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := w.Write(data)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}
