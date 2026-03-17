package config

import (
	"testing"

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
