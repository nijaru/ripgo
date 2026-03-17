# Design spec: ripgrep-like search tool in Go

## Goal

Build a Go CLI that is as close to ripgrep as practical while remaining idiomatic Go: simple package boundaries, explicit data flow, predictable concurrency, standard-library-first internals, and optional feature flags only where they materially improve parity. The target should preserve ripgrep’s core user expectations: recursive search, gitignore-aware traversal, hidden/binary skipping by default, line-oriented output, and structured output modes such as JSON. ([GitHub][1])

## Non-goals

Do not try to literally port ripgrep’s internals. ripgrep’s Rust ecosystem already has a dedicated library stack for search and traversal; Go does not have a direct equivalent, so a faithful Go implementation should mirror behavior and UX, not crate structure. Also do not make PCRE-style regex the default, because Go’s standard `regexp` is linear-time and better aligned with ripgrep’s default “fast and safe first” approach. ([Docs.rs][2])

---

## Product definition

### Primary UX contract

The tool should feel like `rg` in the common path:

* search recursively from cwd or provided paths
* respect `.gitignore`-style excludes by default
* skip hidden files/dirs by default
* skip binary files by default
* print line-oriented matches with file path and line number
* support fixed-string mode, regex mode, counts, context lines, JSON, and replacement-preview-style output later in the roadmap

Those defaults are central to ripgrep’s documented behavior, so they should define parity priorities. ([GitHub][1])

### Idiomatic Go interpretation

“Idiomatic for Go” here means:

* one binary, one main command
* structs for immutable config passed downward
* goroutine pipelines for traversal and scanning
* clear ownership of cancellation via `context.Context`
* standard library wherever it does not materially harm parity
* optional abstractions only at subsystem seams, not everywhere

This should not be an over-engineered framework. It should be a small set of cohesive packages.

---

## Architecture

### Top-level pipeline

The execution model should be:

1. parse CLI into `Config`
2. compile matchers and filters
3. build traversal plan
4. walk filesystem and emit candidate files
5. scan files in parallel
6. aggregate and print results
7. stop early on cancellation / `-l` / match limit

That maps closely to the separation ripgrep itself makes between “gather what to search” and “search it.” BurntSushi explicitly notes that the searching crates do not solve gathering; traversal and filtering are separate concerns. ([GitHub][3])

### Package layout

Use this package structure:

```text
cmd/gogrep/
internal/cli
internal/config
internal/pattern
internal/globset
internal/ignore
internal/walk
internal/filetype
internal/search
internal/printer
internal/stats
internal/bench
```

Responsibilities:

* `cli`: Kong wiring, help text, flag normalization
* `config`: validated runtime config
* `pattern`: regex/fixed-string compilation
* `globset`: include/exclude glob compilation
* `ignore`: ignore rule loading and precedence
* `walk`: filesystem traversal and candidate emission
* `filetype`: binary/hidden/symlink detection, file type filters
* `search`: line scanner, multiline scanner, context windows
* `printer`: text and JSON emitters
* `stats`: counts and exit code decisions
* `bench`: repeatable perf harness against ripgrep

This package split is intentionally narrower than a typical “enterprise” Go app and keeps behavior-centered seams.

---

## CLI framework choice

Use **Kong**, not Cobra, for v1.

Reasoning:

* the tool is a single command with many flags, which matches Kong’s struct-driven style better than Cobra’s subcommand-oriented design
* you want a strongly typed `Config` model early
* you do not need a large subcommand tree initially

Only switch to Cobra if you later add substantial subcommands like `types`, `config`, `debug`, `bench`, or `index`. This keeps the implementation closer to `rg`’s actual UX shape rather than overbuilding the CLI layer.

---

## Matching engine

### Default engine

Use Go’s standard `regexp` as the default regex backend. It is RE2-based and guaranteed linear in input size, which is the right default for a ripgrep-like tool. ([Go Packages][4])

Design:

```go
type Matcher interface {
    Match(line []byte) (locs []int, ok bool)
    MatchFile(data []byte) bool // for multiline mode
}
```

