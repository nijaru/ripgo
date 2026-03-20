//go:build linux

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
		// Linux syscall.Stat_t does not have btime (birth time).
		// ctime (change time) is often the closest approximation.
		stats.CreatedAt = time.Unix(stat.Ctim.Sec, stat.Ctim.Nsec)
		stats.AccessedAt = time.Unix(stat.Atim.Sec, stat.Atim.Nsec)
	}

	return stats
}
