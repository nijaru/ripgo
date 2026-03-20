# ROADMAP.md — ripgo

This document outlines the planned performance optimizations and feature enhancements for `ripgo`.

## ✅ Completed (Session 2026-03-19)

| Feature | Impact |
|---|---|
| **Pure `fs.FS` Core** | Decoupled from `os`; supports virtual filesystems. |
| **Go 1.23 Iterators** | Safe, idiomatic `iter.Seq2` search pipeline. |
| **Literal Pre-filtering** | **2.7x speedup** for regex searches via `bytes.Contains`. |
| **Ignore Trie** | $O(\text{depth})$ ignore set retrieval and pruning. |
| **mmap Support** | Zero-copy I/O for files > 128KB. |
| **CLI Color/TTY** | Parity with `ripgrep` UX (colors, headings). |

## 🚀 SOTA Refinements (Go 1.24/1.26+)

| Feature | Description | Priority |
|---|---|---|
| **`bytes.Lines` (ripgo-in46)** | Modernize line scanning with the new standard iterator (Go 1.24). | High |
| **`os.Root` (ripgo-hw7l)** | Security-first file access via `os.OpenRoot` sandboxing (Go 1.24). | High |
| **Regex Literals (ripgo-sdmc)** | Extract fixed prefixes from regex/PCRE patterns for pre-filtering. | High |
| **`fsref` capability handoff (ripgo-jg8k)** | Remove reopen-by-path between walk and search; use backend-specific file references. | High |
| **Aho-Corasick (ripgo-ehtq)** | Implement for multi-literal skipping and complex pre-filters. | Medium |
| **SIMD Hot Paths** | Evaluate `simd/archsimd` (Go 1.26) for literal searching. | Low |

## ✨ Feature Enhancements

| Feature | Description | Priority |
|---|---|---|
| **File Type Filtering** | Full `--type` and `--type-list` support. | High |
| **Regex Smart-Case** | Case-insensitive unless uppercase present (std/PCRE). | Medium |
| **Sorting Modes** | Support `--sort size`, `--sort modified` in Walker. | Medium |
| **PCRE2 Parity** | Exhaustive lookahead/behind validation. | Low |

## 📚 Library & Architecture

- **Worker Tuning:** Dynamic adjustment based on I/O vs CPU.
- **Weak References:** Evaluate `weak.Pointer` (Go 1.24) for caching ignore sets.
- **JSON v2:** Migrate to `encoding/json/v2` once stabilized.
