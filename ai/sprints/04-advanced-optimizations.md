# Sprint 04: Advanced Optimizations

**Goal:** Implement `mmap` and fine-tune workers/output for extreme performance.

### Task 1: Implement `mmap` for Large Files
**Depends on:** Sprint 01
**Criteria:** Files larger than 1MB are memory-mapped instead of read entirely into RAM via `os.ReadFile` for context searches.
**Technical Notes:** Use `golang.org/x/exp/mmap` or standard `syscall` package.

### Task 2: JSON Output Buffering
**Depends on:** Sprint 02
**Criteria:** `JSONPrinter` uses a buffered writer for high-throughput serialization.
**Technical Notes:** Profile memory allocations during large JSON outputs.

### Task 3: Worker Tuning
**Depends on:** none
**Criteria:** Dynamically scale directory walk workers and search workers based on I/O wait times and available CPU.
**Technical Notes:** May require custom scheduler or tunable config flags beyond just `GOMAXPROCS`.
