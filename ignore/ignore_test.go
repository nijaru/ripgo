package ignore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseIgnoreRule(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		want   IgnoreRule
		wantOK bool
	}{
		{"simple pattern", "*.log", IgnoreRule{Pattern: "*.log"}, true},
		{"negated", "!important.log", IgnoreRule{Pattern: "important.log", Negated: true}, true},
		{"anchored", "/build", IgnoreRule{Pattern: "build", Anchored: true}, true},
		{"directory only", "node_modules/", IgnoreRule{Pattern: "node_modules", DirectoryOnly: true}, true},
		{"anchored dir only", "/dist/", IgnoreRule{Pattern: "dist", Anchored: true, DirectoryOnly: true}, true},
		{"negated dir only", "!keep/", IgnoreRule{Pattern: "keep", Negated: true, DirectoryOnly: true}, true},
		{"empty after strip", "/", IgnoreRule{}, false},
		{"empty negated", "!", IgnoreRule{}, false},
		{"path pattern", "foo/bar.txt", IgnoreRule{Pattern: "foo/bar.txt"}, true},
		{"globstar", "**/*.tmp", IgnoreRule{Pattern: "**/*.tmp"}, true},
		{"source tracking", "*.log", IgnoreRule{Pattern: "*.log", Source: "/repo/.gitignore"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseIgnoreRule(tt.line, "/repo/.gitignore")
			if ok != tt.wantOK {
				t.Fatalf("parseIgnoreRule(%q) ok=%v, want %v", tt.line, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if got.Pattern != tt.want.Pattern ||
				got.Negated != tt.want.Negated ||
				got.DirectoryOnly != tt.want.DirectoryOnly ||
				got.Anchored != tt.want.Anchored {
				t.Errorf("parseIgnoreRule(%q) = %+v, want %+v", tt.line, got, tt.want)
			}
		})
	}
}

func TestParseIgnoreLines(t *testing.T) {
	content := `# Comment
*.log

!important.log
# Another comment
build/
/dist/
`
	rules := parseIgnoreLines(content, "/repo/.gitignore")
	if len(rules) != 4 {
		t.Fatalf("got %d rules, want 4", len(rules))
	}
	if rules[0].Pattern != "*.log" {
		t.Errorf("rule 0: got %q, want *.log", rules[0].Pattern)
	}
	if rules[1].Pattern != "important.log" || !rules[1].Negated {
		t.Errorf("rule 1: got %+v, want negated important.log", rules[1])
	}
	if rules[2].Pattern != "build" || !rules[2].DirectoryOnly {
		t.Errorf("rule 2: got %+v, want dir-only build", rules[2])
	}
	if rules[3].Pattern != "dist" || !rules[3].Anchored {
		t.Errorf("rule 3: got %+v, want anchored dist", rules[3])
	}
}

func TestIgnoreRuleMatch(t *testing.T) {
	tests := []struct {
		name   string
		rule   IgnoreRule
		path   string
		isDir  bool
		expect bool
	}{
		// Literal patterns
		{"literal match", IgnoreRule{Pattern: "README.md"}, "README.md", false, true},
		{"literal no match", IgnoreRule{Pattern: "README.md"}, "CHANGELOG.md", false, false},
		{"literal subdirectory", IgnoreRule{Pattern: "build"}, "src/build", true, true},

		// Wildcard patterns
		{"star extension", IgnoreRule{Pattern: "*.log"}, "debug.log", false, true},
		{"star extension nested", IgnoreRule{Pattern: "*.log"}, "logs/debug.log", false, true},
		{"star no match", IgnoreRule{Pattern: "*.log"}, "debug.txt", false, false},

		// Directory only
		{"dir only matches dir", IgnoreRule{Pattern: "build", DirectoryOnly: true}, "build", true, true},
		{"dir only skips file", IgnoreRule{Pattern: "build", DirectoryOnly: true}, "build", false, false},

		// Anchored
		{"anchored root match", IgnoreRule{Pattern: "build", Anchored: true}, "build", true, true},
		{"anchored sub no match", IgnoreRule{Pattern: "build", Anchored: true}, "src/build", true, false},

		// Negated
		{"negated matches raw", IgnoreRule{Pattern: "*.log", Negated: true}, "debug.log", false, true},

		// Globstar
		{"globstar any depth", IgnoreRule{Pattern: "**/*.o"}, "src/main.o", false, true},
		{"globstar deep", IgnoreRule{Pattern: "**/*.o"}, "a/b/c/main.o", false, true},
		{"globstar no match", IgnoreRule{Pattern: "**/*.o"}, "src/main.c", false, false},

		// Trailing slash directory matching
		{"trailing slash file no match", IgnoreRule{Pattern: "node_modules", DirectoryOnly: true}, "node_modules", false, false},
		{"trailing slash dir match", IgnoreRule{Pattern: "node_modules", DirectoryOnly: true}, "node_modules", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.rule.Match(tt.path, tt.isDir)
			if got != tt.expect {
				t.Errorf("Match(%q, dir=%v) = %v, want %v", tt.path, tt.isDir, got, tt.expect)
			}
		})
	}
}

