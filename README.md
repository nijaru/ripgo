# ripgo

Ripgrep-compatible content search and fd-like path finding, designed library-first in idiomatic Go. Each package is independently importable with zero CLI dependencies.

## Quick Start

### Content Search (`ripgo.Search`)

Stream matches over `iter.Seq2` with automatic BOM sniffing, regex/literal search, and file-type filters:

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

### Path Finding (`ripgo.Find`)

Find paths by name, glob, type, size, depth, or extension without reading file contents:

```go
package main

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

Import individual packages directly for fine-grained control:

```go
import (
	"github.com/nijaru/ripgo/pattern"
	"github.com/nijaru/ripgo/printer"
	"github.com/nijaru/ripgo/search"
)

m, _ := pattern.New(pattern.Config{Pattern: "TODO", SmartCase: true})
s := search.NewSearcher(nil, search.Config{MaxCount: 100, Before: 2, After: 2}, m)
result, _ := s.SearchPath("main.go", nil)

p := printer.NewTextPrinter(printer.TextConfig{LineNumber: true})
p.PrintResult(result)
```

## Packages

| Package | Purpose | Key Exports |
|---|---|---|
| [`ripgo`](doc.go) | Root iterators (`iter.Seq2`) for search and find orchestration | `Search()`, `Find()`, `Option`, `FindOption` |
| [`find`](find/) | Filename, path, glob, regex, and metadata matching | `Matcher`, `Filter`, `Result`, `Config` |
| [`pattern`](pattern/) | Literal fast-paths, RE2 regex, PCRE2, zero-allocation line matching | `Matcher`, `LocationMatcher`, `New()` |
| [`search`](search/) | Line scanning, context lines, replacement, text encodings, mmap | `Searcher`, `Result`, `Match`, `DecodeData` |
| [`walk`](walk/) | Concurrent directory traversal, depth limits, binary detection | `Walker`, `Entry`, `Config` |
| [`fsref`](fsref/) | Capability-based file descriptors with Unix mmap and read fallback | `Ref`, `Root` |
| [`ignore`](ignore/) | Gitignore rules, trie hierarchy, negation, globstar, fast-path checks | `Engine`, `IgnoreRule`, `IgnoreSet` |
| [`printer`](printer/) | Buffered text (ANSI colors/headings), JSON, count, and file printers | `Printer`, `TextPrinter`, `JSONPrinter`, `PathPrinter` |
| [`stats`](stats/) | Atomic match and file counters | `Stats` |

## CLI

A single static binary providing both ripgrep and fd workflows:

```bash
go install github.com/nijaru/ripgo/cmd/ripgo@latest
```

### Grep Usage (`ripgo [FLAGS] PATTERN [PATH...]`)

```bash
ripgo "TODO" .
ripgo -n -C 3 -t go "func main" .
ripgo -i "goroutine" .
ripgo -E utf-16le "pattern" .
ripgo -o "\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b" .
```

### Find Usage (`ripgo find [FLAGS] [PATTERN] [PATH...]`)

```bash
ripgo find --glob '*.go' --type f .
ripgo find --type d --max-depth 2 .
ripgo find --size +10M .
ripgo find --exec 'wc -l {}' .
```

`ripgo find` supports regex, glob, fixed-string, type, extension, size, depth, ignore, symlink, and path-output formatting. Actions are shell-free: `--exec 'command {}'` executes once per match, while `--exec-batch 'command {}'` passes batched paths. `--delete` removes matched files or symlinks without following targets, and supports `--dry-run`.

## Benchmarks

Reproducible benchmarks against `ripgrep` and `fd` using `hyperfine`:

```bash
# Content search benchmark vs ripgrep (rg)
scripts/bench.sh

# Path traversal benchmark vs fd
scripts/bench_find.sh
```

- **Content Search**: Within ~2–5% of `ripgrep` on 10,000+ file trees, with faster process startup on small repositories.
- **Path Traversal**: Within ~1.6–1.8× of `fd` with cycle-safe symlink resolution and `.gitignore` evaluation.

## License

MIT