Implementations:

* `RegexMatcher` using `regexp.Regexp`
* `LiteralMatcher` using `bytes.Index` / `bytes.Contains`
* `SmartMatcher` which selects literal fast path when pattern has no metacharacters

### Optional PCRE-like mode

Add an opt-in secondary engine later using `regexp2`. Keep it off by default because it does not have the same time guarantees as stdlib `regexp`. ([Go Packages][5])

Flags:

* `--engine auto|re2|pcre`
* default `auto`, which picks literal or `re2`
* reject unsupported combinations explicitly

### Why this differs from ripgrep

ripgrep offers a mature search stack with optional PCRE2 support in modern builds; a Go clone should emulate the default user experience first, not prioritize rare regex features over throughput and safety. ([GitHub][6])

---

## Ignore and glob semantics

### Core requirement

This is the subsystem where a naive Go clone will diverge most from ripgrep if you are not careful. ripgrep’s ignore handling is higher-level than simple pattern matching and includes `.gitignore`-style files and custom globs. ([GitHub][3])

### Recommended design

Do **not** rely on `go-gitignore` as the sole ignore engine if “as close to ripgrep as possible” is the goal. It is useful as a bootstrap, but its README explicitly says it uses `filepath.Match` and does not support recursive `**` patterns. That is too large a semantic gap. ([GitHub][7])

Instead:

* use `gobwas/glob` for user-provided include/exclude globs because it is optimized for compile-once/match-many usage ([GitHub][8])
* implement a custom ignore engine for gitignore semantics, or fork/replace the parser layer if needed
* represent ignore rules as a stack of directory-scoped matchers

Suggested data model:

```go
type IgnoreRule struct {
    Pattern      string
    Negated      bool
    DirectoryOnly bool
    Anchored     bool
    Source       string
}

type IgnoreSet struct {
    Parent *IgnoreSet
    Rules  []CompiledRule
}
```

### Precedence model

Implement these semantics explicitly:

1. CLI include/exclude globs
2. per-directory ignore files
3. hidden-file policy
4. binary-file policy
5. symlink policy
6. file type filters

Make precedence a first-class tested component rather than a side effect of walk order.

### Hidden files

Hidden files/dirs should be skipped by default, with explicit opt-in to include them, matching ripgrep defaults. ([GitHub][1])

### Ignore roadmap

Phase 1:

* `.gitignore`
* `.ignore`
* CLI `--glob`
* hidden/binary skip

Phase 2:

* `.rgignore`
* parent directory inheritance rules
* negation edge cases
* type definitions compatible with common `rg` usage

---

## Filesystem traversal

### Walker strategy

Build a custom parallel walker rather than using plain `filepath.WalkDir` directly as the full solution.

Why:

* you need early pruning on ignored directories
* you need bounded parallelism
* you need deterministic enough behavior for tests
* you need cancellation propagation
* you may need separate policies for symlinks, hidden entries, and ignore file loading

Suggested approach:

* one coordinator goroutine manages directory queue
* N workers stat/read directory entries
* each directory worker:

  * loads applicable ignore files
  * decides whether to descend
  * emits file candidates to scanner queue

Types:

```go
type Candidate struct {
    Path      string
    Depth     int
    IgnoreCtx *IgnoreSet
    FileType  TypeInfo
}
```

### Symlinks

Default: do not follow symlinks unless explicitly requested. This avoids loops and stays closer to conservative CLI-tool behavior.

### Path normalization

Normalize internal matching paths to slash-separated logical paths for ignore/glob checks, but preserve OS-native paths for opening files and printing on Windows where appropriate. This keeps behavior consistent across platforms.

---

## Binary and encoding handling

### Binary detection

Implement a fast binary heuristic before full scan:

* read first 8–32 KB
* if NUL byte present, classify as binary
* optionally detect high non-text ratio as a secondary heuristic

Skip binary files by default, matching ripgrep defaults. ([GitHub][1])

### Encoding

