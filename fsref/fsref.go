// Package fsref defines capability-backed file references used by search and
// traversal. Implementations can provide lazy reads and memory mapping without
// exposing filesystem-specific ownership to callers.
package fsref

import (
	"io/fs"
	"os"
	"runtime"
)

// Ref represents a capability-based file reference.
type Ref interface {
	// DisplayPath returns the stable user-facing path.
	DisplayPath() string
	// Info returns the file metadata discovered during walking.
	Info() fs.FileInfo
	// Open returns an ordinary *os.File for fallback or specialized reads.
	Open() (*os.File, error)
	// ReadFile returns the complete file content.
	ReadFile() ([]byte, error)
	// Mmap returns a memory-mapped view of the file.
	Mmap() ([]byte, func() error, error)
}

// pathRef is a compatibility backend that stores a path and uses standard os/fs calls.
type pathRef struct {
	path string
	info fs.FileInfo
	fsys fs.FS
}

// NewPathRef creates a Ref backed by a path and an optional filesystem.
func NewPathRef(path string, info fs.FileInfo, fsys fs.FS) Ref {
	return &pathRef{
		path: path,
		info: info,
		fsys: fsys,
	}
}

func (r *pathRef) DisplayPath() string {
	return r.path
}

func (r *pathRef) Info() fs.FileInfo {
	if r.info == nil && r.fsys != nil {
		r.info, _ = fs.Stat(r.fsys, r.path)
	}
	return r.info
}

func (r *pathRef) Open() (*os.File, error) {
	return os.Open(r.path)
}

func (r *pathRef) ReadFile() ([]byte, error) {
	if r.fsys != nil {
		if rfs, ok := r.fsys.(fs.ReadFileFS); ok {
			return rfs.ReadFile(r.path)
		}
		return fs.ReadFile(r.fsys, r.path)
	}
	return os.ReadFile(r.path)
}

func (r *pathRef) Mmap() ([]byte, func() error, error) {
	return mmapHelper(r)
}

// Root is a thread-safe, auto-closing wrapper around os.Root.
type Root struct {
	inner *os.Root
}

// NewRoot opens a directory as a Root and sets up automatic cleanup.
func NewRoot(path string) (*Root, error) {
	r, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	return NewRootFromOSRoot(r), nil
}

// NewRootFromOSRoot wraps an existing *os.Root and sets up automatic cleanup.
func NewRootFromOSRoot(r *os.Root) *Root {
	w := &Root{r}
	// AddCleanup is triggered when the wrapper 'w' is GC'ed.
	// We pass the inner *os.Root as the argument to avoid a cycle.
	runtime.AddCleanup(w, func(inner *os.Root) {
		inner.Close()
	}, r)
	return w
}

func (r *Root) Open(name string) (*os.File, error) {
	return r.inner.Open(name)
}

func (r *Root) ReadFile(name string) ([]byte, error) {
	return r.inner.ReadFile(name)
}

func (r *Root) Stat(name string) (fs.FileInfo, error) {
	return r.inner.Stat(name)
}

// rootedRef uses a Root to access a file relative to a capability.
type rootedRef struct {
	root *Root
	name string
	path string // Display path
	info fs.FileInfo
}

// NewRootedRef creates a Ref backed by a Root and a relative name.
func NewRootedRef(root *Root, name string, path string, info fs.FileInfo) Ref {
	return &rootedRef{
		root: root,
		name: name,
		path: path,
		info: info,
	}
}

func (r *rootedRef) DisplayPath() string {
	return r.path
}

func (r *rootedRef) Info() fs.FileInfo {
	if r.info == nil {
		r.info, _ = r.root.Stat(r.name)
	}
	return r.info
}

func (r *rootedRef) Open() (*os.File, error) {
	return r.root.Open(r.name)
}

func (r *rootedRef) ReadFile() ([]byte, error) {
	return r.root.ReadFile(r.name)
}

func (r *rootedRef) Mmap() ([]byte, func() error, error) {
	return mmapHelper(r)
}
