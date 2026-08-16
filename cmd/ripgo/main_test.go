package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunFind(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "main.go"), "package main")
	writeCLIFile(t, filepath.Join(root, "README.md"), "docs")

	status, output, stderr := captureRun(t, "find", "--glob", "*.go", root)
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, stderr)
	}
	if output != filepath.Join(root, "main.go")+"\n" {
		t.Fatalf("output = %q, want main.go path", output)
	}
}

func TestRunFindNoMatch(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "main.go"), "package main")

	status, output, stderr := captureRun(t, "find", "missing", root)
	if status != 1 || output != "" || stderr != "" {
		t.Fatalf("status=%d output=%q stderr=%q, want no-match status 1", status, output, stderr)
	}
}

func TestRunLegacySearch(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "main.go")
	writeCLIFile(t, file, "package main\nfunc main() {}\n")

	status, output, stderr := captureRun(t, "package", root)
	if status != 0 || !strings.Contains(output, "package main") || stderr != "" {
		t.Fatalf("status=%d output=%q stderr=%q, want legacy search output", status, output, stderr)
	}
}

func captureRun(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	oldArgs := os.Args
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Args = append([]string{"ripgo"}, args...)
	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter

	status := run(context.Background())
	stdoutWriter.Close()
	stderrWriter.Close()
	output, readErr := io.ReadAll(stdoutReader)
	if readErr != nil {
		t.Fatal(readErr)
	}
	stderr, readErr := io.ReadAll(stderrReader)
	if readErr != nil {
		t.Fatal(readErr)
	}
	stdoutReader.Close()
	stderrReader.Close()
	os.Args = oldArgs
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	return status, string(output), string(stderr)
}

func writeCLIFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
