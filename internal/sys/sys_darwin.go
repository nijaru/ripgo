//go:build darwin

package sys

import (
	"io/fs"
	"syscall"
	"time"
)

func getStats(info fs.FileInfo) Stats {
	mtime := info.ModTime()
	stats := Stats{
		CreatedAt:  mtime,
		AccessedAt: mtime,
		ModifiedAt: mtime,
	}

	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		// Darwin (macOS) provides true birth time (creation time).
		stats.CreatedAt = time.Unix(stat.Birthtimespec.Sec, stat.Birthtimespec.Nsec)
		stats.AccessedAt = time.Unix(stat.Atimespec.Sec, stat.Atimespec.Nsec)
	}

	return stats
}
