# DECISIONS.md — ripgo

## 2026-03-17: Kong over Cobra for CLI
**Decision:** Use `github.com/alecthomas/kong` with struct-driven flag parsing.
**Rationale:** Kong's tag-based approach maps cleanly to a single `Options` struct. Cobra's subcommand model adds ceremony without benefit. Spec explicitly recommends Kong.

---

## 2026-03-17: Smart literal fast-path via `isLiteral()`
**Decision:** Before regex compilation, check if the pattern contains any regex metacharacters. If clean, use `bytes.Index` / `bytes.Contains` instead of `regexp`.
**Rationale:** Most search patterns (`func`, `TODO`, `import`) are literal strings. Skipping the regex engine eliminates compilation overhead and matches faster.

---

## 2026-03-17: Library-first package layout
**Decision:** Move `pattern/`, `search/`, `walk/`, `ignore/`, `printer/`, `stats/` to root level as public packages.
**Rationale:** ripgo is primarily a library. Public packages must not import `internal/`.

---

## 2026-03-18: Custom ignore engine with full gitignore semantics
**Decision:** Custom engine with `IgnoreRule` (pattern + metadata), `IgnoreSet` (per-directory rules with parent chain), and `Engine` (top-level orchestrator).
**Rationale:** Matches ripgrep's semantics exactly. Parent directory ignore rules block entire subtrees.

---

## 2026-03-18: Zero-alloc ShouldIgnore fast path
**Decision:** Cache the cwd-relative base path as `baseRelSlash` in `Engine` at construction.
**Rationale:** Common case (source files, non-hidden, in a loaded ignore set) now hits the prefix fast path — 0 allocations.

---

## 2026-03-19: PCRE support via regexp2
**Decision:** Add `github.com/dlclark/regexp2` behind `-P`/`--pcre2` flag.
**Rationale:** Opt-in dependency. Same `Matcher` interface means search/printer/walk code is unchanged. Users who don't use `-P` pay zero cost.

---

## 2026-03-19: Go 1.23 Iterator API (`iter.Seq2`)
**Decision:** Replace channel-based `Search` API with `iter.Seq2[search.Result, error]`.
**Rationale:** 
- **Safety:** Prevents goroutine leaks if the caller stops iterating early. 
- **Standardization:** Idiomatic for Go 1.23+. 
- **Error Handling:** Standardizes error reporting as the second yield value.

---

## 2026-03-19: Pure `fs.FS` Core
**Decision:** Eliminate all `os.*` vs `fs.FS` branching. Everything consumes `fs.FS`.
**Rationale:**
- **Virtualization:** Library can search any `fs.FS` implementation (embed, zip, in-memory) with zero changes.
- **Simplicity:** Removes messy conditional logic throughout the core packages.
- **OS Integration:** Handled via a single `internal/osfs` implementation.

---

## 2026-03-19: Decoupled `mmap` via `MappableFS`
**Decision:** Define a `MappableFS` interface in the `search` package.
**Rationale:** Allows searcher to benefit from zero-copy memory mapping on the local disk while remaining portable to environments (like WASM) that don't support `syscall.Mmap`.

---

## 2026-03-20: Modernize line scanning with `bytes.Lines`
**Decision:** Replace manual `bytes.IndexByte` loop in `search` with Go 1.24's `bytes.Lines` iterator.
**Rationale:**
- **Idiomatic:** Reduces boilerplate and leverages the new standard library iterator pattern.
- **Performance:** Compiler-optimized for range-over-function, offering better long-term throughput with zero-allocation line yielding.

---

## 2026-03-20: Literal Prefix Extraction for Pre-filtering
**Decision:** Implement `Matcher.Literal()` for all matcher types. Extract prefixes from `RegexMatcher` (via `LiteralPrefix()`) and `PCREMatcher` (via custom logic).
**Rationale:**
- **Optimization:** Allows `Searcher` to use `bytes.Contains` (often SIMD-accelerated) to skip non-matching files entirely before invoking the more expensive regex engines.
- **Consistency:** Provides a unified pre-filtering path for all matcher implementations.

---

## 2026-03-20: Platform-specific `os.Root` Enablement
**Decision:** Enable `os.Root` sandboxing only on Linux. Use standard `os` package fallback on Darwin and Windows.
**Rationale:**
- **Performance:** Benchmarks on Go 1.24 show a **6x performance degradation** on Darwin when using `os.Root` for recursive walks (due to emulated `openat` overhead).
- **Parity:** On Linux, `os.Root` uses native `openat2` and is highly efficient. Restricting it by platform ensures we maintain `ripgrep`-level performance on all systems while using the most secure API where it is fast.
- **Safety:** Used `runtime.AddCleanup` (Go 1.24) to ensure `os.Root` directory handles are properly closed if an `OSFS` instance is garbage collected.

---

## 2026-03-19: Capability-based walk/search handoff (`fsref`)
**Decision:** Replace path-based `walk.Entry -> search.Search` boundary with a capability-based `fsref.Ref` abstraction. Use `runtime.AddCleanup` (Go 1.24) for auto-closing roots.
**Rationale:**
- **Performance:** Eliminates redundant O(N) full-path lookups in the search worker pool. 
- **Platform Optimal:** `rootedRef` allows Linux/Darwin to use direct directory-relative file opening (`openat` under the hood) which is the most efficient way to access discovered files.
- **Auto-Cleanup:** `runtime.AddCleanup` ensures file descriptors are closed as soon as a `Ref` is garbage collected, removing manual reference counting or complex lifetime tracking.

---

## 2026-03-19: `fsref.Root` Wrapper for Cleanup
**Decision:** Use a wrapper struct for `os.Root` and attach the `runtime.AddCleanup` to the wrapper, passing the inner `*os.Root` as the cleanup argument.
**Rationale:** Passing an object as its own cleanup argument in `runtime.AddCleanup` creates a reference cycle that prevents GC. The wrapper breaks this cycle.
