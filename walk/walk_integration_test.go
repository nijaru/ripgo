package walk

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/nijaru/ripgo/ignore"
)

// setupTestDir creates a temporary directory structure for testing.
// Returns the root path and a cleanup function.
func setupTestDir(t *testing.T, files map[string]string) (string, func()) {
	t.Helper()
	root := t.TempDir()

	for rel, content := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	return root, func() { os.RemoveAll(root) }
}

func newTestEngine(t *testing.T, cfg ignore.Config) *ignore.Engine {
	t.Helper()
	engine, err := ignore.NewEngine(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return engine
}

func collectFiles(t *testing.T, w *Walker, root string) []string {
	t.Helper()
	ctx := t.Context()
	fileCh := make(chan string, 256)

	go w.Run(ctx, []string{root}, fileCh)

	var files []string
	for f := range fileCh {
		rel, err := filepath.Rel(root, f)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, filepath.ToSlash(rel))
	}
	sort.Strings(files)
	return files
}

// --- Basic traversal ---

func TestWalkerBasic(t *testing.T) {
	root, _ := setupTestDir(t, map[string]string{
		"a.txt":     "hello",
		"b.go":      "package main",
		"sub/c.txt": "nested",
		"sub/d.go":  "package sub",
		"sub/e.go":  "another",
	})

	engine := newTestEngine(t, ignore.Config{NoIgnore: true, Hidden: true})
	w := NewWalker(nil, Config{Threads: 2}, engine)
	files := collectFiles(t, w, root)

	want := []string{"a.txt", "b.go", "sub/c.txt", "sub/d.go", "sub/e.go"}
	if len(files) != len(want) {
		t.Fatalf("got %d files, want %d: %v", len(files), len(want), files)
	}
	for i, f := range files {
		if f != want[i] {
			t.Errorf("file[%d] = %q, want %q", i, f, want[i])
		}
	}
}

func TestWalkerSingleFile(t *testing.T) {
	root, _ := setupTestDir(t, map[string]string{
		"only.txt": "solo",
	})

	engine := newTestEngine(t, ignore.Config{NoIgnore: true, Hidden: true})
	w := NewWalker(nil, Config{}, engine)
	files := collectFiles(t, w, root)

	if len(files) != 1 || files[0] != "only.txt" {
		t.Fatalf("got %v, want [only.txt]", files)
	}
}

func TestWalkerExplicitFile(t *testing.T) {
	root, _ := setupTestDir(t, map[string]string{
		"a.txt": "1",
		"b.txt": "2",
	})

	engine := newTestEngine(t, ignore.Config{NoIgnore: true, Hidden: true})
	w := NewWalker(nil, Config{}, engine)

	ctx := t.Context()
	fileCh := make(chan string, 1)
	filePath := filepath.Join(root, "a.txt")

	go w.Run(ctx, []string{filePath}, fileCh)

	var files []string
	for f := range fileCh {
		files = append(files, f)
	}

	if len(files) != 1 || files[0] != filePath {
		t.Fatalf("got %v, want [%s]", files, filePath)
	}
}

func TestWalkerEmptyDir(t *testing.T) {
	root := t.TempDir()

	engine := newTestEngine(t, ignore.Config{NoIgnore: true, Hidden: true})
	w := NewWalker(nil, Config{}, engine)
	files := collectFiles(t, w, root)

	if len(files) != 0 {
		t.Fatalf("got %v, want empty", files)
	}
}

// --- Ignore rules ---

func TestWalkerRespectsGitignore(t *testing.T) {
	root, _ := setupTestDir(t, map[string]string{
		".gitignore":  "*.log\ndist/\n",
		"main.go":     "package main",
		"debug.log":   "log content",
		"app.log":     "more logs",
		"src/util.go": "package src",
		"dist/out.js": "built",
	})

	engine := newTestEngine(t, ignore.Config{Hidden: true})
	w := NewWalker(nil, Config{Threads: 1}, engine)
	files := collectFiles(t, w, root)

	want := []string{".gitignore", "main.go", "src/util.go"}
	if len(files) != len(want) {
		t.Fatalf("got %v, want %v", files, want)
	}
	for i, f := range files {
		if f != want[i] {
			t.Errorf("file[%d] = %q, want %q", i, f, want[i])
		}
	}
}

