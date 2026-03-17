package walk

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/nijaru/ripgo/internal/config"
	"github.com/nijaru/ripgo/internal/filetype"
	"github.com/nijaru/ripgo/internal/ignore"
)

type Walker struct {
	cfg          *config.Config
	ignoreEngine *ignore.Engine
	workers      int
}

func NewWalker(cfg *config.Config, engine *ignore.Engine) *Walker {
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

func (w *Walker) Run(ctx context.Context, paths []string, fileCh chan<- string) {
	var pending atomic.Int32
	var closeDirOnce sync.Once

	dirCh := make(chan string, 1024)

	var wg sync.WaitGroup
	wg.Add(w.workers)

	// seed initial work
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

				if w.shouldIgnore(path, entry.IsDir()) {
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
						fileCh <- path
					}
				}
			}

			pending.Add(-1)
			tryClose()
		}
	}
}

func (w *Walker) shouldIgnore(path string, isDir bool) bool {
	return w.ignoreEngine.ShouldIgnore(path, isDir)
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
		if filetype.IsBinary(path) {
			return false
		}
	} else if w.cfg.OnlyBinary && !filetype.IsBinary(path) {
		return false
	}

	return true
}
