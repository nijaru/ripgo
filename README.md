# ripgo

Ripgrep-compatible search engine designed library-first in idiomatic Go. Each package is independently importable with zero CLI dependencies.

## Quick Start

### High-Level API (`ripgo.Search`)

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/nijaru/ripgo"
)

func main() {
	ctx := context.Background()

	for res, err := range ripgo.Search(ctx, `func \w+`, []string{"."},
		ripgo.WithTypes([]string{"go"}),
		ripgo.WithSmartCase(true),
	) {
		if err != nil {
			log.Printf("error: %v", err)
			continue
		}
		for _, m := range res.Matches {
			fmt.Printf("%s:%d:%d: %s\n", res.Path, m.Line, m.Column, m.LineBytes)
		}
	}
}
```

### Low-Level Package Composition

```go
import (
	"github.com/nijaru/ripgo/pattern"
	"github.com/nijaru/ripgo/printer"
	"github.com/nijaru/ripgo/search"
)

m, _ := pattern.New(pattern.Config{Pattern: "TODO", SmartCase: true})
s := search.NewSearcher(nil, search.Config{MaxCount: 100, Before: 2, After: 2}, m)
result, _ := s.SearchPath("file.go", nil)

p := printer.NewTextPrinter(printer.TextConfig{LineNumber: true})
p.PrintResult(result)
```

## Packages

| Package | Purpose | Key Types |
|---|---|---|
| [`ripgo`](doc.go) | High-level orchestrator & `iter.Seq2` streaming API | `Search()`, `Option` |
| [`pattern`](pattern/) | Literal fast-path, regex RE2, and PCRE2 matching | `Matcher`, `Config`, `New()` |
| [`search`](search/) | File scanning, mmap, line context, replace (`-r`), only-matching (`-o`) | `Searcher`, `Result`, `Match`, `Entry` |
| [`walk`](walk/) | Depth-first concurrent traversal, lazy stats, binary detection | `Walker`, `Entry` |
| [`ignore`](ignore/) | Gitignore rules, parent cascading, negation, globstar, type filters | `Engine`, `IgnoreRule`, `IgnoreSet` |
| [`printer`](printer/) | Text (colors/headings/truncation), JSON, count, and file printers | `Printer`, `TextPrinter`, `JSONPrinter` |
| [`stats`](stats/) | Atomic match and file counters | `Stats` |

## CLI

A thin CLI harness is included at `cmd/ripgo` for benchmarking and testing:

```bash
go install github.com/nijaru/ripgo/cmd/ripgo@latest

ripgo "TODO" .
ripgo -n -C 3 -t go "func main" .
ripgo -o "\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b" .
```

## Performance

Benchmarked against `rg` (ripgrep) across 15,000 Go source files in the Kubernetes 1.31 repository:

| Tool | Mean Time |
|---|---|
| `rg` (ripgrep) | 593 ms |
| `ripgo` | 661 ms |

## License

MIT
