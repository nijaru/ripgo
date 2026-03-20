# Sprint 03: Feature Parity

**Goal:** Implement missing core features like file type filtering and smart-case for regex.

### Task 1: Smart-case for `RegexMatcher`
**Depends on:** none
**Criteria:** `RegexMatcher` checks if the pattern contains uppercase characters; if not, and smart-case is enabled, it applies the `(?i)` flag.
**Technical Notes:** Reuse the `hasUppercase` utility from `pattern`.

### Task 2: File Type Filtering (`--type`)
**Depends on:** none
**Criteria:** `ignore.Engine` supports explicit file types (e.g., `go`, `rust`) mapped to glob patterns. CLI supports `--type`, `--type-not`, and `--type-list`.
**Technical Notes:** Create a built-in map of common file types to globs.

### Task 3: Exhaustive Glob Audit
**Depends on:** none
**Criteria:** Compare `ignore` engine behavior against ripgrep's test cases for trailing slashes, anchor points, and nested negations. Ensure full compatibility.
**Technical Notes:** Add robust table-driven tests in `ignore_test.go`.