func TestIgnoreSetIsIgnored(t *testing.T) {
	// Simple: root set with *.log rule
	rootSet := &IgnoreSet{
		Dir: "/repo",
		Rules: []IgnoreRule{
			{Pattern: "*.log"},
		},
	}

	if !rootSet.IsIgnored("debug.log", false) {
		t.Error("expected debug.log to be ignored")
	}
	if rootSet.IsIgnored("debug.txt", false) {
		t.Error("expected debug.txt to NOT be ignored")
	}
}

func TestIgnoreSetNegation(t *testing.T) {
	rootSet := &IgnoreSet{
		Dir: "/repo",
		Rules: []IgnoreRule{
			{Pattern: "*.log"},
			{Pattern: "important.log", Negated: true},
		},
	}

	if !rootSet.IsIgnored("debug.log", false) {
		t.Error("expected debug.log to be ignored")
	}
	if rootSet.IsIgnored("important.log", false) {
		t.Error("expected important.log to be re-included by negation")
	}
}

func TestIgnoreSetLastMatchWins(t *testing.T) {
	rootSet := &IgnoreSet{
		Dir: "/repo",
		Rules: []IgnoreRule{
			{Pattern: "*.log"},
			{Pattern: "debug.log", Negated: true},
			{Pattern: "debug.log"},
		},
	}

	// Last rule is NOT negated → debug.log is ignored
	if !rootSet.IsIgnored("debug.log", false) {
		t.Error("expected debug.log to be ignored (last match wins)")
	}
}

func TestIgnoreSetParentChain(t *testing.T) {
	parentSet := &IgnoreSet{
		Dir: "/repo",
		Rules: []IgnoreRule{
			{Pattern: "*.log"},
		},
	}
	childSet := &IgnoreSet{
		Dir:    "/repo/sub",
		Parent: parentSet,
		Rules: []IgnoreRule{
			{Pattern: "*.tmp"},
		},
	}

	// Parent rule applies for .log files
	if !childSet.IsIgnored("other.log", false) {
		t.Error("expected other.log to be ignored by parent rule")
	}

	// Child rule applies for .tmp files
	if !childSet.IsIgnored("scratch.tmp", false) {
		t.Error("expected scratch.tmp to be ignored by child rule")
	}

	// Neither rule matches
	if childSet.IsIgnored("main.go", false) {
		t.Error("expected main.go to NOT be ignored")
	}
}

