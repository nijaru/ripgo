# ripgo

Ripgrep-compatible search as a Go library. Each package is independently importable — no public package touches `internal/`.

```go
import "github.com/nijaru/ripgo/search"
import "github.com/nijaru/ripgo/pattern"
```

## Quick Start

```go
m, _ := pattern.New(pattern.Config{Pattern: "TODO", SmartCase: true})
s := search.NewSearcher(search.Config{MaxCount: 100, Before: 2, After: 2}, m)
result, _ := s.Search("file.go")

p := printer.NewTextPrinter(printer.TextConfig{LineNumber: true})
p.PrintResult(result)
```

## Packages

| Package | Purpose | Key Types |
|---------|---------|-----------|
| `pattern` | Pattern matching (literal fast-path, regex fallback) | `Matcher`, `Config` |
| `search` | File scanning, context lines, multiline mode | `Searcher`, `Result`, `Match`, `Entry` |
| `walk` | Parallel directory traversal, binary detection | `Walker`, `IsBinary()` |
| `ignore` | Gitignore semantics (negation, `**`, directory chains) | `Engine`, `IgnoreRule`, `IgnoreSet` |
| `printer` | Text, JSON, count, and file-list output | `Printer`, `TextPrinter`, `JSONPrinter` |
| `stats` | Match statistics | `Stats` |

Each package owns its `Config` struct. No package imports anything `internal/`.

## CLI

A thin CLI is included at `cmd/ripgo` for testing and benchmarking:

```sh
go install github.com/nijaru/ripgo/cmd/ripgo@latest
ripgo "TODO" .
ripgo -n -C 3 --glob "*.go" "error" .
```

Run `ripgo --help` for the full flag reference.

## Dependencies

- [Kong](https://github.com/alecthomas/kong) — CLI flags (CLI only)
- [gobwas/glob](https://github.com/gobwas/glob) — glob filters
- stdlib `regexp` — regex engine (RE2)

## Architecture

```
pattern ◄── search ◄── walk
   ▲          ▲         ▲
   │          │      ignore
   └── printer ◄── stats
```

See [ai/DESIGN.md](ai/DESIGN.md) for concurrency model, ignore semantics, and design decisions.

## License

MIT
