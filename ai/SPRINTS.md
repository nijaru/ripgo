# Sprints

| Sprint | Goal | Status |
| :--- | :--- | :--- |
| [01-high-performance-core](sprints/01-high-performance-core.md) | Eliminate allocations in the hot path and optimize file scanning. | done |
| [02-library-refinements](sprints/02-library-refinements.md) | Ensure the package is fully decoupled and ready for external use. | done |
| [03-feature-parity](sprints/03-feature-parity.md) | Implement missing core features like file type filtering and smart-case for regex. | done |
| [04-advanced-optimizations](sprints/04-advanced-optimizations.md) | Implement `mmap` and fine-tune workers/output for extreme performance. | done |
| 05-pcre-and-ci | PCRE2 support, CI integration, go vet clean. | done |
| 06-cli-polish | Single-file fix, submatches in JSON, heading/sort flags, streaming JSON perf. | done |
| 07-architectural-rewrite | Migrate to Go 1.23 iterators, pure fs.FS core, functional options. | done |
| 08-sota-optimization | Optimize ignore engine (Trie/DFA), further reduce virtualization tax. | planned |
