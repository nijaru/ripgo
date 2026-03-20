// Package walk implements parallel directory traversal.
package walk

import (
	"context"
	"io/fs"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/nijaru/ripgo/ignore"
	"github.com/nijaru/ripgo/internal/osfs"
)

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
	Path string
	Info fs.FileInfo
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
				fileCh <- Entry{Path: p, Info: info}
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

func (w *Walker) walkWorker(ctx context.Context, dirCh chan string, fileCh chan<- Entry, pending *atomic.Int32, closeDirOnce *sync.Once) {
	tryClose := func() {
		if pending.Load() == 0 {
			closeDirOnce.Do(func() { close(dirCh) })
		}
	}

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
				tryClose()
				continue
			}

			w.ignoreEngine.LoadIgnoreFile(dir)

			for _, entry := range entries {
				if ctx.Err() != nil {
					return
				}

				name := entry.Name()
				path := dir + "/" + name
				if dir == "." {
					path = name
				}

				if w.ignoreEngine.ShouldIgnore(path, entry.IsDir()) {
					continue
				}

				if entry.IsDir() {
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
								tryClose()
							case dirCh <- p:
							}
						}(path)
					}
				} else {
					info, err := entry.Info()
					if err == nil && w.shouldSearch(path, info) {
						select {
						case <-ctx.Done():
							return
						case fileCh <- Entry{Path: path, Info: info}:
						}
					}
				}
			}

			pending.Add(-1)
			tryClose()
		}
	}
}

func (w *Walker) shouldSearch(path string, info fs.FileInfo) bool {
	if w.cfg.MaxFileSize > 0 && info.Size() > w.cfg.MaxFileSize {
		return false
	}

	return true
}
