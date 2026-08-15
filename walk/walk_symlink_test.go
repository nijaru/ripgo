package walk

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/nijaru/ripgo/ignore"
)

func TestWalkerSymlinks(t *testing.T) {
	root, _ := setupTestDir(t, map[string]string{
		"real/a.txt": "hello",
		"b.txt":      "world",
	})

	// Create a symlink to the 'real' directory
	linkDir := filepath.Join(root, "link-dir")
	if err := os.Symlink("real", linkDir); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	// Create a symlink to the 'b.txt' file
	linkFile := filepath.Join(root, "link-file")
	if err := os.Symlink("b.txt", linkFile); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	engine := newTestEngine(t, ignore.Config{NoIgnore: true, Hidden: true})

	t.Run("FollowSymlinks=false", func(t *testing.T) {
		w := NewWalker(nil, Config{FollowSymlinks: false}, engine)
		files := collectFiles(t, w, root)

		// Should NOT follow 'link-dir' and NOT follow 'link-file'
		// It currently might include 'link-file' as a file because it's not a dir.
		// Ripgrep by default does NOT follow symlinks.

		want := []string{"b.txt", "real/a.txt"}
		sort.Strings(files)
		if len(files) != len(want) {
			t.Fatalf("got %d files %v, want %d %v", len(files), files, len(want), want)
		}
		for i := range want {
			if files[i] != want[i] {
				t.Errorf("file[%d] = %q, want %q", i, files[i], want[i])
			}
		}
	})

	t.Run("FollowSymlinks=true", func(t *testing.T) {
		w := NewWalker(nil, Config{FollowSymlinks: true}, engine)
		files := collectFiles(t, w, root)

		// Should follow 'link-dir' and 'link-file'
		want := []string{"b.txt", "link-dir/a.txt", "link-file", "real/a.txt"}
		sort.Strings(files)
		if len(files) != len(want) {
			t.Fatalf("got %d files %v, want %d %v", len(files), files, len(want), want)
		}
		for i := range want {
			if files[i] != want[i] {
				t.Errorf("file[%d] = %q, want %q", i, files[i], want[i])
			}
		}

		entries := collectEntries(t, NewWalker(nil, Config{
			FollowSymlinks: true,
			EmitDirs:       true,
		}, engine), root)
		for _, entry := range entries {
			switch entry.Path {
			case linkDir:
				if !entry.Symlink || entry.Kind != EntryDirectory {
					t.Errorf("directory symlink entry = %+v, want symlink directory", entry)
				}
			case linkFile:
				if !entry.Symlink || entry.Kind != EntryFile {
					t.Errorf("file symlink entry = %+v, want symlink file", entry)
				}
			}
		}
	})
}
