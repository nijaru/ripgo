# Sprint 02: Library-First Refinements

**Goal:** Ensure the package is fully decoupled and ready for external use.

### Task 1: Decouple Printers from `stdout`
**Depends on:** none
**Criteria:** `printer.NewTextPrinter`, `NewJSONPrinter`, etc., accept an `io.Writer`. All tests pass. `cmd/ripgo` passes `os.Stdout`.
**Technical Notes:** Update interfaces and printer structs.

### Task 2: Public API Audit & Documentation
**Depends on:** none
**Criteria:** All public structs (`Config`, `Result`, `Match`) have GoDoc comments. Unnecessary fields are unexported. `golint` and `godoc` are clean.
**Technical Notes:** Focus on `search`, `ignore`, and `walk` packages.

### Task 3: Support `io/fs.FS` in `Walker`/`Searcher`
**Depends on:** none
**Criteria:** `Walker` and `Searcher` can traverse and read from an `io/fs.FS` interface.
**Technical Notes:** Will require changing how `os.ReadDir` and `os.ReadFile` are called. This may have performance implications, so benchmark carefully.
