package find

import "testing"

func TestMatcherRegexBasename(t *testing.T) {
	matcher, err := NewMatcher(MatcherConfig{Pattern: `^file\.go$`})
	if err != nil {
		t.Fatal(err)
	}

	for name, want := range map[string]bool{
		"src/internal/file.go":     true,
		"src/internal/file.go.bak": false,
		"src/internal/other.go":    false,
	} {
		if got := matcher.Match(name); got != want {
			t.Errorf("Match(%q) = %t, want %t", name, got, want)
		}
	}
}

func TestMatcherModes(t *testing.T) {
	tests := []struct {
		name   string
		config MatcherConfig
		path   string
		want   bool
	}{
		{
			name:   "fixed substring",
			config: MatcherConfig{Pattern: ".go", FixedStrings: true},
			path:   `src\\main.go`,
			want:   true,
		},
		{
			name:   "fixed case insensitive",
			config: MatcherConfig{Pattern: "README", FixedStrings: true, IgnoreCase: true},
			path:   "docs/readme.md",
			want:   true,
		},
		{
			name:   "glob basename",
			config: MatcherConfig{Pattern: "*.go", Glob: true},
			path:   "src/main.go",
			want:   true,
		},
		{
			name:   "glob does not cross basename",
			config: MatcherConfig{Pattern: "src/*.go", Glob: true},
			path:   "src/main.go",
			want:   false,
		},
		{
			name:   "glob full path",
			config: MatcherConfig{Pattern: "src/**/*.go", Glob: true, FullPath: true},
			path:   `src\\internal\\main.go`,
			want:   true,
		},
		{
			name:   "glob recursive segment may be empty",
			config: MatcherConfig{Pattern: "src/**/*.go", Glob: true, FullPath: true},
			path:   "src/main.go",
			want:   true,
		},
		{
			name:   "glob leading recursive segment may be empty",
			config: MatcherConfig{Pattern: "**/*.go", Glob: true, FullPath: true},
			path:   "main.go",
			want:   true,
		},
		{
			name:   "glob case insensitive",
			config: MatcherConfig{Pattern: "*.GO", Glob: true, IgnoreCase: true},
			path:   "main.go",
			want:   true,
		},
		{
			name:   "regex full path",
			config: MatcherConfig{Pattern: `^src/.+/main\.go$`, FullPath: true},
			path:   `src\\internal\\main.go`,
			want:   true,
		},
		{
			name:   "regex case insensitive",
			config: MatcherConfig{Pattern: `README`, IgnoreCase: true},
			path:   "docs/readme.md",
			want:   true,
		},
		{
			name:   "unicode case insensitive",
			config: MatcherConfig{Pattern: "CAFÉ", FixedStrings: true, IgnoreCase: true},
			path:   "café.txt",
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher, err := NewMatcher(tt.config)
			if err != nil {
				t.Fatal(err)
			}
			if got := matcher.Match(tt.path); got != tt.want {
				t.Errorf("Match(%q) = %t, want %t", tt.path, got, tt.want)
			}
		})
	}
}

func TestMatcherEmptyPatternMatchesAll(t *testing.T) {
	for _, config := range []MatcherConfig{
		{},
		{Glob: true},
		{FixedStrings: true, FullPath: true},
	} {
		matcher, err := NewMatcher(config)
		if err != nil {
			t.Fatal(err)
		}
		if !matcher.Match("nested/path.txt") {
			t.Errorf("empty pattern with config %+v did not match", config)
		}
	}
}

func TestMatcherErrors(t *testing.T) {
	tests := []MatcherConfig{
		{Pattern: "["},
		{Pattern: "[", Glob: true},
		{Pattern: "literal", Glob: true, FixedStrings: true},
	}
	for _, config := range tests {
		if matcher, err := NewMatcher(config); err == nil || matcher != nil {
			t.Errorf("NewMatcher(%+v) = (%v, %v), want an error", config, matcher, err)
		}
	}
}

func TestNormalizePath(t *testing.T) {
	tests := map[string]string{
		"":                    "",
		"./src/../main.go":    "main.go",
		`src\\internal\\x.go`: "src/internal/x.go",
		"/tmp/./src":          "/tmp/src",
	}
	for input, want := range tests {
		if got := NormalizePath(input); got != want {
			t.Errorf("NormalizePath(%q) = %q, want %q", input, got, want)
		}
	}
}
