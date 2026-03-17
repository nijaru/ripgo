package walk

import (
	"os"
	"testing"
)

func TestIsBinaryTextFile(t *testing.T) {
	f, err := os.CreateTemp("", "test*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	f.WriteString("hello world\n")
	f.Close()

	if IsBinary(f.Name()) {
		t.Error("expected text file, got binary")
	}
}

func TestIsBinaryWithNUL(t *testing.T) {
	f, err := os.CreateTemp("", "test*.bin")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	f.Write([]byte("hello\x00world"))
	f.Close()

	if !IsBinary(f.Name()) {
		t.Error("expected binary file, got text")
	}
}

func TestIsBinaryNonexistent(t *testing.T) {
	if IsBinary("nonexistent_file_xyz") {
		t.Error("nonexistent file should not be binary")
	}
}
