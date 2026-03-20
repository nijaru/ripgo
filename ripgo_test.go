package ripgo

import (
	"context"
	"os"
	"path/filepath"
	"testing"
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
		for res, _ := range Search(context.Background(), "func", paths, WithTypes([]string{"go"})) {
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
		for res, _ := range Search(context.Background(), "func", paths, WithTypesNot([]string{"rust"})) {
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
		for res, _ := range Search(context.Background(), "func", paths, WithTypes([]string{"go"}), WithTypesNot([]string{"rust"})) {
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

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
