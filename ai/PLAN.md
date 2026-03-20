# ripgo Implementation Plan

This document outlines the high-priority tasks and architectural refinements.

## 1. Advanced Pre-filtering (`ripgo-ehtq`)

### Goal
Implement **Aho-Corasick** for multi-pattern pre-filtering and complex regex optimizations.

### Proposed Changes
- **`internal/aho`**: Implement high-performance Aho-Corasick automaton. Optimize `MatchesAny` for short-circuiting.
- **`pattern.Matcher`**: Add `Literals() [][]byte` method.
    - `RegexMatcher`: Extract candidate literals from alternations (e.g., `(foo|bar)`).
- **`search.Searcher`**: If `len(literals) > 1`, use `aho.Machine` for skip detection.

## 2. Feature Completeness

### File Type Filtering
- **Status**: Logic exists in `ignore.Engine`, but needs verification.
- **Tasks**:
    - Wire `--type` and `--type-not` through to `walk.Walker`.
    - Verify `ripgo -t go "pattern"` correctly filters.
    - Polish `--type-list` output in `main.go`.

### Sorting Modes
- **Status**: CLI supports `path` sorting.
- **Tasks**:
    - Extend `sortResults` in `main.go` for `modified`, `accessed`, and `created`.
    - Evaluate directory-level sorting in `Walker` for streaming large results.

## 3. Performance Validation

### Benchmarks
- **Task**: Create `search/ref_bench_test.go`.
- **Measurement**: Compare `pathRef` (reopen-by-path) vs `rootedRef` (capability-based) on Linux and Darwin.
- **Reporting**: quantify the win from eliminating redundant path resolution.

## 4. Housekeeping
- **Cleanup**: Eventually remove `walk.Entry.Path` and `walk.Entry.Info` once all internal callers fully rely on `fsref.Ref`.
