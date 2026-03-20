# DESIGN.md — ripgo

## Goal

High-performance, Go-native search library and CLI. Focus on idiomatic Go (1.26+), virtual filesystem support (`io/fs`), and extreme performance.

## Architecture

```
┌─────────┐     ┌──────────┐     ┌──────────┐
│   CLI   │────▶│  Options  │────▶│  ripgo   │
│ (Kong)  │     │ (Func)    │     │ (Search) │
└─────────┘     └──────────┘     └──────────┘
                                        │
                                        ▼
                ┌──────────┐     ┌──────────┐
                │  Walker   │◀───│ Iterator │
                │ (fs.FS)   │    │ (Seq2)   │
                └──────────┘     └──────────┘
                       │                │
                       ▼                ▼
                ┌──────────┐     ┌──────────┐
                │  Ignore   │     │ Searcher │
                │ (Engine)  │     │ (mmap)   │
                └──────────┘     └──────────┘
```

## Key Components

### 1. High-Level API (`ripgo` package)
The primary entry point for library consumers.
- `Search(ctx, pattern, paths, ...Option) iter.Seq2[Result, error]`
- Uses Functional Options for configuration.
- Returns a standard Go 1.23+ iterator for safe, idiomatic consumption.

### 2. Capability-Based Filesystem (`internal/fsref`)
The bridge between discovery and scanning.
- **`fsref.Ref`**: Interface representing a capability-based file reference.
- **`rootedRef`**: Uses `os.Root` for capability-based access, avoiding redundant full-path resolution.
- **`pathRef`**: Compatibility backend for standard `fs.FS` and non-OS filesystems.
- **`fsref.Root`**: Thread-safe, auto-closing wrapper for `os.Root` using `runtime.AddCleanup`.

### 3. Virtual Filesystem Core (`io/fs`)
- `internal/osfs`: OS-native implementation with `os.Root` support.
- `MappableFS`: Optional interface for zero-copy memory mapping.
- All components are 100% virtualizable via `fs.FS`.

### 4. Concurrency & Iteration
- **Walker:** Parallel directory traversal emitting `walk.Entry` objects containing `fsref.Ref`.
- **Searcher:** Worker pool consuming `fsref.Ref` directly.
- **Iterator:** Orchestrates workers and yields results. Handles `context.Context` cancellation and automatic cleanup.

### 5. Matching Engine (`pattern` package)
- `LiteralMatcher`: Fast `bytes.Index` for fixed strings.
- `RegexMatcher`: Stdlib `regexp` (RE2).
- `PCREMatcher`: `regexp2` for PCRE-compatible features (lookbehind/ahead).

## Performance Strategy

1. **Capability-based handoff:** Discovery (`Walker`) opens directories once as `os.Root`; Searchers use those capabilities to open files, skipping O(N) path resolution in the hot path.
2. **Zero-copy mmap:** Automatic memory mapping for files > 128KB where supported.
3. **Allocation-free hot path:** Reuse byte slices, use `bytes.Lines` iterator, and avoid `string` conversions.
4. **SIMD Pre-filtering:** Use `bytes.Contains` for single literals and planned **Aho-Corasick** for multi-literal pre-filtering.
