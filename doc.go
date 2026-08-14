// Package ripgo provides a high-level API for searching files.
//
// It orchestrates traversal (walk), filtering (ignore), and pattern matching (pattern)
// into a simple, unified interface for library consumers.
//
// # Basic Usage
//
// The primary entry point is the Search function, which returns an iterator of results:
//
//	ctx := context.Background()
//	results := ripgo.Search(ctx, "TODO", []string{"."}, ripgo.WithIgnoreCase(true))
//
//	for res, err := range results {
//	    if err != nil {
//	        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
//	        continue
//	    }
//	    fmt.Printf("%s: %d matches\n", res.Path, len(res.Matches))
//	}
//
// # Functional Options
//
// Search can be configured using functional options:
//   - WithMultiline(true): Enable multiline matching across line boundaries.
//   - WithWordRegexp(true): Match whole words only.
//   - WithContext(before, after): Include context lines around matches.
//   - WithReplace("template"): Perform string replacement using capture groups.
//   - WithPcre2(true): Use the PCRE2 regex engine instead of Go's default.
//
// # Filesystem Abstraction
//
// ripgo works with any io/fs.FS implementation via the WithFS option. This allows
// searching in-memory files, zip archives, or remote storage:
//
//	myFS := fstest.MapFS{...}
//	results := ripgo.Search(ctx, "pattern", []string{"."}, ripgo.WithFS(myFS))
//
// # Architecture
//
// The project is divided into several focused packages:
//   - github.com/nijaru/ripgo/search: File scanning and match reporting.
//   - github.com/nijaru/ripgo/pattern: Multi-engine pattern matching (Literal, Regex, PCRE2).
//   - github.com/nijaru/ripgo/walk: Parallel directory traversal.
//   - github.com/nijaru/ripgo/ignore: Gitignore-compatible filtering logic.
package ripgo
