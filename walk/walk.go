// Package walk implements parallel directory traversal.
package walk

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	// MinDepth filters emitted entries below this root-relative depth.
	MinDepth int
	// MaxDepth filters emitted entries deeper than this root-relative depth.
	// A non-zero value enables the limit; MaxDepthSet is required for an
	// explicit zero-depth limit.
	MaxDepth int
	// MaxDepthSet enables MaxDepth when it is zero.
	MaxDepthSet bool
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
	// Symlink reports whether the entry path itself is a symbolic link. Info
	// describes the link target when symlink following is enabled.
	Symlink bool
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
type dirIdentity struct {
	path string
	info fs.FileInfo
}

type dirWork struct {
	path      string
	depth     int
	identity  dirIdentity
	ancestors []dirIdentity
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

func (w *Walker) maxDepthSet() bool {
	return w.cfg.MaxDepthSet || w.cfg.MaxDepth != 0
}

func (w *Walker) shouldEmit(depth int) bool {
	if depth < w.cfg.MinDepth {
		return false
	}
	return !w.maxDepthSet() || depth <= w.cfg.MaxDepth
}

func (w *Walker) shouldDescend(depth int) bool {
	return !w.maxDepthSet() || depth < w.cfg.MaxDepth
}

func (w *Walker) directoryIdentity(path string, info fs.FileInfo) dirIdentity {
	return dirIdentity{
		path: canonicalPath(w.fsys, path),
		info: info,
	}
}

func (w *Walker) nextDirWork(parent dirWork, path string, depth int, info fs.FileInfo) (dirWork, bool) {
	if !w.cfg.FollowSymlinks {
		return dirWork{path: path, depth: depth}, true
	}

	identity := w.directoryIdentity(path, info)
	if sameDirectory(parent.identity, identity) {
		return dirWork{}, false
	}
	for _, ancestor := range parent.ancestors {
		if sameDirectory(ancestor, identity) {
			return dirWork{}, false
		}
	}

	ancestors := make([]dirIdentity, len(parent.ancestors)+1)
	copy(ancestors, parent.ancestors)
	ancestors[len(parent.ancestors)] = parent.identity
	return dirWork{
		path:      path,
		depth:     depth,
		identity:  identity,
		ancestors: ancestors,
	}, true
}

func sameDirectory(a, b dirIdentity) bool {
	if a.info != nil && b.info != nil && a.info.Sys() != nil && b.info.Sys() != nil && os.SameFile(a.info, b.info) {
		return true
	}
	return a.path != "" && a.path == b.path
}

func canonicalPath(fsys fs.FS, name string) string {
	current := filepath.ToSlash(filepath.Clean(name))
	if _, ok := fsys.(fs.ReadLinkFS); !ok {
		return current
	}

	// Resolve symlinks in every path component so a link to an ancestor cannot
	// hide a cycle behind a later, ordinary directory component.
	seen := make(map[string]struct{})
	for {
		if _, ok := seen[current]; ok {
			return current
		}
		seen[current] = struct{}{}
		components := strings.Split(current, "/")
		prefix := ""
		start := 0
		if strings.HasPrefix(current, "/") {
			prefix = "/"
			start = 1
		}

		changed := false
		for i := start; i < len(components); i++ {
			if components[i] == "" || components[i] == "." {
				continue
			}
			if prefix == "" || prefix == "/" {
				prefix += components[i]
			} else {
				prefix += "/" + components[i]
			}

			target, err := fs.ReadLink(fsys, prefix)
			if err != nil {
				continue
			}

			resolved := resolveLinkPath(prefix, target)
			if suffix := strings.Join(components[i+1:], "/"); suffix != "" {
				resolved += "/" + suffix
			}
			current = filepath.ToSlash(filepath.Clean(filepath.FromSlash(resolved)))
			changed = true
			break
		}
		if !changed {
			return current
		}
	}
}

func resolveLinkPath(linkPath, target string) string {
	linkPath = filepath.FromSlash(linkPath)
	target = filepath.FromSlash(target)
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(linkPath), target)
	}
	return filepath.ToSlash(filepath.Clean(target))
}

func (w *Walker) isSymlink(path string) bool {
	_, err := fs.ReadLink(w.fsys, path)
	return err == nil
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
			if w.shouldSearch(p, info) && w.shouldEmit(0) {
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
					File:    ref,
					Path:    p,
					Info:    info,
					Kind:    EntryFile,
					Symlink: w.isSymlink(p),
					Depth:   0,
				}:
				}
			}
			continue
		}

		work := dirWork{path: p}
		if w.cfg.FollowSymlinks {
			work.identity = w.directoryIdentity(p, info)
		}
		initialDirs = append(initialDirs, work)
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
				if info == nil && (w.cfg.EmitDirs || w.cfg.FollowSymlinks) {
					info, _ = entry.Info()
				}
				if w.cfg.EmitDirs && w.shouldEmit(depth) {
					select {
					case <-ctx.Done():
						queue.Done()
						return
					case fileCh <- Entry{
						Path:    path,
						Info:    info,
						Kind:    EntryDirectory,
						Symlink: entry.Type()&fs.ModeSymlink != 0,
						Depth:   depth,
					}:
					}
				}
				if !w.shouldDescend(depth) {
					continue
				}
				child, ok := w.nextDirWork(work, path, depth, info)
				if ok {
					subDirs = append(subDirs, child)
				}
				continue
			}

			if info == nil {
				info, err = entry.Info()
				if err != nil && w.cfg.MaxFileSize > 0 {
					continue
				}
			}

			if !w.shouldSearch(path, info) || !w.shouldEmit(depth) {
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
				File:    ref,
				Path:    path,
				Info:    info,
				Kind:    EntryFile,
				Symlink: entry.Type()&fs.ModeSymlink != 0,
				Depth:   depth,
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
