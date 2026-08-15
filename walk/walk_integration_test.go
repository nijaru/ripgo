package walk

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"testing/fstest"
	"time"

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
	fileCh := make(chan Entry, 256)

	go w.Run(ctx, []string{root}, fileCh)

	var files []string
	for e := range fileCh {
		rel, err := filepath.Rel(root, e.File.DisplayPath())
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, filepath.ToSlash(rel))
	}
	sort.Strings(files)
	return files
}

func collectEntries(t *testing.T, w *Walker, paths ...string) []Entry {
	t.Helper()
	fileCh := make(chan Entry, 256)
	go w.Run(t.Context(), paths, fileCh)

	var entries []Entry
	for entry := range fileCh {
		entries = append(entries, entry)
	}
	return entries
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
	fileCh := make(chan Entry, 1)
	filePath := filepath.Join(root, "a.txt")

	go w.Run(ctx, []string{filePath}, fileCh)

	var entries []Entry
	for e := range fileCh {
		entries = append(entries, e)
	}

	if len(entries) != 1 || entries[0].File.DisplayPath() != filePath {
		t.Fatalf("got %v, want [%s]", entries, filePath)
	}
	entry := entries[0]
	if entry.Path != filePath || entry.Kind != EntryFile || entry.Depth != 0 {
		t.Fatalf("entry metadata = %+v, want file path=%q depth=0", entry, filePath)
	}
	if entry.Info == nil || entry.Info.IsDir() {
		t.Fatalf("entry info = %v, want regular-file metadata", entry.Info)
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

func TestWalkerEmitsDirectoriesAndMetadata(t *testing.T) {
	root, _ := setupTestDir(t, map[string]string{
		"a.txt":            "root",
		"sub/b.go":         "package sub",
		"sub/nested/c.txt": "nested",
	})

	engine := newTestEngine(t, ignore.Config{NoIgnore: true, Hidden: true})
	w := NewWalker(nil, Config{Threads: 2, EmitDirs: true}, engine)
	fileCh := make(chan Entry, 256)
	go w.Run(t.Context(), []string{root}, fileCh)

	entries := make(map[string]Entry)
	for entry := range fileCh {
		rel, err := filepath.Rel(root, entry.Path)
		if err != nil {
			t.Fatal(err)
		}
		entries[filepath.ToSlash(rel)] = entry
	}

	wantDepth := map[string]int{
		"a.txt":            1,
		"sub":              1,
		"sub/b.go":         2,
		"sub/nested":       2,
		"sub/nested/c.txt": 3,
	}
	if len(entries) != len(wantDepth) {
		t.Fatalf("got %d entries, want %d: %v", len(entries), len(wantDepth), entries)
	}

	for path, depth := range wantDepth {
		entry, ok := entries[path]
		if !ok {
			t.Errorf("missing entry %q", path)
			continue
		}
		if entry.Depth != depth {
			t.Errorf("%s depth = %d, want %d", path, entry.Depth, depth)
		}
		if entry.DisplayPath() != entry.Path {
			t.Errorf("%s DisplayPath = %q, want %q", path, entry.DisplayPath(), entry.Path)
		}
		if entry.Info == nil {
			t.Errorf("%s has no metadata", path)
			continue
		}

		isDir := entry.Kind == EntryDirectory
		if entry.Info.IsDir() != isDir {
			t.Errorf("%s Info.IsDir() = %t, kind directory = %t", path, entry.Info.IsDir(), isDir)
		}
		if isDir {
			if entry.File != nil {
				t.Errorf("directory %s has a file reference", path)
			}
		} else if entry.File == nil {
			t.Errorf("file %s has no file reference", path)
		}
	}
}

func TestWalkerDoesNotEmitIgnoredDirectories(t *testing.T) {
	root, _ := setupTestDir(t, map[string]string{
		".gitignore":  "dist/\n",
		"dist/out.js": "built",
		"src/main.go": "package main",
	})

	engine := newTestEngine(t, ignore.Config{Hidden: true})
	w := NewWalker(nil, Config{Threads: 1, EmitDirs: true}, engine)
	fileCh := make(chan Entry, 256)
	go w.Run(t.Context(), []string{root}, fileCh)

	paths := make(map[string]struct{})
	for entry := range fileCh {
		rel, err := filepath.Rel(root, entry.Path)
		if err != nil {
			t.Fatal(err)
		}
		paths[filepath.ToSlash(rel)] = struct{}{}
	}

	if _, ok := paths["dist"]; ok {
		t.Fatal("ignored directory was emitted")
	}
	if _, ok := paths["dist/out.js"]; ok {
		t.Fatal("file in ignored directory was emitted")
	}
	if _, ok := paths["src"]; !ok {
		t.Fatal("non-ignored directory was not emitted")
	}
}

func TestWalkerVirtualFSMetadata(t *testing.T) {
	fsys := fstest.MapFS{
		"dir/file.txt": &fstest.MapFile{Data: []byte("content")},
	}
	engine, err := ignore.NewEngineFS(fsys, ignore.Config{NoIgnore: true, Hidden: true})
	if err != nil {
		t.Fatal(err)
	}

	w := NewWalker(fsys, Config{Threads: 1, EmitDirs: true}, engine)
	fileCh := make(chan Entry, 16)
	go w.Run(t.Context(), []string{"."}, fileCh)

	var foundFile, foundDir bool
	for entry := range fileCh {
		switch entry.Path {
		case "dir":
			foundDir = true
			if entry.Kind != EntryDirectory || entry.Info == nil || !entry.Info.IsDir() {
				t.Errorf("invalid directory entry: %+v", entry)
			}
		case "dir/file.txt":
			foundFile = true
			if entry.Kind != EntryFile || entry.Info == nil || entry.Info.IsDir() || entry.File == nil {
				t.Errorf("invalid file entry: %+v", entry)
			}
		}
	}

	if !foundDir || !foundFile {
		t.Fatalf("got directory=%t file=%t, want both", foundDir, foundFile)
	}
}

func TestWalkerDepthLimits(t *testing.T) {
	root, _ := setupTestDir(t, map[string]string{
		"a.txt":        "root",
		"one/b.txt":    "one",
		"one/two/c.md": "two",
	})

	engine := newTestEngine(t, ignore.Config{NoIgnore: true, Hidden: true})

	t.Run("max depth one", func(t *testing.T) {
		w := NewWalker(nil, Config{Threads: 1, EmitDirs: true, MaxDepth: 1}, engine)
		entries := collectEntries(t, w, root)
		paths := make(map[string]int)
		for _, entry := range entries {
			rel, err := filepath.Rel(root, entry.Path)
			if err != nil {
				t.Fatal(err)
			}
			paths[filepath.ToSlash(rel)] = entry.Depth
		}

		want := map[string]int{"a.txt": 1, "one": 1}
		if len(paths) != len(want) {
			t.Fatalf("got %v, want %v", paths, want)
		}
		for path, depth := range want {
			if paths[path] != depth {
				t.Errorf("%s depth = %d, want %d", path, paths[path], depth)
			}
		}
	})

	t.Run("minimum and maximum depth", func(t *testing.T) {
		w := NewWalker(nil, Config{
			Threads:     1,
			EmitDirs:    true,
			MinDepth:    2,
			MaxDepth:    2,
			MaxDepthSet: true,
		}, engine)
		entries := collectEntries(t, w, root)
		paths := make(map[string]int)
		for _, entry := range entries {
			rel, err := filepath.Rel(root, entry.Path)
			if err != nil {
				t.Fatal(err)
			}
			paths[filepath.ToSlash(rel)] = entry.Depth
		}

		want := map[string]int{"one/b.txt": 2, "one/two": 2}
		if len(paths) != len(want) {
			t.Fatalf("got %v, want %v", paths, want)
		}
		for path, depth := range want {
			if paths[path] != depth {
				t.Errorf("%s depth = %d, want %d", path, paths[path], depth)
			}
		}
	})

	t.Run("zero depth", func(t *testing.T) {
		w := NewWalker(nil, Config{Threads: 1, EmitDirs: true, MaxDepthSet: true}, engine)
		if entries := collectEntries(t, w, root); len(entries) != 0 {
			t.Fatalf("got %d entries at max depth zero, want none", len(entries))
		}

		file := filepath.Join(root, "a.txt")
		if entries := collectEntries(t, w, file); len(entries) != 1 || entries[0].Depth != 0 {
			t.Fatalf("explicit file entries = %+v, want one depth-zero entry", entries)
		}
	})
}

func TestWalkerDepthResetsPerRoot(t *testing.T) {
	root1 := t.TempDir()
	root2 := t.TempDir()
	for _, root := range []string{root1, root2} {
		if err := os.Mkdir(filepath.Join(root, "child"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "child", "file.txt"), []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	engine := newTestEngine(t, ignore.Config{NoIgnore: true, Hidden: true})
	w := NewWalker(nil, Config{Threads: 2, EmitDirs: true, MaxDepth: 1}, engine)
	entries := collectEntries(t, w, root1, root2)

	if len(entries) != 2 {
		t.Fatalf("got %d entries, want two root-level directories: %+v", len(entries), entries)
	}
	for _, entry := range entries {
		if entry.Kind != EntryDirectory || entry.Depth != 1 || filepath.Base(entry.Path) != "child" {
			t.Errorf("entry = %+v, want child directory at depth one", entry)
		}
	}
}

func TestWalkerFollowSymlinksIsCycleSafe(t *testing.T) {
	root, _ := setupTestDir(t, map[string]string{
		"real/file.txt": "content",
	})
	if err := os.Symlink("..", filepath.Join(root, "real", "parent")); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	engine := newTestEngine(t, ignore.Config{NoIgnore: true, Hidden: true})
	w := NewWalker(nil, Config{Threads: 2, FollowSymlinks: true, EmitDirs: true}, engine)
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	fileCh := make(chan Entry, 256)
	go w.Run(ctx, []string{root}, fileCh)

	counts := make(map[string]int)
	for {
		select {
		case entry, ok := <-fileCh:
			if !ok {
				if counts["real/parent"] != 1 {
					t.Fatalf("cycle link count = %d, want one: %v", counts["real/parent"], counts)
				}
				if counts["real/parent/real"] != 0 {
					t.Fatalf("cycle was traversed repeatedly: %v", counts)
				}
				return
			}
			rel, err := filepath.Rel(root, entry.Path)
			if err != nil {
				t.Fatal(err)
			}
			counts[filepath.ToSlash(rel)]++
		case <-ctx.Done():
			t.Fatalf("walker did not terminate for symlink cycle: %v", ctx.Err())
		}
	}
}

func TestWalkerVirtualFollowSymlinkCycleIsSafe(t *testing.T) {
	fsys := fstest.MapFS{
		"real/file.txt": &fstest.MapFile{Data: []byte("content")},
		"real/parent":   &fstest.MapFile{Mode: fs.ModeSymlink, Data: []byte("..")},
	}
	engine, err := ignore.NewEngineFS(fsys, ignore.Config{NoIgnore: true, Hidden: true})
	if err != nil {
		t.Fatal(err)
	}

	w := NewWalker(fsys, Config{Threads: 2, FollowSymlinks: true, EmitDirs: true}, engine)
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	fileCh := make(chan Entry, 256)
	go w.Run(ctx, []string{"."}, fileCh)

	counts := make(map[string]int)
	for {
		select {
		case entry, ok := <-fileCh:
			if !ok {
				if counts["real/parent"] != 1 {
					t.Fatalf("cycle link count = %d, want one: %v", counts["real/parent"], counts)
				}
				if counts["real/parent/real"] != 0 {
					t.Fatalf("cycle was traversed repeatedly: %v", counts)
				}
				return
			}
			counts[entry.Path]++
		case <-ctx.Done():
			t.Fatalf("walker did not terminate for virtual symlink cycle: %v", ctx.Err())
		}
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

	fileCh := make(chan Entry, 256)
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
	fileCh := make(chan Entry, 256)
	go w.Run(ctx, []string{dir1, dir2}, fileCh)

	var files []string
	for e := range fileCh {
		// Normalize: just get the basename since roots differ
		files = append(files, filepath.Base(e.File.DisplayPath()))
	}
	sort.Strings(files)

	want := []string{"a.txt", "b.txt"}
	if len(files) != len(want) {
		t.Fatalf("got %v, want %v", files, want)
	}
}
