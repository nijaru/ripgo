package walk

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/nijaru/ripgo/ignore"
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
	// SearchBinary includes binary files in results.
	SearchBinary bool
	// OnlyBinary restricts results to binary files only.
	OnlyBinary bool
}

// Walker performs parallel directory traversal, emitting file paths.
type Walker struct {
	cfg          Config
	ignoreEngine *ignore.Engine
	workers      int
}

// NewWalker creates a walker with the given config and ignore engine.
func NewWalker(cfg Config, engine *ignore.Engine) *Walker {
	workers := cfg.Threads
	if workers == 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	if workers > 4 {
		workers = 4
	}

	return &Walker{
		cfg:          cfg,
		ignoreEngine: engine,
		workers:      workers,
	}
}

// Run walks paths and sends file paths to fileCh.
// fileCh is closed when all work is done.
func (w *Walker) Run(ctx context.Context, paths []string, fileCh chan<- string) {
	var pending atomic.Int32
	var closeDirOnce sync.Once

	dirCh := make(chan string, 1024)

	var wg sync.WaitGroup
	wg.Add(w.workers)

	for _, p := range paths {
		pending.Add(1)
		dirCh <- p
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

func (w *Walker) walkWorker(ctx context.Context, dirCh chan string, fileCh chan<- string, pending *atomic.Int32, closeDirOnce *sync.Once) {
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

			entries, err := os.ReadDir(dir)
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

				path := filepath.Join(dir, entry.Name())

				if w.ignoreEngine.ShouldIgnore(path, entry.IsDir()) {
					continue
				}

				if entry.IsDir() {
					pending.Add(1)
					select {
					case <-ctx.Done():
						return
					case dirCh <- path:
					}
				} else {
					if w.shouldSearch(path) {
						select {
						case <-ctx.Done():
							return
						case fileCh <- path:
						}
					}
				}
			}

			pending.Add(-1)
			tryClose()
		}
	}
}

func (w *Walker) shouldSearch(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	if w.cfg.MaxFileSize > 0 && info.Size() > w.cfg.MaxFileSize {
		return false
	}

	if !w.cfg.SearchBinary && !w.cfg.OnlyBinary {
		if IsBinary(path) {
			return false
		}
	} else if w.cfg.OnlyBinary && !IsBinary(path) {
		return false
	}

	return true
}

// IsBinary detects binary files by checking for NUL bytes in the first 8KB.
func IsBinary(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	buf := make([]byte, 8192)
	n, err := f.Read(buf)
	if err != nil {
		return false
	}

	return bytes.Contains(buf[:n], []byte{0})
}
