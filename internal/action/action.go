// Package action implements the private, opt-in side effects of finder mode.
//
// Finder itself remains a read-only stream. This package owns command process
// lifetimes and deletion so the CLI can make those effects explicit.
package action

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"

	findpkg "github.com/nijaru/ripgo/find"
	"github.com/nijaru/ripgo/walk"
)

// Template is a parsed, shell-free command template.
type Template struct {
	args  []string
	batch bool
}

// Parse splits a command template without invoking a shell. Quoting and
// backslash escaping are accepted only to preserve argument boundaries; no
// expansion, redirection, or shell syntax is interpreted.
func Parse(command string, batch bool) (Template, error) {
	args, err := split(command)
	if err != nil {
		return Template{}, err
	}
	if len(args) == 0 {
		return Template{}, fmt.Errorf("action command must not be empty")
	}

	placeholderCount := 0
	for _, arg := range args {
		placeholderCount += strings.Count(arg, "{}")
	}
	if placeholderCount == 0 {
		return Template{}, fmt.Errorf("action command must contain the {} placeholder")
	}

	if batch {
		standalone := 0
		for _, arg := range args {
			if strings.Contains(arg, "{}") {
				if arg != "{}" {
					return Template{}, fmt.Errorf("batch action requires {} to be a standalone argument")
				}
				standalone++
			}
			for _, placeholder := range []string{"{/}", "{//}", "{.}", "{/.}"} {
				if strings.Contains(arg, placeholder) {
					return Template{}, fmt.Errorf("batch action does not support %s; use {} as a standalone argument", placeholder)
				}
			}
		}
		if standalone != 1 {
			return Template{}, fmt.Errorf("batch action must contain exactly one standalone {} placeholder")
		}
	}

	return Template{args: args, batch: batch}, nil
}

// Run executes the template once for path. The command is not run through a
// shell, and stdout/stderr remain owned by the CLI caller.
func (t Template) Run(ctx context.Context, path string, stdout, stderr io.Writer) error {
	if t.batch {
		return fmt.Errorf("cannot run a batch action as a single path")
	}
	return run(ctx, t.argsForPath(path), stdout, stderr)
}

// RunBatch executes one bounded batch. Each standalone {} argument expands to
// one argv element per path.
func (t Template) RunBatch(ctx context.Context, paths []string, stdout, stderr io.Writer) error {
	if !t.batch {
		return fmt.Errorf("cannot run a per-match action as a batch")
	}
	if len(paths) == 0 {
		return nil
	}

	args := make([]string, 0, len(t.args)+len(paths))
	for _, arg := range t.args {
		if arg == "{}" {
			args = append(args, paths...)
			continue
		}
		args = append(args, arg)
	}
	return run(ctx, args, stdout, stderr)
}

func (t Template) argsForPath(path string) []string {
	args := make([]string, len(t.args))
	for i, arg := range t.args {
		args[i] = replace(arg, path)
	}
	return args
}

func replace(arg, path string) string {
	// Replace the longer tokens first so each placeholder is unambiguous.
	replacements := [...]struct{ old, new string }{
		{"{//}", filepath.Dir(path)},
		{"{/.}", trimExtension(filepath.Base(path))},
		{"{/}", filepath.Base(path)},
		{"{.}", trimExtension(path)},
		{"{}", path},
	}
	for _, item := range replacements {
		arg = strings.ReplaceAll(arg, item.old, item.new)
	}
	return arg
}

func trimExtension(path string) string {
	ext := filepath.Ext(path)
	if ext == "" || ext == "." {
		return path
	}
	return strings.TrimSuffix(path, ext)
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "" {
		return fmt.Errorf("action command has no executable")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("run %q: %w", args[0], err)
	}
	return nil
}

// Delete removes the matched directory entry without following a symlink.
// Real directories are refused; callers must select only files or symlinks.
func Delete(ctx context.Context, result findpkg.Result) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if result.Path == "" {
		return fmt.Errorf("cannot delete an empty path")
	}
	info, err := os.Lstat(result.Path)
	if err != nil {
		return fmt.Errorf("inspect %q before delete: %w", result.Path, err)
	}
	if info.IsDir() || (result.Kind == walk.EntryDirectory && !result.Symlink) {
		return fmt.Errorf("refusing to delete directory %q", result.Path)
	}
	if err := os.Remove(result.Path); err != nil {
		return fmt.Errorf("delete %q: %w", result.Path, err)
	}
	return nil
}

// ValidateDeleteTypes enforces the non-recursive deletion contract before the
// finder starts producing results.
func ValidateDeleteTypes(types []findpkg.Type) error {
	if len(types) == 0 {
		return fmt.Errorf("--delete requires --type f or --type l")
	}
	for _, typ := range types {
		if typ != findpkg.TypeFile && typ != findpkg.TypeSymlink {
			return fmt.Errorf("--delete cannot be used with directory results")
		}
	}
	return nil
}

func split(command string) ([]string, error) {
	var args []string
	var token strings.Builder
	inSingle, inDouble, escaped, tokenStarted := false, false, false, false

	flush := func() {
		if tokenStarted {
			args = append(args, token.String())
			token.Reset()
			tokenStarted = false
		}
	}

	for _, r := range command {
		if r == 0 {
			return nil, fmt.Errorf("action command contains NUL")
		}
		if escaped {
			token.WriteRune(r)
			escaped = false
			tokenStarted = true
			continue
		}
		if inSingle {
			if r == '\'' {
				inSingle = false
			} else {
				token.WriteRune(r)
			}
			tokenStarted = true
			continue
		}
		if inDouble {
			switch r {
			case '"':
				inDouble = false
			case '\\':
				escaped = true
			default:
				token.WriteRune(r)
			}
			tokenStarted = true
			continue
		}

		switch {
		case r == '\\':
			escaped = true
			tokenStarted = true
		case r == '\'':
			inSingle = true
			tokenStarted = true
		case r == '"':
			inDouble = true
			tokenStarted = true
		case unicode.IsSpace(r):
			flush()
		default:
			token.WriteRune(r)
			tokenStarted = true
		}
	}

	if escaped {
		return nil, fmt.Errorf("action command ends with an escape")
	}
	if inSingle || inDouble {
		return nil, fmt.Errorf("action command has an unterminated quote")
	}
	flush()
	return args, nil
}
