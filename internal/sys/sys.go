package sys

import (
	"io/fs"
	"time"
)

// Stats holds platform-specific file timestamps.
type Stats struct {
	CreatedAt  time.Time
	AccessedAt time.Time
	ModifiedAt time.Time
}

// GetStats extracts platform-specific timestamps from fs.FileInfo.
// It falls back to ModTime if specific timestamps are not available.
func GetStats(info fs.FileInfo) Stats {
	return getStats(info)
}
