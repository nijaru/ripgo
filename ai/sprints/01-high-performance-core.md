# Sprint 01: High-Performance Core

**Goal:** Eliminate allocations in the hot path and optimize file scanning. Every improvement should be measurable by `go test -bench`.

### Task 1: Zero-Allocation Case-Insensitive Search
**Depends on:** none
**Criteria:** `pattern.LiteralMatcher` no longer uses `bytes.ToLower` per line. Benchmarks show 0 allocs for case-insensitive literal match.
**Technical Notes:** Consider custom bit-masking loop or specialized case-folding byte search.

### Task 2: Unified Binary Detection & I/O
**Depends on:** none
**Criteria:** `IsBinary` logic moves to `search.Searcher`. `walk.Walker` no longer opens files just to read 8KB. 
**Technical Notes:** Use the first read buffer in `search.Searcher` to detect NUL bytes before proceeding with regex/literal matching.

### Task 3: Reduce Allocations in `ignore.matchGlobStar`
**Depends on:** none
**Criteria:** `matchGlobStarPartsOpt` uses index-based segments instead of `strings.Split`/`strings.Join`.
**Technical Notes:** Pass a pre-split `[]string` of the path or slice indices.

### Task 4: `ShouldIgnore` Fast-Path
**Depends on:** none
**Criteria:** Eliminate redundant `filepath.Clean` and `filepath.ToSlash` inside the `ShouldIgnore` traversal loop.
**Technical Notes:** Ensure `Walker` maintains normalized paths to pass to `ignore.Engine`.
