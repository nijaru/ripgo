package find

import (
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/nijaru/ripgo/ignore"
	"github.com/nijaru/ripgo/walk"
)

// Type identifies a finder result kind.
type Type uint8

const (
	// TypeFile selects regular file entries, excluding symlink entries.
	TypeFile Type = iota
	// TypeDirectory selects directory entries, excluding symlink entries.
	TypeDirectory
	// TypeSymlink selects entries whose path is a symbolic link.
	TypeSymlink
)

// TypeDir is an alias for TypeDirectory.
const TypeDir = TypeDirectory

// TypeLink is an alias for TypeSymlink.
const TypeLink = TypeSymlink

// Config holds finder matching, metadata filtering, and traversal settings.
// Traversal settings are passed to walk.Walker by the high-level finder API;
// NewFilter uses the matching and metadata fields only.
type Config struct {
	// Matcher controls basename or full-path matching.
	Matcher MatcherConfig
	// Types selects result kinds. Multiple values are ORed; an empty list keeps
	// all kinds.
	Types []Type
	// Extensions selects file extensions without a leading dot. Multiple values
	// are ORed; an empty list keeps all extensions.
	Extensions []string
	// MinSize is an inclusive lower bound. Zero disables the lower bound.
	MinSize int64
	// MaxSize is an inclusive upper bound when MaxSizeSet is true. This separate
	// bit allows an explicit zero-byte limit.
	MaxSize int64
	// MaxSizeSet enables MaxSize.
	MaxSizeSet bool
	// Walk controls traversal, including hidden entries, symlinks, and depth.
	Walk walk.Config
	// Ignore controls hidden and ignore-file behavior during traversal.
	Ignore ignore.Config
	// OmitInfo leaves Result.Info nil and avoids resolving regular-file
	// metadata. It cannot be combined with size filters.
	OmitInfo bool
	// FS selects the filesystem used by the high-level finder API. Nil uses the
	// local operating-system filesystem.
	FS fs.FS
}

// Filter applies finder matching and metadata predicates to walk entries.
type Filter struct {
	matcher        *Matcher
	types          map[Type]struct{}
	extensions     map[string]struct{}
	minSize        int64
	maxSize        int64
	maxSizeSet     bool
	matchAllTypes  bool
	followSymlinks bool
}

// WalkerConfig returns the traversal settings required by finder mode.
// Entries that cannot satisfy the configured type or extension filters are not
// emitted, while matching directory and symlink entries remain available.
func (c Config) WalkerConfig() walk.Config {
	cfg := c.Walk
	cfg.EmitDirs = c.emitsType(TypeDirectory)
	cfg.EmitSymlinks = c.emitsType(TypeSymlink)
	cfg.LazyFileInfo = c.OmitInfo || c.lazyFileInfo()
	cfg.IgnoreDanglingSymlinks = true
	return cfg
}

func (c Config) emitsType(typ Type) bool {
	if len(c.Types) == 0 {
		return len(c.Extensions) == 0 || typ == TypeFile
	}
	for _, configured := range c.Types {
		if configured == typ {
			return true
		}
	}
	return false
}

func (c Config) lazyFileInfo() bool {
	if len(c.Types) == 0 {
		return false
	}
	for _, typ := range c.Types {
		if typ == TypeFile {
			return false
		}
	}
	return true
}

// IgnoreConfig returns the ignore and hidden-entry settings for traversal.
func (c Config) IgnoreConfig() ignore.Config {
	return c.Ignore
}

