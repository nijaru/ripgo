package ripgo

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"testing/fstest"

	findpkg "github.com/nijaru/ripgo/find"
	"github.com/nijaru/ripgo/walk"
)

func TestSearchTypeFilter(t *testing.T) {
	t.Run("include_type_go", func(t *testing.T) {
		root := t.TempDir()

		writeFile(t, filepath.Join(root, "main.go"), "package main\nfunc main() {}\n")
		writeFile(t, filepath.Join(root, "lib.go"), "package lib\nfunc helper() {}\n")
		writeFile(t, filepath.Join(root, "main.rs"), "fn main() {}\n")
		writeFile(t, filepath.Join(root, "README.md"), "# Project\n")

		paths := []string{root}
		var matchedFiles []string
		for res := range Search(context.Background(), "func", paths, WithTypes([]string{"go"})) {
			if res.Path != "" {
				matchedFiles = append(matchedFiles, filepath.Base(res.Path))
			}
		}

		if len(matchedFiles) != 2 {
			t.Errorf("expected 2 .go files, got %d: %v", len(matchedFiles), matchedFiles)
		}
		for _, f := range matchedFiles {
			if filepath.Ext(f) != ".go" {
				t.Errorf("got non-go file: %s", f)
			}
		}
	})

	t.Run("exclude_type_rust", func(t *testing.T) {
		root := t.TempDir()

		writeFile(t, filepath.Join(root, "main.go"), "package main\nfunc main() {}\n")
		writeFile(t, filepath.Join(root, "lib.rs"), "fn helper() {}\n")
		writeFile(t, filepath.Join(root, "lib.go"), "package lib\n")

		paths := []string{root}
		var matchedFiles []string
		for res := range Search(context.Background(), "func", paths, WithTypesNot([]string{"rust"})) {
			if res.Path != "" {
				matchedFiles = append(matchedFiles, filepath.Base(res.Path))
			}
		}

		for _, f := range matchedFiles {
			if filepath.Ext(f) == ".rs" {
				t.Errorf("got excluded .rs file: %s", f)
			}
		}
	})

	t.Run("include_and_exclude", func(t *testing.T) {
		root := t.TempDir()

		writeFile(t, filepath.Join(root, "main.go"), "package main\nfunc main() {}\n")
		writeFile(t, filepath.Join(root, "lib.go"), "package lib\n")
		writeFile(t, filepath.Join(root, "main.rs"), "fn main() {}\n")

		paths := []string{root}
		var matchedFiles []string
		for res := range Search(t.Context(), "func", paths, WithTypes([]string{"go"}), WithTypesNot([]string{"rust"})) {
			if res.Path != "" {
				matchedFiles = append(matchedFiles, filepath.Base(res.Path))
			}
		}

		for _, f := range matchedFiles {
			if filepath.Ext(f) != ".go" {
				t.Errorf("got non-go file with combined filters: %s", f)
			}
		}
	})
}

func TestSearchOnlyMatchingEndToEnd(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sample.txt"), "hello 123 world 456\n")

	var matches []string
	for res, err := range Search(t.Context(), `\d+`, []string{root}, WithOnlyMatching(true)) {
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range res.Matches {
			matches = append(matches, string(m.LineBytes))
		}
	}

	if len(matches) != 1 || matches[0] != "123" {
		t.Errorf("expected ['123'], got %v", matches)
	}
}

func TestSearchReplaceEndToEnd(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sample.txt"), "hello world\n")

	var replaced []string
	for res, err := range Search(t.Context(), `world`, []string{root}, WithReplace("universe")) {
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range res.Matches {
			replaced = append(replaced, string(m.ReplaceBytes))
		}
	}

	if len(replaced) != 1 || replaced[0] != "hello universe" {
		t.Errorf("expected ['hello universe'], got %v", replaced)
	}
}

func TestSearchEarlyExitCancellation(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 50; i++ {
		writeFile(t, filepath.Join(root, "dir", "file.go"), "package main\nfunc test() {}\n")
	}

	count := 0
	for res, err := range Search(t.Context(), "func", []string{root}) {
		if err != nil {
			continue
		}
		if len(res.Matches) > 0 {
			count++
			if count == 1 {
				break // Early break to test iterator cleanup and goroutine termination
			}
		}
	}

	if count != 1 {
		t.Errorf("expected count 1 after early break, got %d", count)
	}
}

