package osfs

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestOSFS(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "sub")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatal(err)
	}

	file := filepath.Join(subdir, "sample.txt")
	want := []byte("osfs contents\n")
	mustWriteFile(t, file, want)

	fsys := New()
	t.Cleanup(func() {
		_ = fsys.Close()
	})

	t.Run("open stat read dir read file", func(t *testing.T) {
		fileHandle, err := fsys.Open(file)
		if err != nil {
			t.Fatal(err)
		}
		defer fileHandle.Close()

		data, err := io.ReadAll(fileHandle)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, want) {
			t.Fatalf("Open() read %q, want %q", data, want)
		}

		info, err := fsys.Stat(file)
		if err != nil {
			t.Fatal(err)
		}
		if info.Name() != "sample.txt" {
			t.Fatalf("Stat().Name() = %q, want %q", info.Name(), "sample.txt")
		}

		readDir, err := fsys.ReadDir(subdir)
		if err != nil {
			t.Fatal(err)
		}
		if len(readDir) != 1 {
			t.Fatalf("ReadDir() len = %d, want 1", len(readDir))
		}
		if got := readDir[0].Name(); got != "sample.txt" {
			t.Fatalf("ReadDir()[0].Name() = %q, want %q", got, "sample.txt")
		}

		data, err = fsys.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, want) {
			t.Fatalf("ReadFile() = %q, want %q", data, want)
		}
	})

	t.Run("symlink metadata", func(t *testing.T) {
		link := filepath.Join(dir, "sub-link")
		if err := os.Symlink("sub", link); err != nil {
			t.Skipf("symlinks not supported: %v", err)
		}

		target, err := fsys.ReadLink(link)
		if err != nil {
			t.Fatal(err)
		}
		if target != "sub" {
			t.Fatalf("ReadLink() = %q, want %q", target, "sub")
		}

		info, err := fsys.Lstat(link)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode()&fs.ModeSymlink == 0 {
			t.Fatalf("Lstat().Mode() = %v, want symlink", info.Mode())
		}
	})

	t.Run("open root", func(t *testing.T) {
		root, err := fsys.OpenRoot(dir)
		if err != nil {
			t.Fatal(err)
		}

		data, err := root.ReadFile(filepath.Join("sub", "sample.txt"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, want) {
			t.Fatalf("Root.ReadFile() = %q, want %q", data, want)
		}

		info, err := root.Stat(filepath.Join("sub", "sample.txt"))
		if err != nil {
			t.Fatal(err)
		}
		if info.Name() != "sample.txt" {
			t.Fatalf("Root.Stat().Name() = %q, want %q", info.Name(), "sample.txt")
		}

		fileHandle, err := root.Open(filepath.Join("sub", "sample.txt"))
		if err != nil {
			t.Fatal(err)
		}
		defer fileHandle.Close()

		data, err = io.ReadAll(fileHandle)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, want) {
			t.Fatalf("Root.Open() read %q, want %q", data, want)
		}
	})
}

func mustWriteFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
