package find

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nijaru/ripgo/ignore"
	"github.com/nijaru/ripgo/walk"
)

func TestConfigTraversalAndFilters(t *testing.T) {
	root := t.TempDir()
	for name, data := range map[string]string{
		".gitignore":     "ignored/\n",
		"visible.go":     "package visible",
		".hidden.go":     "package hidden",
		"nested/deep.go": "package deep",
		"ignored/out.go": "package ignored",
		"not-a-go.txt":   "text",
	} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(root, "link.go")
	if err := os.Symlink("visible.go", link); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	config := Config{
		Matcher:    MatcherConfig{Pattern: `\.go$`},
		Types:      []Type{TypeFile},
		Extensions: []string{"go"},
		Walk: walk.Config{
			Threads:        1,
			FollowSymlinks: true,
			MinDepth:       1,
			MaxDepth:       1,
			MaxDepthSet:    true,
		},
		Ignore: ignore.Config{Hidden: false},
	}
	filter, err := NewFilter(config)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := ignore.NewEngine(config.Ignore)
	if err != nil {
		t.Fatal(err)
	}

	walker := walk.NewWalker(nil, config.WalkerConfig(), engine)
	fileCh := make(chan walk.Entry, 32)
	go walker.Run(t.Context(), []string{root}, fileCh)

	var got []string
	for entry := range fileCh {
		if filter.Match(entry) {
			got = append(got, filepath.Base(entry.Path))
		}
	}
	if len(got) != 1 || got[0] != "visible.go" {
		t.Fatalf("filtered paths = %v, want [visible.go]", got)
	}

	symlinkConfig := config
	symlinkConfig.Types = []Type{TypeSymlink}
	symlinkFilter, err := NewFilter(symlinkConfig)
	if err != nil {
		t.Fatal(err)
	}
	if !symlinkFilter.Match(walk.Entry{Path: link, Kind: walk.EntryFile, Symlink: true}) {
		t.Fatal("symlink type filter rejected a followed file link")
	}
}
