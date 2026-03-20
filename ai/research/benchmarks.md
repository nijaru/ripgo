## Benchmarks - 2026-03-20 02:40:16 UTC
Environment: go version go1.26.1 darwin/arm64
| Command | Mean [s] | Min [s] | Max [s] | Relative |
|:---|---:|---:|---:|---:|
| `./ripgo -q 'func' data/kubernetes-1.31.0` | 4.916 ± 0.023 | 4.866 | 4.945 | 924.22 ± 91.48 |
| `rg -q 'func' data/kubernetes-1.31.0` | 0.005 ± 0.001 | 0.005 | 0.007 | 1.00 |

## Benchmarks - 2026-03-20 02:40:29 UTC
Environment: go version go1.26.1 darwin/arm64
Dataset: chi v5.2.1
| Command | Mean [ms] | Min [ms] | Max [ms] | Relative |
|:---|---:|---:|---:|---:|
| `./ripgo -q 'func' data/chi-5.2.1` | 13.9 ± 0.4 | 12.6 | 15.3 | 2.47 ± 0.34 |
| `rg -q 'func' data/chi-5.2.1` | 5.6 ± 0.7 | 4.5 | 15.4 | 1.00 |

## Benchmarks - 2026-03-20 02:42:49 UTC
Environment: go version go1.26.1 darwin/arm64
Dataset: chi v5.2.1
| Command | Mean [ms] | Min [ms] | Max [ms] | Relative |
|:---|---:|---:|---:|---:|
| `./ripgo -q 'func' data/chi-5.2.1` | 5.4 ± 0.3 | 4.6 | 8.1 | 1.00 ± 0.13 |
| `rg -q 'func' data/chi-5.2.1` | 5.4 ± 0.7 | 4.5 | 12.5 | 1.00 |

