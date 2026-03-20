// Package osfs provides an fs.FS implementation for the local OS filesystem.
// Unlike os.DirFS, it supports absolute paths and paths that escape the root.
// It uses os.Root for sandboxed, secure, and fast filesystem access (Go 1.24+).
package osfs

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/nijaru/ripgo/internal/fsref"
)

// OSFS implements fs.FS, fs.StatFS, and fs.ReadDirFS for the local OS.
type OSFS struct {
	root *os.Root
	cwd  string
}

// New creates a new OSFS.
// On Unix-like systems, it attempts to open / as the root.
// On Windows, it currently falls back to standard os package calls.
func New() *OSFS {
	cwd, _ := os.Getwd()
	if runtime.GOOS != "linux" {
		return &OSFS{cwd: cwd}
	}

	r, err := os.OpenRoot("/")
	if err != nil {
		// Fallback to standard os package if / cannot be opened
		return &OSFS{cwd: cwd}
	}
	f := &OSFS{root: r, cwd: cwd}
	// Use runtime.AddCleanup (Go 1.24+) to ensure the root is closed
	// if the OSFS object is garbage collected before Close() is called.
	runtime.AddCleanup(f, func(r *os.Root) {
		r.Close()
	}, r)
	return f
}

// Close releases the underlying root resource.
func (f *OSFS) Close() error {
	if f.root != nil {
		return f.root.Close()
	}
	return nil
}

// rel returns a path relative to the root for use with os.Root methods.
func (f *OSFS) rel(name string) string {
	if f.root == nil {
		return name
	}
	// os.Root expects relative paths.
	// If the input name is relative, we must resolve it against the CWD
	// where the OSFS was created, then strip the leading slash of the absolute result.
	var abs string
	if filepath.IsAbs(name) {
		abs = filepath.Clean(name)
	} else {
		// Join with cached CWD and Clean to handle .. correctly.
		abs = filepath.Clean(filepath.Join(f.cwd, name))
	}
	// On Unix, filepath.IsAbs checks for leading /
	return strings.TrimPrefix(abs, "/")
}

func (f *OSFS) Open(name string) (fs.File, error) {
	if f.root != nil {
		return f.root.Open(f.rel(name))
	}
	return os.Open(name)
}

func (f *OSFS) Stat(name string) (fs.FileInfo, error) {
	if f.root != nil {
		return f.root.Stat(f.rel(name))
	}
	return os.Stat(name)
}

func (f *OSFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if f.root != nil {
		return fs.ReadDir(f.root.FS(), f.rel(name))
	}
	return os.ReadDir(name)
}

func (f *OSFS) ReadFile(name string) ([]byte, error) {
	if f.root != nil {
		return f.root.ReadFile(f.rel(name))
	}
	return os.ReadFile(name)
}

// OpenRoot returns a wrapped Root for the given directory.
func (f *OSFS) OpenRoot(dir string) (*fsref.Root, error) {
	if f.root != nil {
		r, err := f.root.OpenRoot(f.rel(dir))
		if err != nil {
			return nil, err
		}
		return fsref.NewRootFromOSRoot(r), nil
	}

	abs := dir
	if !filepath.IsAbs(dir) {
		abs = filepath.Join(f.cwd, dir)
	}
	return fsref.NewRoot(abs)
}

// Mmap zero-copy maps the file into memory.
func (f *OSFS) Mmap(name string) ([]byte, func() error, error) {
	var file *os.File
	var err error

	if f.root != nil {
		file, err = f.root.Open(f.rel(name))
	} else {
		file, err = os.Open(name)
	}

	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, nil, err
	}

	if info.Size() == 0 {
		return nil, nil, fmt.Errorf("cannot mmap empty file")
	}

	data, err := syscall.Mmap(int(file.Fd()), 0, int(info.Size()), syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		return nil, nil, err
	}

	unmap := func() error {
		return syscall.Munmap(data)
	}

	return data, unmap, nil
}

var _ fs.FS = (*OSFS)(nil)
var _ fs.StatFS = (*OSFS)(nil)
var _ fs.ReadDirFS = (*OSFS)(nil)
var _ fs.ReadFileFS = (*OSFS)(nil)
