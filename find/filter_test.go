package find

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/nijaru/ripgo/walk"
)

func TestFilterTypes(t *testing.T) {
	entries := []walk.Entry{
		{Path: "main.go", Kind: walk.EntryFile},
		{Path: "src", Kind: walk.EntryDirectory},
		{Path: "link.go", Kind: walk.EntryFile, Symlink: true},
		{Path: "link-dir", Kind: walk.EntryDirectory, Symlink: true},
	}

	tests := []struct {
		name  string
		types []Type
		want  []string
	}{
		{name: "all", want: []string{"main.go", "src", "link.go", "link-dir"}},
		{name: "file", types: []Type{TypeFile}, want: []string{"main.go"}},
		{name: "directory", types: []Type{TypeDirectory}, want: []string{"src"}},
		{name: "symlink", types: []Type{TypeSymlink}, want: []string{"link.go", "link-dir"}},
		{name: "or", types: []Type{TypeFile, TypeDirectory}, want: []string{"main.go", "src"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := NewFilter(Config{Types: tt.types})
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, entry := range entries {
				if filter.Match(entry) {
					got = append(got, entry.Path)
				}
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("got[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestFilterExtensionsAndSize(t *testing.T) {
	fsys := fstest.MapFS{
		"small.go":  &fstest.MapFile{Data: []byte("go")},
		"readme.md": &fstest.MapFile{Data: []byte("docs")},
		"large.txt": &fstest.MapFile{Data: []byte("too large")},
	}

	entries := make([]walk.Entry, 0, len(fsys))
	for _, name := range []string{"small.go", "readme.md", "large.txt"} {
		info, err := fs.Stat(fsys, name)
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, walk.Entry{Path: name, Info: info, Kind: walk.EntryFile})
	}

	filter, err := NewFilter(Config{
		Extensions: []string{".go", "md"},
		MinSize:    2,
		MaxSize:    4,
		MaxSizeSet: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if !filter.Match(entries[0]) || !filter.Match(entries[1]) {
		t.Fatalf("extension and size filter rejected expected entries")
	}
	if filter.Match(entries[2]) {
		t.Fatalf("extension and size filter accepted oversized entry")
	}

	caseInsensitive, err := NewFilter(Config{
		Extensions: []string{"GO"},
	})
	if err != nil {
		t.Fatal(err)
	}
	uppercase := entries[0]
	uppercase.Path = "small.GO"
	if !caseInsensitive.Match(uppercase) {
		t.Fatal("ignore-case extension filter rejected uppercase extension")
	}
}

func TestFilterMatcherAndResult(t *testing.T) {
	filter, err := NewFilter(Config{Matcher: MatcherConfig{
		Pattern:  `^src/.+\.go$`,
		FullPath: true,
	}})
	if err != nil {
		t.Fatal(err)
	}

	entry := walk.Entry{
		Path:  `src\\internal\\main.go`,
		Kind:  walk.EntryFile,
		Depth: 2,
	}
	if !filter.Match(entry) {
		t.Fatal("full-path matcher rejected normalized path")
	}

	result := NewResult(entry)
	if result.Path != entry.Path || result.Kind != walk.EntryFile || result.Depth != 2 || result.Symlink {
		t.Fatalf("result = %+v, want metadata copied from entry", result)
	}
}

func TestConfigTraversalSettings(t *testing.T) {
	config := Config{
		Walk: walk.Config{
			FollowSymlinks: true,
			MinDepth:       2,
			MaxDepth:       3,
			MaxDepthSet:    true,
			EmitDirs:       false,
		},
	}
	walkConfig := config.WalkerConfig()
	if !walkConfig.EmitDirs || !walkConfig.EmitSymlinks || walkConfig.LazyFileInfo || !walkConfig.IgnoreDanglingSymlinks || !walkConfig.FollowSymlinks || walkConfig.MinDepth != 2 || walkConfig.MaxDepth != 3 || !walkConfig.MaxDepthSet {
		t.Fatalf("WalkerConfig() = %+v, want default finder traversal settings", walkConfig)
	}

	config.Types = []Type{TypeDirectory}
	if !config.WalkerConfig().LazyFileInfo {
		t.Fatal("directory-only finder did not enable lazy file metadata")
	}
}

func TestNewFilterErrors(t *testing.T) {
	tests := []Config{
		{MinSize: -1},
		{MaxSize: -1, MaxSizeSet: true},
		{MinSize: 4, MaxSize: 3, MaxSizeSet: true},
		{Types: []Type{Type(99)}},
		{Extensions: []string{"."}},
	}
	for _, config := range tests {
		if filter, err := NewFilter(config); err == nil || filter != nil {
			t.Errorf("NewFilter(%+v) = (%v, %v), want an error", config, filter, err)
		}
	}
}
