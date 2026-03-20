//go:build !darwin && !linux

package sys

import (
	"io/fs"
)

func getStats(info fs.FileInfo) Stats {
	mtime := info.ModTime()
	return Stats{
		CreatedAt:  mtime,
		AccessedAt: mtime,
		ModifiedAt: mtime,
	}
}
