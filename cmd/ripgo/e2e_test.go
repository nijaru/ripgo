package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

type commandResult struct {
	status int
	stdout string
	stderr string
}

func TestFindBuiltCLI(t *testing.T) {
	binary := buildTestBinary(t)
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "root.go"), "package root")
	writeCLIFile(t, filepath.Join(root, "main.txt"), "main")
	writeCLIFile(t, filepath.Join(root, "src", "main.go"), "package main")
	writeCLIFile(t, filepath.Join(root, "src", "deep", "deep.go"), "package deep")
	writeCLIFile(t, filepath.Join(root, ".hidden.go"), "package hidden")
	writeCLIFile(t, filepath.Join(root, "ignored", "ignored.go"), "package ignored")
	writeCLIFile(t, filepath.Join(root, ".gitignore"), "ignored/\n")
	linkPath := filepath.Join(root, "link.go")
	linkErr := os.Symlink("root.go", linkPath)
	danglingPath := filepath.Join(root, "dangling")
	danglingErr := os.Symlink("missing", danglingPath)

	t.Run("regex and glob", func(t *testing.T) {
		result := runBinary(t, binary, "find", "main", root)
		if result.status != 0 {
			t.Fatalf("status=%d stderr=%q", result.status, result.stderr)
		}
		if !strings.Contains(result.stdout, filepath.Join(root, "main.txt")) || !strings.Contains(result.stdout, filepath.Join(root, "src", "main.go")) {
			t.Fatalf("regex output=%q, want both main paths", result.stdout)
		}

		result = runBinary(t, binary, "find", "--glob", "*.go", "--sort", "path", root)
		if result.status != 0 {
			t.Fatalf("glob status=%d stderr=%q", result.status, result.stderr)
		}
		lines := nonEmptyLines(result.stdout)
		if !sort.StringsAreSorted(lines) || len(lines) != 4 {
			t.Fatalf("glob lines=%v, want sorted visible go files and link", lines)
		}
	})

	t.Run("type extension hidden ignore", func(t *testing.T) {
		result := runBinary(t, binary, "find", "", "--type", "f", "--extension", "go", root)
		if result.status != 0 {
			t.Fatalf("status=%d stderr=%q", result.status, result.stderr)
		}
		if strings.Contains(result.stdout, ".hidden.go") || strings.Contains(result.stdout, "ignored.go") {
			t.Fatalf("default filters leaked hidden or ignored output: %q", result.stdout)
		}

		result = runBinary(t, binary, "find", "", "--type", "f", "--extension", "go", "--hidden", "--no-ignore", root)
		if result.status != 0 || !strings.Contains(result.stdout, ".hidden.go") || !strings.Contains(result.stdout, "ignored.go") {
			t.Fatalf("enabled filters status=%d stdout=%q stderr=%q", result.status, result.stdout, result.stderr)
		}
	})

	t.Run("depth and output", func(t *testing.T) {
		result := runBinary(t, binary, "find", "", "--type", "f", "--max-depth", "1", root)
		if result.status != 0 || !strings.Contains(result.stdout, filepath.Join(root, "root.go")) || strings.Contains(result.stdout, "main.go") {
			t.Fatalf("depth status=%d stdout=%q stderr=%q", result.status, result.stdout, result.stderr)
		}

		result = runBinary(t, binary, "find", "--type", "d", "--max-depth", "1", root)
		if result.status != 0 || !strings.Contains(result.stdout, filepath.Join(root, "src")) {
			t.Fatalf("path-only find status=%d stdout=%q stderr=%q", result.status, result.stdout, result.stderr)
		}

		result = runBinary(t, binary, "find", "--glob", "*.go", "--absolute", "--print0", root)
		if result.status != 0 || !strings.Contains(result.stdout, "\x00") || strings.Contains(result.stdout, "\n") {
			t.Fatalf("print0 status=%d stdout=%q stderr=%q", result.status, result.stdout, result.stderr)
		}
	})

	if linkErr == nil {
		t.Run("symlink", func(t *testing.T) {
			result := runBinary(t, binary, "find", "--type", "l", root)
			if result.status != 0 || !strings.Contains(result.stdout, linkPath) {
				t.Fatalf("without follow result=%+v, want link path", result)
			}

			result = runBinary(t, binary, "find", "--type", "l", "--follow-symlinks", root)
			if result.status != 1 || result.stdout != "" {
				t.Fatalf("with follow result=%+v, want no symlink result", result)
			}

			result = runBinary(t, binary, "find", "--type", "f", "--follow-symlinks", root)
			if result.status != 0 || !strings.Contains(result.stdout, linkPath) || result.stderr != "" {
				t.Fatalf("followed file result=%+v, want link path without dangling-link error", result)
			}
		})
	}
	if danglingErr != nil {
		t.Logf("dangling symlink unavailable: %v", danglingErr)
	}

	t.Run("no match and malformed arguments", func(t *testing.T) {
		result := runBinary(t, binary, "find", "does-not-exist", root)
		if result.status != 1 || result.stdout != "" {
			t.Fatalf("no-match result=%+v, want exit 1 and empty output", result)
		}

		result = runBinary(t, binary, "find", "--type", "unknown", root)
		if result.status != 2 || result.stdout == "" && result.stderr == "" {
			t.Fatalf("malformed result=%+v, want exit 2 with diagnostic", result)
		}
	})

	t.Run("legacy content search", func(t *testing.T) {
		result := runBinary(t, binary, "main", filepath.Join(root, "main.txt"))
		if result.status != 0 || !strings.Contains(result.stdout, "main") {
			t.Fatalf("legacy result=%+v", result)
		}
	})
}

func buildTestBinary(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "ripgo")
	cmd := exec.CommandContext(t.Context(), "go", "build", "-o", binary, ".")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, output)
	}
	return binary
}

func runBinary(t *testing.T, binary string, args ...string) commandResult {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), binary, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	status := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			status = exitErr.ExitCode()
		} else {
			t.Fatalf("run %v: %v", args, err)
		}
	}
	return commandResult{status: status, stdout: stdout.String(), stderr: stderr.String()}
}

func nonEmptyLines(output string) []string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	return lines
}
