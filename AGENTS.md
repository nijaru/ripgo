# AGENTS.md — ripgo

Ripgrep-like search tool in Go. Idiomatic, standard-library-first, high performance.

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
| Test | `go test ./internal/...` |
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
cmd/ripgo/main.go       — pipeline orchestration
internal/cli             — Kong flag parsing
internal/config          — validated runtime Config
internal/pattern         — regex/fixed-string Matcher
internal/ignore          — .gitignore/.ignore engine + glob
internal/walk            — parallel directory traversal
internal/filetype        — binary/hidden detection
internal/search          — line-mode scanning
internal/printer         — text/JSON/count/files output
internal/stats           — match counts and exit codes
```

## Verification

After any change:
1. `go build ./cmd/ripgo` — must compile
2. `go test ./internal/...` — all tests pass
3. `go vet ./...` — clean
4. `timeout 5 ripgo "pattern" .` — search works, exits cleanly