func TestSearchGlobIncludesSubdir(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "src", "upper.go"), "Needle\n")
	writeFile(t, filepath.Join(root, "src", "skip.txt"), "Needle\n")
	writeFile(t, filepath.Join(root, "root.go"), "Needle\n")

	paths := []string{root}
	var matchedFiles []string
	for res := range Search(t.Context(), "Needle", paths, WithGlobIncludes("*.go")) {
		if res.Path != "" && len(res.Matches) > 0 {
			matchedFiles = append(matchedFiles, filepath.Base(res.Path))
		}
	}

	if len(matchedFiles) != 2 {
		t.Fatalf("expected 2 files (upper.go and root.go), got %d: %v", len(matchedFiles), matchedFiles)
	}
}

func TestFindMapFS(t *testing.T) {
	fsys := fstest.MapFS{
		"src/main.go":   &fstest.MapFile{Data: []byte("package main")},
		"src/readme.md": &fstest.MapFile{Data: []byte("docs")},
		"root.go":       &fstest.MapFile{Data: []byte("package root")},
	}

	var paths []string
	for result, err := range Find(t.Context(), "src/*.go", nil,
		WithFindFS(fsys),
		WithFindGlob(true),
		WithFindFullPath(true),
	) {
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, result.Path)
		if result.Kind != walk.EntryFile || result.Info == nil || result.Depth != 2 {
			t.Fatalf("result = %+v, want file metadata at depth 2", result)
		}
	}
	if len(paths) != 1 || paths[0] != "src/main.go" {
		t.Fatalf("found paths = %v, want [src/main.go]", paths)
	}
}

func TestFindGlobExcludesWithHiddenPaths(t *testing.T) {
	fsys := fstest.MapFS{
		".hidden.go":  &fstest.MapFile{Data: []byte("package hidden")},
		".git/config": &fstest.MapFile{Data: []byte("internal")},
		"visible.go":  &fstest.MapFile{Data: []byte("package visible")},
	}

	var paths []string
	for result, err := range Find(t.Context(), "", nil,
		WithFindFS(fsys),
		WithFindHidden(true),
		WithFindType(findpkg.TypeFile),
		WithFindGlobExcludes(".git", ".git/**"),
	) {
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, result.Path)
	}
	if slices.Equal(paths, []string{".git/config"}) {
		t.Fatalf("finder emitted excluded .git path: %v", paths)
	}
	if !slices.Contains(paths, ".hidden.go") || !slices.Contains(paths, "visible.go") {
		t.Fatalf("finder paths = %v, want hidden and visible files", paths)
	}
}

func TestFindOmitMetadata(t *testing.T) {
	fsys := &countingMapFS{MapFS: fstest.MapFS{
		"main.go": &fstest.MapFile{Data: []byte("package main")},
	}}

	var result findpkg.Result
	found := false
	for got, err := range Find(t.Context(), "*.go", nil,
		WithFindFS(fsys),
		WithFindGlob(true),
		WithFindType(findpkg.TypeFile),
		WithFindMetadata(false),
		WithFindNoIgnore(true),
	) {
		if err != nil {
			t.Fatal(err)
		}
		result = got
		found = true
	}
	if !found || result.Path != "main.go" || result.Info != nil {
		t.Fatalf("result = %+v, want path-only metadata", result)
	}
	if got := fsys.infoCalls.Load(); got != 0 {
		t.Fatalf("file metadata calls = %d, want none", got)
	}

	fsys.MapFS["src/child.go"] = &fstest.MapFile{Data: []byte("package child")}
	for result, err := range Find(t.Context(), "", nil,
		WithFindFS(fsys),
		WithFindType(findpkg.TypeDirectory),
		WithFindMetadata(false),
		WithFindNoIgnore(true),
	) {
		if err != nil {
			t.Fatal(err)
		}
		if result.Info != nil {
			t.Fatalf("directory result = %+v, want omitted metadata", result)
		}
	}
}

