# Handoff — ripgo Architectural Rewrite

## Status
The library has been refactored from a CLI-first tool to a **SOTA Go-native library**. 

### Completed Architectural Shifts
1.  **Pure `fs.FS` Core:** All `os.*` branching removed. Everything (walk, search, ignore) uses `fs.FS`.
2.  **`OSFS` Implementation:** New `internal/osfs` package handles absolute paths and `syscall.Mmap` for the local disk.
3.  **Go 1.23 `iter.Seq2` API:** `ripgo.Search` uses modern iterators, preventing goroutine leaks and simplifying error handling.
4.  **Functional Options:** Improved DX with `ripgo.WithThreads`, `ripgo.WithHidden`, etc.
5.  **Decoupled `mmap`:** `MappableFS` interface allows zero-copy search on custom filesystems.

## Tasks for Flash

### 1. Optimize Ignore Engine (ripgo-koxn)
Current `.gitignore` matching iterates through a list of globs per directory.
- **Goal:** Refactor `ignore.Engine` to use a Trie or a state-machine based approach.
- **Why:** Massive speedup in monorepos with hundreds of ignore rules.

### 2. Add More Sort Modes
- **Goal:** Implement `--sort modified`, `--sort size`, `--sort created`.
- **Note:** This requires gathering `fs.FileInfo` during the search or traversal phase and passing it through `search.Result`.

### 3. CLI Polish
- **Color:** Use `fatih/color` or similar to highlight matches in the CLI.
- **TTY Detection:** Default `--heading` to true ONLY if stdout is a TTY.

## Library API Example (Post-Rewrite)
```go
// Search the embedded assets FS for "TODO"
for res, err := range ripgo.Search(ctx, "TODO", []string{"."}, ripgo.WithFS(myEmbedFS)) {
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        continue
    }
    fmt.Printf("Match in %s at line %d\n", res.Path, res.Matches[0].Line)
}
```

## Maintenance
- Ensure `internal/osfs` remains the only package importing `os` or `syscall`.
- Keep the `ripgo` root package clean as the only high-level orchestrator.
