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

func TestRunVersions(t *testing.T) {
	for _, args := range [][]string{{"--version"}, {"find", "--version"}, {"search", "--version"}} {
		status, output, stderr := captureRun(t, args...)
		if status != 0 || output != "0.1.0\n" || stderr != "" {
			t.Fatalf("args=%v status=%d output=%q stderr=%q", args, status, output, stderr)
		}
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

func TestRunFindExecActions(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "a.txt"), "a")
	writeCLIFile(t, filepath.Join(root, "b.txt"), "b")
	writeCLIFile(t, filepath.Join(root, "c.txt"), "c")
	actionDir := t.TempDir()
	logPath := filepath.Join(actionDir, "exec.log")
	script := writeActionScript(t, actionDir, "printf '%s\\n' \"$@\" >> "+shellQuote(logPath))

	status, output, stderr := captureRun(t, "find", "", "--type", "f", "--exec", script+" {} {/} {//} {.} {/.}", root)
	if status != 0 || output != "" || stderr != "" {
		t.Fatalf("exec status=%d output=%q stderr=%q", status, output, stderr)
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(log), filepath.Join(root, "a.txt")) || !strings.Contains(string(log), "a.txt") || !strings.Contains(string(log), filepath.Join(root, "a")) {
		t.Fatalf("placeholder log = %q", log)
	}

	batchLog := filepath.Join(actionDir, "batch.log")
	batchScript := writeActionScript(t, actionDir, "printf '%s\\n' \"$#\" >> "+shellQuote(batchLog))
	status, output, stderr = captureRun(t, "find", "", "--type", "f", "--exec-batch", batchScript+" {}", "--exec-batch-size", "2", root)
	if status != 0 || output != "" || stderr != "" {
		t.Fatalf("batch status=%d output=%q stderr=%q", status, output, stderr)
	}
	batch, err := os.ReadFile(batchLog)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(batch)); got != "2\n1" {
		t.Fatalf("batch log = %q, want bounded batches of two and one", got)
	}
}

func TestRunFindActionsPropagateFailuresAndNoMatch(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "main.go"), "package main")
	actionDir := t.TempDir()
	marker := filepath.Join(actionDir, "marker")
	script := writeActionScript(t, actionDir, "touch "+shellQuote(marker)+"; exit 7")

	status, output, stderr := captureRun(t, "find", "", "--type", "f", "--exec", script+" {}", root)
	if status != 2 || output != "" || !strings.Contains(stderr, "Error executing action") {
		t.Fatalf("failure status=%d output=%q stderr=%q", status, output, stderr)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("action script did not run: %v", err)
	}
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}

	status, output, stderr = captureRun(t, "find", "missing", "--type", "f", "--exec", script+" {}", root)
	if status != 1 || output != "" || stderr != "" {
		t.Fatalf("no-match action status=%d output=%q stderr=%q", status, output, stderr)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("no-match action created side effect, err=%v", err)
	}
}

func TestRunFindDeleteSafety(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file.txt")
	writeCLIFile(t, file, "data")

	status, output, stderr := captureRun(t, "find", "", "--delete", root)
	if status != 2 || output != "" || !strings.Contains(stderr, "requires --type") {
		t.Fatalf("unrestricted delete status=%d output=%q stderr=%q", status, output, stderr)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("restricted delete changed file: %v", err)
	}

	status, output, stderr = captureRun(t, "find", "", "--type", "f", "--delete", "--dry-run", root)
	if status != 0 || !strings.Contains(output, file) || stderr != "" {
		t.Fatalf("dry-run status=%d output=%q stderr=%q", status, output, stderr)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("dry-run removed file: %v", err)
	}

	status, output, stderr = captureRun(t, "find", "", "--type", "f", "--delete", root)
	if status != 0 || output != "" || stderr != "" {
		t.Fatalf("delete status=%d output=%q stderr=%q", status, output, stderr)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("delete left file or returned unexpected error: %v", err)
	}

	target := filepath.Join(root, "target")
	link := filepath.Join(root, "link")
	writeCLIFile(t, target, "target")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	status, output, stderr = captureRun(t, "find", "", "--type", "l", "--delete", root)
	if status != 0 || output != "" || stderr != "" {
		t.Fatalf("symlink delete status=%d output=%q stderr=%q", status, output, stderr)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("symlink delete touched target: %v", err)
	}
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Fatalf("symlink remains or unexpected error: %v", err)
	}
}

func writeActionScript(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "action.sh")
	writeCLIFile(t, path, "#!/bin/sh\nset -eu\n"+body+"\n")
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\\"'\\\"'") + "'"
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

func TestRunEncoding(t *testing.T) {
	root := t.TempDir()

	// Write UTF-16LE with BOM
	utf16LE := []byte{0xff, 0xfe, 'h', 0, 'e', 0, 'l', 0, 'l', 0, 'o', 0, ' ', 0, 'w', 0, 'o', 0, 'r', 0, 'l', 0, 'd', 0, '\n', 0}
	if err := os.WriteFile(filepath.Join(root, "utf16.txt"), utf16LE, 0o644); err != nil {
		t.Fatal(err)
	}

	// Write Latin-1
	latin1 := []byte{'c', 'a', 'f', 0xe9, ' ', 'l', 'a', 't', 't', 'e', '\n'}
	if err := os.WriteFile(filepath.Join(root, "latin1.txt"), latin1, 0o644); err != nil {
		t.Fatal(err)
	}

	// Auto-BOM finds utf16.txt
	status, output, stderr := captureRun(t, "hello", root)
	if status != 0 || !strings.Contains(output, "hello world") || stderr != "" {
		t.Fatalf("auto-BOM search status=%d output=%q stderr=%q", status, output, stderr)
	}

	// Explicit latin-1 flag
	status, output, stderr = captureRun(t, "-E", "latin1", "café", root)
	if status != 0 || !strings.Contains(output, "café latte") || stderr != "" {
		t.Fatalf("explicit -E latin1 status=%d output=%q stderr=%q", status, output, stderr)
	}

	// Unknown encoding error
	status, output, stderr = captureRun(t, "-E", "invalid-enc-123", "pattern", root)
	if status != 2 || !strings.Contains(stderr, "unknown or unsupported encoding") {
		t.Fatalf("invalid encoding status=%d output=%q stderr=%q, want status 2 with error", status, output, stderr)
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
