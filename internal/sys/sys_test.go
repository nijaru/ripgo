package sys

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGetStats(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.txt")
	
	// Create file
	if err := os.WriteFile(path, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	
	stats := GetStats(info)
	
	// Basic sanity checks
	if stats.ModifiedAt.IsZero() {
		t.Error("ModifiedAt should not be zero")
	}
	if stats.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if stats.AccessedAt.IsZero() {
		t.Error("AccessedAt should not be zero")
	}

	// On Darwin, we can specifically check if CreatedAt is within a reasonable range
	// (now +/- 1 minute) to ensure we aren't just getting the zero value.
	now := time.Now()
	if stats.CreatedAt.After(now.Add(time.Minute)) || stats.CreatedAt.Before(now.Add(-time.Minute)) {
		t.Errorf("CreatedAt %v is not near now %v", stats.CreatedAt, now)
	}
}
