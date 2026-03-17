package config

import (
	"testing"

	"github.com/nijaru/ripgo/internal/cli"
)

func TestNewDefaults(t *testing.T) {
	cfg, err := New(cli.Options{Pattern: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Pattern != "test" {
		t.Errorf("Pattern = %q, want 'test'", cfg.Pattern)
	}
	if cfg.Threads == 0 {
		t.Error("Threads should be > 0")
	}
	if cfg.OutputMode != OutputModeNormal {
		t.Error("expected normal output mode")
	}
}

func TestSmartCase(t *testing.T) {
	// SmartCase works when FixedStrings is false (regex mode)
	cfg, err := New(cli.Options{Pattern: "lowercase", SmartCase: true})
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.IgnoreCase {
		t.Error("expected IgnoreCase for all-lowercase pattern with smart case")
	}

	cfg, err = New(cli.Options{Pattern: "UpperCase", SmartCase: true})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.IgnoreCase {
		t.Error("expected case-sensitive for pattern with uppercase")
	}
}

func TestContextFlag(t *testing.T) {
	cfg, err := New(cli.Options{Pattern: "test", Context: 3})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ContextBefore != 3 || cfg.ContextAfter != 3 {
		t.Errorf("ContextBefore=%d, ContextAfter=%d, want 3", cfg.ContextBefore, cfg.ContextAfter)
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
		{"", 0, true},
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
		{cli.Options{Pattern: "x"}, OutputModeNormal},
		{cli.Options{Pattern: "x", Count: true}, OutputModeCount},
		{cli.Options{Pattern: "x", FilesWithMatches: true}, OutputModeFiles},
		{cli.Options{Pattern: "x", Quiet: true}, OutputModeQuiet},
		{cli.Options{Pattern: "x", Json: true}, OutputModeJSON},
	}

	for _, tt := range tests {
		cfg, err := New(tt.opts)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.OutputMode != tt.want {
			t.Errorf("OutputMode = %d, want %d for opts %+v", cfg.OutputMode, tt.want, tt.opts)
		}
	}
}
