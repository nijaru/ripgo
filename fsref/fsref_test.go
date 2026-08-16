package fsref

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestPathRef(t *testing.T) {
	dir := t.TempDir()
	rel := "sample.txt"
	path := filepath.Join(dir, rel)
	want := []byte("path ref contents\n")

	mustWriteFile(t, path, want)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("display path and open", func(t *testing.T) {
		ref := NewPathRef(path, info, nil)

		if got := ref.DisplayPath(); got != path {
			t.Fatalf("DisplayPath() = %q, want %q", got, path)
		}

		if got := ref.Info(); got != info {
			t.Fatalf("Info() = %v, want %v", got, info)
		}

		file, err := ref.Open()
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()

		data, err := io.ReadAll(file)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, want) {
			t.Fatalf("Open() read %q, want %q", data, want)
		}
	})

	t.Run("read file and mmap", func(t *testing.T) {
		ref := NewPathRef(path, info, nil)

		data, err := ref.ReadFile()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, want) {
			t.Fatalf("ReadFile() = %q, want %q", data, want)
		}

		mapped, unmap, err := ref.Mmap()
		if err != nil {
			t.Fatal(err)
		}
		if unmap == nil {
			t.Fatal("Mmap() returned nil unmap")
		}
		if !bytes.Equal(mapped, want) {
			t.Fatalf("Mmap() = %q, want %q", mapped, want)
		}
		if err := unmap(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestPathRefInfoViaFS(t *testing.T) {
	dir := t.TempDir()
	rel := "sample.txt"
	path := filepath.Join(dir, rel)
	want := []byte("lazy stat contents\n")

	mustWriteFile(t, path, want)

	ref := NewPathRef(rel, nil, os.DirFS(dir))

	info := ref.Info()
	if info == nil {
		t.Fatal("Info() = nil, want file info")
	}
	if info.Name() != rel {
		t.Fatalf("Info().Name() = %q, want %q", info.Name(), rel)
	}

	data, err := ref.ReadFile()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, want) {
		t.Fatalf("ReadFile() = %q, want %q", data, want)
	}
}

func TestRootedRef(t *testing.T) {
	dir := t.TempDir()
	rel := "sample.txt"
	path := filepath.Join(dir, rel)
	want := []byte("rooted ref contents\n")

	mustWriteFile(t, path, want)

	root, err := NewRoot(dir)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("root api", func(t *testing.T) {
		data, err := root.ReadFile(rel)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, want) {
			t.Fatalf("Root.ReadFile() = %q, want %q", data, want)
		}

		info, err := root.Stat(rel)
		if err != nil {
			t.Fatal(err)
		}
		if info.Name() != rel {
			t.Fatalf("Root.Stat().Name() = %q, want %q", info.Name(), rel)
		}

		file, err := root.Open(rel)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()

		data, err = io.ReadAll(file)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, want) {
			t.Fatalf("Root.Open() read %q, want %q", data, want)
		}
	})

	t.Run("display path and mmap", func(t *testing.T) {
		ref := NewRootedRef(root, rel, path, nil)

		if got := ref.DisplayPath(); got != path {
			t.Fatalf("DisplayPath() = %q, want %q", got, path)
		}

		info := ref.Info()
		if info == nil {
			t.Fatal("Info() = nil, want file info")
		}
		if info.Name() != rel {
			t.Fatalf("Info().Name() = %q, want %q", info.Name(), rel)
		}

		file, err := ref.Open()
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()

		data, err := io.ReadAll(file)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, want) {
			t.Fatalf("Open() read %q, want %q", data, want)
		}

		data, unmap, err := ref.Mmap()
		if err != nil {
			t.Fatal(err)
		}
		if unmap == nil {
			t.Fatal("Mmap() returned nil unmap")
		}
		if !bytes.Equal(data, want) {
			t.Fatalf("Mmap() = %q, want %q", data, want)
		}
		if err := unmap(); err != nil {
			t.Fatal(err)
		}
	})
}

func mustWriteFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
