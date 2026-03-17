# AGENTS.md — ripgo

Ripgrep-like search tool in Go. Idiomatic, standard-library-first, high performance. Primarily a Go-native ripgrep library with a thin CLI.

## Stack

| Component | Choice | Notes |
|---|---|---|
| Language | Go 1.26+ | Green Tea GC, `range`-over-func |
| CLI | Kong v1.14 | Struct-driven flags, single command |
| Glob | gobwas/glob | Compile-once, match-many |
| Regex | stdlib `regexp` | RE2, linear time |
| Alt regex | `regexp2` (future) | PCRE-like, opt-in only |

## Commands

| Action | Command |
|---|---|
| Build | `go build -o ripgo ./cmd/ripgo` |
| Install | `go install ./cmd/ripgo` |
| Test | `go test ./...` |
| Vet | `go vet ./...` |
| Format | `goimports -w . && gofumpt -w .` |
| Tidy | `go mod tidy` |

## Code Style

- **Receivers:** 1-2 letter type abbreviation — `(w *Walker)`, `(p *Printer)`
- **No `interface{}`** — use `any`
- **No `ioutil`** — use `os`/`io`
- **No `sort.Slice`** — use `slices.Sort`
- **No `context.TODO()` in tests** — use `t.Context()`
- **Errors:** `fmt.Errorf("...: %w", err)`
- **Comments:** Why not what. No TODOs.

## Architecture

```
# Public library (importable)
pattern/            — Matcher interface, LiteralMatcher, RegexMatcher
search/             — Searcher, Result, Match, Config
walk/               — Walker, binary detection, directory traversal
ignore/             — Engine, rule parsing, glob matching
printer/            — Printer interface + Text/JSON/Count/Files
stats/              — Stats, match counts

# Private (CLI wiring only)
internal/cli        — Kong flag parsing
internal/config     — CLI flags → library Config translation

# Binary
cmd/ripgo/main.go   — thin pipeline orchestration
```

Dependency graph:
```
pattern ◄── search ◄── walk
   ▲          ▲         ▲
   │          │      ignore
   └── printer ◄── stats

cli ◄── config ──► wires everything
```

Public packages own their config structs. `internal/config` translates CLI flags into library configs. No public package imports anything `internal/`.

## Verification

After any change:
1. `go build ./cmd/ripgo` — must compile
2. `go test ./...` — all tests pass
3. `go vet ./...` — clean
4. `timeout 5 ripgo "pattern" .` — search works, exits cleanly

## CLI vs Library

The CLI (`cmd/ripgo`) is for testing, benchmarking, and rg comparison. The primary deliverable is the library packages (`pattern/`, `search/`, `walk/`, `ignore/`). External tools should be able to `import "github.com/nijaru/ripgo/search"` without pulling in any CLI dependencies.