v1 should support UTF-8 and pass raw bytes otherwise, treating search primarily as byte-oriented line scanning. Full transcoding support is a later feature. The output layer should avoid corrupting paths or matched lines when bytes are not valid UTF-8.

---

## Search subsystem

### Scanning modes

Implement two scanner paths.

**Line mode** (default):

* buffered incremental reads
* find newline boundaries
* apply matcher line by line
* maintain line numbers and optional column offsets
* support before/after context windows

**Multiline mode**:

* full-file read subject to size threshold
* apply whole-file matcher
* map match offsets back to line/column spans

ripgrep is fundamentally line-oriented, so line mode should be optimized first. ([GitHub][1])

### Fixed-string fast path

This is essential for performance parity in common cases.

If pattern contains no regex metacharacters, bypass `regexp` entirely and use byte search. Precompute case-folded variants when case-insensitive. This is likely the single highest-value optimization after pruning traversal.

### Context lines

Design search results as spans, not one-off lines:

```go
type Match struct {
    Path       string
    Line       int
    Column     int
    LineBytes  []byte
    Submatches [][2]int
}

type Result struct {
    Path    string
    Matches []Match
    Before  [][]byte
    After   [][]byte
}
```

This makes it easier to support:

* normal output
* `-n`, `-c`, `-l`
* JSON
* replacement preview
* context grouping

---

## Output subsystem

### Printers

Create separate printers behind an interface:

```go
type Printer interface {
    BeginFile(path string) error
    PrintMatch(Result) error
    EndFile(path string) error
    Finish(Stats) error
}
```

Implement:

* `TextPrinter`
* `JSONPrinter`
* `CountPrinter`
* `FilesOnlyPrinter`

Keep printers single-threaded behind an aggregation channel so output order is stable enough and stdout locking is trivial.

### Ordering

Default to near-discovery order, not strict lexical order, for performance. Add an optional stable sort mode later if needed.

### Color

Keep color support minimal and isolated to `printer`. No color logic should leak into the search layer.

---

## Concurrency model

### Guiding principle

Use concurrency to overlap traversal and file scanning, but avoid unbounded goroutines. Idiomatic Go here means a small number of explicit worker pools, not “goroutine per file.”

Suggested pools:

* 1 directory coordinator
* `walkWorkers = min(4, GOMAXPROCS)`
* `scanWorkers = GOMAXPROCS`
* 1 output aggregator

Channels:

```go
dirs       chan DirTask
candidates chan Candidate
results    chan Result
errors     chan error
```

Cancellation:

* all goroutines take `context.Context`
* first fatal error or user interrupt cancels the pipeline
* `-l` / first-match modes can short-circuit per file or globally

### Memory controls

Avoid whole-file buffering except in multiline mode or small-file optimization. Reuse buffers with `sync.Pool` in the search layer only after profiling proves benefit.

---

## Configuration model

Define a single immutable `Config` built from CLI parsing and validation.

```go
type Config struct {
    Paths              []string
    Pattern            string
    FixedStrings       bool
    IgnoreCase         bool
    SmartCase          bool
    Multiline          bool
    Hidden             bool
    SearchBinary       bool
    FollowSymlinks     bool
    ContextBefore      int
    ContextAfter       int
    MaxFileSize        int64
    GlobIncludes       []string
    GlobExcludes       []string
    Engine             EngineKind
    OutputMode         OutputMode
    Threads            int
}
```

No package should read flags directly except `cli`.

---

## Performance strategy

### Expected performance

A Go clone will likely remain slower than ripgrep overall unless you invest heavily in traversal, ignore semantics, buffering, and specialized search paths. The performance gap is not mainly about regex syntax; it is mostly about the maturity of the integrated traversal + filtering + search stack. ripgrep’s own library split reflects that separation. ([Docs.rs][2])

### High-value optimizations

Implement in this order:

1. directory pruning from ignore rules
2. fixed-string fast path
3. bounded parallel scanning
4. early binary rejection
5. avoid allocations in hot path
6. small-file mmap experiment only if profiling justifies it
7. selective prefiltering for regex patterns if practical

### Benchmark harness