// NewFilter validates cfg and compiles its matching and metadata predicates.
func NewFilter(cfg Config) (*Filter, error) {
	if cfg.OmitInfo && (cfg.MinSize != 0 || cfg.MaxSizeSet) {
		return nil, fmt.Errorf("find: metadata cannot be omitted when size filters are enabled")
	}
	if cfg.MinSize < 0 {
		return nil, fmt.Errorf("find: minimum size must not be negative: %d", cfg.MinSize)
	}
	if cfg.MaxSize < 0 {
		return nil, fmt.Errorf("find: maximum size must not be negative: %d", cfg.MaxSize)
	}
	if cfg.MaxSizeSet && cfg.MinSize > cfg.MaxSize {
		return nil, fmt.Errorf("find: minimum size %d exceeds maximum size %d", cfg.MinSize, cfg.MaxSize)
	}

	matcher, err := NewMatcher(cfg.Matcher)
	if err != nil {
		return nil, err
	}

	types := make(map[Type]struct{}, len(cfg.Types))
	for _, typ := range cfg.Types {
		switch typ {
		case TypeFile, TypeDirectory, TypeSymlink:
			types[typ] = struct{}{}
		default:
			return nil, fmt.Errorf("find: unknown result type %d", typ)
		}
	}

	extensions := make(map[string]struct{}, len(cfg.Extensions))
	for _, extension := range cfg.Extensions {
		extension = strings.ToLower(strings.TrimPrefix(extension, "."))
		if extension == "" {
			return nil, fmt.Errorf("find: extension must not be empty")
		}
		extensions[extension] = struct{}{}
	}

	return &Filter{
		matcher:        matcher,
		types:          types,
		extensions:     extensions,
		minSize:        cfg.MinSize,
		maxSize:        cfg.MaxSize,
		maxSizeSet:     cfg.MaxSizeSet,
		matchAllTypes:  len(types) == 0,
		followSymlinks: cfg.Walk.FollowSymlinks,
	}, nil
}

// Match reports whether entry satisfies every configured finder predicate,
// matching the entry's display path.
func (f *Filter) Match(entry walk.Entry) bool {
	return f.MatchPath(entry, entry.DisplayPath())
}

// MatchPath reports whether entry satisfies every configured finder predicate,
// matching matchPath instead of the entry's display path. Find uses this to
// match full paths relative to each supplied root while preserving result
// output paths.
func (f *Filter) MatchPath(entry walk.Entry, matchPath string) bool {
	if f == nil || f.matcher == nil {
		return false
	}
	if !f.matchType(entry) || !f.matchSize(entry) || !f.matchExtension(entry) {
		return false
	}
	return f.matcher.Match(matchPath)
}

func (f *Filter) matchType(entry walk.Entry) bool {
	if f.matchAllTypes {
		return true
	}

	if entry.Symlink && !f.followSymlinks {
		_, ok := f.types[TypeSymlink]
		return ok
	}

	var typ Type
	if entry.Kind == walk.EntryDirectory {
		typ = TypeDirectory
	} else {
		typ = TypeFile
	}
	_, ok := f.types[typ]
	return ok
}

func (f *Filter) matchSize(entry walk.Entry) bool {
	if f.minSize == 0 && !f.maxSizeSet {
		return true
	}
	if entry.Info == nil {
		return false
	}

	size := entry.Info.Size()
	return size >= f.minSize && (!f.maxSizeSet || size <= f.maxSize)
}

func (f *Filter) matchExtension(entry walk.Entry) bool {
	if len(f.extensions) == 0 {
		return true
	}
	if entry.Kind != walk.EntryFile {
		return false
	}

	extension := strings.ToLower(strings.TrimPrefix(path.Ext(entry.DisplayPath()), "."))
	_, ok := f.extensions[extension]
	return ok
}

// Result is the public, metadata-only representation of a finder match.
type Result struct {
	// Path is the stable display path of the matched entry.
	Path string
	// Info is the metadata snapshot used by filters.
	Info fs.FileInfo
	// Kind identifies whether the result is a file or directory.
	Kind walk.EntryKind
	// Symlink reports whether Path itself is a symbolic link.
	Symlink bool
	// Depth is relative to the supplied traversal root.
	Depth int
}

// NewResult converts a walker entry into a public finder result.
func NewResult(entry walk.Entry) Result {
	return Result{
		Path:    entry.DisplayPath(),
		Info:    entry.Info,
		Kind:    entry.Kind,
		Symlink: entry.Symlink,
		Depth:   entry.Depth,
	}
}
