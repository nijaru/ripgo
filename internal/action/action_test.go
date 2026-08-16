package action

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	findpkg "github.com/nijaru/ripgo/find"
	"github.com/nijaru/ripgo/walk"
)

func TestParseAndRunPlaceholders(t *testing.T) {
	template, err := Parse(`printf {} {/} {//} {.} {/.}`, false)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("root", "dir", "file.txt")
	args := template.argsForPath(path)
	want := []string{"printf", path, "file.txt", filepath.Join("root", "dir"), filepath.Join("root", "dir", "file"), "file"}
	if !sameStrings(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}

	quoted, err := Parse(`echo "two words" 'three words' escaped\ value {}`, false)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := quoted.argsForPath("path"), []string{"echo", "two words", "three words", "escaped value", "path"}; !sameStrings(got, want) {
		t.Fatalf("quoted args = %#v, want %#v", got, want)
	}
}

func TestParseRejectsMalformedTemplates(t *testing.T) {
	commands := []string{"", "echo", `echo "unterminated {}`, `echo {}\`, "echo \x00 {}"}
	for _, command := range commands {
		if _, err := Parse(command, false); err == nil {
			t.Errorf("Parse(%q) succeeded, want error", command)
		}
	}
	for _, command := range []string{"echo prefix-{}", "echo {/} {}", "echo {} {}"} {
		if _, err := Parse(command, true); err == nil {
			t.Errorf("Parse(%q, batch) succeeded, want error", command)
		}
	}
}

func TestBatchExpansionAndBoundaries(t *testing.T) {
	template, err := Parse(`printf {} --label`, true)
	if err != nil {
		t.Fatal(err)
	}
	got := template.argsForBatch([]string{"one", "two"})
	want := []string{"printf", "one", "two", "--label"}
	if !sameStrings(got, want) {
		t.Fatalf("batch args = %#v, want %#v", got, want)
	}
}

func TestRunNonZeroAndCancellation(t *testing.T) {
	template, err := Parse(`/bin/sh -c 'exit 7' {}`, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := template.Run(context.Background(), "path", &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
		t.Fatal("non-zero command succeeded")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := template.Run(ctx, "path", &bytes.Buffer{}, &bytes.Buffer{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled command error = %v, want context.Canceled", err)
	}

	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip("requires a POSIX shell for the in-flight cancellation test")
	}
	ctx, cancel = context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	slow, err := Parse(`/bin/sh -c 'sleep 5' {}`, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := slow.Run(ctx, "path", &bytes.Buffer{}, &bytes.Buffer{}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timed-out command error = %v, want context deadline", err)
	}
}

func TestDeleteIsSymlinkSafe(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	link := filepath.Join(root, "link")
	if err := os.WriteFile(target, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	result := findpkg.Result{Path: link, Kind: walk.EntryFile, Symlink: true}
	if err := Delete(context.Background(), result); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("target stat after link deletion: %v", err)
	}
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Fatalf("link still exists or unexpected error: %v", err)
	}
}

func TestDeleteRejectsDirectoriesAndCancellation(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "dir")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := Delete(context.Background(), findpkg.Result{Path: dir, Kind: walk.EntryDirectory}); err == nil {
		t.Fatal("directory deletion succeeded")
	}
	if err := Delete(context.Background(), findpkg.Result{Path: dir, Kind: walk.EntryFile}); err == nil {
		t.Fatal("directory deletion with stale file metadata succeeded")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Delete(ctx, findpkg.Result{Path: filepath.Join(root, "missing"), Kind: walk.EntryFile}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled delete error = %v, want context.Canceled", err)
	}
}

func TestValidateDeleteTypes(t *testing.T) {
	for _, types := range [][]findpkg.Type{nil, {findpkg.TypeDirectory}, {findpkg.TypeFile, findpkg.TypeDirectory}} {
		if err := ValidateDeleteTypes(types); err == nil {
			t.Errorf("ValidateDeleteTypes(%v) succeeded", types)
		}
	}
	if err := ValidateDeleteTypes([]findpkg.Type{findpkg.TypeFile, findpkg.TypeSymlink}); err != nil {
		t.Fatal(err)
	}
}

func (t Template) argsForBatch(paths []string) []string {
	args := make([]string, 0, len(t.args)+len(paths))
	for _, arg := range t.args {
		if arg == "{}" {
			args = append(args, paths...)
		} else {
			args = append(args, arg)
		}
	}
	return args
}

func sameStrings(a, b []string) bool {
	return strings.Join(a, "\x00") == strings.Join(b, "\x00")
}
