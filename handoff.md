# Handoff — ripgo Capability Handoff Complete

## Current Status
The **Capability-based `fsref` handoff** (`ripgo-jg8k`) is fully implemented and verified. The search hot path no longer re-resolves files by full path after discovery, using `os.Root` relative access instead.

### ✅ Completed in this Session
- **`internal/fsref` Abstraction:**
    - Introduced `Ref` interface for capability-based file access.
    - Implemented `pathRef` (standard `os`/`fs` fallback) and `rootedRef` (optimized `os.Root` access).
    - **`fsref.Root` Wrapper:** Uses `runtime.AddCleanup` (Go 1.24/1.26) to automatically close directory handles when no longer reachable, avoiding manual reference counting and cycle leaks.
- **Walker/Searcher Integration:**
    - `walk.Walker` now obtains `fsref.Root` capabilities from `OSFS` and emits `walk.Entry` with a `File fsref.Ref`.
    - `search.Searcher.Search` now consumes `fsref.Ref`, using `root.Open(name)` (optimized `openat`) to avoid redundant path resolution.
    - `OSFS.OpenRoot` optimized to use `f.root.OpenRoot(dir)` to maintain a chain of rooted capabilities.
- **Modernization (Go 1.26.1):**
    - Leveraged `os.Root`, `runtime.AddCleanup`, and `iter.Seq2`.
    - Fixed a `runtime.AddCleanup` panic by introducing a wrapper to avoid self-referencing cycles.

## Context & Decisions
- **Decision:** Use a `Root` wrapper in `internal/fsref` for `runtime.AddCleanup`.
- **Rationale:** Passing `*os.Root` as its own cleanup argument caused an immediate panic ("ptr is equal to arg"). The wrapper breaks this cycle.
- **Decision:** Keep `walk.Entry.Path` and `walk.Entry.Info` for now.
- **Rationale:** Minimize breaking changes in the first pass; `ripgo.Search` already switched to `entry.File`.

## Next Steps (Actionable)
The following specs are ready for implementation by the next agent:

1. **Aho-Corasick Pre-filtering (`ripgo-ehtq`):**
    - **Spec:** `ai/design/aho-corasick-spec.md`
    - **Goal:** Implement `internal/aho` for multi-literal skipping (e.g., `(foo|bar|baz)` or multiple `-e` patterns).
2. **File Type Filtering & Sorting:**
    - **Spec:** `ai/design/next-steps-spec.md`
    - **Goal:** Wire up `--type` (logic exists in `ignore.Engine`) and implement `--sort` (modified, accessed, created) in `cmd/ripgo/main.go`.
3. **Benchmarks:**
    - **Goal:** Run direct A/B benchmarks of `pathRef` vs `rootedRef` to quantify the win.

## Environment
- **Go Version:** 1.26.1
- **All tests passing:** `go test ./...`
- **Clean build:** `go build ./cmd/ripgo`