func TestIgnoreSetChildNegation(t *testing.T) {
	// Child re-declares parent's pattern and negates a specific file.
	// This is the correct way to re-include in a child directory.
	parentSet := &IgnoreSet{
		Dir: "/repo",
		Rules: []IgnoreRule{
			{Pattern: "*.log"},
		},
	}
	childSet := &IgnoreSet{
		Dir:    "/repo/sub",
		Parent: parentSet,
		Rules: []IgnoreRule{
			{Pattern: "*.log"},
			{Pattern: "keep.log", Negated: true},
		},
	}

	// Child re-includes keep.log (re-declared *.log then negated)
	if childSet.IsIgnored("keep.log", false) {
		t.Error("expected keep.log to be re-included by child negation")
	}

	// Other .log files still ignored
	if !childSet.IsIgnored("other.log", false) {
		t.Error("expected other.log to still be ignored")
	}
}

func TestIgnoreSetAncestorDirectory(t *testing.T) {
	// When a parent ignores a directory, files inside are ignored
	// regardless of child rules. This is enforced at the Engine level
	// (ShouldIgnore checks if the directory is ignored by parent).
	parentSet := &IgnoreSet{
		Dir: "/repo",
		Rules: []IgnoreRule{
			{Pattern: "build", DirectoryOnly: true},
		},
	}
	childSet := &IgnoreSet{
		Dir:    "/repo/build",
		Parent: parentSet,
		Rules: []IgnoreRule{
			{Pattern: "output.o", Negated: true},
		},
	}

	// Direct IsIgnored checks path components — single component "output.o"
	// has no ancestor dirs, so child's negation rule wins.
	// The Engine.ShouldIgnore handles the directory-is-ignored check separately.
	if childSet.IsIgnored("output.o", false) {
		t.Error("IsIgnored sees child negation for single-component path")
	}

	// Verify parent would ignore "build" directory
	if !parentSet.IsIgnored("build", true) {
		t.Error("expected parent to ignore build/ directory")
	}

	// Verify parent would ignore "other/build" (non-anchored pattern)
	if !parentSet.IsIgnored("other/build", true) {
		t.Error("expected parent to ignore other/build/ directory")
	}
}

func TestIgnoreSetAncestorNotIgnored(t *testing.T) {
	parentSet := &IgnoreSet{
		Dir:   "/repo",
		Rules: []IgnoreRule{},
	}
	childSet := &IgnoreSet{
		Dir:    "/repo/sub",
		Parent: parentSet,
		Rules: []IgnoreRule{
			{Pattern: "*.o"},
		},
	}

	// sub is not ignored by parent, so child rules apply normally
	if !childSet.IsIgnored("main.o", false) {
		t.Error("expected main.o to be ignored by child rule")
	}
	if childSet.IsIgnored("main.c", false) {
		t.Error("expected main.c to NOT be ignored")
	}
}

func TestIgnoreSetAnchored(t *testing.T) {
	parentSet := &IgnoreSet{
		Dir: "/repo",
		Rules: []IgnoreRule{
			{Pattern: "dist", Anchored: true, DirectoryOnly: true},
		},
	}

	// Anchored: /dist only matches at repo root
	if !parentSet.IsIgnored("dist", true) {
		t.Error("expected root dist/ to be ignored")
	}

	// Subdirectory dist/ should NOT match anchored pattern
	if parentSet.IsIgnored("src/dist", true) {
		t.Error("expected src/dist/ to NOT be ignored by anchored /dist")
	}
}

func TestEngineShouldIgnore(t *testing.T) {
	dir := t.TempDir()

	// Write .gitignore
	ignoreContent := "*.log\nbuild/\n!important.log\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(ignoreContent), 0o644); err != nil {
		t.Fatal(err)
	}

	engine, err := NewEngine(Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.LoadIgnoreFile(dir); err != nil {
		t.Fatal(err)
	}

	// *.log matches
	if !engine.ShouldIgnore(filepath.Join(dir, "debug.log"), false) {
		t.Error("expected debug.log to be ignored")
	}

	// !important.log re-includes
	if engine.ShouldIgnore(filepath.Join(dir, "important.log"), false) {
		t.Error("expected important.log to be re-included")
	}

	// build/ matches directories
	if !engine.ShouldIgnore(filepath.Join(dir, "build"), true) {
		t.Error("expected build/ directory to be ignored")
	}

	// build/ does not match files (directory-only)
	if engine.ShouldIgnore(filepath.Join(dir, "build"), false) {
		t.Error("expected build file (not dir) to NOT be ignored by build/ rule")
	}
}