func TestWalkerHiddenFiles(t *testing.T) {
	root, _ := setupTestDir(t, map[string]string{
		".hidden":     "secret",
		"visible.txt": "public",
	})

	// Without Hidden flag, dotfiles are ignored
	engine := newTestEngine(t, ignore.Config{NoIgnore: true})
	w := NewWalker(nil, Config{}, engine)
	files := collectFiles(t, w, root)

	if len(files) != 1 || files[0] != "visible.txt" {
		t.Fatalf("got %v, want [visible.txt]", files)
	}

	// With Hidden flag, dotfiles are included
	engine2 := newTestEngine(t, ignore.Config{NoIgnore: true, Hidden: true})
	w2 := NewWalker(nil, Config{}, engine2)
	files2 := collectFiles(t, w2, root)

	want := []string{".hidden", "visible.txt"}
	if len(files2) != len(want) {
		t.Fatalf("got %v, want %v", files2, want)
	}
}

func TestWalkerGlobExclude(t *testing.T) {
	root, _ := setupTestDir(t, map[string]string{
		"main.go": "package main",
		"test.go": "package main",
		"read.md": "docs",
	})

	engine := newTestEngine(t, ignore.Config{
		NoIgnore:     true,
		Hidden:       true,
		GlobExcludes: []string{"*.md"},
	})
	w := NewWalker(nil, Config{}, engine)
	files := collectFiles(t, w, root)

	want := []string{"main.go", "test.go"}
	if len(files) != len(want) {
		t.Fatalf("got %v, want %v", files, want)
	}
}

// --- MaxFileSize ---

func TestWalkerMaxFileSize(t *testing.T) {
	root, _ := setupTestDir(t, map[string]string{
		"small.txt": "hi",
		"large.txt": "this is a longer file content that exceeds the limit",
	})

	engine := newTestEngine(t, ignore.Config{NoIgnore: true, Hidden: true})
	w := NewWalker(nil, Config{MaxFileSize: 10}, engine)
	files := collectFiles(t, w, root)

	if len(files) != 1 || files[0] != "small.txt" {
		t.Fatalf("got %v, want [small.txt]", files)
	}
}

// --- Context cancellation ---

func TestWalkerContextCancel(t *testing.T) {
	root, _ := setupTestDir(t, map[string]string{
		"a.txt": "1",
		"b.txt": "2",
		"c.txt": "3",
	})

	engine := newTestEngine(t, ignore.Config{NoIgnore: true, Hidden: true})
	w := NewWalker(nil, Config{}, engine)

	ctx, cancel := context.WithCancel(t.Context())
	cancel() // cancel immediately

	fileCh := make(chan string, 256)
	w.Run(ctx, []string{root}, fileCh)

	// Should close without hanging; files may or may not be emitted
	for range fileCh {
	}
}

// --- Multiple roots ---

func TestWalkerMultipleRoots(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	os.WriteFile(filepath.Join(dir1, "a.txt"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(dir2, "b.txt"), []byte("b"), 0o644)

	engine := newTestEngine(t, ignore.Config{NoIgnore: true, Hidden: true})
	w := NewWalker(nil, Config{}, engine)

	ctx := t.Context()
	fileCh := make(chan string, 256)
	go w.Run(ctx, []string{dir1, dir2}, fileCh)

	var files []string
	for f := range fileCh {
		// Normalize: just get the basename since roots differ
		files = append(files, filepath.Base(f))
	}
	sort.Strings(files)

	want := []string{"a.txt", "b.txt"}
	if len(files) != len(want) {
		t.Fatalf("got %v, want %v", files, want)
	}
}
