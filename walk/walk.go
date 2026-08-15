// Package walk implements parallel directory traversal.
package walk

import (
	"context"
	"io/fs"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/nijaru/ripgo/ignore"
	"github.com/nijaru/ripgo/internal/fsref"
	"github.com/nijaru/ripgo/internal/osfs"
)

type rootProvider interface {
	OpenRoot(dir string) (*fsref.Root, error)
}

// Config holds walker options.
type Config struct {
	// Threads is the number of walker workers.
	// 0 means auto (GOMAXPROCS).
	Threads int
	// FollowSymlinks traverses symbolic links.
	FollowSymlinks bool
	// MaxFileSize skips files larger than this. 0 means unlimited.
	MaxFileSize int64
	// EmitDirs emits directory entries in addition to files. Supplied directory
	// roots are traversal anchors and are not emitted.
	EmitDirs bool
}

// EntryKind identifies the kind of filesystem entry.
type EntryKind uint8

const (
	// EntryFile identifies a non-directory entry.
	EntryFile EntryKind = iota
	// EntryDirectory identifies a directory entry.
	EntryDirectory
)

// Entry represents a filesystem entry found during traversal.
type Entry struct {
	// File is the capability-backed file reference for file entries. It is nil
	// for directory entries.
	File fsref.Ref
	// Path is the stable user-facing path.
	Path string
	// Info is the metadata discovered during walking.
	Info fs.FileInfo
	// Kind identifies whether this is a file or directory.
	Kind EntryKind
	// Depth is relative to the supplied traversal root. Explicit file targets
	// have depth zero; children of a directory root start at depth one.
	Depth int
}

// DisplayPath returns the stable user-facing path for the entry.
func (e Entry) DisplayPath() string {
	if e.Path != "" {
		return e.Path
	}
	if e.File != nil {
		return e.File.DisplayPath()
	}
	return ""
}

// dirWork identifies a directory and its depth relative to a traversal root.
type dirWork struct {
	path  string
	depth int
}

// dirQueue manages concurrent work distribution among walk workers without
// spawning temporary goroutines. It pops depth-first (LIFO) to keep the working
// set small, cache-hot, and bounded.
type dirQueue struct {
	mu     sync.Mutex
	cond   *sync.Cond
	dirs   []dirWork
	active int
	closed bool
}

func newDirQueue(initialCap int) *dirQueue {
	q := &dirQueue{
		dirs: make([]dirWork, 0, initialCap),
	}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *dirQueue) Push(dirs ...dirWork) {
	if len(dirs) == 0 {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return
	}
	q.dirs = append(q.dirs, dirs...)
	q.cond.Broadcast()
}

func (q *dirQueue) Pop(ctx context.Context) (dirWork, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for len(q.dirs) == 0 {
		if q.closed || q.active == 0 || ctx.Err() != nil {
			q.closed = true
			q.cond.Broadcast()
			return dirWork{}, false
		}
		q.cond.Wait()
	}

	if ctx.Err() != nil {
		q.closed = true
		q.cond.Broadcast()
		return dirWork{}, false
	}

	// LIFO pop for depth-first traversal
	lastIdx := len(q.dirs) - 1
	dir := q.dirs[lastIdx]
	q.dirs = q.dirs[:lastIdx]
	q.active++
	return dir, true
}

func (q *dirQueue) Done() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.active--
	if q.active == 0 && len(q.dirs) == 0 {
		q.closed = true
		q.cond.Broadcast()
	}
}

func (q *dirQueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	q.cond.Broadcast()
}

// Walker performs parallel directory traversal, emitting file and optional
// directory entries.
type Walker struct {
	fsys         fs.FS
	cfg          Config
	ignoreEngine *ignore.Engine
	workers      int
}

// NewWalker creates a walker with the given config and ignore engine.
// If fsys is nil, it defaults to the local OS filesystem.
func NewWalker(fsys fs.FS, cfg Config, engine *ignore.Engine) *Walker {
	if fsys == nil {
		fsys = osfs.New()
	}
	workers := cfg.Threads
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
	}

	return &Walker{
		fsys:         fsys,
		cfg:          cfg,
		ignoreEngine: engine,
		workers:      workers,
	}
}

