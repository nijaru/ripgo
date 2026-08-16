# ripgo

Ripgrep-compatible content search and fd-like path finding, designed library-first in idiomatic Go. Each package is independently importable with zero CLI dependencies.

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

### High-Level API (`ripgo.Find`)

Use the finder for fd-style name and metadata queries. It streams metadata-only results and does not read file contents:

```go
import (
	"context"
	"fmt"
	"log"

	"github.com/nijaru/ripgo"
	"github.com/nijaru/ripgo/find"
)

func main() {
	ctx := context.Background()
	for result, err := range ripgo.Find(ctx, `\.go$`, []string{"."},
		ripgo.WithFindType(find.TypeFile),
		ripgo.WithFindExtension("go"),
	) {
		if err != nil {
			log.Printf("error: %v", err)
			continue
		}
		fmt.Println(result.Path)
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
| [`ripgo`](doc.go) | High-level search and finder orchestration with `iter.Seq2` | `Search()`, `Find()`, `Option`, `FindOption` |
| [`find`](find/) | Filename, path, and metadata matching for finder mode | `Matcher`, `Filter`, `Result` |
| [`pattern`](pattern/) | Literal fast-path, regex RE2, and PCRE2 matching | `Matcher`, `Config`, `New()` |
| [`search`](search/) | File scanning, mmap, line context, replace (`-r`), only-matching (`-o`) | `Searcher`, `Result`, `Match`, `Entry` |
| [`walk`](walk/) | Depth-first concurrent traversal, lazy stats, binary detection | `Walker`, `Entry` |
| [`ignore`](ignore/) | Gitignore rules, parent cascading, negation, globstar, type filters | `Engine`, `IgnoreRule`, `IgnoreSet` |
| [`printer`](printer/) | Text (colors/headings/truncation), JSON, count, and file printers | `Printer`, `TextPrinter`, `JSONPrinter` |
| [`stats`](stats/) | Atomic match and file counters | `Stats` |

## CLI

A thin CLI is included at `cmd/ripgo`:

```bash
go install github.com/nijaru/ripgo/cmd/ripgo@latest

ripgo "TODO" .
ripgo -n -C 3 -t go "func main" .
ripgo -o "\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b" .

# Find paths by name, without reading file contents
ripgo find --glob '*.go' --type f .
ripgo find --type d --max-depth 2 .
```

`ripgo find` is a read-only fd-like subset. It supports regex, glob, fixed-string, type, extension, size, depth, ignore, symlink, and path-output filters. It does not yet implement fd actions such as `--exec` or `--delete`; see `ripgo find --help` for the current surface.

## License

MIT
