package config

import (
	"testing"

	findpkg "github.com/nijaru/ripgo/find"
	"github.com/nijaru/ripgo/internal/cli"
	"github.com/nijaru/ripgo/pattern"
)

func TestNewDefaults(t *testing.T) {
	cfg, err := New(cli.Options{Pattern: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Pattern.Pattern != "test" {
		t.Errorf("Pattern = %q, want 'test'", cfg.Pattern.Pattern)
	}
	if cfg.Threads == 0 {
		t.Error("Threads should be > 0")
	}
	if cfg.OutputMode() != OutputNormal {
		t.Error("expected normal output mode")
	}
}

func TestSmartCase(t *testing.T) {
	cfg, err := New(cli.Options{Pattern: "lowercase", SmartCase: true})
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Pattern.SmartCase {
		t.Error("expected SmartCase to be set")
	}

	// Verify that pattern.New applies SmartCase logic correctly.
	m, err := pattern.New(cfg.Pattern)
	if err != nil {
		t.Fatal(err)
	}
	lm, ok := m.(*pattern.LiteralMatcher)
	if !ok {
		t.Fatal("expected literal matcher for lowercase pattern")
	}
	if !lm.CaseFold() {
		t.Error("expected case-insensitive matching for all-lowercase with smart case")
	}
}

func TestContextFlag(t *testing.T) {
	cfg, err := New(cli.Options{Pattern: "test", Context: 3})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Search.Before != 3 || cfg.Search.After != 3 {
		t.Errorf("Before=%d, After=%d, want 3", cfg.Search.Before, cfg.Search.After)
	}
}

func TestDefaultPaths(t *testing.T) {
	cfg, err := New(cli.Options{Pattern: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Paths) != 1 || cfg.Paths[0] != "." {
		t.Errorf("Paths = %v, want ['.']", cfg.Paths)
	}
}

func TestParseSize(t *testing.T) {
	tests := []struct {
		input   string
		want    int64
		wantErr bool
	}{
		{"100", 100, false},
		{"1K", 1024, false},
		{"10M", 10 * 1024 * 1024, false},
		{"1G", 1024 * 1024 * 1024, false},
	}

	for _, tt := range tests {
		got, err := parseSize(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseSize(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
		}
		if got != tt.want {
			t.Errorf("parseSize(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestNewFind(t *testing.T) {
	maxDepth := 2
	cfg, err := NewFind(cli.FindOptions{
		Pattern:   "*.go",
		Paths:     []string{"src"},
		Glob:      true,
		FullPath:  true,
		Type:      []string{"f", "d"},
		Extension: []string{"go"},
		MaxDepth:  &maxDepth,
		MinSize:   "1K",
		MaxSize:   "2K",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Pattern != "*.go" || len(cfg.Paths) != 1 || cfg.Paths[0] != "src" {
		t.Fatalf("find config = %+v, want pattern and path", cfg)
	}
	if !cfg.Find.Matcher.Glob || !cfg.Find.Matcher.FullPath || len(cfg.Find.Types) != 2 || cfg.Find.Types[0] != findpkg.TypeFile || cfg.Find.Types[1] != findpkg.TypeDirectory {
		t.Fatalf("finder matching config = %+v, want glob/full-path/two types", cfg.Find)
	}
	if cfg.Find.MaxSize != 2*1024 || !cfg.Find.MaxSizeSet || cfg.Find.MinSize != 1024 {
		t.Fatalf("finder size config = %+v, want 1K..2K", cfg.Find)
	}
	if cfg.Find.Walk.MaxDepth != 2 || !cfg.Find.Walk.MaxDepthSet {
		t.Fatalf("finder depth config = %+v, want explicit max depth 2", cfg.Find.Walk)
	}
}

func TestNewFindErrors(t *testing.T) {
	if _, err := NewFind(cli.FindOptions{Type: []string{"unknown"}}); err == nil {
		t.Fatal("unknown finder type did not fail")
	}
	if _, err := NewFind(cli.FindOptions{MinDepth: -1}); err == nil {
		t.Fatal("negative minimum depth did not fail")
	}
}

func TestOutputModes(t *testing.T) {
	tests := []struct {
		opts cli.Options
		want OutputMode
	}{
		{cli.Options{Pattern: "x"}, OutputNormal},
		{cli.Options{Pattern: "x", Count: true}, OutputCount},
		{cli.Options{Pattern: "x", FilesWithMatches: true}, OutputFiles},
		{cli.Options{Pattern: "x", Quiet: true}, OutputQuiet},
		{cli.Options{Pattern: "x", Json: true}, OutputJSON},
	}

	for _, tt := range tests {
		cfg, err := New(tt.opts)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.OutputMode() != tt.want {
			t.Errorf("OutputMode = %d, want %d for opts %+v", cfg.OutputMode(), tt.want, tt.opts)
		}
	}
}
