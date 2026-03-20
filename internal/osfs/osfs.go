// Package osfs provides an fs.FS implementation for the local OS filesystem.
// Unlike os.DirFS, it supports absolute paths and paths that escape the root.
package osfs

import (
	"io/fs"
	"os"
	"syscall"
)

// OSFS implements fs.FS, fs.StatFS, and fs.ReadDirFS for the local OS.
type OSFS struct{}

func New() OSFS { return OSFS{} }

func (OSFS) Open(name string) (fs.File, error) {
	return os.Open(name)
}

func (OSFS) Stat(name string) (fs.FileInfo, error) {
	return os.Stat(name)
}

func (OSFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return os.ReadDir(name)
}

func (OSFS) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

// Mmap zero-copy maps the file into memory.
func (OSFS) Mmap(name string) ([]byte, func() error, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, nil, err
	}

	data, err := syscall.Mmap(int(f.Fd()), 0, int(info.Size()), syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		return nil, nil, err
	}

	unmap := func() error {
		return syscall.Munmap(data)
	}

	return data, unmap, nil
}

var _ fs.FS = OSFS{}
var _ fs.StatFS = OSFS{}
var _ fs.ReadDirFS = OSFS{}
var _ fs.ReadFileFS = OSFS{}