// Run walks paths and sends entries to fileCh.
// fileCh is closed when all work is done.
func (w *Walker) Run(ctx context.Context, paths []string, fileCh chan<- Entry) {
	defer close(fileCh)

	queue := newDirQueue(len(paths) * 2)
	stopAfter := context.AfterFunc(ctx, func() {
		queue.Close()
	})
	defer stopAfter()

	var initialDirs []dirWork
	for _, p := range paths {
		p = filepath.ToSlash(filepath.Clean(p))

		info, err := fs.Stat(w.fsys, p)
		if err == nil && !info.IsDir() {
			// Explicit file target: search it
			if w.shouldSearch(p, info) {
				var ref fsref.Ref
				dir := filepath.Dir(p)
				base := filepath.Base(p)

				if rp, ok := w.fsys.(rootProvider); ok {
					if root, err := rp.OpenRoot(dir); err == nil {
						ref = fsref.NewRootedRef(root, base, p, info)
					}
				}
				if ref == nil {
					ref = fsref.NewPathRef(p, info, w.fsys)
				}
				select {
				case <-ctx.Done():
					return
				case fileCh <- Entry{
					File:  ref,
					Path:  p,
					Info:  info,
					Kind:  EntryFile,
					Depth: 0,
				}:
				}
			}
			continue
		}

		initialDirs = append(initialDirs, dirWork{path: p})
	}

	if len(initialDirs) == 0 {
		return
	}

	queue.Push(initialDirs...)

	var wg sync.WaitGroup
	wg.Add(w.workers)

	for range w.workers {
		go func() {
			defer wg.Done()
			w.walkWorker(ctx, queue, fileCh)
		}()
	}

	wg.Wait()
}

func (w *Walker) walkWorker(ctx context.Context, queue *dirQueue, fileCh chan<- Entry) {
	var subDirs []dirWork

	for {
		work, ok := queue.Pop(ctx)
		if !ok {
			return
		}
		dir := work.path

		entries, err := fs.ReadDir(w.fsys, dir)
		if err != nil {
			queue.Done()
			continue
		}

		var root *fsref.Root
		if rp, ok := w.fsys.(rootProvider); ok {
			root, _ = rp.OpenRoot(dir)
		}

		ictx, _ := w.ignoreEngine.LoadIgnoreFile(dir)
		subDirs = subDirs[:0]

		for _, entry := range entries {
			if ctx.Err() != nil {
				queue.Done()
				return
			}

			name := entry.Name()
			var path string
			if dir == "." {
				path = name
			} else {
				path = dir + "/" + name
			}

			isDir := entry.IsDir()
			var info fs.FileInfo
			if entry.Type()&fs.ModeSymlink != 0 {
				if !w.cfg.FollowSymlinks {
					continue
				}
				info, err = fs.Stat(w.fsys, path)
				if err != nil {
					continue
				}
				isDir = info.IsDir()
			}

			if w.ignoreEngine.ShouldIgnore(path, isDir, ictx) {
				continue
			}

			depth := work.depth + 1
			if isDir {
				if w.cfg.EmitDirs {
					if info == nil {
						info, _ = entry.Info()
					}
					select {
					case <-ctx.Done():
						queue.Done()
						return
					case fileCh <- Entry{
						Path:  path,
						Info:  info,
						Kind:  EntryDirectory,
						Depth: depth,
					}:
					}
				}
				subDirs = append(subDirs, dirWork{path: path, depth: depth})
				continue
			}

			if info == nil {
				info, err = entry.Info()
				if err != nil && w.cfg.MaxFileSize > 0 {
					continue
				}
			}

			if !w.shouldSearch(path, info) {
				continue
			}

			var ref fsref.Ref
			if root != nil {
				ref = fsref.NewRootedRef(root, name, path, info)
			} else {
				ref = fsref.NewPathRef(path, info, w.fsys)
			}

			select {
			case <-ctx.Done():
				queue.Done()
				return
			case fileCh <- Entry{
				File:  ref,
				Path:  path,
				Info:  info,
				Kind:  EntryFile,
				Depth: depth,
			}:
			}
		}

		if len(subDirs) > 0 {
			queue.Push(subDirs...)
		}
		queue.Done()
	}
}

func (w *Walker) shouldSearch(path string, info fs.FileInfo) bool {
	if w.cfg.MaxFileSize > 0 && info != nil && info.Size() > w.cfg.MaxFileSize {
		return false
	}
	return true
}
