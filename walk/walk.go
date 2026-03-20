// Package walk implements parallel directory traversal.
package walk

import (
	"context"
	"io/fs"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"

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
	// 0 means auto (GOMAXPROCS, capped at 4).
	Threads int
	// FollowSymlinks traverses symbolic links.
	FollowSymlinks bool
	// MaxFileSize skips files larger than this. 0 means unlimited.
	MaxFileSize int64
}

// Entry represents a file found during traversal.
type Entry struct {
	File fsref.Ref
}

// Walker performs parallel directory traversal, emitting file entries.
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
	if workers == 0 {
		workers = runtime.GOMAXPROCS(0)
	}

	return &Walker{
		fsys:         fsys,
		cfg:          cfg,
		ignoreEngine: engine,
		workers:      workers,
	}
}

// Run walks paths and sends file entries to fileCh.
// fileCh is closed when all work is done.
func (w *Walker) Run(ctx context.Context, paths []string, fileCh chan<- Entry) {
	var pending atomic.Int32
	var closeDirOnce sync.Once

	dirCh := make(chan string, 1024)

	var wg sync.WaitGroup
	wg.Add(w.workers)

	for _, p := range paths {
		p = filepath.ToSlash(filepath.Clean(p))

		info, err := fs.Stat(w.fsys, p)
		if err == nil && !info.IsDir() {
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
				fileCh <- Entry{File: ref}
			}
			continue
		}

		pending.Add(1)
		dirCh <- p
	}

	if pending.Load() == 0 {
		closeDirOnce.Do(func() { close(dirCh) })
	}

	for range w.workers {
		go func() {
			defer wg.Done()
			w.walkWorker(ctx, dirCh, fileCh, &pending, &closeDirOnce)
		}()
	}

	wg.Wait()
	close(fileCh)
}

func tryCloseWalk(pending *atomic.Int32, closeDirOnce *sync.Once, dirCh chan string) {
	if pending.Load() == 0 {
		closeDirOnce.Do(func() { close(dirCh) })
	}
}

func (w *Walker) walkWorker(ctx context.Context, dirCh chan string, fileCh chan<- Entry, pending *atomic.Int32, closeDirOnce *sync.Once) {
	for {
		select {
		case <-ctx.Done():
			return
		case dir, ok := <-dirCh:
			if !ok {
				return
			}

			entries, err := fs.ReadDir(w.fsys, dir)
			if err != nil {
				pending.Add(-1)
				tryCloseWalk(pending, closeDirOnce, dirCh)
				continue
			}

			var root *fsref.Root
			if rp, ok := w.fsys.(rootProvider); ok {
				// We ignore error here and fall back to pathRef
				root, _ = rp.OpenRoot(dir)
			}

			w.ignoreEngine.LoadIgnoreFile(dir)

			var pathBuf []byte

			for _, entry := range entries {
				if ctx.Err() != nil {
					return
				}

				name := entry.Name()
				var path string
				if dir == "." {
					path = name
				} else {
					pathBuf = append(pathBuf[:0], dir...)
					pathBuf = append(pathBuf, '/')
					pathBuf = append(pathBuf, name...)
					// Use an unsafe string for checks to avoid allocations for ignored files
					path = unsafe.String(unsafe.SliceData(pathBuf), len(pathBuf))
				}

				isDir := entry.IsDir()
				var info fs.FileInfo
				if entry.Type()&fs.ModeSymlink != 0 {
					if !w.cfg.FollowSymlinks {
						continue
					}
					// Follow symlink
					var err error
					info, err = fs.Stat(w.fsys, path)
					if err != nil {
						continue
					}
					isDir = info.IsDir()
				}

				if w.ignoreEngine.ShouldIgnore(path, isDir) {
					continue
				}

				// File passed ignore checks. If it's not in the root dir, allocate a safe string for persistence.
				if dir != "." {
					path = dir + "/" + name
				}

				if isDir {
					pending.Add(1)
					select {
					case <-ctx.Done():
						return
					case dirCh <- path:
					default:
						go func(p string) {
							select {
							case <-ctx.Done():
								pending.Add(-1)
								tryCloseWalk(pending, closeDirOnce, dirCh)
							case dirCh <- p:
							}
						}(path)
					}
				} else {
					if info == nil {
						var err error
						info, err = entry.Info()
						if err != nil {
							continue
						}
					}

					if w.shouldSearch(path, info) {
						var ref fsref.Ref
						if root != nil {
							ref = fsref.NewRootedRef(root, name, path, info)
						} else {
							ref = fsref.NewPathRef(path, info, w.fsys)
						}

						select {
						case <-ctx.Done():
							return
						case fileCh <- Entry{File: ref}:
						}
					}
				}
			}

			pending.Add(-1)
			tryCloseWalk(pending, closeDirOnce, dirCh)
		}
	}
}

func (w *Walker) shouldSearch(path string, info fs.FileInfo) bool {
	if w.cfg.MaxFileSize > 0 && info.Size() > w.cfg.MaxFileSize {
		return false
	}

	return true
}
