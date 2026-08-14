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


func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