func TestEngineCLIExcludes(t *testing.T) {
	engine, err := NewEngine(Config{
		GlobExcludes: []string{"*.tmp"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if !engine.ShouldIgnore("scratch.tmp", false) {
		t.Error("expected scratch.tmp to be excluded by CLI glob")
	}
	if engine.ShouldIgnore("scratch.txt", false) {
		t.Error("expected scratch.txt to NOT be excluded")
	}
}

func TestEngineCLIIncludes(t *testing.T) {
	engine, err := NewEngine(Config{
		GlobIncludes: []string{"*.go"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if !engine.ShouldIgnore("main.rs", false) {
		t.Error("expected main.rs to be excluded (not in include glob)")
	}
	if engine.ShouldIgnore("main.go", false) {
		t.Error("expected main.go to be included")
	}
}

func TestEngineHidden(t *testing.T) {
	engine, err := NewEngine(Config{Hidden: false})
	if err != nil {
		t.Fatal(err)
	}

	if !engine.ShouldIgnore(".git/config", false) {
		t.Error("expected .git/config to be hidden")
	}
	if !engine.ShouldIgnore("src/.hidden/file.txt", false) {
		t.Error("expected path with hidden component to be hidden")
	}
	if engine.ShouldIgnore("src/public/file.txt", false) {
		t.Error("expected public file to NOT be hidden")
	}
}

func TestEngineHiddenOff(t *testing.T) {
	engine, err := NewEngine(Config{Hidden: true})
	if err != nil {
		t.Fatal(err)
	}

	if engine.ShouldIgnore(".git/config", false) {
		t.Error("expected .git/config to be visible with Hidden=true")
	}
}

func TestEngineNoIgnore(t *testing.T) {
	dir := t.TempDir()

	// Write .gitignore
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	engine, err := NewEngine(Config{NoIgnore: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.LoadIgnoreFile(dir); err != nil {
		t.Fatal(err)
	}

	// NoIgnore means .gitignore is not loaded
	if engine.ShouldIgnore(filepath.Join(dir, "debug.log"), false) {
		t.Error("expected debug.log to NOT be ignored when NoIgnore=true")
	}
}

func TestEngineLoadIgnoreFileIdempotent(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	engine, err := NewEngine(Config{})
	if err != nil {
		t.Fatal(err)
	}

	// Load twice — should not panic or duplicate
	if err := engine.LoadIgnoreFile(dir); err != nil {
		t.Fatal(err)
	}
	if err := engine.LoadIgnoreFile(dir); err != nil {
		t.Fatal(err)
	}

	rules := engine.GetIgnoreRules(dir)
	if len(rules) != 1 {
		t.Errorf("got %d rules after double load, want 1", len(rules))
	}
}

func TestEngineGetIgnoreRulesEmpty(t *testing.T) {
	engine, err := NewEngine(Config{})
	if err != nil {
		t.Fatal(err)
	}

	rules := engine.GetIgnoreRules("/nonexistent")
	if rules != nil {
		t.Errorf("expected nil rules for nonexistent dir, got %v", rules)
	}
}

func TestEngineIgnoreAndInclude(t *testing.T) {
	dir := t.TempDir()

	ignoreContent := "*.log\ntemp/\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(ignoreContent), 0o644); err != nil {
		t.Fatal(err)
	}

	engine, err := NewEngine(Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.LoadIgnoreFile(dir); err != nil {
		t.Fatal(err)
	}

	// .log files are ignored by .gitignore
	if !engine.ShouldIgnore(filepath.Join(dir, "debug.log"), false) {
		t.Error("expected debug.log to be ignored")
	}

	// temp/ directory is ignored
	if !engine.ShouldIgnore(filepath.Join(dir, "temp"), true) {
		t.Error("expected temp/ dir to be ignored")
	}

	// .go files are not ignored
	if engine.ShouldIgnore(filepath.Join(dir, "main.go"), false) {
		t.Error("expected main.go to NOT be ignored")
	}
}

func TestEngineMultipleIgnoreFiles(t *testing.T) {
	dir := t.TempDir()

	// Write both .gitignore and .ignore
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".ignore"), []byte("*.tmp\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	engine, err := NewEngine(Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.LoadIgnoreFile(dir); err != nil {
		t.Fatal(err)
	}

	// Both rules should apply
	if !engine.ShouldIgnore(filepath.Join(dir, "debug.log"), false) {
		t.Error("expected debug.log to be ignored by .gitignore")
	}
	if !engine.ShouldIgnore(filepath.Join(dir, "scratch.tmp"), false) {
		t.Error("expected scratch.tmp to be ignored by .ignore")
	}
}

func TestEngineAncestorDirectoryIgnored(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "build")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Root .gitignore ignores build/ directory
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("build/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Child .gitignore in build/ tries to re-include output.o
	if err := os.WriteFile(filepath.Join(subDir, ".gitignore"), []byte("!output.o\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	engine, err := NewEngine(Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.LoadIgnoreFile(dir); err != nil {
		t.Fatal(err)
	}
	if err := engine.LoadIgnoreFile(subDir); err != nil {
		t.Fatal(err)
	}

	// build/ directory itself is ignored by root
	if !engine.ShouldIgnore(subDir, true) {
		t.Error("expected build/ directory to be ignored")
	}

	// Files inside build/ are also ignored (directory-is-ignored check)
	if !engine.ShouldIgnore(filepath.Join(subDir, "output.o"), false) {
		t.Error("expected build/output.o to be ignored (parent ignores build/)")
	}
	if !engine.ShouldIgnore(filepath.Join(subDir, "other.o"), false) {
		t.Error("expected build/other.o to be ignored (parent ignores build/)")
	}
}

// Benchmark for rule matching
func BenchmarkMatchPattern(b *testing.B) {
	tests := []struct {
		name     string
		pattern  string
		anchored bool
		path     string
		isDir    bool
	}{
		{"literal", "README.md", false, "README.md", false},
		{"wildcard", "*.log", false, "src/debug.log", false},
		{"globstar", "**/*.o", false, "src/lib/main.o", false},
		{"anchored", "build", true, "build/output", true},
		{"path", "src/vendor", false, "src/vendor/lib.go", false},
	}
	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			for b.Loop() {
				matchPattern(tt.pattern, tt.anchored, tt.path, tt.isDir)
			}
		})
	}
}

// BenchmarkShouldIgnore measures the full per-file filtering cost.
func BenchmarkShouldIgnore(b *testing.B) {
	dir := b.TempDir()
	gitignore := "*.log\n*.tmp\nbuild/\nnode_modules/\n.git/\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(gitignore), 0o644); err != nil {
		b.Fatal(err)
	}

	engine, err := NewEngine(Config{})
	if err != nil {
		b.Fatal(err)
	}
	if err := engine.LoadIgnoreFile(dir); err != nil {
		b.Fatal(err)
	}

	// Pre-compute the paths we'll test
	type testCase struct {
		name string
		path string
		dir  bool
	}
	paths := []testCase{
		{"source file", filepath.Join(dir, "src/main.go"), false},
		{"gitignore match", filepath.Join(dir, "debug.log"), false},
		{"deep nested", filepath.Join(dir, "src/pkg/utils/helper.go"), false},
		{"build dir", filepath.Join(dir, "build"), true},
		{"hidden file", filepath.Join(dir, ".hidden"), false},
	}

	for _, tc := range paths {
		b.Run(tc.name, func(b *testing.B) {
			for b.Loop() {
				engine.ShouldIgnore(tc.path, tc.dir)
			}
		})
	}
}