func TestFindDirectoriesAndExplicitFile(t *testing.T) {
	fsys := fstest.MapFS{
		"src/main.go": &fstest.MapFile{Data: []byte("package main")},
	}

	var dirs []findpkg.Result
	for result, err := range Find(t.Context(), "src", []string{"."},
		WithFindFS(fsys),
		WithFindType(findpkg.TypeDirectory),
	) {
		if err != nil {
			t.Fatal(err)
		}
		dirs = append(dirs, result)
	}
	if len(dirs) != 1 || dirs[0].Path != "src" || dirs[0].Kind != walk.EntryDirectory {
		t.Fatalf("directory results = %+v, want src directory", dirs)
	}

	var files []findpkg.Result
	for result, err := range Find(t.Context(), "main.go", []string{"src/main.go"},
		WithFindFS(fsys),
		WithFindType(findpkg.TypeFile),
	) {
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, result)
	}
	if len(files) != 1 || files[0].Path != "src/main.go" || files[0].Depth != 0 {
		t.Fatalf("explicit file results = %+v, want depth-zero file", files)
	}
}

func TestFindDirectoryFilterAvoidsFileMetadata(t *testing.T) {
	fsys := &countingMapFS{MapFS: fstest.MapFS{
		"src/main.go": &fstest.MapFile{Data: []byte("package main")},
	}}
	var dirs []findpkg.Result
	for result, err := range Find(t.Context(), "", nil,
		WithFindFS(fsys),
		WithFindType(findpkg.TypeDirectory),
		WithFindNoIgnore(true),
	) {
		if err != nil {
			t.Fatal(err)
		}
		dirs = append(dirs, result)
	}
	if len(dirs) != 1 || dirs[0].Path != "src" || dirs[0].Info == nil {
		t.Fatalf("directory results = %+v, want src metadata", dirs)
	}
	if got := fsys.infoCalls.Load(); got != 1 {
		t.Fatalf("file metadata calls = %d, want only the emitted directory metadata", got)
	}
}

type countingMapFS struct {
	fstest.MapFS
	infoCalls atomic.Int64
}

type countingDirEntry struct {
	fs.DirEntry
	calls *atomic.Int64
}

func (e countingDirEntry) Info() (fs.FileInfo, error) {
	e.calls.Add(1)
	return e.DirEntry.Info()
}

func (f *countingMapFS) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, err := fs.ReadDir(f.MapFS, name)
	if err != nil {
		return nil, err
	}
	wrapped := make([]fs.DirEntry, len(entries))
	for i, entry := range entries {
		wrapped[i] = countingDirEntry{DirEntry: entry, calls: &f.infoCalls}
	}
	return wrapped, nil
}

func TestFindReportsPermissionErrors(t *testing.T) {
	root := t.TempDir()
	blocked := filepath.Join(root, "blocked")
	if err := os.Mkdir(blocked, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(blocked, "file.txt"), []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(blocked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o700) })

	hadError := false
	for result, err := range Find(t.Context(), "", []string{root}, WithFindNoIgnore(true), WithFindHidden(true)) {
		if err != nil {
			hadError = true
			continue
		}
		if result.Path == filepath.Join(blocked, "file.txt") {
			t.Fatal("finder emitted a file from an unreadable directory")
		}
	}
	if !hadError {
		t.Skip("filesystem permissions are not enforced for this test process")
	}
}

func TestFindReportsErrorsAndSupportsCancellation(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	var foundError bool
	for _, err := range Find(t.Context(), "", []string{missing}) {
		if err != nil {
			foundError = true
		}
	}
	if !foundError {
		t.Fatal("Find did not report a missing-root error")
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for result, err := range Find(ctx, "", []string{"."}) {
		if err != nil {
			t.Fatal(err)
		}
		if result.Path != "" {
			t.Fatalf("canceled Find emitted %q", result.Path)
		}
	}
}

func TestFindEarlyBreak(t *testing.T) {
	fsys := fstest.MapFS{
		"a.txt": &fstest.MapFile{Data: []byte("a")},
		"b.txt": &fstest.MapFile{Data: []byte("b")},
	}
	count := 0
	for result, err := range Find(t.Context(), "", nil, WithFindFS(fsys)) {
		if err != nil {
			t.Fatal(err)
		}
		if result.Path != "" {
			count++
		}
		break
	}
	if count != 1 {
		t.Fatalf("early-break result count = %d, want one", count)
	}
}

func TestFindResultMetadataUsesFileInfo(t *testing.T) {
	fsys := fstest.MapFS{"file.txt": &fstest.MapFile{Data: []byte("content")}}
	for result, err := range Find(t.Context(), "file", nil, WithFindFS(fsys)) {
		if err != nil {
			t.Fatal(err)
		}
		if result.Info == nil || result.Info.IsDir() || result.Symlink {
			t.Fatalf("result metadata = %+v, want regular file", result)
		}
		return
	}
	t.Fatal("Find returned no file result")
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
