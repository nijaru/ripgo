# STATUS.md — ripgo

**Last updated:** 2026-03-19
**Focus:** Capability-based walk/search handoff design, performance architecture, and execution prep.

## Current State

| Metric | Value |
|---|---|
| Binary | `ripgo` — compiles, runs, exits cleanly |
| Performance | **~1.0x - 1.8x** of `ripgrep` speed (dataset dependent) |
| Tests | All passing (including Go 1.24/1.26 modernizations) |
| go vet | clean (0 warnings) |
| Milestone | Modernization (Go 1.24/1.26) ✅, Literal Pre-filter ✅, fsref Capability Handoff ✅ |
| Active task | `ripgo-ehtq` — Aho-Corasick pre-filtering |
| Open tasks | 1 (`ripgo-ehtq`) |

## Session Work (2026-03-19)

### 1. Modernization (Go 1.24/1.26)
- **`fsref` Capability Handoff (ripgo-jg8k) ✅:** Replaced internal path-based `walk.Entry -> search.Search` boundary with a capability-based file reference abstraction (`fsref.Ref`).
    - **Internal Abstraction:** Added `internal/fsref` with `Ref` interface, `pathRef` (compatibility), and `rootedRef` (optimized).
    - **Auto-Closing Roots:** Implemented `fsref.Root` wrapper using `runtime.AddCleanup` (Go 1.24) to ensure directory handles are closed exactly when no longer reachable, without manual reference counting.
    - **Walker Integration:** `walk.Walker` now uses a `rootProvider` interface to obtain `fsref.Root` capabilities from `OSFS` for each directory.
    - **Optimized Hot Path:** `search.Searcher.Search` now consumes `fsref.Ref` directly, using `root.Open(name)` (via `openat` under the hood) to avoid redundant full-path resolution.
- **`os.Root` Deep Integration:** Optimized `OSFS.OpenRoot` to leverage existing parent root handles via `root.OpenRoot(name)` where available.

### 2. SOTA Refinements (Performance)
- **Literal Prefix Extraction (ripgo-sdmc):** Implemented `Literal()` for `RegexMatcher` (via `re.LiteralPrefix()`) and `PCREMatcher` (via manual prefix extraction).
- **Literal Pre-filtering:** `Searcher` now skips entire files using `bytes.Contains` if a mandatory literal is identified.

### 3. Architecture & Security
- **Consolidated Documentation ✅:** Merged separate specs into `ai/PLAN.md`, updated `ai/DESIGN.md` with finalized architecture, and updated `ai/DECISIONS.md` with recent architectural choices.
- **Capability-Based FS:** `fsref` enables platform-optimal filesystem access (Linux `openat2`, Darwin fd-relative) behind a narrow, safe interface.

## Next Steps

See [ai/PLAN.md](./PLAN.md) for detailed implementation details.

### 1. Advanced Pre-filtering (`ripgo-ehtq`)
- Implement **Aho-Corasick** for multi-pattern searches.

### 2. Features
- **File Type Filtering:** Wire `--type` and `--type-not` through to the Walker.
- **Sorting Modes:** Add support for `--sort size` and `--sort modified`.

### 3. Validation
- Run benchmarks to quantify the win from the `fsref` capability handoff.