Treat ripgrep as the external reference.

Benchmark matrix:

* literal vs regex
* small repo vs large monorepo
* hot cache vs cold cache
* line mode vs multiline
* hidden on/off
* JSON vs text output

Metrics:

* wall time
* CPU time
* bytes read
* files scanned
* directories pruned
* allocations/op
* p50/p95 latency for single invocation

This should live in `internal/bench` plus reproducible repo fixtures.

---

## Testing strategy

### Golden behavior tests

Create fixture trees with:

* nested `.gitignore`
* hidden files
* binary files
* symlinks
* negated ignore rules
* include/exclude globs
* mixed path separators

For each fixture:

* compare expected file set
* compare output lines
* compare JSON schema
* compare exit codes

### Differential tests against ripgrep

For supported flags, run both tools on the same fixture trees and compare:

* matched file paths
* line numbers
* counts
* JSON event structure where intentionally aligned

Mark known divergences explicitly. This is the fastest way to stay close to ripgrep without pretending you have identical internals.

### Property tests

Use fuzzing for:

* ignore parser
* path normalization
* regex/literal mode selection
* line/column mapping in multiline mode

---

## Roadmap

### Milestone 1: usable core

* Kong CLI
* recursive traversal
* stdlib regex + literal mode
* hidden/binary skip
* basic `.gitignore`
* text output with line numbers
* worker pools and cancellation

### Milestone 2: parity-oriented core

* custom ignore engine with recursive pattern support
* `--glob`
* context lines
* files-only/count modes
* JSON output
* smart case
* better Windows path handling

### Milestone 3: advanced features

* multiline
* replacement preview
* optional PCRE-like engine
* type filters
* config files
* richer diagnostics / `--debug`

### Milestone 4: polish and validation

* differential benchmark suite versus `rg`
* perf tuning from pprof
* edge-case compatibility cleanup
* shell completions and packaging

---

## Recommended initial implementation choices

Use this concrete stack:

* CLI: **Kong**
* regex default: stdlib `regexp` ([Go Packages][4])
* optional advanced regex: `regexp2` behind explicit flag only ([Go Packages][5])
* glob filters: `gobwas/glob` ([GitHub][8])
* ignore handling: custom engine, with `go-gitignore` used only as a temporary bootstrap or reference, not as the final parity path because of the `**` limitation ([GitHub][7])
* traversal/search/output: your own packages

That is the closest path to “ripgrep-like, but idiomatic Go.”

## Critical design decision

The single most important decision is this:

**Build a real ignore/traversal subsystem, not just a regex scanner plus `WalkDir`.**

That is where most ripgrep-like behavior lives. BurntSushi’s notes on the Rust side make the same separation clear: the search crates search; the higher-level ignore/traversal layer decides what gets searched. ([GitHub][3])

If you want, I can turn this into a more concrete engineering doc with:

* flag-by-flag compatibility table
* exact package APIs
* milestone tickets
* pseudocode for walker/searcher/printer orchestration.

[1]: https://github.com/BurntSushi/ripgrep?utm_source=chatgpt.com "ripgrep recursively searches directories for a regex pattern ..."
[2]: https://docs.rs/grep?utm_source=chatgpt.com "grep - Rust"
[3]: https://github.com/BurntSushi/ripgrep/discussions/2509?utm_source=chatgpt.com "Using grep crate (as library) #2509"
[4]: https://pkg.go.dev/regexp?utm_source=chatgpt.com "regexp package"
[5]: https://pkg.go.dev/github.com/dlclark/regexp2?utm_source=chatgpt.com "regexp2 package - github.com/dlclark/regexp2"
[6]: https://github.com/burntsushi/ripgrep/releases?utm_source=chatgpt.com "Releases · BurntSushi/ripgrep"
[7]: https://github.com/monochromegane/go-gitignore?utm_source=chatgpt.com "monochromegane/go-gitignore"
[8]: https://github.com/gobwas/glob/blob/master/readme.md?utm_source=chatgpt.com "glob/readme.md at master · gobwas/glob"
