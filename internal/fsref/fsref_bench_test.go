package fsref

import (
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkRef_ReadFile(b *testing.B) {
	// Create a temp dir with some files to read.
	dir := b.TempDir()
	content := make([]byte, 4096)
	for i := range content {
		content[i] = byte('a' + (i % 26))
	}
	for i := range 100 {
		os.WriteFile(filepath.Join(dir, filepath.Base(dir)+string(rune('0'+i%10))+".txt"), content, 0o644)
	}

	// Collect file paths and info
	type fileEntry struct {
		name string
		path string
		info os.FileInfo
	}
	var files []fileEntry
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		info, _ := e.Info()
		files = append(files, fileEntry{
			name: e.Name(),
			path: filepath.Join(dir, e.Name()),
			info: info,
		})
	}

	b.Run("pathRef", func(b *testing.B) {
		for b.Loop() {
			for _, f := range files {
				ref := NewPathRef(f.path, f.info, nil)
				data, err := ref.ReadFile()
				if err != nil {
					b.Fatal(err)
				}
				_ = data
			}
		}
	})

	b.Run("rootedRef", func(b *testing.B) {
		root, err := NewRoot(dir)
		if err != nil {
			b.Fatal(err)
		}
		b.Cleanup(func() { /* cleanup via GC */ })

		for b.Loop() {
			for _, f := range files {
				ref := NewRootedRef(root, f.name, f.path, f.info)
				data, err := ref.ReadFile()
				if err != nil {
					b.Fatal(err)
				}
				_ = data
			}
		}
	})
}

func BenchmarkRef_Open(b *testing.B) {
	dir := b.TempDir()
	content := []byte("hello world\n")
	for i := range 50 {
		os.WriteFile(filepath.Join(dir, "file"+string(rune('0'+i%10))+".txt"), content, 0o644)
	}

	type fileEntry struct {
		name string
		path string
		info os.FileInfo
	}
	var files []fileEntry
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		info, _ := e.Info()
		files = append(files, fileEntry{
			name: e.Name(),
			path: filepath.Join(dir, e.Name()),
			info: info,
		})
	}

	b.Run("pathRef", func(b *testing.B) {
		for b.Loop() {
			for _, f := range files {
				ref := NewPathRef(f.path, f.info, nil)
				file, err := ref.Open()
				if err != nil {
					b.Fatal(err)
				}
				file.Close()
			}
		}
	})

	b.Run("rootedRef", func(b *testing.B) {
		root, err := NewRoot(dir)
		if err != nil {
			b.Fatal(err)
		}

		for b.Loop() {
			for _, f := range files {
				ref := NewRootedRef(root, f.name, f.path, f.info)
				file, err := ref.Open()
				if err != nil {
					b.Fatal(err)
				}
				file.Close()
			}
		}
	})
}
